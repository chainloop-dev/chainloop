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
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"math/big"
	"os"
	"sort"
	"strconv"
	"time"

	"github.com/chainloop-dev/chainloop/pkg/attestation/crafter/materials/aicodingsession"
)

func parseSession(path, expectedID, agentVersion string) (*aicodingsession.Evidence, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, errors.New("parse Pi session: copied transcript unavailable")
	}

	parsed, err := parseJSONL(data, expectedID)
	if err != nil {
		return nil, err
	}

	return buildEvidence(parsed, agentVersion), nil
}

func parseJSONL(data []byte, expectedID string) (*parsedSession, error) {
	lines := bytes.Split(data, []byte("\n"))
	endsWithNewline := len(data) > 0 && data[len(data)-1] == '\n'
	entries := make([]*sessionEntry, 0, len(lines))
	byID := make(map[string]*sessionEntry)
	var result *parsedSession

	for index, line := range lines {
		line = bytes.TrimSpace(line)
		if len(line) == 0 {
			continue
		}

		lineNumber := index + 1
		if result == nil {
			header, startedAt, err := decodeHeader(line, expectedID, lineNumber)
			if err != nil {
				return nil, err
			}
			result = &parsedSession{Header: header, StartedAt: startedAt}
			continue
		}

		if !json.Valid(line) {
			if index == len(lines)-1 && !endsWithNewline {
				result.TruncatedLine = lineNumber
				break
			}
			return nil, fmt.Errorf("parse Pi session: invalid JSON on line %d", lineNumber)
		}

		entry, err := decodeEntry(line, lineNumber)
		if err != nil {
			return nil, err
		}
		if _, duplicate := byID[entry.ID]; duplicate {
			return nil, fmt.Errorf("parse Pi session: duplicate entry ID on line %d", lineNumber)
		}

		entries = append(entries, entry)
		byID[entry.ID] = entry
	}

	if result == nil {
		return nil, errors.New("parse Pi session: missing header")
	}
	if err := validateNoCycles(entries, byID); err != nil {
		return nil, err
	}

	selected, err := selectLastBranch(entries, byID)
	if err != nil {
		return nil, err
	}
	result.Selected = selected

	return result, nil
}

func decodeHeader(line []byte, expectedID string, lineNumber int) (sessionHeader, time.Time, error) {
	var header sessionHeader
	if err := json.Unmarshal(line, &header); err != nil {
		return header, time.Time{}, fmt.Errorf("parse Pi session: invalid header JSON on line %d", lineNumber)
	}
	if header.Type != "session" || header.Version != sessionVersion || header.ID == "" || header.Cwd == "" {
		return header, time.Time{}, fmt.Errorf("parse Pi session: invalid v3 header on line %d", lineNumber)
	}
	if expectedID != "" && header.ID != expectedID {
		return header, time.Time{}, fmt.Errorf("parse Pi session: session ID mismatch on line %d", lineNumber)
	}

	startedAt, err := time.Parse(time.RFC3339Nano, header.Timestamp)
	if err != nil {
		return header, time.Time{}, fmt.Errorf("parse Pi session: invalid header timestamp on line %d", lineNumber)
	}

	return header, startedAt, nil
}

func decodeEntry(line []byte, lineNumber int) (*sessionEntry, error) {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(line, &fields); err != nil || fields == nil {
		return nil, fmt.Errorf("parse Pi session: invalid JSON on line %d", lineNumber)
	}

	typeName, ok := rawString(fields["type"])
	if !ok || typeName == "" {
		return nil, fmt.Errorf("parse Pi session: missing entry type on line %d", lineNumber)
	}
	id, ok := rawString(fields["id"])
	if !ok || id == "" {
		return nil, fmt.Errorf("parse Pi session: missing entry ID on line %d", lineNumber)
	}

	parentRaw, present := fields["parentId"]
	if !present {
		return nil, fmt.Errorf("parse Pi session: missing parent ID on line %d", lineNumber)
	}
	parentID, hasParent, ok := rawParent(parentRaw)
	if !ok {
		return nil, fmt.Errorf("parse Pi session: invalid parent ID on line %d", lineNumber)
	}

	timestamp, ok := rawString(fields["timestamp"])
	if !ok || timestamp == "" {
		return nil, fmt.Errorf("parse Pi session: missing entry timestamp on line %d", lineNumber)
	}
	if _, err := time.Parse(time.RFC3339Nano, timestamp); err != nil {
		return nil, fmt.Errorf("parse Pi session: invalid entry timestamp on line %d", lineNumber)
	}
	customType, _ := rawString(fields["customType"])

	return &sessionEntry{
		Type:       typeName,
		ID:         id,
		ParentID:   parentID,
		HasParent:  hasParent,
		Timestamp:  timestamp,
		CustomType: customType,
		Message:    cloneRaw(fields["message"]),
		Raw:        cloneRaw(line),
		Line:       lineNumber,
	}, nil
}

