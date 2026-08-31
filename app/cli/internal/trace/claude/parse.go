package claude

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/chainloop-dev/chainloop/pkg/attestation/crafter/materials/aicodingsession"
	"github.com/chainloop-dev/chainloop/app/cli/internal/trace/state"
)

// Pricing rates live in pricing.json (loaded by pricing.go via go:embed).

const (
	maxScannerBuffer = 10 * 1024 * 1024 // 10 MB for large assistant messages
	maxWarnings      = 50               // cap to prevent unbounded growth from corrupt files
)

// Transcript record types, as written by Claude Code into the session JSONL.
const (
	recordTypeUser      = "user"
	recordTypeAssistant = "assistant"
	recordTypeProgress  = "progress"
)

// unknownValue stands in for a model or tool name the transcript didn't carry.
const unknownValue = "unknown"

func findJSONLPath(sessionDir, sessionID string) (string, error) {
	if sessionID != "" {
		return state.RawSessionPath(sessionDir, sessionID), nil
	}

	entries, err := os.ReadDir(sessionDir)
	if err != nil {
		return "", fmt.Errorf("read session directory: %w", err)
	}

	var newest string
	var newestTime time.Time
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".jsonl") {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		if newest == "" || info.ModTime().After(newestTime) {
			newest = filepath.Join(sessionDir, e.Name())
			newestTime = info.ModTime()
		}
	}

	if newest == "" {
		return "", fmt.Errorf("no .jsonl files found in %s", sessionDir)
	}

	return newest, nil
}

// scanJSONL opens a JSONL file and calls fn for each successfully parsed record.
// Malformed lines are appended to warnings rather than causing an error.
// If rawLines is non-nil, each valid JSON line is appended as a json.RawMessage.
func scanJSONL(path string, warnings *[]string, fn func(*jsonlRecord), rawLines *[]json.RawMessage) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()

	scanner := bufio.NewScanner(f)
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, maxScannerBuffer)

	lineNo := 0
	for scanner.Scan() {
		lineNo++
		raw := bytes.TrimSpace(scanner.Bytes())
		if len(raw) == 0 {
			continue
		}

		var record jsonlRecord
		if err := json.Unmarshal(raw, &record); err != nil {
			addWarning(warnings, fmt.Sprintf("Malformed JSON at %s:%d", filepath.Base(path), lineNo))
			continue
		}

		if rawLines != nil {
			*rawLines = append(*rawLines, json.RawMessage(append([]byte(nil), raw...)))
		}

		fn(&record)
	}

	return scanner.Err()
}

func parseJSONL(path string) (*sessionData, []json.RawMessage, error) {
	data := &sessionData{
		tokenUsageByModel: make(map[string]*tokenUsage),
		toolCounts:        make(map[string]int),
		messageCounts:     make(map[string]int),
		modelsSeen:        make(map[string]int),
	}

	var rawLines []json.RawMessage

	err := scanJSONL(path, &data.warnings, func(record *jsonlRecord) {
		msgType := record.Type
		switch msgType {
		case recordTypeUser, recordTypeAssistant, recordTypeProgress:
			data.messageCounts[msgType]++
		}

		if record.Timestamp != "" {
			if data.tsMin == "" || record.Timestamp < data.tsMin {
				data.tsMin = record.Timestamp
			}
			if data.tsMax == "" || record.Timestamp > data.tsMax {
				data.tsMax = record.Timestamp
			}
		}

		if data.sessionID == "" && record.SessionID != "" {
			data.sessionID = record.SessionID
		}
		if data.slug == "" && record.Slug != "" {
			data.slug = record.Slug
		}
		if data.version == "" && record.Version != "" {
			data.version = record.Version
		}
		if data.cwd == "" && record.Cwd != "" {
			data.cwd = record.Cwd
		}
		if data.gitBranch == "" && record.GitBranch != "" {
			data.gitBranch = record.GitBranch
		}

		if msgType == recordTypeAssistant && record.Message != nil {
			model := record.Message.Model
			if model == "" {
				model = unknownValue
			}
			data.modelsSeen[model]++

			if record.Message.Usage != nil {
				getOrCreateUsage(data.tokenUsageByModel, model).add(record.Message.Usage)
			}

			// Content can be a string or an array of blocks; only parse arrays
			var blocks []jsonlContentBlock
			if json.Unmarshal(record.Message.Content, &blocks) == nil {
				for _, block := range blocks {
					if block.Type == "tool_use" {
						name := block.Name
						if name == "" {
							name = unknownValue
						}
						data.toolCounts[name]++
					}
				}
			}
		}
	}, &rawLines)
	if err != nil {
		return nil, nil, fmt.Errorf("reading session file: %w", err)
	}

	return data, rawLines, nil
}

