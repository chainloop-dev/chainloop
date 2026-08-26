//
// Copyright 2026 The Chainloop Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package redaction

import (
	"context"
	"strings"
	"sync"
	"testing"

	betterleaksconfig "github.com/betterleaks/betterleaks/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Synthetic credentials shaped to match the default ruleset. Note that
// AKIAIOSFODNN7EXAMPLE would NOT match: the aws-access-token rule allowlists
// anything ending in "EXAMPLE". The character class after the AKIA prefix is
// base32, so no 0, 1, 8 or 9.
//
// The AWS pair is assembled from fragments so the literal never appears in a
// source file. GitHub's push protection recognises the same AWS patterns
// betterleaks does, and would reject a push containing a realistic-looking key
// even in test data.
const (
	fakeAWSKey    = "AKIA" + "4G7TI63VCBIRS4GW"
	fakeAWSSecret = "kQ7zXn2VbW9pLm4RtY6" + "uHs3JdF8gA1cE5oPzQwXn"
	fakeGitHubPAT = "ghp_erOZlZv0B1e3amrQugdwZ8Ro2W4kDql9WPTf"
)

var fakeAnthropicKey = "sk-ant-api03-" + strings.Repeat("a", 93) + "AA"

// awsPair is the shape an AWS leak has to take to be detected at all: the
// aws-access-token rule is composite and requires a secret access key nearby.
// The `\n` is the escaped two-character sequence, which is how a newline appears
// inside a JSON string leaf - the real shape of a transcript.
const awsPair = `AWS_ACCESS_KEY_ID=` + fakeAWSKey + `\nAWS_SECRET_ACCESS_KEY=` + fakeAWSSecret

func TestDefaultScannerDetects(t *testing.T) {
	scanner, err := DefaultScanner()
	require.NoError(t, err)

	testCases := []struct {
		name     string
		text     string
		wantRule string
	}{
		{
			name:     "aws access token paired with its secret access key",
			text:     `"content": "run export ` + awsPair + ` first"`,
			wantRule: "aws-access-token",
		},
		{
			name:     "anthropic api key",
			text:     `"content": "ANTHROPIC_API_KEY=` + fakeAnthropicKey + `"`,
			wantRule: "anthropic-api-key",
		},
		{
			name:     "github personal access token",
			text:     `"repository": "https://oauth2:` + fakeGitHubPAT + `@github.com/example/repo.git"`,
			wantRule: "github-pat",
		},
		{
			// A session transcript is attacker-influenceable text, so neither
			// in-band bypass marker may suppress a finding.
			name:     "gitleaks:allow must not suppress the finding",
			text:     `"content": "` + awsPair + ` // gitleaks:allow"`,
			wantRule: "aws-access-token",
		},
		{
			name:     "betterleaks:allow must not suppress the finding",
			text:     `"content": "` + awsPair + ` // betterleaks:allow"`,
			wantRule: "aws-access-token",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			findings, err := scanner.Scan(context.Background(), tc.text)
			require.NoError(t, err)
			require.NotEmpty(t, findings, "expected the default ruleset to match")

			var rules []string
			for _, f := range findings {
				rules = append(rules, f.RuleID)
				assert.NotEmpty(t, f.Secret, "a finding without a secret cannot be located")
				// The whole redaction strategy depends on this: a secret that is
				// not a substring of the scanned text cannot be replaced.
				assert.Contains(t, tc.text, f.Secret, "the secret must be a substring of the scanned text")
			}
			assert.Contains(t, rules, tc.wantRule)
		})
	}
}

func TestDefaultScannerIgnoresCleanText(t *testing.T) {
	scanner, err := DefaultScanner()
	require.NoError(t, err)

	// Realistic decoys that must survive redaction untouched.
	clean := []string{
		`"commit_start": "9f8e7d6c5b4a39281706f5e4d3c2b1a0f9e8d7c6"`,
		`"id": "3bf79921-3c03-81b6-afff-cb246849866f"`,
		`"total_tokens": 1234567890`,
		`"content": "hello world"`,
		`"path": "app/controlplane/internal/service/attestation.go"`,
		`"models_used": ["claude-opus-5", "claude-sonnet-5"]`,
	}

	for _, text := range clean {
		t.Run(text, func(t *testing.T) {
			findings, err := scanner.Scan(context.Background(), text)
			require.NoError(t, err)
			assert.Empty(t, findings)
		})
	}
}

// TestDefaultPlaceholderIsNotDetectable is the gate on the placeholder format.
// The convergence loop in Redact terminates only because a placeholder is not
// itself detected as a secret; if one rule matched it, redaction would replace
// its own output forever and fail with ErrNotConverged. Checked against every
// rule in the shipped ruleset rather than a hand-picked sample, so growing the
// ruleset cannot quietly invalidate the assumption.
func TestDefaultPlaceholderIsNotDetectable(t *testing.T) {
	scanner, err := DefaultScanner()
	require.NoError(t, err)

	cfg, err := betterleaksconfig.Default()
	require.NoError(t, err)
	require.NotEmpty(t, cfg.Rules)
	t.Logf("checking placeholders for %d rules", len(cfg.Rules))

	ruleIDs := make([]string, 0, len(cfg.Rules)+1)
	for id := range cfg.Rules {
		ruleIDs = append(ruleIDs, id)
	}
	// The empty rule id yields the bare "[REDACTED]" placeholder.
	ruleIDs = append(ruleIDs, "")

	for _, id := range ruleIDs {
		placeholder := DefaultPlaceholder(id)
		// Scan the placeholder bare and in the key/value shape that triggers the
		// generic keyword-based rules.
		for _, text := range []string{placeholder, `"api_key": "` + placeholder + `"`} {
			findings, err := scanner.Scan(context.Background(), text)
			require.NoError(t, err)
			assert.Empty(t, findings, "placeholder for rule %q is itself detected as a secret in %q", id, text)
		}
	}
}

