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

package pi

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/chainloop-dev/chainloop/app/cli/internal/trace"
	"github.com/chainloop-dev/chainloop/app/cli/internal/trace/state"
	"github.com/chainloop-dev/chainloop/internal/schemavalidators"
	"github.com/chainloop-dev/chainloop/pkg/attestation/crafter/materials/aicodingsession"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	testSessionID = "pi-session-1"
	testRootID    = "root0001"
)

func TestParseSessionEvidence(t *testing.T) {
	rawDir := t.TempDir()
	fixture, err := os.ReadFile(filepath.Join("testdata", "session-v3.jsonl"))
	require.NoError(t, err)
	writeRawSession(t, rawDir, fixture)

	result, err := New().ParseSession(context.Background(), &trace.ParseOpts{
		SessionDir:   rawDir,
		SessionID:    testSessionID,
		AgentVersion: "0.80.10",
	})
	require.NoError(t, err)

	assert.Equal(t, "pi", result.Data.Agent.Name)
	assert.Equal(t, "0.80.10", result.Data.Agent.Version)
	assert.Equal(t, testSessionID, result.Data.Session.ID)
	assert.Equal(t, "2026-01-02T03:04:00Z", result.Data.Session.StartedAt)
	assert.Equal(t, "2026-01-02T03:04:18Z", result.Data.Session.EndedAt)
	assert.Equal(t, 18, result.Data.Session.DurationSeconds)

	require.NotNil(t, result.Data.Model)
	assert.Equal(t, "gemini-2.5-pro", result.Data.Model.Primary)
	assert.Equal(t, "google", result.Data.Model.Provider)
	assert.Equal(t, []string{"claude-sonnet-4", "gemini-2.5-pro", "gpt-5"}, result.Data.Model.ModelsUsed)

	require.NotNil(t, result.Data.Usage)
	assert.Equal(t, 300, result.Data.Usage.InputTokens)
	assert.Equal(t, 60, result.Data.Usage.OutputTokens)
	assert.Equal(t, 395, result.Data.Usage.TotalTokens)
	assert.Equal(t, 30, result.Data.Usage.CacheReadInputTokens)
	assert.Equal(t, 7, result.Data.Usage.CacheCreationInputTokens)
	assert.InDelta(t, 0.3, result.Data.Usage.EstimatedCostUSD, 0.0000001)

	require.NotNil(t, result.Data.ToolsUsed)
	assert.Equal(t, []aicodingsession.ToolSummary{
		{ToolName: "bash", InvocationCount: 1},
		{ToolName: "edit", InvocationCount: 2},
		{ToolName: "write", InvocationCount: 1},
	}, result.Data.ToolsUsed.Summary)
	assert.Equal(t, 4, result.Data.ToolsUsed.TotalInvocations)

	require.NotNil(t, result.Data.Conversation)
	assert.Equal(t, 7, result.Data.Conversation.TotalMessages)
	assert.Equal(t, 3, result.Data.Conversation.UserMessages)
	assert.Equal(t, 4, result.Data.Conversation.AssistantMessages)

	ids := rawEntryIDs(t, result.Data.RawSession["main"])
	assert.Equal(t, []string{
		"u0000001", "a0000001", "b0000001", "u0000003", "a0000003", "c0000001",
		"cm000002", "u0000004", "a0000004", "q0000001", "a0000005",
	}, ids)
	assert.NotContains(t, ids, "m0000001")
	assert.NotContains(t, ids, "m0000002")
	assert.NotContains(t, ids, "x0000001")
	assert.NotContains(t, ids, "z0000001")

	assert.Equal(t, []string{
		"Pi entry on line 17 has invalid usage.input; ignored",
		"Pi entry on line 17 has invalid usage.output; ignored",
		"Pi entry on line 17 has invalid usage.cacheRead; ignored",
		"Pi entry on line 17 has invalid usage.totalTokens; ignored",
		"Pi entry on line 17 has invalid usage.cost.total; ignored",
	}, result.Data.Warnings)

	require.NoError(t, validateEvidenceData(result.Data))

	actual, err := json.MarshalIndent(result, "", "  ")
	require.NoError(t, err)
	goldenPath := filepath.Join("testdata", "session-v3.golden.json")
	if os.Getenv("UPDATE_GOLDEN") == "1" {
		require.NoError(t, os.WriteFile(goldenPath, append(actual, '\n'), 0o600))
	}
	want, err := os.ReadFile(goldenPath)
	require.NoError(t, err)
	assert.JSONEq(t, string(want), string(actual))

	second, err := json.Marshal(result)
	require.NoError(t, err)
	third, err := json.Marshal(result)
	require.NoError(t, err)
	assert.Equal(t, second, third, "evidence ordering must be deterministic")
}

