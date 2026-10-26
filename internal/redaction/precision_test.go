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
	"encoding/json"
	"strings"
	"testing"

	betterleaksconfig "github.com/betterleaks/betterleaks/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Values shaped like the ones the generic rules matched in recorded AI coding
// sessions, and real secrets of the same shape. Assembled from fragments, like
// the other synthetic credentials in this package.
const (
	sampleUUID   = "3bf79921-3c03-81b6-" + "afff-cb246849866f"
	sampleDigest = "9f8e7d6c5b4a39281706f5e4d3c2b1a0" + "f9e8d7c69f8e7d6c5b4a39281706f5e4"
	// samplePassword is a password with enough entropy to look real.
	samplePassword = "Xk9mQ2vL" + "p7wRt4zN"
	// sampleAPIKey is a random-looking key the generic-api-key rule matches.
	sampleAPIKey = "q8Zr2LmX7v" + "Pk4TnW9sYb"
	sampleEmail  = "sarah@chainloop.local"
	// sampleSSHFingerprint is the unpadded base64 of a SHA-256 hash, as
	// OpenSSH prints the fingerprint of a public key after SHA256:
	sampleSSHFingerprint = "q3Rk8vLm2+Zx7Pw4" + "Tn9sYbHc1/Fd6Gj0" + "Ue5Ai8Ko2Ws"

	ruleGenericAPIKey = "generic-api-key"
	ruleGitHubPAT     = "github-pat"
	ruleCredentialURI = "generic-credential-uri"
)

// defaultRulesScanner returns a scanner with the betterleaks default ruleset and
// none of the allowlists of rulesConfig. It shows that a value the allowlists
// keep is one the default ruleset matches, so that a test of an allowlist
// cannot pass for the wrong reason.
func defaultRulesScanner(t *testing.T) *betterleaksScanner {
	t.Helper()
	s, err := newScannerWithRules(betterleaksconfig.DefaultConfig)
	require.NoError(t, err)
	return s
}