// TestDefaultScannerRepeatedScansDoNotAccumulate guards SkipFindingAppend. The
// scanner is a process-wide singleton and Redact scans the same document more
// than once, so a detector that retained every finding would grow without bound
// across attestations.
func TestDefaultScannerRepeatedScansDoNotAccumulate(t *testing.T) {
	scanner, err := DefaultScanner()
	require.NoError(t, err)

	text := `"content": "` + awsPair + `"`

	first, err := scanner.Scan(context.Background(), text)
	require.NoError(t, err)
	require.Len(t, first, 2, "the composite rule reports the key id plus its secret component")

	for range 5 {
		again, err := scanner.Scan(context.Background(), text)
		require.NoError(t, err)
		assert.Equal(t, first, again, "repeated scans must return the same findings, not accumulate")
	}
}

// TestDefaultScannerConcurrentScans exercises the shared detector under -race.
// Run mutates per-run state on the detector itself, so the scanner has to
// serialise access.
func TestDefaultScannerConcurrentScans(t *testing.T) {
	scanner, err := DefaultScanner()
	require.NoError(t, err)

	texts := []string{
		`"content": "` + awsPair + `"`,
		`"content": "nothing to see here"`,
		`"repository": "https://oauth2:` + fakeGitHubPAT + `@github.com/example/repo.git"`,
	}

	var wg sync.WaitGroup
	errs := make([]error, 12)
	for i := range errs {
		wg.Add(1)
		go func(idx int, text string) {
			defer wg.Done()
			_, errs[idx] = scanner.Scan(context.Background(), text)
		}(i, texts[i%len(texts)])
	}
	wg.Wait()

	for _, err := range errs {
		require.NoError(t, err)
	}
}

func TestDefaultScannerCancelledContext(t *testing.T) {
	scanner, err := DefaultScanner()
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err = scanner.Scan(ctx, `"content": "`+awsPair+`"`)
	require.ErrorIs(t, err, context.Canceled)
}

// TestDefaultScannerReportsCompositeComponents pins down the most dangerous
// asymmetry in the ruleset. aws-access-token is a composite rule: it fires only
// when a secret access key is found nearby, it reports the AWS key *id* as its
// secret, and the rule matching the actual credential is marked as not
// independently reportable. Reporting only the primary finding would therefore
// redact the harmless public identifier and leave the real credential in place,
// so the scanner has to flatten the components back out.
func TestDefaultScannerReportsCompositeComponents(t *testing.T) {
	scanner, err := DefaultScanner()
	require.NoError(t, err)

	findings, err := scanner.Scan(context.Background(), `"content": "`+awsPair+`"`)
	require.NoError(t, err)

	secretsByRule := make(map[string]string, len(findings))
	for _, f := range findings {
		secretsByRule[f.RuleID] = f.Secret
	}

	assert.Equal(t, fakeAWSKey, secretsByRule["aws-access-token"])
	assert.Equal(t, fakeAWSSecret, secretsByRule["aws-secret-access-key"],
		"the secret access key must be reported, not just the key id")
}

// TestRedactCredentialInURIConverges is a regression test for a rule that
// matches a *position* rather than a value. Once the token in a URI's userinfo
// is replaced, the rule for credentials embedded in a URI matches the
// placeholder sitting in its place, reporting it as the secret. Rewriting that
// would satisfy the rule again on the next pass, forever, so the engine has to
// recognise its own output. Uses the real ruleset because the behaviour is a
// property of the rules, not of the engine alone.
func TestRedactCredentialInURIConverges(t *testing.T) {
	scanner, err := DefaultScanner()
	require.NoError(t, err)

	doc := []byte(`{"repository":"https://oauth2:` + fakeGitHubPAT + `@github.com/example/repo.git"}`)

	once, report, err := New(scanner).Redact(context.Background(), doc)
	require.NoError(t, err)
	require.True(t, report.Changed())
	assert.NotContains(t, string(once), fakeGitHubPAT)
	assert.Contains(t, string(once), "[REDACTED:github-pat]")
	// Two passes: one that redacts, one that confirms nothing is left.
	assert.Equal(t, 2, report.Passes)

	// Redacting the result again must be a no-op, not a rename.
	twice, report, err := New(scanner).Redact(context.Background(), once)
	require.NoError(t, err)
	assert.Equal(t, string(once), string(twice))
	assert.False(t, report.Changed())
}

func BenchmarkDefaultScannerInit(b *testing.B) {
	for b.Loop() {
		if _, err := newBetterleaksScanner(); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkRedact(b *testing.B) {
	scanner, err := DefaultScanner()
	require.NoError(b, err)

	// A synthetic multi-megabyte transcript with a secret near the end.
	turn := `{"role":"assistant","content":"` + strings.Repeat("some ordinary transcript text ", 40) + `"},`
	var sb strings.Builder
	sb.WriteString(`{"data":{"raw_session":{"main":[`)
	for sb.Len() < 5<<20 {
		sb.WriteString(turn)
	}
	sb.WriteString(`{"role":"user","content":"` + awsPair + `"}]}}}`)
	doc := []byte(sb.String())

	r := New(scanner)
	b.SetBytes(int64(len(doc)))
	b.ResetTimer()
	for b.Loop() {
		if _, _, err := r.Redact(context.Background(), doc); err != nil {
			b.Fatal(err)
		}
	}
}
