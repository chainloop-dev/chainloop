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
	"os"
	"path/filepath"
	"testing"

	"github.com/chainloop-dev/chainloop/app/cli/internal/trace/pi"
	"github.com/chainloop-dev/chainloop/pkg/attestation/crafter/materials/aicodingsession"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const piCheckChildID = "subagent-result-1"

func piSubagentCheckSession() *Session {
	const assistant = `{"role":"assistant","content":[{"type":"toolCall","id":"read-1","name":"read","arguments":{"path":"README.md"}}],"usage":{"input":3,"output":5,"cacheRead":11,"cacheWrite":19,"cost":{"total":0.025}}}`
	const toolResult = `{"role":"toolResult","toolCallId":"read-1","toolName":"read","content":[{"type":"text","text":"file contents"}]}`
	s := cacheSession(pi.Name,
		`{"type":"message","id":"root","message":{"role":"assistant","usage":{"input":7,"output":9,"cacheRead":13,"cacheWrite":17,"cost":{"total":0.1}}}}`,
		`{"type":"message","id":"result","message":{"role":"toolResult","toolName":"subagent","details":{"mode":"single","results":[{"agent":"worker","task":"read","usage":{"input":3,"output":5,"cacheRead":11,"cacheWrite":19,"cost":0.025},"messages":[`+assistant+`,`+toolResult+`]}]}}}`,
	)
	d := evidence(s)
	d.Usage = &aicodingsession.Usage{
		InputTokens: 10, OutputTokens: 14, TotalTokens: 24,
		CacheReadInputTokens: 24, CacheCreationInputTokens: 36, EstimatedCostUSD: 0.125,
	}
	d.Subagents = []aicodingsession.Subagent{{
		ID: piCheckChildID, Type: "worker", Description: "read",
		Tokens: aicodingsession.SubagentTokens{Input: 3, Output: 5},
	}}
	d.RawSession[piCheckChildID] = []json.RawMessage{
		json.RawMessage(`{"type":"message","message":` + assistant + `}`),
		json.RawMessage(`{"type":"message","message":` + toolResult + `}`),
	}
	return s
}

func TestCheckPiSubagentsWhenStoredEvidenceChanges(t *testing.T) {
	for _, tc := range []struct {
		name   string
		change func(*aicodingsession.Data)
	}{
		{name: "complete child content and exact usage"},
		{name: "missing child conversation", change: func(d *aicodingsession.Data) { delete(d.RawSession, piCheckChildID) }},
		{name: "lost tool result", change: func(d *aicodingsession.Data) {
			d.RawSession[piCheckChildID] = d.RawSession[piCheckChildID][:1]
		}},
		{name: "changed tool content", change: func(d *aicodingsession.Data) {
			d.RawSession[piCheckChildID][1] = json.RawMessage(`{"message":{"role":"toolResult","content":[{"type":"text","text":"truncated"}]}}`)
		}},
		{name: "wrong child identity", change: func(d *aicodingsession.Data) { d.Subagents[0].ID = "wrong" }},
		{name: "lost child tokens", change: func(d *aicodingsession.Data) { d.Subagents[0].Tokens.Input = 0 }},
		{name: "double counted usage", change: func(d *aicodingsession.Data) { d.Usage.InputTokens += 3; d.Usage.TotalTokens += 3 }},
		{name: "lost cache", change: func(d *aicodingsession.Data) { d.Usage.CacheReadInputTokens = 13 }},
		{name: "lost native cost", change: func(d *aicodingsession.Data) { d.Usage.EstimatedCostUSD = 0.1 }},
		{name: "unrelated stream", change: func(d *aicodingsession.Data) { d.RawSession["unrelated"] = nil }},
		{name: "wrong evidence version", change: func(d *aicodingsession.Data) { d.SchemaVersion = "v2" }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// Given
			session := piSubagentCheckSession()
			if tc.change != nil {
				tc.change(evidence(session))
			}
			// When
			err := checkSubagents(session)
			// Then
			if tc.change == nil {
				require.NoError(t, err)
				require.NoError(t, checkCacheTokenAccounting(session))
			} else {
				require.Error(t, err)
			}
		})
	}
}

func TestPiSubagentSourcePinWhenMissingOrModified(t *testing.T) {
	for _, modified := range []bool{false, true} {
		t.Run(map[bool]string{false: "missing", true: "modified"}[modified], func(t *testing.T) {
			// Given
			dir := t.TempDir()
			if modified {
				require.NoError(t, os.WriteFile(filepath.Join(dir, "index.ts"), []byte("different extension"), 0o600))
			}
			// When
			err := verifyPiSubagentSource(dir)
			// Then
			require.Error(t, err)
			if modified {
				assert.ErrorContains(t, err, piSubagentCommit)
			}
		})
	}
}