func rawString(raw json.RawMessage) (string, bool) {
	if len(raw) == 0 {
		return "", false
	}
	var value string
	if err := json.Unmarshal(raw, &value); err != nil {
		return "", false
	}
	return value, true
}

func rawParent(raw json.RawMessage) (string, bool, bool) {
	if bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		return "", false, true
	}
	value, ok := rawString(raw)
	return value, ok, ok
}

func cloneRaw(raw []byte) json.RawMessage {
	return append(json.RawMessage(nil), raw...)
}

func validateNoCycles(entries []*sessionEntry, byID map[string]*sessionEntry) error {
	state := make(map[string]uint8, len(entries))
	var visit func(*sessionEntry) error
	visit = func(entry *sessionEntry) error {
		switch state[entry.ID] {
		case 1:
			return fmt.Errorf("parse Pi session: cycle includes entry on line %d", entry.Line)
		case 2:
			return nil
		}

		state[entry.ID] = 1
		if entry.HasParent {
			if parent := byID[entry.ParentID]; parent != nil {
				if err := visit(parent); err != nil {
					return err
				}
			}
		}
		state[entry.ID] = 2
		return nil
	}

	for _, entry := range entries {
		if err := visit(entry); err != nil {
			return err
		}
	}
	return nil
}

func selectLastBranch(entries []*sessionEntry, byID map[string]*sessionEntry) ([]*sessionEntry, error) {
	if len(entries) == 0 {
		return nil, nil
	}

	selected := make([]*sessionEntry, 0, len(entries))
	for current := entries[len(entries)-1]; current != nil; {
		selected = append(selected, current)
		if !current.HasParent {
			break
		}

		parent := byID[current.ParentID]
		if parent == nil {
			return nil, fmt.Errorf("parse Pi session: selected entry on line %d has a missing parent", current.Line)
		}
		current = parent
	}

	for left, right := 0, len(selected)-1; left < right; left, right = left+1, right-1 {
		selected[left], selected[right] = selected[right], selected[left]
	}
	return selected, nil
}

type messageView struct {
	Role     string
	Provider string
	Model    string
	Content  json.RawMessage
	Usage    json.RawMessage
	HasUsage bool
}

type evidenceAccumulator struct {
	inputTokens     int
	outputTokens    int
	cacheRead       int
	cacheWrite      int
	totalTokens     int
	totalCost       float64
	models          map[string]struct{}
	primaryModel    string
	primaryProvider string
	tools           map[string]int
	userMessages    int
	assistantMsgs   int
	warnings        []string
}

