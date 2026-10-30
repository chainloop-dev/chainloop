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

package action

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/chainloop-dev/chainloop/app/cli/internal/trace"
	"github.com/chainloop-dev/chainloop/app/cli/internal/trace/pi"
	"github.com/chainloop-dev/chainloop/app/cli/internal/trace/state"
	"github.com/chainloop-dev/chainloop/pkg/attestation/crafter/materials/aicodingsession"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPiSubagentMaterialWhenCopiedTranscript(t *testing.T) {
	// Given: an official-example result with an embedded read exchange.
	const sessionID = "3713ab-session"
	const transcript = `{"type":"session","version":3,"id":"3713ab-session","cwd":"/work/repo","timestamp":"2026-10-30T10:00:00Z"}
{"type":"message","id":"result01","parentId":null,"timestamp":"2026-10-30T10:00:01Z","message":{"role":"toolResult","toolName":"subagent","details":{"mode":"single","results":[{"agent":"worker","task":"read file","exitCode":0,"usage":{"input":3,"output":5,"cacheRead":7,"cacheWrite":11,"cost":0.02},"messages":[{"role":"assistant","content":[{"type":"toolCall","id":"read-1","name":"read","arguments":{"path":"README.md"}}]},{"role":"toolResult","toolCallId":"read-1","toolName":"read","content":[{"type":"text","text":"file content retained"}]}]}]}}}
`
	path := filepath.Join(t.TempDir(), "session.jsonl")
	require.NoError(t, os.WriteFile(path, []byte(transcript), 0o600))
	provider := pi.New()
	store := state.NewOutOfTreeStore(t.TempDir())
	require.NoError(t, provider.CopySessionData(store, trace.SessionLocation{SessionID: sessionID, TranscriptPath: path}))
	parsed, err := provider.ParseSession(t.Context(), &trace.ParseOpts{SessionID: sessionID, SessionDir: store.RawSessionDir()})
	require.NoError(t, err)
	adder := &fakeMaterialAdder{}

	// When: assemble the same session material that push attaches.
	attested, _ := attachSessionEvidence(t.Context(), adder, store, []sessionEvidence{{
		sessionID: sessionID, provider: provider, evidence: parsed,
	}}, zerolog.Nop(), false)

	// Then: the encoded material contains child messages, not just metadata.
	require.Equal(t, []string{sessionID}, attested)
	var got aicodingsession.Evidence
	require.NoError(t, json.Unmarshal([]byte(adder.byName("ai-coding-session-3713ab").content), &got))
	require.Len(t, got.Data.Subagents, 1)
	assert.Equal(t, parsed.Data.Subagents, got.Data.Subagents)
	assert.Equal(t, parsed.Data.Usage, got.Data.Usage)
	stream := got.Data.RawSession[got.Data.Subagents[0].ID]
	require.Len(t, stream, 2)
	var result struct {
		Message struct {
			Role       string `json:"role"`
			ToolCallID string `json:"toolCallId"`
			Content    []struct {
				Text string `json:"text"`
			} `json:"content"`
		} `json:"message"`
	}
	require.NoError(t, json.Unmarshal(stream[1], &result))
	assert.Equal(t, "toolResult", result.Message.Role)
	assert.Equal(t, "read-1", result.Message.ToolCallID)
	require.Len(t, result.Message.Content, 1)
	assert.Equal(t, "file content retained", result.Message.Content[0].Text)
}
