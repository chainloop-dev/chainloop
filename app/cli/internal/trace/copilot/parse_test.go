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

package copilot

import (
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/chainloop-dev/chainloop/app/cli/internal/trace"
	"github.com/chainloop-dev/chainloop/app/cli/internal/trace/state"
	"github.com/chainloop-dev/chainloop/pkg/attestation/crafter/materials/aicodingsession"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	testSessionID   = "b4257b63-a7e8-4e3f-9bf0-019c98db1744"
	testHookSession = "s-1"
	testModel       = "gpt-5.4"
	toolCreate      = "create"
	toolBash        = "bash"
)

const header = `{"type":"session.start","data":{"sessionId":"` + testSessionID + `","copilotVersion":"1.0.82"},"id":"h","timestamp":"2026-04-07T21:00:00Z"}`

func parseLog(t *testing.T, content string) (*aicodingsession.Data, error) {
	t.Helper()
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(state.RawSessionPath(dir, testSessionID), []byte(content), 0o600)) //nolint:gosec // temporary fixture path and a fixed session ID
	evidence, err := New().ParseSession(t.Context(), &trace.ParseOpts{SessionDir: dir, SessionID: testSessionID})
	if err != nil {
		return nil, err
	}
	return &evidence.Data, nil
}

func TestParseSessionFixture(t *testing.T) {
	content, err := os.ReadFile("testdata/session.jsonl")
	require.NoError(t, err)
	d, err := parseLog(t, string(content))
	require.NoError(t, err)

	assert.Equal(t, Name, d.Agent.Name)
	assert.Equal(t, "1.0.82", d.Agent.Version)
	assert.Equal(t, testSessionID, d.Session.ID)
	assert.Equal(t, "2026-04-07T21:07:18Z", d.Session.StartedAt)
	assert.Equal(t, "2026-04-07T21:08:42Z", d.Session.EndedAt)
	assert.Equal(t, 83, d.Session.DurationSeconds)

	assert.Equal(t, "claude-sonnet-4.6", d.Model.Primary)
	assert.Equal(t, modelProvider, d.Model.Provider)
	assert.Equal(t, []string{"claude-sonnet-4.6"}, d.Model.ModelsUsed)

	require.NotNil(t, d.Usage)
	assert.Equal(t, 21106, d.Usage.InputTokens)
	assert.Equal(t, 311, d.Usage.OutputTokens)
	assert.Equal(t, 21417, d.Usage.TotalTokens)
	assert.Equal(t, 41648, d.Usage.CacheReadInputTokens)
	assert.Equal(t, 0, d.Usage.CacheCreationInputTokens)
	assert.InDelta(t, 0.0805, d.Usage.EstimatedCostUSD, 1e-9)

	assert.Equal(t, 3, d.ToolsUsed.TotalInvocations)
	assert.Equal(t, []aicodingsession.ToolSummary{
		{ToolName: toolBash, InvocationCount: 1},
		{ToolName: toolCreate, InvocationCount: 1},
		{ToolName: "report_intent", InvocationCount: 1},
	}, d.ToolsUsed.Summary)

	assert.Equal(t, 1, d.Conversation.UserMessages)
	assert.Equal(t, 3, d.Conversation.AssistantMessages)
	assert.Equal(t, 4, d.Conversation.TotalMessages)
	assert.Empty(t, d.Warnings)

	lines := strings.Split(strings.TrimSpace(string(content)), "\n")
	require.Len(t, d.RawSession["main"], len(lines))
	for _, line := range d.RawSession["main"] {
		var obj map[string]any
		require.NoError(t, json.Unmarshal(line, &obj), "each raw line is a JSON object")
	}

	// A message becomes the shared line shape, with the native event kept.
	var user trace.RawSessionEntry
	require.NoError(t, json.Unmarshal(d.RawSession["main"][3], &user))
	assert.Equal(t, trace.RoleUser, user.Type)
	assert.JSONEq(t, `[{"type":"text","text":"Create a test dir with a README"}]`, string(user.Message.Content))
	assert.NotEmpty(t, user.SourceEvent)

	var assistant trace.RawSessionEntry
	require.NoError(t, json.Unmarshal(d.RawSession["main"][8], &assistant))
	assert.Equal(t, trace.RoleAssistant, assistant.Type)
	assert.Contains(t, string(assistant.Message.Content), `"type":"tool_use"`)
	assert.Contains(t, string(assistant.Message.Content), `"name":"create"`)

	// An event type that the parser does not read stays as Copilot wrote it.
	assert.JSONEq(t, lines[1], string(d.RawSession["main"][1]))
}

