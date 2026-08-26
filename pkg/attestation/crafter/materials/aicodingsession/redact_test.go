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

package aicodingsession

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"strconv"
	"strings"
	"testing"

	"github.com/chainloop-dev/chainloop/internal/redaction"
	"github.com/chainloop-dev/chainloop/internal/schemavalidators"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Synthetic credentials for testdata/session-with-secrets.json.
//
// The fixture holds placeholders rather than the credentials themselves, and
// readFixture substitutes them. Nothing credential-shaped is committed: a
// realistic-looking secret in test data gets flagged by whatever scans the
// repository, and there has been one instance of each kind already — GitHub's
// push protection rejects the AWS patterns, and Checkov's basic-auth check
// rejects a token embedded in a URL. The repository URL is therefore substituted
// whole, since it is the "://user:pass@host" shape that is recognised rather
// than the token in it.
//
// The values are assembled from fragments for the same reason: joined, they
// would be flagged in this file instead.
const (
	fixtureAWSKey       = "AKIA" + "4G7TI63VCBIRS4GW"
	fixtureAWSSecret    = "kQ7zXn2VbW9pLm4RtY6" + "uHs3JdF8gA1cE5oPzQwXn"
	fixtureGitHubPAT    = "ghp_erOZlZv0B1e3amrQ" + "ugdwZ8Ro2W4kDql9WPTf"
	fixtureAnthropicKey = "sk-ant-api03-sT5wsx9DwmaHZDL0dUWKNhAhULxa35sUzyLFK9" +
		"5QBTZMDJTYn8p0J7ZQbwpYGYCQeW5eXAAGtVSmhp7UO9vxHJtSBC0xpAA"
	fixtureRepository = "https://oauth2:" + fixtureGitHubPAT + "@github.com/example/repo.git"
)

// fixtureSecrets maps each placeholder in the session fixtures to the value the
// detector has to see. Keep in sync with the copy in the materials package,
// which needs the same substitution to reach the crafter through a real file.
var fixtureSecrets = map[string]string{
	"__AWS_ACCESS_KEY_ID__":               fixtureAWSKey,
	"__AWS_SECRET_ACCESS_KEY__":           fixtureAWSSecret,
	"__GITHUB_PAT__":                      fixtureGitHubPAT,
	"__ANTHROPIC_API_KEY__":               fixtureAnthropicKey,
	"__GIT_REPOSITORY_WITH_CREDENTIALS__": fixtureRepository,
}

// readFixture loads a session fixture with its credential placeholders resolved.
func readFixture(t *testing.T, path string) []byte {
	t.Helper()

	content, err := os.ReadFile(path)
	require.NoError(t, err)

	for placeholder, secret := range fixtureSecrets {
		content = bytes.ReplaceAll(content, []byte(placeholder), []byte(secret))
	}
	return content
}

func TestEligible(t *testing.T) {
	testCases := []struct {
		path         string
		wantEligible bool
	}{
		// Protected: identifiers, enums and timestamps the platform joins on.
		{"/chainloop.material.evidence.id", false},
		{"/schema", false},
		{"/data/schema_version", false},
		{"/data/agent/name", false},
		{"/data/agent/version", false},
		{"/data/session/id", false},
		{"/data/session/slug", false},
		{"/data/session/started_at", false},
		{"/data/session/ended_at", false},
		{"/data/git_context/branch", false},
		{"/data/git_context/commit_start", false},
		{"/data/git_context/commit_end", false},
		{"/data/git_context/commits/0", false},
		{"/data/git_context/commits/17", false},
		{"/data/code_changes/files/0/path", false},
		{"/data/code_changes/files/3/status", false},
		{"/data/code_changes/files/3/attribution", false},
		{"/data/code_changes/files/3/session_ids/2", false},
		{"/data/model/primary", false},
		{"/data/model/provider", false},
		{"/data/model/models_used/1", false},
		{"/data/tools_used/summary/4/tool_name", false},
		{"/data/subagents/0/id", false},
		{"/data/subagents/0/type", false},

		// Eligible: free-form text, wherever it lives.
		{"/data/git_context/repository", true},
		{"/data/git_context/work_dir", true},
		{"/data/warnings/0", true},
		{"/data/subagents/0/description", true},
		{"/data/raw_session/main/0/message/content", true},
		{"/data/raw_session/reviewer/12/message/content/0/text", true},
		// A field a future schema version might add must be scanned by default.
		{"/data/something_new", true},
		{"/data/session/some_new_free_text", true},
		// Near-misses on protected patterns must stay eligible.
		{"/data/code_changes/files/0/path/0", true},
		{"/data/git_context/commits", true},
		{"/data/subagents/0", true},
	}

	for _, tc := range testCases {
		t.Run(tc.path, func(t *testing.T) {
			assert.Equal(t, tc.wantEligible, eligible(tc.path))
		})
	}
}

