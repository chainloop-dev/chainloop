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

package schemavalidators_test

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/chainloop-dev/chainloop/internal/schemavalidators"
	"github.com/stretchr/testify/require"
)

func TestTranscriptContract(t *testing.T) {
	for _, tc := range []struct {
		name, transcript string
		messages         int
		wantErr          bool
	}{
		{name: "Claude text", transcript: `{"main":[{"type":"user","message":{"content":"Hello"}}]}`, messages: 1},
		{name: "Cursor role", transcript: `{"main":[{"role":"assistant","message":{"content":[{"type":"text","text":"Hello"}]}}]}`, messages: 1},
		{name: "assistant API error", transcript: `{"main":[{"type":"assistant","isApiErrorMessage":true,"error":"request failed"}]}`, messages: 1},
		{name: "normalized Codex with original event", transcript: `{"main":[{"type":"assistant","message":{"content":[{"type":"tool_use","id":"c1","name":"Bash","input":{"command":"pwd"}}]},"source_event":{"type":"response_item","payload":{"type":"function_call"}}},{"type":"user","message":{"content":[{"type":"tool_result","tool_use_id":"c1","content":"/repo"}]}}]}`, messages: 1},
		{name: "unknown metadata alongside messages", transcript: `{"main":[{"type":"future_event","value":1},{"type":"user","message":{"content":"Hello"}}]}`, messages: 1},
		{name: "metadata without conversation", transcript: `{"main":[{"type":"session_meta","payload":{}}]}`},
		{name: "empty transcript without conversation", transcript: `{"main":[]}`},
		{name: "native conversation without normalized message", transcript: `{"main":[{"type":"response_item","payload":{"type":"message","role":"user","content":[{"type":"input_text","text":"Hello"}]}}]}`, messages: 1, wantErr: true},
		{name: "native conversation without summary", transcript: `{"main":[{"type":"response_item","payload":{"type":"message","role":"user","content":[{"type":"input_text","text":"Hello"}]}}]}`, wantErr: true},
		{name: "missing main", transcript: `{}`, wantErr: true},
		{name: "non-object entry", transcript: `{"main":["Hello"]}`, wantErr: true},
		{name: "message without content", transcript: `{"main":[{"type":"user","message":{}}]}`, messages: 1, wantErr: true},
		{name: "native role buried in message", transcript: `{"main":[{"type":"message","message":{"role":"user","content":"Hello"}}]}`, wantErr: true},
		{name: "content object", transcript: `{"main":[{"type":"assistant","message":{"content":{"text":"Hello"}}}]}`, messages: 1, wantErr: true},
		{name: "block without type", transcript: `{"main":[{"type":"user","message":{"content":[{"text":"Hello"}]}}]}`, messages: 1, wantErr: true},
		{name: "text block without text", transcript: `{"main":[{"type":"user","message":{"content":[{"type":"text"}]}}]}`, messages: 1, wantErr: true},
		{name: "call without id", transcript: `{"main":[{"type":"assistant","message":{"content":[{"type":"tool_use","name":"Bash"}]}}]}`, messages: 1, wantErr: true},
		{name: "result without correlation id", transcript: `{"main":[{"type":"user","message":{"content":[{"type":"tool_result","content":"ok"}]}}]}`, messages: 1, wantErr: true},
		{name: "native text in tool output", transcript: `{"main":[{"type":"user","message":{"content":[{"type":"tool_result","tool_use_id":"c1","content":[{"type":"input_text","text":"ok"}]}]}}]}`, messages: 1, wantErr: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			fixture, err := os.ReadFile("testdata/ai_coding_session_valid.json")
			require.NoError(t, err)
			var data map[string]any
			require.NoError(t, json.Unmarshal(fixture, &data))
			var transcript any
			require.NoError(t, json.Unmarshal([]byte(tc.transcript), &transcript))
			data["raw_session"] = transcript
			data["conversation"] = map[string]any{"total_messages": tc.messages}
			err = schemavalidators.ValidateAICodingSession(data, "")
			if tc.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}
