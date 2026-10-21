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
	"encoding/json"
	"testing"

	"github.com/chainloop-dev/chainloop/pkg/attestation/crafter/materials/aicodingsession"
	"github.com/stretchr/testify/require"
)

func TestTranscriptRequiresRenderableMessages(t *testing.T) {
	const prompt = `{"type":"user","message":{"content":[{"type":"text","text":"Create greeting.txt"}]}}`
	const answer = `{"type":"assistant","message":{"content":[{"type":"text","text":"Created greeting.txt"}]}}`
	const call = `{"type":"assistant","message":{"content":[{"type":"tool_use","id":"call-1","name":"apply_patch","input":{"input":"Create greeting.txt"}}]}}`
	const result = `{"type":"user","message":{"content":[{"type":"tool_result","tool_use_id":"call-1","content":"Success"}]}}`
	for _, tc := range []struct {
		name    string
		entries []string
		tools   int
		wantErr bool
	}{
		{name: "messages", entries: []string{prompt, answer}},
		{name: "completed tool exchange", entries: []string{prompt, call, result}, tools: 1},
		{name: "native messages alone", entries: []string{`{"type":"response_item","payload":{"type":"message","role":"user","content":[{"type":"input_text","text":"Create greeting.txt"}]}}`}, wantErr: true},
		{name: "filename only in metadata", entries: []string{`{"type":"user","message":{"content":"Hello"},"source_event":{"file":"greeting.txt"}}`, `{"type":"assistant","message":{"content":"Hello"}}`}, wantErr: true},
		{name: "no assistant", entries: []string{prompt}, wantErr: true},
		{name: "missing tool result", entries: []string{prompt, call}, tools: 1, wantErr: true},
		{name: "empty tool result", entries: []string{prompt, call, `{"type":"user","message":{"content":[{"type":"tool_result","tool_use_id":"call-1","content":""}]}}`}, tools: 1, wantErr: true},
		{name: "unrelated tool result", entries: []string{prompt, call, `{"type":"user","message":{"content":[{"type":"tool_result","tool_use_id":"other","content":"Success"}]}}`}, tools: 1, wantErr: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var entries []json.RawMessage
			for _, entry := range tc.entries {
				entries = append(entries, json.RawMessage(entry))
			}
			d := aicodingsession.Data{
				SchemaVersion: "v1",
				Agent:         aicodingsession.Agent{Name: "codex"},
				Session:       aicodingsession.Session{ID: "test", StartedAt: "2026-10-21T10:00:00Z"},
				Conversation:  &aicodingsession.Conversation{TotalMessages: 2},
				ToolsUsed:     &aicodingsession.ToolsUsed{TotalInvocations: tc.tools},
				RawSession:    map[string][]json.RawMessage{"main": entries},
			}
			s := &Session{Stored: &StoredSession{Evidence: *aicodingsession.NewEvidence(d)}}
			if tc.wantErr {
				require.Error(t, checkTranscript(s))
			} else {
				require.NoError(t, checkTranscript(s))
			}
		})
	}
}
