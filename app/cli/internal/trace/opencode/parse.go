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

package opencode

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"sort"
	"time"

	"github.com/chainloop-dev/chainloop/app/cli/internal/trace"
	"github.com/chainloop-dev/chainloop/pkg/attestation/crafter/materials/aicodingsession"
)

// unknownValue stands in for a model or tool name the export didn't carry.
const unknownValue = "unknown"

// parseExport reads and parses the opencode export JSON at path into
// structured evidence. Unknown fields are ignored.
func parseExport(path string) (*aicodingsession.Evidence, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read export file: %w", err)
	}

	var export exportData
	if err := json.Unmarshal(data, &export); err != nil {
		return nil, fmt.Errorf("parse export JSON: %w", err)
	}

	return buildEvidence(&export), nil
}

// buildEvidence converts the parsed export into an aicodingsession.Evidence.
func buildEvidence(export *exportData) *aicodingsession.Evidence {
	var totalInput, totalOutput, totalCacheRead, totalCacheWrite int
	var totalCost float64
	modelsSeen := make(map[string]int)
	toolCounts := make(map[string]int)
	var userMessages, assistantMessages int
	var providerID string

	for _, msg := range export.Messages {
		switch msg.Info.Role {
		case "user":
			userMessages++
		case "assistant":
			assistantMessages++
			if msg.Info.ModelID != "" {
				modelsSeen[msg.Info.ModelID]++
			}
			// Provider is derived from the first assistant message that carries one.
			if providerID == "" && msg.Info.ProviderID != "" {
				providerID = msg.Info.ProviderID
			}
			if msg.Info.Tokens != nil {
				totalInput += msg.Info.Tokens.Input
				totalOutput += msg.Info.Tokens.Output
				totalCacheRead += msg.Info.Tokens.Cache.Read
				totalCacheWrite += msg.Info.Tokens.Cache.Write
			}
			totalCost += msg.Info.Cost
		}

		// Count completed tool invocations from tool parts.
		for _, part := range msg.Parts {
			if part.Type == "tool" && part.State != nil && part.State.Status == "completed" {
				name := part.Tool
				if name == "" {
					name = unknownValue
				}
				toolCounts[name]++
			}
		}
	}

	// Prefer session-level cost/tokens when available (opencode computes
	// these natively); fall back to per-message aggregation.
	if export.Info.Cost != nil {
		totalCost = *export.Info.Cost
	}
	if export.Info.Tokens != nil {
		totalInput = export.Info.Tokens.Input
		totalOutput = export.Info.Tokens.Output
		totalCacheRead = export.Info.Tokens.Cache.Read
		totalCacheWrite = export.Info.Tokens.Cache.Write
	}

	primaryModel := unknownValue
	maxCount := 0
	for model, count := range modelsSeen {
		if count > maxCount {
			primaryModel = model
			maxCount = count
		}
	}

	modelsUsed := make([]string, 0, len(modelsSeen))
	for model := range modelsSeen {
		modelsUsed = append(modelsUsed, model)
	}
	sort.Strings(modelsUsed)

	// Timestamps: opencode uses epoch milliseconds.
	startedAt := formatEpochMs(export.Info.Time.Created)
	endedAt := formatEpochMs(export.Info.Time.Updated)
	var durationSeconds int
	if export.Info.Time.Created > 0 && export.Info.Time.Updated > 0 {
		durationSeconds = int((export.Info.Time.Updated - export.Info.Time.Created) / 1000)
	}

	toolSummary := make([]aicodingsession.ToolSummary, 0, len(toolCounts))
	totalInvocations := 0
	for name, count := range toolCounts {
		toolSummary = append(toolSummary, aicodingsession.ToolSummary{ToolName: name, InvocationCount: count})
		totalInvocations += count
	}
	sort.Slice(toolSummary, func(i, j int) bool {
		if toolSummary[i].InvocationCount != toolSummary[j].InvocationCount {
			return toolSummary[i].InvocationCount > toolSummary[j].InvocationCount
		}
		return toolSummary[i].ToolName < toolSummary[j].ToolName
	})

	// Round cost to 4 decimal places to match Claude provider's precision.
	totalCost = math.Round(totalCost*10000) / 10000

	// Raw session: convert each opencode message into a frontend-compatible
	// entry (one json.RawMessage per message) so the conversation timeline
	// can render individual messages. The entry shape mirrors Claude's
	// JSONL format: {type, uuid, timestamp, message: {role, content}}.
	rawSession := map[string][]json.RawMessage{
		"main": buildRawSessionEntries(export.Messages),
	}

	return aicodingsession.NewEvidence(aicodingsession.Data{
		SchemaVersion: "v1",
		Agent: aicodingsession.Agent{
			Name:    Name,
			Version: export.Info.Version,
		},
		Session: aicodingsession.Session{
			ID:              export.Info.ID,
			Slug:            export.Info.Slug,
			StartedAt:       startedAt,
			EndedAt:         endedAt,
			DurationSeconds: durationSeconds,
		},
		Model: &aicodingsession.Model{
			Primary:    primaryModel,
			Provider:   providerID,
			ModelsUsed: modelsUsed,
		},
		Usage: &aicodingsession.Usage{
			InputTokens:              totalInput,
			OutputTokens:             totalOutput,
			TotalTokens:              totalInput + totalOutput,
			CacheReadInputTokens:     totalCacheRead,
			CacheCreationInputTokens: totalCacheWrite,
			EstimatedCostUSD:         totalCost,
		},
		ToolsUsed: &aicodingsession.ToolsUsed{
			Summary:          toolSummary,
			TotalInvocations: totalInvocations,
		},
		Conversation: &aicodingsession.Conversation{
			TotalMessages:     userMessages + assistantMessages,
			UserMessages:      userMessages,
			AssistantMessages: assistantMessages,
		},
		RawSession: rawSession,
	})
}

