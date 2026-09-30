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
	"regexp"
	"strconv"
	"strings"
	"sync"
	"testing"

	betterleaksconfig "github.com/betterleaks/betterleaks/config"
	betterleaksregexp "github.com/betterleaks/betterleaks/regexp"
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
	fakeGitHubPAT = "ghp_erOZlZv0B1e3amrQ" + "ugdwZ8Ro2W4kDql9WPTf"
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

// TestDefaultScannerUsesRE2 pins the regex engine. The library default is the
// standard library engine, which is several times slower on the default ruleset
// and made large sessions take tens of minutes to redact.
func TestDefaultScannerUsesRE2(t *testing.T) {
	_, err := DefaultScanner()
	require.NoError(t, err)

	assert.Equal(t, "re2", betterleaksregexp.Version())
}

func TestSplitLines(t *testing.T) {
	testCases := []struct {
		name    string
		text    string
		size    int
		overlap int
		want    []string
	}{
		{
			name: "empty text is a single empty chunk",
			text: "",
			size: 4,
			want: []string{""},
		},
		{
			name: "text that fits is a single chunk",
			text: "a\nb\n",
			size: 16,
			want: []string{"a\nb\n"},
		},
		{
			name: "splits only at line ends",
			text: "a\nb\nc\nd\n",
			size: 4,
			want: []string{"a\nb\n", "c\nd\n"},
		},
		{
			name:    "each chunk re-reads the last lines of the previous one",
			text:    "a\nb\nc\nd\n",
			size:    4,
			overlap: 1,
			want:    []string{"a\nb\n", "b\nc\nd\n"},
		},
		{
			name: "a final line without a newline is kept",
			text: "a\nb\nc",
			size: 4,
			want: []string{"a\nb\n", "c"},
		},
		{
			name: "a line longer than the chunk size is never split",
			text: "aaaaaaaa\nb\n",
			size: 4,
			want: []string{"aaaaaaaa\n", "b\n"},
		},
		{
			// A composite primary on a long line can have its component on the
			// next line, so the last line is re-read however long it is.
			name:    "the last line is re-read even when it is longer than the chunk size",
			text:    "aaaaaaaa\nb\nc\n",
			size:    4,
			overlap: 1,
			want:    []string{"aaaaaaaa\n", "aaaaaaaa\nb\nc\n"},
		},
		{
			// Without the cap, a run of huge lines would be scanned again by
			// every following chunk.
			name:    "earlier lines are re-read only within the chunk size in bytes",
			text:    "aaaaaaaa\nbbbbbbbb\nc\n",
			size:    4,
			overlap: 2,
			want:    []string{"aaaaaaaa\n", "aaaaaaaa\nbbbbbbbb\n", "bbbbbbbb\nc\n"},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var got []string
			for _, c := range splitLines(tc.text, tc.size, tc.overlap) {
				got = append(got, tc.text[c.start:c.end])
			}
			assert.Equal(t, tc.want, got)
		})
	}
}

// TestSplitLinesCoversText checks that chunking never drops content: the chunks
// start at the beginning, end at the end, each one moves forward, and none leaves
// a gap after its predecessor.
func TestSplitLinesCoversText(t *testing.T) {
	var sb strings.Builder
	for i := range 500 {
		sb.WriteString(strings.Repeat("x", i%37))
		sb.WriteString("\n")
	}
	sb.WriteString("tail without newline")
	text := sb.String()

	for _, overlap := range []int{0, 1, 5, 64} {
		chunks := splitLines(text, 256, overlap)
		require.Greater(t, len(chunks), 1)

		assert.Equal(t, 0, chunks[0].start)
		assert.Equal(t, len(text), chunks[len(chunks)-1].end)
		for i := 1; i < len(chunks); i++ {
			prev, cur := chunks[i-1], chunks[i]
			assert.Equal(t, byte('\n'), text[prev.end-1], "overlap=%d: chunk %d must end at a line end", overlap, i-1)
			assert.LessOrEqual(t, cur.start, prev.end, "overlap=%d: chunk %d leaves a gap", overlap, i)
			assert.Greater(t, cur.end, prev.end, "overlap=%d: chunk %d adds no new text", overlap, i)
			if overlap == 0 {
				assert.Equal(t, prev.end, cur.start, "chunks must not overlap without an overlap")
			}
		}
	}
}

