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
	"time"

	"github.com/chainloop-dev/chainloop/pkg/attestation/crafter/materials/aicodingsession"
)

// The official example at Pi commit 71023aec1546d6d677a65669c5665c0927ce0dd0
// (v1.3.0), packages/coding-agent/examples/extensions/subagent/index.ts,
// persists completed results in toolResult.details. Children use --no-session;
// neither parentSession nor filesystem discovery identifies these children.
func (a *evidenceAccumulator) addSubagentResults(entry *sessionEntry, raw json.RawMessage) {
	if len(raw) == 0 || isNull(raw) {
		return
	}
	var details struct {
		Mode    string            `json:"mode"`
		Results []json.RawMessage `json:"results"`
	}
	if json.Unmarshal(raw, &details) != nil {
		a.warnings = append(a.warnings, fmt.Sprintf("Pi subagent result on line %d has invalid details; ignored", entry.Line))
		return
	}
	switch details.Mode {
	case "single", "parallel", "chain":
	default:
		return
	}
	for index, rawResult := range details.Results {
		var result struct {
			Agent    string          `json:"agent"`
			Task     string          `json:"task"`
			Messages json.RawMessage `json:"messages"`
			Usage    json.RawMessage `json:"usage"`
		}
		// Decode siblings separately: malformed optional extension data must
		// not discard the root or a valid neighboring child.
		if json.Unmarshal(rawResult, &result) != nil || result.Agent == "" {
			a.warnings = append(a.warnings, fmt.Sprintf("Pi subagent result on line %d has an invalid child at index %d; ignored", entry.Line, index+1))
			continue
		}
		id := fmt.Sprintf("subagent-%s-%d", entry.ID, index+1)
		messages := a.subagentMessages(entry, id, result.Messages)
		if len(messages) > 0 {
			a.childStreams[id] = messages
		} else {
			a.warnings = append(a.warnings, fmt.Sprintf("Pi subagent %s has no conversation", id))
		}

		var usage map[string]json.RawMessage
		if len(result.Usage) > 0 && !isNull(result.Usage) && (json.Unmarshal(result.Usage, &usage) != nil || usage == nil) {
			a.warnings = append(a.warnings, fmt.Sprintf("Pi subagent %s has invalid usage; ignored", id))
		}
		// The example already sums assistant usage, including failed turns.
		// Never add the embedded messages' usage again. contextTokens is only
		// the last context size, not a cumulative evidence total.
		input, _ := nonnegativeInt(usage["input"])
		output, _ := nonnegativeInt(usage["output"])
		a.addInt(entry.Line, "subagent.input", usage["input"], &a.inputTokens)
		a.addInt(entry.Line, "subagent.output", usage["output"], &a.outputTokens)
		a.addInt(entry.Line, "subagent.cacheRead", usage["cacheRead"], &a.cacheRead)
		a.addInt(entry.Line, "subagent.cacheWrite", usage["cacheWrite"], &a.cacheWrite)
		a.addCost(entry.Line, usage["cost"])
		a.subagents = append(a.subagents, aicodingsession.Subagent{
			ID: id, Type: result.Agent, Description: result.Task,
			Tokens: aicodingsession.SubagentTokens{Input: input, Output: output},
		})
	}
}

// Wrap native child messages like root Pi entries without translating away
// content, tool results, or unknown fields. Source timestamps are milliseconds;
// missing timestamps fall back to the containing result's persisted time.
func (a *evidenceAccumulator) subagentMessages(entry *sessionEntry, id string, raw json.RawMessage) []json.RawMessage {
	var children []json.RawMessage
	if len(raw) == 0 || json.Unmarshal(raw, &children) != nil {
		return nil
	}
	var messages []json.RawMessage
	var lastID *string
	for index, child := range children {
		message, ok := decodeMessage(child)
		if !ok {
			a.warnings = append(a.warnings, fmt.Sprintf("Pi subagent %s has an invalid message at index %d; ignored", id, index+1))
			continue
		}
		if message.Role == "assistant" && message.Model != "" {
			a.models[message.Model] = struct{}{}
		}
		timestamp := entry.Timestamp
		var source struct {
			Timestamp int64 `json:"timestamp"`
		}
		if json.Unmarshal(child, &source) == nil && source.Timestamp > 0 {
			if at := time.UnixMilli(source.Timestamp).UTC(); at.Year() >= 1 && at.Year() <= 9999 {
				timestamp = at.Format(time.RFC3339Nano)
			}
		}
		messageID := fmt.Sprintf("%s-%d", id, index+1)
		wrapped, err := json.Marshal(struct {
			Type      string          `json:"type"`
			ID        string          `json:"id"`
			ParentID  *string         `json:"parentId"`
			Timestamp string          `json:"timestamp"`
			Message   json.RawMessage `json:"message"`
		}{Type: "message", ID: messageID, ParentID: lastID, Timestamp: timestamp, Message: craftableRaw(child)})
		if err != nil {
			a.warnings = append(a.warnings, fmt.Sprintf("Pi subagent %s has an unencodable message; ignored", id))
			continue
		}
		messages = append(messages, wrapped)
		lastID = &messageID
	}
	return messages
}