// TestRulesConfigKeepsNonSecrets covers the values that the generic rules of
// the default ruleset replace in AI coding sessions but which are not secrets.
// Each case is a line of a tool result or a message, followed by a line break,
// which is the shape it has in a transcript.
func TestRulesConfigKeepsNonSecrets(t *testing.T) {
	scanner, err := DefaultScanner()
	require.NoError(t, err)
	unfiltered := defaultRulesScanner(t)

	testCases := []struct {
		name string
		text string
		// wantDefaultRule is the rule of the default ruleset that replaces part
		// of the text.
		wantDefaultRule string
	}{
		{
			name:            "uuid as the value of a key",
			text:            "workflow key: " + sampleUUID + "\n",
			wantDefaultRule: ruleGenericAPIKey,
		},
		{
			name:            "uuid as an idempotency key",
			text:            "idempotency_key=" + sampleUUID + "\n",
			wantDefaultRule: ruleGenericAPIKey,
		},
		{
			name:            "sha256 evidence digest in cli output",
			text:            "attestation pushed, access sha256:" + sampleDigest + "\n",
			wantDefaultRule: ruleGenericAPIKey,
		},
		{
			name:            "minified javascript in a tool result",
			text:            "host: localhost\nuser: admin\n!function(e){var t={};e.password=t.password||n.password,e.user=u}\n",
			wantDefaultRule: genericPasswordRuleID,
		},
		{
			name:            "documentation of development credentials",
			text:            "- **Dex OIDC**: localhost:5556, users " + sampleEmail + " / john@chainloop.local (password: `password`)\n- **Database**: host: localhost, user: postgres\n",
			wantDefaultRule: genericPasswordRuleID,
		},
		{
			name:            "key word as the value, followed by another line",
			text:            "host: localhost\nuser: postgres\npassword: password\nport: 5432\n",
			wantDefaultRule: genericPasswordRuleID,
		},
		{
			name:            "key word as the value in a go string literal",
			text:            `text: "host: localhost\nuser: postgres\npassword: password\nport: 5432\n",` + "\n",
			wantDefaultRule: genericPasswordRuleID,
		},
		{
			name:            "minified javascript with a plain identifier operand",
			text:            "host: localhost\nuser: admin\n!function(e){e.password=e.password||n,e.user=u}\n",
			wantDefaultRule: genericPasswordRuleID,
		},
		{
			name:            "legacy placeholder shown again",
			text:            "host: localhost\nuser: admin\npassword: [REDACTED:generic-password]\nport: 5432\n",
			wantDefaultRule: genericPasswordRuleID,
		},
		{
			name:            "placeholder shown again in a go string literal",
			text:            `text: "host: localhost\nuser: admin\npassword: [CHAINLOOP_TRACE_REDACTED:generic-password]\nport: 5432\n",` + "\n",
			wantDefaultRule: genericPasswordRuleID,
		},
		{
			name:            "elided password of a uri in documentation",
			text:            "the issuer is `http://localhost:…@chainloop.local`\n",
			wantDefaultRule: ruleCredentialURI,
		},
		{
			name:            "duration of a go test",
			text:            "=== RUN   TestRedactPassword\n--- PASS: TestRedactPassword: (0.01s)\nPASS\n",
			wantDefaultRule: genericPasswordRuleID,
		},
		{
			name:            "placeholder token in a curl example of a document",
			text:            "curl -H 'Authorization: Bearer REPLACE_ME' https://api.example.com\n",
			wantDefaultRule: "curl-auth-header",
		},
		{
			name:            "port read as the password of a uri",
			text:            "issuer: http://localhost:8000@chainloop.local\n",
			wantDefaultRule: ruleCredentialURI,
		},
		{
			// The file content of a Write tool call. The scanner sees it
			// JSON-encoded, so the password of the match ran from the port,
			// through the escaped line breaks, to the user part of an email
			// address on a later line.
			name:            "uri password running through line breaks to an email address",
			text:            "API_URL=http://localhost:48002\nCAS_ENDPOINT=localhost:49001\nTEST_EMAIL=" + sampleEmail + "\n",
			wantDefaultRule: ruleCredentialURI,
		},
		{
			// git log --show-signature on a commit signed with an SSH key
			name:            "ssh key fingerprint of a commit signature",
			text:            "Good \"git\" signature for " + sampleEmail + " with ED25519 key SHA256:" + sampleSSHFingerprint + "\n",
			wantDefaultRule: ruleGenericAPIKey,
		},
		{
			name:            "rsa key fingerprint of a commit signature",
			text:            "Good \"git\" signature for " + sampleEmail + " with RSA key SHA256:" + sampleSSHFingerprint + "\n",
			wantDefaultRule: ruleGenericAPIKey,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			_, report, err := New(unfiltered).RedactText(context.Background(), tc.text)
			require.NoError(t, err)
			require.Contains(t, report.RuleIDs(), tc.wantDefaultRule, "the default ruleset must replace part of this text, or this case tests nothing")

			got, report, err := New(scanner).RedactText(context.Background(), tc.text)
			require.NoError(t, err)
			assert.Equal(t, tc.text, got)
			assert.False(t, report.Changed(), "replaced by %v", report.RuleIDs())
		})
	}
}

// Codex tool results carry JSON-encoded command output inside a text leaf.
// Check both that shape and plain output, with a real password beside the
// selector so the exemption cannot hide other findings in the same leaf.
func TestRedactPasswordFieldSelectors(t *testing.T) {
	scanner, err := DefaultScanner()
	require.NoError(t, err)
	unfiltered := defaultRulesScanner(t)

	for _, selector := range []string{`[id="1-password"]`, `input[name="password"]`} {
		for _, encoded := range []bool{false, true} {
			name := selector
			if encoded {
				name += "/json tool output"
			}
			t.Run(name, func(t *testing.T) {
				text := "const AUTH0 = {\n  email: 'input[name=\"login\"]',\n  password: '" + selector + "',\n};\n"
				wrap := func(text string) string {
					if !encoded {
						return text
					}
					b, err := json.Marshal(map[string]string{"output": text})
					require.NoError(t, err)
					return string(b)
				}

				_, report, err := New(unfiltered).RedactText(context.Background(), wrap(text))
				require.NoError(t, err)
				require.Contains(t, report.RuleIDs(), genericPasswordRuleID, "the original ruleset must detect the selector")

				got, report, err := New(scanner).RedactText(context.Background(), wrap(text))
				require.NoError(t, err)
				assert.Equal(t, wrap(text), got)
				assert.False(t, report.Changed())

				text += "user: " + sampleEmail + "\npassword: " + samplePassword + ";\n"
				got, report, err = New(scanner).RedactText(context.Background(), wrap(text))
				require.NoError(t, err)
				assert.Equal(t, wrap(strings.ReplaceAll(text, samplePassword, DefaultPlaceholder(genericPasswordRuleID))), got)
				assert.Equal(t, map[string]int{genericPasswordRuleID: 1}, report.ByRule)
			})
		}
	}
}