// TestChunkOverlapCoversComponentWindows is the gate on chunkOverlapLines. A
// composite rule only fires when its components sit within a window of lines
// around the primary match, so a chunk has to re-read at least that many lines of
// its predecessor or a pair straddling the boundary is missed. Checked against the
// shipped ruleset so that upgrading it cannot silently widen a window.
func TestChunkOverlapCoversComponentWindows(t *testing.T) {
	cfg, err := betterleaksconfig.Default()
	require.NoError(t, err)

	lines := regexp.MustCompile(`(\d+)L`)
	for id, rule := range cfg.Rules {
		for _, c := range rule.Components {
			if c.Within == "" {
				// Unbounded: the component may be anywhere in the fragment. With
				// chunking that means anywhere in the same chunk, which is how
				// betterleaks itself treats files.
				continue
			}
			matches := lines.FindAllStringSubmatch(c.Within, -1)
			require.NotEmpty(t, matches, "rule %q component %q has a window %q without a line bound; chunking only preserves line windows", id, c.RuleID, c.Within)
			for _, m := range matches {
				n, err := strconv.Atoi(m[1])
				require.NoError(t, err)
				assert.LessOrEqual(t, n, chunkOverlapLines, "rule %q component %q window %q exceeds the chunk overlap", id, c.RuleID, c.Within)
			}
		}
	}
}

// chunkedScanner returns a scanner with a small chunk size, so that tests can
// exercise chunk boundaries without multi-megabyte inputs.
func chunkedScanner(t *testing.T, size int) *betterleaksScanner {
	t.Helper()
	s, err := newBetterleaksScanner()
	require.NoError(t, err)
	s.chunkSize = size
	return s
}

func TestChunkedScanDetects(t *testing.T) {
	const size = 512
	filler := func(n int) string {
		var sb strings.Builder
		for i := range n {
			sb.WriteString(`  "content": "ordinary transcript line ` + strconv.Itoa(i) + `",` + "\n")
		}
		return sb.String()
	}

	keyLine := `  "content": "AWS_ACCESS_KEY_ID=` + fakeAWSKey + `",` + "\n"
	secretLine := `  "content": "AWS_SECRET_ACCESS_KEY=` + fakeAWSSecret + `",` + "\n"
	// Pads the text so that the next chunk boundary falls right after keyLine.
	pad := `  "content": "` + strings.Repeat("p", size-len(keyLine)-len(`  "content": "",`)-1) + `",` + "\n"
	// A key id at the end of a line longer than a whole chunk: the line is a
	// chunk of its own, and its secret starts the next one.
	longKeyLine := `  "content": "` + strings.Repeat("p ", size) + `AWS_ACCESS_KEY_ID=` + fakeAWSKey + `",` + "\n"

	testCases := []struct {
		name      string
		text      string
		wantRules []string
		// straddle, when set, names two substrings the test relies on being in
		// different chunks once the overlap is left out.
		straddle [2]string
	}{
		{
			name:      "secrets in chunks far apart are all found",
			text:      `  "repository": "https://oauth2:` + fakeGitHubPAT + `@github.com/example/repo.git",` + "\n" + filler(200) + `  "content": "ANTHROPIC_API_KEY=` + fakeAnthropicKey + `"` + "\n",
			wantRules: []string{"github-pat", "anthropic-api-key"},
		},
		{
			// aws-access-token needs its secret access key within 5 lines. The
			// boundary falls between the two, so only the overlap lets the chunk
			// holding the secret also see the key id.
			name:      "a composite pair straddling a chunk boundary is found",
			text:      pad + keyLine + secretLine + filler(50),
			wantRules: []string{"aws-access-token", "aws-secret-access-key"},
			straddle:  [2]string{fakeAWSKey, fakeAWSSecret},
		},
		{
			name:      "a composite pair straddling a boundary after a line longer than a chunk is found",
			text:      filler(1) + longKeyLine + secretLine + filler(50),
			wantRules: []string{"aws-access-token", "aws-secret-access-key"},
			straddle:  [2]string{fakeAWSKey, fakeAWSSecret},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			scanner := chunkedScanner(t, size)
			require.Greater(t, len(splitLines(tc.text, size, chunkOverlapLines)), 1, "the test text must span several chunks")

			// Pins the premise: without the overlap no chunk holds both halves, so
			// the case really goes through the overlap rather than past it.
			if tc.straddle != [2]string{} {
				for _, c := range splitLines(tc.text, size, 0) {
					chunk := tc.text[c.start:c.end]
					require.False(t, strings.Contains(chunk, tc.straddle[0]) && strings.Contains(chunk, tc.straddle[1]),
						"the pair must straddle a chunk boundary")
				}
			}

			findings, err := scanner.Scan(context.Background(), tc.text)
			require.NoError(t, err)

			var rules []string
			for _, f := range findings {
				rules = append(rules, f.RuleID)
				assert.Contains(t, tc.text, f.Secret)
			}
			for _, want := range tc.wantRules {
				assert.Contains(t, rules, want)
			}
		})
	}
}