func TestParseSessionUsageSources(t *testing.T) {
	cases := []struct {
		name        string
		events      string
		wantInput   int
		wantOutput  int
		wantCache   int
		wantWarning string
		wantNoUsage bool
		wantPrimary string
	}{
		{
			name: "shutdown totals plus records after the last shutdown",
			events: `{"type":"session.usage_record","data":{"usage":{"model":"gpt-5.4","inputTokens":1000,"outputTokens":50,"cacheReadTokens":800,"apiCallId":"c1"}}}
{"type":"session.shutdown","data":{"modelMetrics":{"gpt-5.4":{"requests":{"count":1},"usage":{"inputTokens":1000,"outputTokens":50,"cacheReadTokens":800,"cacheWriteTokens":0}}},"currentModel":"gpt-5.4"}}
{"type":"session.usage_record","data":{"usage":{"model":"gpt-5.4","inputTokens":100,"outputTokens":5,"cacheReadTokens":0,"apiCallId":"c2"}}}`,
			wantInput: 300, wantOutput: 55, wantCache: 800, wantPrimary: testModel,
		},
		{
			name: "two shutdowns of a resumed session add up",
			events: `{"type":"session.shutdown","data":{"modelMetrics":{"gpt-5.4":{"usage":{"inputTokens":100,"outputTokens":10}}}}}
{"type":"session.shutdown","data":{"modelMetrics":{"gpt-5.4":{"usage":{"inputTokens":200,"outputTokens":20}}}}}`,
			wantInput: 300, wantOutput: 30,
		},
		{
			name: "a subagent shutdown is not counted",
			events: `{"type":"session.shutdown","data":{"modelMetrics":{"gpt-5.4":{"usage":{"inputTokens":100,"outputTokens":10}}}}}
{"type":"session.shutdown","agentId":"sub-1","data":{"modelMetrics":{"gpt-5.4":{"usage":{"inputTokens":999,"outputTokens":999}}}}}`,
			wantInput: 100, wantOutput: 10,
		},
		{
			name: "records only, a rewritten record counts once",
			events: `{"type":"session.usage_record","data":{"usage":{"model":"gpt-5.4","inputTokens":500,"outputTokens":20,"cacheReadTokens":400,"apiCallId":"c3"}}}
{"type":"session.usage_record","data":{"usage":{"model":"gpt-5.4","inputTokens":500,"outputTokens":25,"cacheReadTokens":400,"apiCallId":"c3"}}}
{"type":"session.usage_record","agentId":"sub-1","data":{"usage":{"model":"gpt-5.4","inputTokens":300,"outputTokens":30,"apiCallId":"c4"}}}`,
			wantInput: 400, wantOutput: 55, wantCache: 400,
			wantWarning: "token usage comes from the records of each call",
		},
		{
			name: "message output tokens only",
			events: `{"type":"session.model_change","data":{"newModel":"gpt-5.4"}}
{"type":"assistant.message","data":{"content":"hi","outputTokens":7}}`,
			wantOutput: 7, wantWarning: "only the output tokens", wantPrimary: testModel,
		},
		{
			name: "model from each message",
			events: `{"type":"assistant.message","data":{"model":"gpt-6.1-sol","content":"a","outputTokens":3}}
{"type":"assistant.message","data":{"model":"gpt-6.1-sol","content":"b","outputTokens":4}}`,
			wantOutput: 7, wantWarning: "only the output tokens", wantPrimary: "gpt-6.1-sol",
		},
		{
			name:        "no usage",
			events:      `{"type":"user.message","data":{"content":"hi"}}`,
			wantNoUsage: true, wantWarning: "no token usage",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			d, err := parseLog(t, header+"\n"+tc.events+"\n")
			require.NoError(t, err)
			if tc.wantNoUsage {
				assert.Nil(t, d.Usage)
			} else {
				require.NotNil(t, d.Usage)
				assert.Equal(t, tc.wantInput, d.Usage.InputTokens)
				assert.Equal(t, tc.wantOutput, d.Usage.OutputTokens)
				assert.Equal(t, tc.wantInput+tc.wantOutput, d.Usage.TotalTokens)
				assert.Equal(t, tc.wantCache, d.Usage.CacheReadInputTokens)
			}
			if tc.wantWarning != "" {
				assert.True(t, containsWarning(d.Warnings, tc.wantWarning), "warnings %v", d.Warnings)
			}
			if tc.wantPrimary != "" {
				assert.Equal(t, tc.wantPrimary, d.Model.Primary)
			}
		})
	}
}