func TestRedact(t *testing.T) {
	testCases := []struct {
		name string
		file string
		// wantUnchanged asserts the document is handed back byte-for-byte, so a
		// session without secrets keeps its digest.
		wantUnchanged  bool
		wantRules      []string
		wantByRule     map[string]int
		mustNotContain []string
	}{
		{
			name:      "secrets across transcript threads, warnings and the repository URL",
			file:      "testdata/session-with-secrets.json",
			wantRules: []string{"anthropic-api-key", "aws-access-token", "aws-secret-access-key", "github-pat"},
			// The AWS key id appears in both raw_session threads and in a subagent
			// description, and is only detectable at all because its secret access
			// key sits next to one of them; the PAT is in the repository URL and in
			// a warning. Note that once a secret is detected anywhere it is removed
			// everywhere it appears, including the two leaves where the composite
			// rule would not have fired on its own.
			wantByRule: map[string]int{
				"anthropic-api-key":     1,
				"aws-access-token":      3,
				"aws-secret-access-key": 1,
				"github-pat":            2,
			},
			mustNotContain: []string{fixtureAWSKey, fixtureAWSSecret, fixtureGitHubPAT, fixtureAnthropicKey},
		},
		{
			name:          "false-positive shaped content is left alone",
			file:          "testdata/session-fp-shaped.json",
			wantUnchanged: true,
		},
		{
			name:          "clean session",
			file:          "../testdata/ai-coding-session.json",
			wantUnchanged: true,
		},
		{
			name:          "minimal session without a raw_session",
			file:          "../testdata/ai-coding-session-minimal.json",
			wantUnchanged: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			in := readFixture(t, tc.file)

			got, report, err := Redact(context.Background(), in)
			require.NoError(t, err)
			require.NotNil(t, report)

			if tc.wantUnchanged {
				assert.Equal(t, string(in), string(got), "output must be byte-identical")
				assert.False(t, report.Changed())
				return
			}

			assert.True(t, report.Changed())
			assert.Equal(t, tc.wantRules, report.RuleIDs())
			assert.Equal(t, tc.wantByRule, report.ByRule)
			assert.Empty(t, report.Unlocated)
			for _, s := range tc.mustNotContain {
				assert.NotContains(t, string(got), s)
			}

			// The redacted document must still be a valid AI coding session.
			require.NoError(t, validateEvidence(got))

			// Protected leaves must survive verbatim.
			for _, path := range []string{
				"chainloop.material.evidence.id",
				"schema",
				"data/schema_version",
				"data/agent/name",
				"data/session/id",
				"data/session/started_at",
				"data/git_context/commit_start",
				"data/git_context/commit_end",
				"data/model/primary",
				"data/subagents/0/id",
				"data/subagents/0/type",
				"data/code_changes/files/0/path",
				"data/tools_used/summary/0/tool_name",
			} {
				assert.Equal(t, jsonAt(t, in, path), jsonAt(t, got, path), "path %q must not change", path)
			}

			// Surrounding prose in a redacted leaf must be preserved.
			assert.Contains(t, string(got), "deploy this, use AWS_ACCESS_KEY_ID=")
			assert.Contains(t, string(got), "[REDACTED:aws-access-token]")
			assert.Contains(t, string(got), "and a <config> block with \\\"quoted\\\" values. Done ✅")
		})
	}
}

func TestRedactIsIdempotent(t *testing.T) {
	in := readFixture(t, "testdata/session-with-secrets.json")

	once, _, err := Redact(context.Background(), in)
	require.NoError(t, err)
	twice, report, err := Redact(context.Background(), once)
	require.NoError(t, err)

	assert.Equal(t, string(once), string(twice))
	assert.False(t, report.Changed(), "a second pass must find nothing")
}

func TestRedactRejectsInvalidInput(t *testing.T) {
	_, _, err := Redact(context.Background(), []byte(`not json`))
	require.Error(t, err)
}

// A duplicate key would let a secret ride along unscanned: decoding keeps only
// the last value, so the earlier one never reaches the scanner, and with nothing
// replaced the original bytes would be uploaded as clean. Redaction has to fail
// rather than pass the document through.
func TestRedactRejectsDuplicateKeys(t *testing.T) {
	doc := []byte(`{
      "chainloop.material.evidence.id": "CHAINLOOP_AI_CODING_SESSION",
      "schema": "https://schemas.chainloop.dev/aicodingsession/0.1/ai-coding-session.schema.json",
      "data": {
        "schema_version": "v1",
        "agent": {"name": "claude-code"},
        "session": {"id": "abc", "started_at": "2026-03-25T15:10:49.161Z", "duration_seconds": 1},
        "raw_session": {
          "main": [
            {"content": "AWS_ACCESS_KEY_ID=` + fixtureAWSKey + `", "content": "nothing here"}
          ]
        }
      }
    }`)

	out, _, err := Redact(context.Background(), doc)

	require.ErrorIs(t, err, redaction.ErrDuplicateKey)
	assert.Nil(t, out, "nothing may be returned for a document that cannot be scanned")
}

// validateEvidence mirrors the crafter's own validation of the data field.
func validateEvidence(doc []byte) error {
	var envelope struct {
		Data json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(doc, &envelope); err != nil {
		return err
	}
	var raw any
	if err := json.Unmarshal(envelope.Data, &raw); err != nil {
		return err
	}
	return schemavalidators.ValidateAICodingSession(raw, schemavalidators.AICodingSessionVersion0_1)
}

// jsonAt resolves a slash-separated path against a JSON document and returns
// the value's canonical encoding, so two documents can be compared at that path.
// Slashes rather than dots because object keys here contain dots
// ("chainloop.material.evidence.id").
func jsonAt(t *testing.T, doc []byte, path string) string {
	t.Helper()

	var node any
	require.NoError(t, json.Unmarshal(doc, &node))

	for seg := range strings.SplitSeq(path, "/") {
		switch v := node.(type) {
		case map[string]any:
			child, ok := v[seg]
			require.True(t, ok, "missing key %q in path %q", seg, path)
			node = child
		case []any:
			idx, err := strconv.Atoi(seg)
			require.NoError(t, err, "bad index %q in path %q", seg, path)
			require.Less(t, idx, len(v), "index out of range in path %q", path)
			node = v[idx]
		default:
			require.Failf(t, "cannot descend", "into %T at %q in path %q", node, seg, path)
		}
	}

	out, err := json.Marshal(node)
	require.NoError(t, err)
	return string(out)
}