func TestParseSessionRejectsInvalidStructure(t *testing.T) {
	header := `{"type":"session","version":3,"id":"pi-session-1","timestamp":"2026-01-02T03:04:00Z","cwd":"/work/repo"}`
	root := `{"type":"message","id":"root0001","parentId":null,"timestamp":"2026-01-02T03:04:01Z","message":{"role":"user"}}`

	cases := []struct {
		name    string
		content string
		want    string
	}{
		{name: "missing header", content: "", want: "missing header"},
		{name: "invalid header", content: `{}`, want: "invalid v3 header"},
		{name: "unsupported version", content: `{"type":"session","version":2,"id":"pi-session-1","timestamp":"2026-01-02T03:04:00Z","cwd":"/work/repo"}`, want: "invalid v3 header"},
		{name: "mismatched session", content: `{"type":"session","version":3,"id":"other","timestamp":"2026-01-02T03:04:00Z","cwd":"/work/repo"}`, want: "session ID mismatch"},
		{name: "duplicate ID", content: header + "\n" + root + "\n" + `{"type":"label","id":"root0001","parentId":"root0001","timestamp":"2026-01-02T03:04:02Z"}` + "\n", want: "duplicate entry ID"},
		{name: "selected orphan", content: header + "\n" + root + "\n" + `{"type":"label","id":"leaf0001","parentId":"missing1","timestamp":"2026-01-02T03:04:02Z"}` + "\n", want: "missing parent"},
		{name: "cycle", content: header + "\n" + `{"type":"label","id":"cycle001","parentId":"cycle002","timestamp":"2026-01-02T03:04:01Z"}` + "\n" + `{"type":"label","id":"cycle002","parentId":"cycle001","timestamp":"2026-01-02T03:04:02Z"}` + "\n", want: "cycle"},
		{name: "malformed interior", content: header + "\n{" + "\n" + root + "\n", want: "invalid JSON on line 2"},
		{name: "malformed final with newline", content: header + "\n{\n", want: "invalid JSON on line 2"},
		{name: "missing entry timestamp", content: header + "\n" + `{"type":"label","id":"leaf0001","parentId":null}` + "\n", want: "missing entry timestamp"},
		{name: "invalid entry timestamp", content: header + "\n" + `{"type":"label","id":"leaf0001","parentId":null,"timestamp":"not-a-time"}` + "\n", want: "invalid entry timestamp"},
		{name: "structurally invalid final fragment", content: header + "\n" + root + "\n" + `{"type":"label"}`, want: "missing entry ID"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rawDir := t.TempDir()
			writeRawSession(t, rawDir, []byte(tc.content))
			_, err := New().ParseSession(context.Background(), &trace.ParseOpts{SessionDir: rawDir, SessionID: testSessionID})
			require.Error(t, err)
			assert.ErrorContains(t, err, tc.want)
			assert.NotContains(t, err.Error(), rawDir, "errors must not expose transcript paths")
		})
	}
}

func TestParseSessionAllowsAbandonedOrphan(t *testing.T) {
	content := `{"type":"session","version":3,"id":"pi-session-1","timestamp":"2026-01-02T03:04:00Z","cwd":"/work/repo"}` + "\n" +
		`{"type":"future","id":"orphan01","parentId":"missing1","timestamp":"2026-01-02T03:04:01Z"}` + "\n" +
		`{"type":"message","id":"root0001","parentId":null,"timestamp":"2026-01-02T03:04:02Z","message":{"role":"user"}}` + "\n"

	result, err := parseBytes(t, []byte(content))
	require.NoError(t, err)
	assert.Equal(t, []string{testRootID}, rawEntryIDs(t, result.Data.RawSession["main"]))
}