func TestParseSessionErrors(t *testing.T) {
	t.Run("session ID mismatch", func(t *testing.T) {
		_, err := parseLog(t, `{"type":"session.start","data":{"sessionId":"other"}}`+"\n")
		require.Error(t, err)
	})
	t.Run("missing session.start", func(t *testing.T) {
		_, err := parseLog(t, `{"type":"user.message","data":{"content":"hi"}}`+"\n")
		require.Error(t, err)
	})
	t.Run("bad lines give warnings and stay readable", func(t *testing.T) {
		d, err := parseLog(t, header+"\nnot json\n"+`{"type":"user.message","data":{"content":["unexpected"]}}`+"\n"+`{"type":"future.event","data":{"content":["any shape"]}}`+"\n")
		require.NoError(t, err)
		assert.True(t, containsWarning(d.Warnings, "invalid Copilot event on line 2"), "warnings %v", d.Warnings)
		assert.True(t, containsWarning(d.Warnings, "unreadable Copilot user.message event on line 3"), "warnings %v", d.Warnings)
		// The header, the unreadable message, and the future event stay.
		assert.Len(t, d.RawSession["main"], 3)
	})
	t.Run("missing copy", func(t *testing.T) {
		_, err := New().ParseSession(t.Context(), &trace.ParseOpts{SessionDir: t.TempDir(), SessionID: testSessionID})
		require.Error(t, err)
	})
}

func TestToolInput(t *testing.T) {
	cases := []struct {
		name, args, want string
	}{
		{name: "object stays", args: `{"path":"a"}`, want: `{"path":"a"}`},
		{name: "patch string is wrapped", args: `"*** Begin Patch"`, want: `{"input":"*** Begin Patch"}`},
		{name: "null is wrapped", args: `null`, want: `{"input":null}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.JSONEq(t, tc.want, string(toolInput(json.RawMessage(tc.args))))
		})
	}
}

func TestEstimateCost(t *testing.T) {
	u := tokenUsage{Input: 1_000_000, Output: 1_000_000, CacheRead: 1_000_000}
	cases := []struct {
		model       string
		want        float64
		wantWarning bool
	}{
		{model: "claude-sonnet-4.6", want: 3 + 15 + 0.3},
		{model: "anthropic/claude-sonnet-4.6", want: 3 + 15 + 0.3},
		{model: testModel, want: 2.5 + 15 + 0.25},
		{model: "some-new-model", want: 3 + 15 + 0.3, wantWarning: true},
	}
	for _, tc := range cases {
		t.Run(tc.model, func(t *testing.T) {
			var warnings []string
			assert.InDelta(t, tc.want, estimateCost(tc.model, u, &warnings), 1e-9)
			assert.Equal(t, tc.wantWarning, len(warnings) == 1)
		})
	}
}

func containsWarning(warnings []string, part string) bool {
	for _, w := range warnings {
		if strings.Contains(w, part) {
			return true
		}
	}
	return false
}