func processSubagents(sessionDir, sessionID string, warnings *[]string) ([]aicodingsession.Subagent, map[string]*tokenUsage, map[string][]json.RawMessage, error) {
	subagentsDir := state.RawSubagentDir(sessionDir, sessionID)

	metaFiles, err := filepath.Glob(filepath.Join(subagentsDir, "agent-*.meta.json"))
	if err != nil {
		return nil, nil, nil, fmt.Errorf("glob subagent meta files: %w", err)
	}
	if len(metaFiles) == 0 {
		return nil, nil, nil, nil
	}

	var subagents []aicodingsession.Subagent
	totalSubUsage := make(map[string]*tokenUsage)
	rawSubagents := make(map[string][]json.RawMessage)

	for _, metaPath := range metaFiles {
		base := filepath.Base(metaPath)
		agentID := strings.TrimPrefix(base, "agent-")
		agentID = strings.TrimSuffix(agentID, ".meta.json")

		metaData, err := os.ReadFile(metaPath)
		if err != nil {
			continue
		}
		var meta subagentMeta
		if err := json.Unmarshal(metaData, &meta); err != nil {
			continue
		}

		var rawLines []json.RawMessage
		subInput, subOutput := parseSubagentUsage(
			filepath.Join(subagentsDir, fmt.Sprintf("agent-%s.jsonl", agentID)),
			totalSubUsage,
			warnings,
			&rawLines,
		)

		if len(rawLines) > 0 {
			rawSubagents[agentID] = rawLines
		}

		subagents = append(subagents, aicodingsession.Subagent{
			ID:          agentID,
			Type:        meta.AgentType,
			Description: meta.Description,
			Tokens:      aicodingsession.SubagentTokens{Input: subInput, Output: subOutput},
		})
	}

	return subagents, totalSubUsage, rawSubagents, nil
}

func parseSubagentUsage(jsonlPath string, totalUsage map[string]*tokenUsage, warnings *[]string, rawLines *[]json.RawMessage) (subInput, subOutput int) {
	if err := scanJSONL(jsonlPath, warnings, func(record *jsonlRecord) {
		if record.Type == recordTypeAssistant && record.Message != nil && record.Message.Usage != nil {
			model := record.Message.Model
			if model == "" {
				model = unknownValue
			}
			u := record.Message.Usage
			subInput += u.InputTokens
			subOutput += u.OutputTokens
			getOrCreateUsage(totalUsage, model).add(u)
		}
	}, rawLines); err != nil {
		addWarning(warnings, fmt.Sprintf("Failed to read subagent %s: %v", filepath.Base(jsonlPath), err))
	}

	return
}

func mergeUsage(main, sub map[string]*tokenUsage) map[string]*tokenUsage {
	if len(sub) == 0 {
		return main
	}

	merged := make(map[string]*tokenUsage)
	for _, source := range []map[string]*tokenUsage{main, sub} {
		for model, u := range source {
			bucket := getOrCreateUsage(merged, model)
			bucket.InputTokens += u.InputTokens
			bucket.OutputTokens += u.OutputTokens
			bucket.CacheCreationInputTokens += u.CacheCreationInputTokens
			bucket.CacheCreation5mTokens += u.CacheCreation5mTokens
			bucket.CacheCreation1hTokens += u.CacheCreation1hTokens
			bucket.CacheReadInputTokens += u.CacheReadInputTokens
		}
	}

	return merged
}

func computeCost(usage map[string]*tokenUsage, warnings *[]string) float64 {
	totalCost := 0.0
	for model, u := range usage {
		p, ok := pricing[model]
		if !ok {
			p = defaultPricing
			addWarning(warnings,
				fmt.Sprintf("Unknown model '%s', using default (sonnet) pricing", model))
		}

		cost := float64(u.InputTokens)*p.Input/1_000_000 +
			float64(u.OutputTokens)*p.Output/1_000_000 +
			float64(u.CacheCreation5mTokens)*p.CacheWrite/1_000_000 +
			float64(u.CacheCreation1hTokens)*p.CacheWrite1h/1_000_000 +
			float64(u.CacheReadInputTokens)*p.CacheRead/1_000_000
		totalCost += cost
	}

	return math.Round(totalCost*10000) / 10000
}