func TestParseSessionFinalLineHandling(t *testing.T) {
	header := `{"type":"session","version":3,"id":"pi-session-1","timestamp":"2026-01-02T03:04:00Z","cwd":"/work/repo"}`
	entry := `{"type":"message","id":"root0001","parentId":null,"timestamp":"2026-01-02T03:04:01Z","message":{"role":"user"}}`

	t.Run("valid unterminated line", func(t *testing.T) {
		result, err := parseBytes(t, []byte(header+"\n"+entry))
		require.NoError(t, err)
		assert.Empty(t, result.Data.Warnings)
		assert.Equal(t, []string{testRootID}, rawEntryIDs(t, result.Data.RawSession["main"]))
	})

	t.Run("truncated final line", func(t *testing.T) {
		result, err := parseBytes(t, []byte(header+"\n"+entry+"\n"+`{"type":"message"`))
		require.NoError(t, err)
		assert.Equal(t, []string{"ignored truncated final Pi session line 3"}, result.Data.Warnings)
		assert.Equal(t, []string{testRootID}, rawEntryIDs(t, result.Data.RawSession["main"]))
	})
}

func TestParseSessionNonFiniteUsage(t *testing.T) {
	content := `{"type":"session","version":3,"id":"pi-session-1","timestamp":"2026-01-02T03:04:00Z","cwd":"/work/repo"}
{"type":"message","id":"first001","parentId":null,"timestamp":"2026-01-02T03:04:01Z","message":{"role":"assistant","usage":{"input":1e400,"cost":{"total":1e400}}}}
`

	result, err := parseBytes(t, []byte(content))
	require.NoError(t, err)
	assert.Zero(t, result.Data.Usage.InputTokens)
	assert.Zero(t, result.Data.Usage.EstimatedCostUSD)
	assert.Equal(t, []string{
		"Pi entry on line 2 has invalid usage.input; ignored",
		"Pi entry on line 2 has invalid usage.cost.total; ignored",
	}, result.Data.Warnings)
	assert.NoError(t, validateEvidenceData(result.Data), "ignored usage must not make the evidence uncraftable")
}

func TestParseSessionUsageOverflow(t *testing.T) {
	maxInt := int(^uint(0) >> 1)
	content := fmt.Sprintf(`{"type":"session","version":3,"id":"pi-session-1","timestamp":"2026-01-02T03:04:00Z","cwd":"/work/repo"}
{"type":"message","id":"first001","parentId":null,"timestamp":"2026-01-02T03:04:01Z","message":{"role":"assistant","usage":{"input":%d,"output":1e3,"cost":{"total":1.7e308}}}}
{"type":"message","id":"second01","parentId":"first001","timestamp":"2026-01-02T03:04:02Z","message":{"role":"assistant","usage":{"input":1,"cost":{"total":1.7e308}}}}
`, maxInt)

	result, err := parseBytes(t, []byte(content))
	require.NoError(t, err)
	assert.Equal(t, maxInt, result.Data.Usage.InputTokens)
	assert.Equal(t, 1000, result.Data.Usage.OutputTokens)
	assert.InDelta(t, 1.7e308, result.Data.Usage.EstimatedCostUSD, 1e292)
	assert.Equal(t, []string{
		"Pi entry on line 3 has invalid usage.input; ignored",
		"Pi entry on line 3 has invalid usage.cost.total; ignored",
	}, result.Data.Warnings)
}

func parseBytes(t *testing.T, content []byte) (*aicodingsession.Evidence, error) {
	t.Helper()
	rawDir := t.TempDir()
	writeRawSession(t, rawDir, content)
	return New().ParseSession(context.Background(), &trace.ParseOpts{SessionDir: rawDir, SessionID: testSessionID})
}

func writeRawSession(t *testing.T, rawDir string, content []byte) {
	t.Helper()
	require.NoError(t, os.MkdirAll(rawDir, 0o755))
	//nolint:gosec // rawDir is always t.TempDir and the session ID is fixed.
	require.NoError(t, os.WriteFile(state.RawSessionPath(rawDir, testSessionID), content, 0o600))
}

func rawEntryIDs(t *testing.T, entries []json.RawMessage) []string {
	t.Helper()
	ids := make([]string, 0, len(entries))
	for _, entry := range entries {
		var envelope struct {
			ID string `json:"id"`
		}
		require.NoError(t, json.Unmarshal(entry, &envelope))
		ids = append(ids, envelope.ID)
	}
	return ids
}

func validateEvidenceData(data aicodingsession.Data) error {
	raw, err := json.Marshal(data)
	if err != nil {
		return err
	}
	var value any
	if err := json.Unmarshal(raw, &value); err != nil {
		return err
	}
	return schemavalidators.ValidateAICodingSession(value, schemavalidators.AICodingSessionVersion0_1)
}
