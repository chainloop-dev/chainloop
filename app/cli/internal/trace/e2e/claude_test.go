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

package e2e

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseClaudeHookMessages(t *testing.T) {
	testCases := []struct {
		name    string
		raw     string
		want    []HookMessage
		wantErr bool
	}{
		{
			name: "hook response with banner and context",
			raw: `{"type":"system","subtype":"init","session_id":"s1"}
{"type":"system","subtype":"hook_started","hook_event":"SessionStart"}
{"type":"system","subtype":"hook_response","hook_event":"SessionStart","stdout":"{\"systemMessage\":\"hello\",\"hookSpecificOutput\":{\"hookEventName\":\"SessionStart\",\"additionalContext\":\"capture specs\"}}\n"}
{"type":"assistant","message":{"content":[{"type":"text","text":"ok"}]}}
`,
			want: []HookMessage{{Event: "SessionStart", UserMessage: "hello", ModelContext: "capture specs"}},
		},
		{
			name: "hook with empty or plain stdout gives no message",
			raw: `{"type":"system","subtype":"hook_response","hook_event":"PreToolUse","stdout":""}
{"type":"system","subtype":"hook_response","hook_event":"PostToolUse","stdout":"plain text"}
`,
		},
		{
			name:    "line that is not JSON",
			raw:     "not json\n",
			wantErr: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := parseClaudeHookMessages([]byte(tc.raw))
			if tc.wantErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.want, got)
		})
	}
}