// formatEpochMs converts an opencode epoch-ms timestamp to RFC3339;
// returns "" for non-positive values.
func formatEpochMs(ms float64) string {
	if ms <= 0 {
		return ""
	}
	t := time.UnixMilli(int64(ms)).UTC()
	return t.Format(time.RFC3339)
}

// buildRawSessionEntries converts opencode messages into the frontend-compatible
// RawSessionEntry format (one entry per message). Each entry carries the
// message role, ID, timestamp, and content blocks (text and tool_use) so the
// ConversationTimeline can render the full transcript.
func buildRawSessionEntries(messages []messageEntry) []json.RawMessage {
	entries := make([]json.RawMessage, 0, len(messages))
	for _, msg := range messages {
		entry := trace.RawSessionEntry{
			Type:      msg.Info.Role,
			UUID:      msg.Info.ID,
			Timestamp: msgTimeRFC3339(msg.Info.Time),
			Message: trace.RawSessionMessage{
				Role:  msg.Info.Role,
				Model: msg.Info.ModelID,
			},
		}

		var blocks []any
		for _, p := range msg.Parts {
			switch p.Type {
			case "text":
				if p.Text != "" {
					blocks = append(blocks, trace.RawSessionTextBlock{Type: "text", Text: p.Text})
				}
			case "tool":
				if p.State == nil || p.State.Status != "completed" {
					continue
				}
				toolName := p.Tool
				if toolName == "" {
					toolName = unknownValue
				}
				blocks = append(blocks, trace.RawSessionToolUseBlock{
					Type:  "tool_use",
					ID:    p.ID,
					Name:  toolName,
					Input: p.State.Input,
				})
			}
		}

		if len(blocks) > 0 {
			content, _ := json.Marshal(blocks)
			entry.Message.Content = content
		}

		entries = append(entries, trace.MustRawSessionEntry(entry))
	}
	return entries
}

func msgTimeRFC3339(t *msgTime) string {
	if t == nil || t.Created <= 0 {
		return ""
	}
	return formatEpochMs(t.Created)
}