func buildEvidence(parsed *parsedSession, agentVersion string) *aicodingsession.Evidence {
	acc := evidenceAccumulator{models: make(map[string]struct{}), tools: make(map[string]int)}
	raw := make([]json.RawMessage, 0, len(parsed.Selected))

	for _, entry := range parsed.Selected {
		if !isTreeMarker(entry) {
			raw = append(raw, craftableRaw(entry.Raw))
		}
		if entry.Type != "message" {
			continue
		}

		message, ok := decodeMessage(entry.Message)
		if !ok {
			continue
		}
		switch message.Role {
		case "user":
			acc.userMessages++
		case "assistant":
			acc.assistantMsgs++
			acc.primaryModel = message.Model
			acc.primaryProvider = message.Provider
			if message.Model != "" {
				acc.models[message.Model] = struct{}{}
			}
			acc.addUsage(entry.Line, message)
			acc.addTools(message.Content)
		}
	}

	if parsed.TruncatedLine > 0 {
		acc.warnings = append(acc.warnings, fmt.Sprintf("ignored truncated final Pi session line %d", parsed.TruncatedLine))
	}

	endedAt := parsed.StartedAt
	if len(parsed.Selected) > 0 {
		if value, err := time.Parse(time.RFC3339Nano, parsed.Selected[len(parsed.Selected)-1].Timestamp); err == nil {
			endedAt = value
		}
	}
	duration := max(0, int(endedAt.Sub(parsed.StartedAt).Seconds()))

	return aicodingsession.NewEvidence(aicodingsession.Data{
		SchemaVersion: "v1",
		Agent: aicodingsession.Agent{
			Name:    Name,
			Version: agentVersion,
		},
		Session: aicodingsession.Session{
			ID:              parsed.Header.ID,
			StartedAt:       parsed.StartedAt.UTC().Format(time.RFC3339Nano),
			EndedAt:         endedAt.UTC().Format(time.RFC3339Nano),
			DurationSeconds: duration,
		},
		Model: &aicodingsession.Model{
			Primary:    acc.primaryModel,
			Provider:   acc.primaryProvider,
			ModelsUsed: sortedSet(acc.models),
		},
		Usage: &aicodingsession.Usage{
			InputTokens:              acc.inputTokens,
			OutputTokens:             acc.outputTokens,
			TotalTokens:              acc.totalTokens,
			CacheReadInputTokens:     acc.cacheRead,
			CacheCreationInputTokens: acc.cacheWrite,
			EstimatedCostUSD:         roundedCost(acc.totalCost),
		},
		ToolsUsed: &aicodingsession.ToolsUsed{
			Summary:          sortedTools(acc.tools),
			TotalInvocations: toolTotal(acc.tools),
		},
		Conversation: &aicodingsession.Conversation{
			TotalMessages:     acc.userMessages + acc.assistantMsgs,
			UserMessages:      acc.userMessages,
			AssistantMessages: acc.assistantMsgs,
		},
		RawSession: map[string][]json.RawMessage{"main": raw},
		Warnings:   acc.warnings,
	})
}

func isTreeMarker(entry *sessionEntry) bool {
	return entry.Type == "custom" && entry.CustomType == treeMarkerCustomType
}

// craftableRaw preserves a native Pi entry unless encoding/json cannot decode
// one of its numbers into the generic float64 representation used by the
// evidence crafter. In that case only out-of-range numbers become null; the
// parser has already warned about invalid known usage fields.
func craftableRaw(raw json.RawMessage) json.RawMessage {
	var probe any
	if json.Unmarshal(raw, &probe) == nil {
		return cloneRaw(raw)
	}

	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	if err := decoder.Decode(&probe); err != nil {
		return cloneRaw(raw)
	}

	normalizeJSONNumbers(probe)
	encoded, err := json.Marshal(probe)
	if err != nil {
		return cloneRaw(raw)
	}
	return encoded
}

func normalizeJSONNumbers(value any) {
	switch node := value.(type) {
	case map[string]any:
		for key, child := range node {
			if number, ok := child.(json.Number); ok {
				parsed, err := strconv.ParseFloat(number.String(), 64)
				if err != nil || math.IsInf(parsed, 0) || math.IsNaN(parsed) {
					node[key] = nil
				}
				continue
			}
			normalizeJSONNumbers(child)
		}
	case []any:
		for index, child := range node {
			if number, ok := child.(json.Number); ok {
				parsed, err := strconv.ParseFloat(number.String(), 64)
				if err != nil || math.IsInf(parsed, 0) || math.IsNaN(parsed) {
					node[index] = nil
				}
				continue
			}
			normalizeJSONNumbers(child)
		}
	}
}

func decodeMessage(raw json.RawMessage) (messageView, bool) {
	var fields map[string]json.RawMessage
	if len(raw) == 0 || json.Unmarshal(raw, &fields) != nil || fields == nil {
		return messageView{}, false
	}

	role, ok := rawString(fields["role"])
	if !ok {
		return messageView{}, false
	}
	provider, _ := rawString(fields["provider"])
	model, _ := rawString(fields["model"])
	usage, hasUsage := fields["usage"]

	return messageView{
		Role:     role,
		Provider: provider,
		Model:    model,
		Content:  cloneRaw(fields["content"]),
		Usage:    cloneRaw(usage),
		HasUsage: hasUsage,
	}, true
}

