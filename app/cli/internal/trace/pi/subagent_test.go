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
	"encoding/json"
	"fmt"
	"testing"

	"github.com/chainloop-dev/chainloop/pkg/attestation/crafter/materials/aicodingsession"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Shape verified against the official example, not arbitrary Pi extensions:
// https://github.com/earendil-works/pi/blob/71023aec1546d6d677a65669c5665c0927ce0dd0/packages/coding-agent/examples/extensions/subagent/index.ts
const childResult = `{"agent":"worker","agentSource":"project","task":"Read README.md","exitCode":0,` +
	`"usage":{"input":3,"output":5,"cacheRead":11,"cacheWrite":19,"cost":0.025,"contextTokens":38,"turns":2},` +
	`"messages":[` +
	`{"role":"assistant","model":"child-model","provider":"child-provider","timestamp":1767323042000,"content":[{"type":"toolCall","id":"child-call","name":"read","arguments":{"path":"README.md"}}],"usage":{"input":1,"output":2,"cacheRead":4,"cost":{"total":0.01}}},` +
	`{"role":"toolResult","toolCallId":"child-call","toolName":"read","content":[{"type":"text","text":"full file content"}]},` +
	`{"role":"assistant","content":[{"type":"text","text":"child answer"}],"usage":{"input":2,"output":3,"cacheRead":7,"cacheWrite":19,"cost":{"total":0.015}}}` +
	`]}`

func subagentSession(details string) []byte {
	return []byte(`{"type":"session","version":3,"id":"pi-session-1","timestamp":"2026-01-02T03:04:00Z","cwd":"/work/repo","parentSession":"not-a-child"}
{"type":"message","id":"root0001","parentId":null,"timestamp":"2026-01-02T03:04:01Z","message":{"role":"assistant","model":"root-model","provider":"root-provider","content":[{"type":"toolCall","id":"call-1","name":"subagent","arguments":{"agent":"worker"}}],"usage":{"input":7,"output":9,"cacheRead":13,"cacheWrite":17,"cost":{"total":0.1}}}}
{"type":"message","id":"abandoned","parentId":"root0001","timestamp":"2026-01-02T03:04:02Z","message":{"role":"toolResult","toolName":"subagent","details":{"mode":"single","results":[{"agent":"abandoned","usage":{"input":999}}]}}}
{"type":"message","id":"result01","parentId":"root0001","timestamp":"2026-01-02T03:04:03Z","message":{"role":"toolResult","toolCallId":"call-1","toolName":"subagent","details":` + details + `}}
{"type":"message","id":"final001","parentId":"result01","timestamp":"2026-01-02T03:04:04Z","message":{"role":"assistant","model":"root-final","provider":"root-provider","content":[{"type":"text","text":"done"}]}}
`)
}

func TestPiSubagentEvidenceWhenSupportedResult(t *testing.T) {
	for _, mode := range []string{"single", "parallel", "chain"} {
		t.Run(mode, func(t *testing.T) {
			// Given: aggregate counters duplicate usage in the child messages.
			results := childResult
			count := 1
			cost := 0.125
			if mode != "single" {
				results += "," + childResult
				count = 2
				cost = 0.15
			}
			session := subagentSession(`{"mode":"` + mode + `","results":[` + results + `]}`)

			// When: parse through the provider's copied-session entrypoint.
			got, err := parseBytes(t, session)

			// Then: children are distinct, and each aggregate is counted once.
			require.NoError(t, err)
			require.Len(t, got.Data.Subagents, count)
			assert.Equal(t, &aicodingsession.Usage{
				InputTokens: 7 + 3*count, OutputTokens: 9 + 5*count, TotalTokens: 16 + 8*count,
				CacheReadInputTokens: 13 + 11*count, CacheCreationInputTokens: 17 + 19*count,
				EstimatedCostUSD: cost,
			}, got.Data.Usage)
			assert.Equal(t, "v1", got.Data.SchemaVersion)
			assert.Equal(t, "root-final", got.Data.Model.Primary)
			assert.Equal(t, []string{"child-model", "root-final", "root-model"}, got.Data.Model.ModelsUsed)
			assert.Equal(t, 2, got.Data.Conversation.TotalMessages, "root conversation summary remains root-only")
			assert.Equal(t, []string{"root0001", "result01", "final001"}, rawEntryIDs(t, got.Data.RawSession["main"]))
			for i, sub := range got.Data.Subagents {
				assert.Equal(t, aicodingsession.Subagent{
					ID: fmt.Sprintf("subagent-result01-%d", i+1), Type: "worker", Description: "Read README.md",
					Tokens: aicodingsession.SubagentTokens{Input: 3, Output: 5},
				}, sub)
				stream := got.Data.RawSession[sub.ID]
				require.Len(t, stream, 3)
				var wrapped struct {
					Type      string          `json:"type"`
					ID        string          `json:"id"`
					ParentID  string          `json:"parentId"`
					Timestamp string          `json:"timestamp"`
					Message   json.RawMessage `json:"message"`
				}
				require.NoError(t, json.Unmarshal(stream[1], &wrapped))
				assert.Equal(t, "message", wrapped.Type)
				assert.Equal(t, sub.ID+"-2", wrapped.ID)
				assert.Equal(t, sub.ID+"-1", wrapped.ParentID)
				assert.Equal(t, "2026-01-02T03:04:03Z", wrapped.Timestamp)
				assert.JSONEq(t, `{"role":"toolResult","toolCallId":"child-call","toolName":"read","content":[{"type":"text","text":"full file content"}]}`, string(wrapped.Message))
				assert.Contains(t, string(stream[2]), "child answer")
			}
			assert.NoError(t, validateEvidenceData(got.Data))
			again, err := parseBytes(t, session)
			require.NoError(t, err)
			assert.Equal(t, got, again)
		})
	}
}