// TestChunkedScanIsDeterministic guards the order of findings. Chunks are
// scanned concurrently, but when two rules report the same secret the first
// finding names the placeholder, so the order has to be stable or the redacted
// document's digest would change from one run to the next.
func TestChunkedScanIsDeterministic(t *testing.T) {
	scanner := chunkedScanner(t, 256)

	// A distinct token per chunk, so that the order in which concurrently
	// scanned chunks report is visible in the result rather than deduplicated
	// away.
	var sb strings.Builder
	const tokens = 60
	for i := range tokens {
		pat := fakeGitHubPAT[:len(fakeGitHubPAT)-2] + string(rune('A'+i%26)) + string(rune('a'+i/26))
		sb.WriteString(`  "repository": "https://oauth2:` + pat + `@github.com/example/repo.git",` + "\n")
		sb.WriteString(`  "content": "` + strings.Repeat("ordinary transcript text ", 10) + `",` + "\n")
	}
	text := sb.String()

	first, err := scanner.Scan(context.Background(), text)
	require.NoError(t, err)
	require.GreaterOrEqual(t, len(first), tokens)

	for range 10 {
		again, err := scanner.Scan(context.Background(), text)
		require.NoError(t, err)
		assert.Equal(t, first, again)
	}
}

// TestChunkedRedact runs the whole redaction over a document that spans many
// chunks and checks that nothing is left behind.
func TestChunkedRedact(t *testing.T) {
	scanner := chunkedScanner(t, 256)

	var sb strings.Builder
	sb.WriteString(`{"data":{"raw_session":{"main":[`)
	for i := range 200 {
		if i > 0 {
			sb.WriteString(",")
		}
		content := "ordinary transcript text " + strconv.Itoa(i)
		switch i {
		case 3:
			content = "git clone https://oauth2:" + fakeGitHubPAT + "@github.com/example/repo.git"
		case 197:
			content = "ANTHROPIC_API_KEY=" + fakeAnthropicKey
		}
		sb.WriteString(`{"role":"user","content":"` + content + `"}`)
	}
	sb.WriteString(`]}}}`)

	out, report, err := New(scanner).Redact(context.Background(), []byte(sb.String()))
	require.NoError(t, err)
	assert.NotContains(t, string(out), fakeGitHubPAT)
	assert.NotContains(t, string(out), fakeAnthropicKey)
	assert.Equal(t, 1, report.ByRule["github-pat"])
	assert.Equal(t, 1, report.ByRule["anthropic-api-key"])
}

// multiChunkText is indented-JSON-shaped text spanning many 256-byte chunks, with
// a distinct token on every tenth line.
func multiChunkText() string {
	var sb strings.Builder
	for i := range 300 {
		if i%10 == 0 {
			pat := fakeGitHubPAT[:len(fakeGitHubPAT)-2] + string(rune('A'+i/10%26)) + "x"
			sb.WriteString(`  "repository": "https://oauth2:` + pat + `@github.com/example/repo.git",` + "\n")
			continue
		}
		sb.WriteString(`  "content": "ordinary transcript line ` + strconv.Itoa(i) + `",` + "\n")
	}
	return sb.String()
}

// uniqueFindings is the set of findings, which is what must not change however
// the text is chunked.
func uniqueFindings(findings []Finding) []Finding {
	seen := make(map[Finding]struct{}, len(findings))
	var out []Finding
	for _, f := range findings {
		if _, dup := seen[f]; !dup {
			seen[f] = struct{}{}
			out = append(out, f)
		}
	}
	return out
}

// TestLineSpansRoundTrip checks that chunks converted to line ranges and back
// are the same byte ranges, which is what lets a later pass reuse them.
func TestLineSpansRoundTrip(t *testing.T) {
	testCases := []struct {
		name    string
		text    string
		size    int
		overlap int
	}{
		{name: "empty", text: "", size: 4},
		{name: "single chunk", text: "a\nb\n", size: 16},
		{name: "trailing newline", text: multiChunkText(), size: 256, overlap: chunkOverlapLines},
		{name: "no trailing newline", text: strings.TrimSuffix(multiChunkText(), "\n"), size: 256, overlap: chunkOverlapLines},
		{name: "no overlap", text: multiChunkText(), size: 256},
		{name: "long lines", text: "aaaaaaaa\nbbbbbbbb\nc", size: 4, overlap: 2},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			spans := splitLines(tc.text, tc.size, tc.overlap)
			assert.Equal(t, spans, byteSpans(tc.text, lineSpans(tc.text, spans)))
		})
	}
}