// Extra escaping must not turn a credential into an allowlisted selector.
func TestRedactOverescapedPasswordFieldSelectors(t *testing.T) {
	scanner, err := DefaultScanner()
	require.NoError(t, err)

	for _, selector := range []string{`[id="1-password"]`, `input[name="password"]`} {
		for _, backslashes := range []int{2, 3} {
			secret := strings.ReplaceAll(selector, `"`, strings.Repeat(`\`, backslashes)+`"`)
			t.Run(secret, func(t *testing.T) {
				text := "user: " + sampleEmail + "\npassword: '" + secret + "'\n"
				got, report, err := New(scanner).RedactText(context.Background(), text)
				require.NoError(t, err)
				assert.Equal(t, strings.ReplaceAll(text, secret, DefaultPlaceholder(genericPasswordRuleID)), got)
				assert.Equal(t, map[string]int{genericPasswordRuleID: 1}, report.ByRule)
			})
		}
	}
}

// TestRulesConfigStillRedactsSecrets is the counterpart of
// TestRulesConfigKeepsNonSecrets: a real secret of each shape is still
// replaced.
func TestRulesConfigStillRedactsSecrets(t *testing.T) {
	scanner, err := DefaultScanner()
	require.NoError(t, err)

	testCases := []struct {
		name     string
		text     string
		secret   string
		wantRule string
		// keep is text next to the secret that must survive.
		keep string
	}{
		{
			name:     "api key that is not a uuid",
			text:     "workflow key: " + sampleAPIKey + "\n",
			secret:   sampleAPIKey,
			wantRule: ruleGenericAPIKey,
		},
		{
			name:     "hex key without a digest prefix",
			text:     "access_token: " + sampleDigest + "\n",
			secret:   sampleDigest,
			wantRule: ruleGenericAPIKey,
		},
		{
			name:     "base64 key without a fingerprint prefix",
			text:     "api_key: " + sampleSSHFingerprint + "\n",
			secret:   sampleSSHFingerprint,
			wantRule: ruleGenericAPIKey,
		},
		{
			name:     "key longer than a fingerprint after a SHA256 prefix",
			text:     "signing key SHA256:" + strings.Repeat(sampleAPIKey, 3) + "\n",
			secret:   strings.Repeat(sampleAPIKey, 3),
			wantRule: ruleGenericAPIKey,
		},
		{
			name:     "key that continues a fingerprint with a separator",
			text:     "signing key SHA256:" + strings.Repeat(sampleAPIKey, 3)[:43] + "-" + sampleAPIKey + "\n",
			secret:   sampleAPIKey,
			wantRule: ruleGenericAPIKey,
		},
		{
			name:     "api key on the line after an ssh key fingerprint",
			text:     "Good \"git\" signature with ED25519 key SHA256:" + sampleSSHFingerprint + "\napi_key: " + sampleAPIKey + "\n",
			secret:   sampleAPIKey,
			wantRule: ruleGenericAPIKey,
			keep:     "SHA256:" + sampleSSHFingerprint,
		},
		{
			name:     "password next to an email username",
			text:     "host: db.internal\nuser: " + sampleEmail + "\npassword: " + samplePassword + "\n",
			secret:   samplePassword,
			wantRule: genericPasswordRuleID,
			keep:     sampleEmail,
		},
		{
			name:     "bearer token in a curl command",
			text:     "curl -H 'Authorization: Bearer " + sampleAPIKey + "' https://api.example.com\n",
			secret:   sampleAPIKey,
			wantRule: "curl-auth-header",
		},
		{
			name:     "password with a selector suffix",
			text:     "user: " + sampleEmail + "\npassword: '" + samplePassword + `[id="1-password"]'` + "\n",
			secret:   samplePassword,
			wantRule: genericPasswordRuleID,
		},
		{
			name:     "password with a selector prefix",
			text:     "user: " + sampleEmail + "\npassword: '" + `input[name="password"]` + samplePassword + "'\n",
			secret:   samplePassword,
			wantRule: genericPasswordRuleID,
		},
		{
			name:     "password with a logical operator in it",
			text:     "host: db.internal\nuser: " + sampleEmail + "\npassword: Xk9m||Q2vL" + "p7wRt4z\n",
			secret:   "Xk9m||Q2vL" + "p7wRt4z",
			wantRule: genericPasswordRuleID,
		},
		{
			name:     "numeric password in a connection uri",
			text:     "dsn: postgres://admin:" + "12345@db.internal:5432/app\n",
			secret:   "admin:" + "12345@",
			wantRule: ruleCredentialURI,
		},
		{
			name:     "password in a connection uri",
			text:     "dsn: postgres://admin:" + samplePassword + "@db.internal:5432/app\n",
			secret:   samplePassword,
			wantRule: ruleCredentialURI,
		},
		{
			name:     "password in a connection uri on the line after a host and port",
			text:     "API_URL=http://localhost:48002\nDSN=postgres://admin:" + samplePassword + "@db.internal:5432/app\n",
			secret:   samplePassword,
			wantRule: ruleCredentialURI,
			keep:     "localhost:48002",
		},
		{
			name:     "password in a uri followed by a line break and an email address",
			text:     "API_URL=http://admin:" + samplePassword + "\nTEST_EMAIL=" + sampleEmail + "\n",
			secret:   samplePassword,
			wantRule: ruleCredentialURI,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			got, report, err := New(scanner).RedactText(context.Background(), tc.text)
			require.NoError(t, err)
			assert.NotContains(t, got, tc.secret)
			assert.Contains(t, report.RuleIDs(), tc.wantRule)
			if tc.keep != "" {
				assert.Contains(t, got, tc.keep)
			}
		})
	}
}

// TestRedactKeepsEchoedPlaceholders covers transcripts that show placeholders
// of an earlier redaction: test assertions, a pull request check table, or a
// policy result. They are left as they are, and not counted as replacements.
func TestRedactKeepsEchoedPlaceholders(t *testing.T) {
	scanner, err := DefaultScanner()
	require.NoError(t, err)

	texts := []string{
		"wantURI := \"https://tracker.example.com/issue/1?token=[CHAINLOOP_TRACE_REDACTED:github-pat]\"\n",
		"assert.Contains(t, string(content), \"[CHAINLOOP_TRACE_REDACTED:aws-access-token]\")\n",
		"| ai-config-no-secrets | fail | Secret (generic-password) detected [CHAINLOOP_TRACE_REDACTED:generic-password] |\n",
		"password: [CHAINLOOP_TRACE_REDACTED:generic-password]\napi_key: [CHAINLOOP_TRACE_REDACTED:generic-api-key]\n",
		// The log line the CLI prints after a redaction.
		"10:04AM INF redacted secrets from the AI coding session before upload count=3 rules=[\"generic-password\",\"generic-username\"]\n",
	}

	for _, text := range texts {
		t.Run(strings.SplitN(text, " ", 2)[0], func(t *testing.T) {
			got, report, err := New(scanner).RedactText(context.Background(), text)
			require.NoError(t, err)
			assert.Equal(t, text, got)
			assert.False(t, report.Changed(), "replaced by %v", report.RuleIDs())
		})
	}
}

// TestRedactAllowMarkerWithDefaultRules checks an allow marker against the real
// ruleset: a test fixture whose line carries one is kept, the same value on a
// line without one is not.
func TestRedactAllowMarkerWithDefaultRules(t *testing.T) {
	scanner, err := DefaultScanner()
	require.NoError(t, err)

	fixture := "\tconst pat = \"" + fakeGitHubPAT + "\" // gitleaks:allow\n"
	text := "119" + fixture + "120\twantURI := \"https://tracker.example.com/issue/1?token=\" + pat\nexport GITHUB_TOKEN=" + fakeGitHubPAT + "\n"

	got, report, err := New(scanner, WithAllowMarkers()).RedactText(context.Background(), text)
	require.NoError(t, err)
	assert.Contains(t, got, fixture, "the line with the marker is kept")
	assert.Contains(t, got, "export GITHUB_TOKEN=[CHAINLOOP_TRACE_REDACTED:github-pat]\n")
	assert.Equal(t, map[string]int{ruleGitHubPAT: 1}, report.Allowed)
	assert.Equal(t, map[string]int{ruleGitHubPAT: 1}, report.ByRule)
}