func buildOutput(data *sessionData, subagents []aicodingsession.Subagent, merged map[string]*tokenUsage, warnings []string, cost float64, rawSession map[string][]json.RawMessage) *aicodingsession.Evidence {
	var durationSeconds int
	if data.tsMin != "" && data.tsMax != "" {
		t0, err0 := time.Parse(time.RFC3339, data.tsMin)
		t1, err1 := time.Parse(time.RFC3339, data.tsMax)
		if err0 == nil && err1 == nil {
			durationSeconds = int(t1.Sub(t0).Seconds())
		}
	}

	var totalInput, totalOutput, totalCacheRead, totalCacheCreation int
	for _, u := range merged {
		totalInput += u.InputTokens
		totalOutput += u.OutputTokens
		totalCacheRead += u.CacheReadInputTokens
		totalCacheCreation += u.CacheCreationInputTokens
	}

	primaryModel := unknownValue
	maxCount := 0
	for model, count := range data.modelsSeen {
		if count > maxCount {
			primaryModel = model
			maxCount = count
		}
	}

	// Derive from merged usage to include subagent-only models
	modelsUsed := make([]string, 0, len(merged))
	for model := range merged {
		modelsUsed = append(modelsUsed, model)
	}
	sort.Strings(modelsUsed)

	toolSummary := make([]aicodingsession.ToolSummary, 0, len(data.toolCounts))
	totalInvocations := 0
	for name, count := range data.toolCounts {
		toolSummary = append(toolSummary, aicodingsession.ToolSummary{ToolName: name, InvocationCount: count})
		totalInvocations += count
	}
	sort.Slice(toolSummary, func(i, j int) bool {
		return toolSummary[i].InvocationCount > toolSummary[j].InvocationCount
	})

	return &aicodingsession.Evidence{
		Schema: aicodingsession.EvidenceSchemaURL,
		ID:     aicodingsession.EvidenceID,
		Data: aicodingsession.Data{
			SchemaVersion: "v1",
			Agent: aicodingsession.Agent{
				Name:    "claude-code",
				Version: data.version,
			},
			Session: aicodingsession.Session{
				ID:              data.sessionID,
				Slug:            data.slug,
				StartedAt:       data.tsMin,
				EndedAt:         data.tsMax,
				DurationSeconds: durationSeconds,
			},
			GitContext: &aicodingsession.GitContext{
				Branch:  data.gitBranch,
				WorkDir: data.cwd,
			},
			Model: &aicodingsession.Model{
				Primary:    primaryModel,
				Provider:   "anthropic",
				ModelsUsed: modelsUsed,
			},
			Usage: &aicodingsession.Usage{
				InputTokens:              totalInput,
				OutputTokens:             totalOutput,
				TotalTokens:              totalInput + totalOutput,
				CacheReadInputTokens:     totalCacheRead,
				CacheCreationInputTokens: totalCacheCreation,
				EstimatedCostUSD:         cost,
			},
			ToolsUsed: &aicodingsession.ToolsUsed{
				Summary:          toolSummary,
				TotalInvocations: totalInvocations,
			},
			Conversation: &aicodingsession.Conversation{
				TotalMessages:     data.messageCounts[recordTypeUser] + data.messageCounts[recordTypeAssistant] + data.messageCounts[recordTypeProgress],
				UserMessages:      data.messageCounts[recordTypeUser],
				AssistantMessages: data.messageCounts[recordTypeAssistant],
			},
			Subagents:  subagents,
			RawSession: rawSession,
			Warnings:   warnings,
		},
	}
}

func addWarning(warnings *[]string, msg string) {
	if warnings == nil {
		return
	}
	if len(*warnings) < maxWarnings {
		*warnings = append(*warnings, msg)
	} else if len(*warnings) == maxWarnings {
		*warnings = append(*warnings, fmt.Sprintf("... further warnings suppressed (limit %d)", maxWarnings))
	}
}

func getOrCreateUsage(m map[string]*tokenUsage, model string) *tokenUsage {
	if u, ok := m[model]; ok {
		return u
	}
	u := &tokenUsage{}
	m[model] = u

	return u
}