// TestDocumentScannerMatchesFreshScan is the safety property of rescanning
// incrementally: after the document changes, the scanner bound to it reports the
// same secrets as a scan from scratch would.
func TestDocumentScannerMatchesFreshScan(t *testing.T) {
	replaceFirst := func(text string) string {
		i := strings.Index(text, "ghp_")
		return text[:i] + "[REDACTED:github-pat]" + text[i+len(fakeGitHubPAT):]
	}

	testCases := []struct {
		name string
		edit func(string) string
	}{
		{name: "unchanged", edit: func(s string) string { return s }},
		{name: "one secret replaced in place", edit: replaceFirst},
		{name: "every secret replaced in place", edit: func(s string) string {
			for strings.Contains(s, "ghp_") {
				s = replaceFirst(s)
			}
			return s
		}},
		{name: "a line inserted", edit: func(s string) string {
			return `  "content": "ANTHROPIC_API_KEY=` + fakeAnthropicKey + `",` + "\n" + s
		}},
		{name: "a line removed", edit: func(s string) string {
			return s[strings.IndexByte(s, '\n')+1:]
		}},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			scanner := chunkedScanner(t, 256)
			doc := scanner.forDocument()
			text := multiChunkText()

			first, err := doc.Scan(context.Background(), text)
			require.NoError(t, err)
			fresh, err := scanner.Scan(context.Background(), text)
			require.NoError(t, err)
			require.Equal(t, fresh, first, "the first scan of a document is a scan from scratch")

			edited := tc.edit(text)
			got, err := doc.Scan(context.Background(), edited)
			require.NoError(t, err)
			want, err := scanner.Scan(context.Background(), edited)
			require.NoError(t, err)
			assert.ElementsMatch(t, uniqueFindings(want), uniqueFindings(got))
		})
	}
}

// TestDocumentScannerRescansOnlyChangedChunks pins the point of scanning
// incrementally: a pass after a few in-place replacements scans the chunks that
// changed, not the whole document again.
func TestDocumentScannerRescansOnlyChangedChunks(t *testing.T) {
	scanner := chunkedScanner(t, 256)
	doc := scanner.forDocument()
	text := multiChunkText()

	_, err := doc.Scan(context.Background(), text)
	require.NoError(t, err)

	i := strings.Index(text, "ghp_")
	edited := text[:i] + "[REDACTED:github-pat]" + text[i+len(fakeGitHubPAT):]

	before := scanner.detector.TotalBytes.Load()
	_, err = doc.Scan(context.Background(), edited)
	require.NoError(t, err)
	scanned := scanner.detector.TotalBytes.Load() - before

	assert.Positive(t, scanned, "the changed chunk has to be scanned again")
	assert.Less(t, scanned, uint64(len(edited)/4), "only the chunks around the change are scanned again")
}

// TestRedactIncrementalMatchesFullRescan runs whole redactions with and without
// incremental rescanning and requires the same result.
func TestRedactIncrementalMatchesFullRescan(t *testing.T) {
	var sb strings.Builder
	sb.WriteString(`{"data":{"raw_session":{"main":[`)
	for i := range 200 {
		if i > 0 {
			sb.WriteString(",")
		}
		content := "ordinary transcript text " + strconv.Itoa(i)
		switch i % 50 {
		case 3:
			content = "git clone https://oauth2:" + fakeGitHubPAT + "@github.com/example/repo.git"
		case 17:
			content = "ANTHROPIC_API_KEY=" + fakeAnthropicKey
		case 31:
			content = "export " + awsPair
		}
		sb.WriteString(`{"role":"user","content":"` + content + `"}`)
	}
	sb.WriteString(`]}}}`)
	doc := []byte(sb.String())

	scanner := chunkedScanner(t, 256)
	// Embedding hides forDocument, so this one scans every pass from scratch.
	full := struct{ Scanner }{scanner}

	want, wantReport, err := New(full).Redact(context.Background(), doc)
	require.NoError(t, err)
	got, gotReport, err := New(scanner).Redact(context.Background(), doc)
	require.NoError(t, err)

	assert.Equal(t, string(want), string(got))
	assert.Equal(t, wantReport, gotReport)
	assert.True(t, gotReport.Changed())
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