func (a *evidenceAccumulator) addUsage(line int, message messageView) {
	if !message.HasUsage || isNull(message.Usage) {
		return
	}

	var usage map[string]json.RawMessage
	if json.Unmarshal(message.Usage, &usage) != nil || usage == nil {
		a.warn(line, "usage")
		return
	}

	a.addInt(line, "input", usage["input"], &a.inputTokens)
	a.addInt(line, "output", usage["output"], &a.outputTokens)
	a.addInt(line, "cacheRead", usage["cacheRead"], &a.cacheRead)
	a.addInt(line, "cacheWrite", usage["cacheWrite"], &a.cacheWrite)
	a.addInt(line, "totalTokens", usage["totalTokens"], &a.totalTokens)

	costRaw, exists := usage["cost"]
	if !exists || isNull(costRaw) {
		return
	}
	var cost map[string]json.RawMessage
	if json.Unmarshal(costRaw, &cost) != nil || cost == nil {
		a.warn(line, "cost.total")
		return
	}
	a.addCost(line, cost["total"])
}

func (a *evidenceAccumulator) addInt(line int, field string, raw json.RawMessage, total *int) {
	if len(raw) == 0 || isNull(raw) {
		return
	}
	value, ok := nonnegativeInt(raw)
	if !ok || value > int(^uint(0)>>1)-*total {
		a.warn(line, field)
		return
	}
	*total += value
}

func nonnegativeInt(raw json.RawMessage) (int, bool) {
	number, ok := rawNumber(raw)
	if !ok {
		return 0, false
	}

	value, ok := new(big.Rat).SetString(number.String())
	if !ok || value.Sign() < 0 || !value.IsInt() {
		return 0, false
	}
	maxInt := new(big.Int).SetUint64(uint64(^uint(0) >> 1))
	if value.Num().Cmp(maxInt) > 0 {
		return 0, false
	}

	return int(value.Num().Int64()), true
}

func (a *evidenceAccumulator) addCost(line int, raw json.RawMessage) {
	if len(raw) == 0 || isNull(raw) {
		return
	}
	number, ok := rawNumber(raw)
	if !ok {
		a.warn(line, "cost.total")
		return
	}
	value, err := strconv.ParseFloat(number.String(), 64)
	if err != nil || value < 0 || math.IsInf(value, 0) || math.IsNaN(value) || math.IsInf(a.totalCost+value, 0) {
		a.warn(line, "cost.total")
		return
	}
	a.totalCost += value
}

func roundedCost(value float64) float64 {
	if value > math.MaxFloat64/10000 {
		return value
	}
	return math.Round(value*10000) / 10000
}

func rawNumber(raw json.RawMessage) (json.Number, bool) {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	var value any
	if err := decoder.Decode(&value); err != nil {
		return "", false
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return "", false
	}
	number, ok := value.(json.Number)
	return number, ok
}

func isNull(raw json.RawMessage) bool {
	return bytes.Equal(bytes.TrimSpace(raw), []byte("null"))
}

func (a *evidenceAccumulator) warn(line int, field string) {
	a.warnings = append(a.warnings, fmt.Sprintf("Pi entry on line %d has invalid usage.%s; ignored", line, field))
}

func (a *evidenceAccumulator) addTools(raw json.RawMessage) {
	var blocks []map[string]json.RawMessage
	if len(raw) == 0 || json.Unmarshal(raw, &blocks) != nil {
		return
	}
	for _, block := range blocks {
		typeName, _ := rawString(block["type"])
		name, _ := rawString(block["name"])
		if typeName == "toolCall" && name != "" {
			a.tools[name]++
		}
	}
}

func sortedSet(values map[string]struct{}) []string {
	result := make([]string, 0, len(values))
	for value := range values {
		result = append(result, value)
	}
	sort.Strings(result)
	return result
}

func sortedTools(counts map[string]int) []aicodingsession.ToolSummary {
	names := make([]string, 0, len(counts))
	for name := range counts {
		names = append(names, name)
	}
	sort.Strings(names)

	result := make([]aicodingsession.ToolSummary, 0, len(names))
	for _, name := range names {
		result = append(result, aicodingsession.ToolSummary{ToolName: name, InvocationCount: counts[name]})
	}
	return result
}

func toolTotal(counts map[string]int) int {
	total := 0
	for _, count := range counts {
		total += count
	}
	return total
}