func TestPiSubagentEvidenceWhenOptionalDetailsMalformed(t *testing.T) {
	for _, tc := range []struct {
		name     string
		details  string
		children int
		input    int
	}{
		{name: "cancellation", details: `{}`},
		{name: "wrong type", details: `"extension-specific"`},
		{name: "unknown mode", details: `{"mode":"other","results":[` + childResult + `]}`},
		{name: "malformed child", details: `{"mode":"single","results":[{"agent":123}]}`},
		{name: "malformed sibling", details: `{"mode":"parallel","results":[{"agent":123},` + childResult + `]}`, children: 1, input: 3},
		{name: "failed child", details: `{"mode":"chain","results":[{"agent":"worker","exitCode":1,"messages":[],"usage":{"input":2,"output":3,"cacheRead":5,"cost":0.01}}]}`, children: 1, input: 2},
		{name: "malformed optional usage", details: `{"mode":"single","results":[{"agent":"worker","messages":[{"role":"assistant","content":[]}],"usage":"bad"}]}`, children: 1},
		{name: "malformed optional messages", details: `{"mode":"single","results":[{"agent":"worker","messages":"bad","usage":{"input":2}}]}`, children: 1, input: 2},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// Given / When
			got, err := parseBytes(t, subagentSession(tc.details))
			// Then: optional extension failures never discard the root evidence.
			require.NoError(t, err)
			assert.Len(t, got.Data.Subagents, tc.children)
			if tc.name == "malformed sibling" {
				require.Len(t, got.Data.Subagents, 1)
				assert.Equal(t, "subagent-result01-2", got.Data.Subagents[0].ID)
			}
			assert.Equal(t, 7+tc.input, got.Data.Usage.InputTokens)
			assert.NoError(t, validateEvidenceData(got.Data))
		})
	}
}

func TestPiSubagentEvidenceWhenUnrelatedExtension(t *testing.T) {
	// Given: matching details on an unrelated tool and a parent-session header.
	session := subagentSession(`{}`)
	session = append(session, []byte(`{"type":"message","id":"unrelated","parentId":"final001","timestamp":"2026-01-02T03:04:05Z","message":{"role":"toolResult","toolName":"read","details":{"mode":"single","results":[`+childResult+`]}}}
`)...)
	// When
	got, err := parseBytes(t, session)
	// Then
	require.NoError(t, err)
	assert.Empty(t, got.Data.Subagents)
	assert.Len(t, got.Data.RawSession, 1)
	assert.Equal(t, 7, got.Data.Usage.InputTokens)
}

func TestPiSubagentUsageWhenFailedChildHasInvalidCounters(t *testing.T) {
	// Given: a failed child with a preserved response, malformed counters,
	// and a contextTokens value that must never become the evidence total.
	session := subagentSession(`{"mode":"chain","results":[{"agent":"worker","task":"attempt","exitCode":1,"stopReason":"error",` +
		`"usage":{"input":-1,"output":3,"cacheRead":"bad","cacheWrite":5,"contextTokens":999,"cost":1e400},` +
		`"messages":[{"role":"assistant","stopReason":"error","errorMessage":"provider failed","content":[{"type":"text","text":"partial answer"}],"usage":{"input":1e400}}]}]}`)
	// When
	got, err := parseBytes(t, session)
	// Then: only valid aggregate counters contribute; evidence is craftable.
	require.NoError(t, err)
	require.Len(t, got.Data.Subagents, 1)
	assert.Equal(t, aicodingsession.SubagentTokens{Output: 3}, got.Data.Subagents[0].Tokens)
	assert.Equal(t, &aicodingsession.Usage{
		InputTokens: 7, OutputTokens: 12, TotalTokens: 19,
		CacheReadInputTokens: 13, CacheCreationInputTokens: 22, EstimatedCostUSD: 0.1,
	}, got.Data.Usage)
	require.Len(t, got.Data.RawSession[got.Data.Subagents[0].ID], 1)
	assert.Contains(t, string(got.Data.RawSession[got.Data.Subagents[0].ID][0]), "partial answer")
	assert.NotEmpty(t, got.Data.Warnings)
	assert.NoError(t, validateEvidenceData(got.Data))
}
