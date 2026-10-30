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
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"math/big"
	"reflect"

	"github.com/chainloop-dev/chainloop/pkg/attestation/crafter/materials/aicodingsession"
)

type piSourceRecord struct {
	Type    string `json:"type"`
	ID      string `json:"id"`
	Message struct {
		Role     string                     `json:"role"`
		ToolName string                     `json:"toolName"`
		Usage    map[string]json.RawMessage `json:"usage"`
		Details  struct {
			Mode    string            `json:"mode"`
			Results []json.RawMessage `json:"results"`
		} `json:"details"`
	} `json:"message"`
}

type piSourceChild struct {
	Agent    string                     `json:"agent"`
	Task     string                     `json:"task"`
	Usage    map[string]json.RawMessage `json:"usage"`
	Messages []json.RawMessage          `json:"messages"`
}

// Compare stored children with native completed results, independently of the
// provider parser. Nonempty aggregate metadata alone cannot pass this check.
func checkPiSubagents(d *aicodingsession.Data) error {
	var expected []aicodingsession.Subagent
	hasToolResult := false
	for _, raw := range d.RawSession["main"] {
		var r piSourceRecord
		if json.Unmarshal(raw, &r) != nil || !piSourceSubagent(r) {
			continue
		}
		for index, rawChild := range r.Message.Details.Results {
			var child piSourceChild
			if err := json.Unmarshal(rawChild, &child); err != nil {
				return fmt.Errorf("decode Pi source child: %w", err)
			}
			id := fmt.Sprintf("subagent-%s-%d", r.ID, index+1)
			var usage aicodingsession.Usage
			addPiSourceUsage(&usage, child.Usage, child.Usage["cost"])
			expected = append(expected, aicodingsession.Subagent{
				ID: id, Type: child.Agent, Description: child.Task,
				Tokens: aicodingsession.SubagentTokens{Input: usage.InputTokens, Output: usage.OutputTokens},
			})
			stream := d.RawSession[id]
			if len(stream) == 0 || len(stream) != len(child.Messages) {
				return fmt.Errorf("Pi child %s conversation is missing or incomplete", id)
			}
			for i, source := range child.Messages {
				var entry struct {
					Message json.RawMessage `json:"message"`
				}
				if json.Unmarshal(stream[i], &entry) != nil {
					return fmt.Errorf("Pi child %s has an invalid stored message", id)
				}
				// Material assembly may indent or reorder fields.
				var stored, original bytes.Buffer
				if json.Compact(&stored, entry.Message) != nil || json.Compact(&original, source) != nil {
					return fmt.Errorf("Pi child %s has invalid message JSON", id)
				}
				var got, want map[string]json.RawMessage
				if json.Unmarshal(stored.Bytes(), &got) != nil || json.Unmarshal(original.Bytes(), &want) != nil || !reflect.DeepEqual(got, want) {
					return fmt.Errorf("Pi child %s message %d differs from source content", id, i+1)
				}
				var message struct {
					Role    string `json:"role"`
					Content []struct {
						Text string `json:"text"`
					} `json:"content"`
				}
				if json.Unmarshal(source, &message) == nil && message.Role == "toolResult" {
					for _, block := range message.Content {
						hasToolResult = hasToolResult || block.Text != ""
					}
				}
			}
		}
	}
	if len(expected) == 0 || !reflect.DeepEqual(expected, d.Subagents) {
		return errors.New("Pi subagents do not match native completed results")
	}
	if !hasToolResult {
		return errors.New("Pi subagent scenario has no readable child tool result")
	}
	if len(d.RawSession) != len(expected)+1 {
		return errors.New("Pi evidence has unrelated child streams")
	}
	want := piSourceUsage(d.RawSession["main"])
	if d.SchemaVersion != "v1" || d.Usage == nil || *d.Usage != want {
		return fmt.Errorf("Pi usage does not count parent and child aggregates exactly once: got %+v, want %+v", d.Usage, want)
	}
	return nil
}

func piSourceSubagent(r piSourceRecord) bool {
	if r.Type != "message" || r.Message.Role != "toolResult" || r.Message.ToolName != "subagent" {
		return false
	}
	switch r.Message.Details.Mode {
	case "single", "parallel", "chain":
		return true
	default:
		return false
	}
}

func piSourceUsage(records []json.RawMessage) aicodingsession.Usage {
	var total aicodingsession.Usage
	for _, raw := range records {
		var r piSourceRecord
		if json.Unmarshal(raw, &r) != nil || r.Type != "message" {
			continue
		}
		if r.Message.Role == "assistant" {
			var cost struct {
				Total json.RawMessage `json:"total"`
			}
			if json.Unmarshal(r.Message.Usage["cost"], &cost) != nil {
				cost.Total = nil
			}
			addPiSourceUsage(&total, r.Message.Usage, cost.Total)
		}
		if piSourceSubagent(r) {
			for _, rawChild := range r.Message.Details.Results {
				var child piSourceChild
				if json.Unmarshal(rawChild, &child) == nil && child.Agent != "" {
					addPiSourceUsage(&total, child.Usage, child.Usage["cost"])
				}
			}
		}
	}
	if total.OutputTokens <= int(^uint(0)>>1)-total.InputTokens {
		total.TotalTokens = total.InputTokens + total.OutputTokens
	}
	if total.EstimatedCostUSD <= math.MaxFloat64/10000 {
		total.EstimatedCostUSD = math.Round(total.EstimatedCostUSD*10000) / 10000
	}
	return total
}

func addPiSourceUsage(total *aicodingsession.Usage, usage map[string]json.RawMessage, rawCost json.RawMessage) {
	for _, field := range []struct {
		name  string
		total *int
	}{
		{"input", &total.InputTokens},
		{"output", &total.OutputTokens},
		{"cacheRead", &total.CacheReadInputTokens},
		{"cacheWrite", &total.CacheCreationInputTokens},
	} {
		value, ok := new(big.Rat).SetString(string(usage[field.name]))
		if !ok || value.Sign() < 0 || !value.IsInt() || !value.Num().IsInt64() {
			continue
		}
		count := value.Num().Int64()
		if count <= int64(int(^uint(0)>>1)-*field.total) {
			*field.total += int(count)
		}
	}
	var cost float64
	if json.Unmarshal(rawCost, &cost) == nil && cost >= 0 && !math.IsInf(cost+total.EstimatedCostUSD, 0) {
		total.EstimatedCostUSD += cost
	}
}
