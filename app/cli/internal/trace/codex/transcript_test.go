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

package codex

import (
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/chainloop-dev/chainloop/app/cli/internal/trace"
	"github.com/chainloop-dev/chainloop/app/cli/internal/trace/state"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTranscriptContract(t *testing.T) {
	raw, err := os.ReadFile("testdata/session.jsonl")
	require.NoError(t, err)
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(state.RawSessionPath(dir, testSessionID), raw, 0600)) //nolint:gosec // temporary fixture path and a fixed session ID
	evidence, err := New().ParseSession(t.Context(), &trace.ParseOpts{SessionDir: dir, SessionID: testSessionID})
	require.NoError(t, err)
	lines := strings.Split(strings.TrimSpace(string(raw)), "\n")
	entries := evidence.Data.RawSession["main"]
	require.Len(t, entries, len(lines))
	var texts []string
	calls := map[string]trace.RawSessionToolUseBlock{}
	results := map[string]string{}
	for i, rawEntry := range entries {
		var entry trace.RawSessionEntry
		require.NoError(t, json.Unmarshal(rawEntry, &entry))
		if entry.Type != trace.RoleUser && entry.Type != trace.RoleAssistant {
			assert.JSONEq(t, lines[i], string(rawEntry))
			continue
		}
		assert.NotEmpty(t, entry.UUID)
		assert.Equal(t, entry.Type, entry.Message.Role)
		assert.JSONEq(t, lines[i], string(entry.SourceEvent))
		var blocks []json.RawMessage
		require.NoError(t, json.Unmarshal(entry.Message.Content, &blocks))
		for _, rawBlock := range blocks {
			var block struct{ Type string }
			require.NoError(t, json.Unmarshal(rawBlock, &block))
			switch block.Type {
			case "text":
				var text trace.RawSessionTextBlock
				require.NoError(t, json.Unmarshal(rawBlock, &text))
				texts = append(texts, text.Text)
			case "tool_use":
				var call trace.RawSessionToolUseBlock
				require.NoError(t, json.Unmarshal(rawBlock, &call))
				calls[call.ID] = call
			case "tool_result":
				var result trace.RawSessionToolResultBlock
				require.NoError(t, json.Unmarshal(rawBlock, &result))
				var output string
				require.NoError(t, json.Unmarshal(result.Content, &output))
				results[result.ToolUseID] = output
			}
		}
	}
	assert.Equal(t, []string{"Create hello.txt", "Created hello.txt"}, texts)
	require.Len(t, calls, 2)
	assert.Equal(t, "apply_patch", calls["call-1"].Name)
	assert.Contains(t, string(calls["call-1"].Input), "hello.txt")
	assert.Equal(t, "exec_command", calls["call-2"].Name)
	assert.JSONEq(t, `{"cmd":"cat hello.txt"}`, string(calls["call-2"].Input))
	assert.Equal(t, map[string]string{"call-1": "Success", "call-2": "hello"}, results)
}

func TestTranscriptReasoningAndUnknownEvents(t *testing.T) {
	for _, tc := range []struct {
		name, payload string
		wantType      string
	}{
		{name: "reasoning summary", payload: `{"type":"reasoning","summary":[{"text":"Check the tests"}]}`, wantType: trace.RoleAssistant},
		{name: "encrypted reasoning", payload: `{"type":"reasoning","encrypted_content":"opaque"}`, wantType: responseItem},
		{name: "future event", payload: `{"type":"future_item","arguments":{"new":"shape"}}`, wantType: responseItem},
		{name: "non-JSON arguments", payload: `{"type":"function_call","name":"Bash","call_id":"c1","arguments":"not JSON"}`, wantType: trace.RoleAssistant},
		{name: "array tool output", payload: `{"type":"custom_tool_call_output","call_id":"c1","output":[{"type":"input_text","text":"Success"}]}`, wantType: trace.RoleUser},
	} {
		t.Run(tc.name, func(t *testing.T) {
			raw := json.RawMessage(`{"type":"response_item","timestamp":"2026-10-21T10:00:00Z","payload":` + tc.payload + `}`)
			var event rolloutEntry
			var payload rolloutPayload
			require.NoError(t, json.Unmarshal(raw, &event))
			require.NoError(t, json.Unmarshal(event.Payload, &payload))
			got, err := transcriptEntry(event, payload, raw, testModel, 1)
			require.NoError(t, err)
			var entry trace.RawSessionEntry
			require.NoError(t, json.Unmarshal(got, &entry))
			assert.Equal(t, tc.wantType, entry.Type)
			if tc.wantType == responseItem {
				assert.JSONEq(t, string(raw), string(got))
			} else {
				assert.JSONEq(t, string(raw), string(entry.SourceEvent))
				assert.Contains(t, string(entry.Message.Content), map[string]string{"reasoning summary": "Check the tests", "non-JSON arguments": "not JSON", "array tool output": `"type":"text"`}[tc.name])
			}
		})
	}
}
