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

package claude

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/chainloop-dev/chainloop/app/cli/internal/trace"
	"github.com/chainloop-dev/chainloop/pkg/attestation/crafter/materials/aicodingsession"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const testdataDir = "testdata"

func TestFindJSONLPath(t *testing.T) {
	t.Run("explicit session ID", func(t *testing.T) {
		path, err := findJSONLPath(testdataDir, "test-session")
		require.NoError(t, err)
		assert.Equal(t, filepath.Join(testdataDir, "test-session.jsonl"), path)
	})

	t.Run("auto-detect picks most recent file", func(t *testing.T) {
		// Explicitly set older-session.jsonl to a past mtime so the test is
		// deterministic regardless of clone state (git doesn't preserve mtimes).
		oldTime := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
		require.NoError(t, os.Chtimes(filepath.Join(testdataDir, "older-session.jsonl"), oldTime, oldTime))

		path, err := findJSONLPath(testdataDir, "")
		require.NoError(t, err)
		assert.Contains(t, path, "test-session.jsonl")
	})

	t.Run("explicit session ID returns path without checking existence", func(t *testing.T) {
		path, err := findJSONLPath(testdataDir, "nonexistent")
		require.NoError(t, err)
		assert.Contains(t, path, "nonexistent.jsonl")
	})

	t.Run("missing directory", func(t *testing.T) {
		_, err := findJSONLPath("/nonexistent/path", "")
		assert.Error(t, err)
	})
}

func TestParseJSONL(t *testing.T) {
	data, rawLines, err := parseJSONL(filepath.Join(testdataDir, "test-session.jsonl"))
	require.NoError(t, err)

	// 6 valid JSON lines out of 7 (1 malformed)
	assert.Len(t, rawLines, 6)

	assert.Equal(t, "test-session", data.sessionID)
	assert.Equal(t, "test-slug", data.slug)
	assert.Equal(t, "2.1.81", data.version)
	assert.Equal(t, "/tmp/test", data.cwd)
	assert.Equal(t, "main", data.gitBranch)

	// Timestamps (min/max)
	assert.Equal(t, "2026-03-24T00:00:00Z", data.tsMin)
	assert.Equal(t, "2026-03-24T00:05:00Z", data.tsMax)

	// Message counts
	assert.Equal(t, 2, data.messageCounts["user"])
	assert.Equal(t, 3, data.messageCounts["assistant"])
	assert.Equal(t, 1, data.messageCounts["progress"])

	// Token usage
	opusUsage, ok := data.tokenUsageByModel["claude-opus-4-6"]
	require.True(t, ok)
	assert.Equal(t, 300, opusUsage.InputTokens)              // 100+150+50
	assert.Equal(t, 600, opusUsage.OutputTokens)             // 200+300+100
	assert.Equal(t, 130, opusUsage.CacheCreationInputTokens) // 50+0+80
	// Line 2 has only the flat field (back-compat: all 50 tokens attributed to 5m).
	// Line 6 has the nested cache_creation breakdown (30 -> 5m, 50 -> 1h).
	assert.Equal(t, 80, opusUsage.CacheCreation5mTokens)  // 50 (back-compat) + 0 + 30 (nested)
	assert.Equal(t, 50, opusUsage.CacheCreation1hTokens)  // 0 + 0 + 50 (nested)
	assert.Equal(t, 3500, opusUsage.CacheReadInputTokens) // 1000+2000+500

	// Tool counts
	assert.Equal(t, 2, data.toolCounts["Bash"])
	assert.Equal(t, 1, data.toolCounts["Read"])

	// Model seen counts
	assert.Equal(t, 3, data.modelsSeen["claude-opus-4-6"])

	// Malformed line warning
	assert.Len(t, data.warnings, 1)
	assert.Contains(t, data.warnings[0], "Malformed JSON")
}

func TestProcessSubagents(t *testing.T) {
	var warnings []string
	subagents, subUsage, rawSubagents, err := processSubagents(testdataDir, "test-session", &warnings)
	require.NoError(t, err)

	require.Len(t, subagents, 1)
	assert.Equal(t, "abc123", subagents[0].ID)
	assert.Equal(t, "Explore", subagents[0].Type)
	assert.Equal(t, "Explore codebase structure", subagents[0].Description)
	assert.Equal(t, 30, subagents[0].Tokens.Input)
	assert.Equal(t, 50, subagents[0].Tokens.Output)

	haikuUsage, ok := subUsage["claude-haiku-4-5-20251001"]
	require.True(t, ok)
	assert.Equal(t, 30, haikuUsage.InputTokens)
	assert.Equal(t, 50, haikuUsage.OutputTokens)

	// Raw subagent lines captured
	require.Contains(t, rawSubagents, "abc123")
	assert.Len(t, rawSubagents["abc123"], 2)
}

func TestProcessSubagentsNoDir(t *testing.T) {
	var warnings []string
	subagents, subUsage, rawSubagents, err := processSubagents(testdataDir, "nonexistent", &warnings)
	require.NoError(t, err)
	assert.Nil(t, subagents)
	assert.Nil(t, subUsage)
	assert.Nil(t, rawSubagents)
}

func TestMergeUsage(t *testing.T) {
	main := map[string]*tokenUsage{
		"opus": {InputTokens: 100, OutputTokens: 200},
	}
	sub := map[string]*tokenUsage{
		"opus":  {InputTokens: 50, OutputTokens: 100},
		"haiku": {InputTokens: 30, OutputTokens: 50},
	}

	merged := mergeUsage(main, sub)
	assert.Equal(t, 150, merged["opus"].InputTokens)
	assert.Equal(t, 300, merged["opus"].OutputTokens)
	assert.Equal(t, 30, merged["haiku"].InputTokens)
	assert.Equal(t, 50, merged["haiku"].OutputTokens)
}

func TestMergeUsageEmptySub(t *testing.T) {
	main := map[string]*tokenUsage{
		"opus": {InputTokens: 100},
	}
	merged := mergeUsage(main, nil)
	assert.Same(t, main["opus"], merged["opus"])
}

func TestComputeCost(t *testing.T) {
	usage := map[string]*tokenUsage{
		"claude-opus-4-6": {
			InputTokens:              1_000_000,
			OutputTokens:             1_000_000,
			CacheCreationInputTokens: 1_000_000,
			CacheCreation5mTokens:    1_000_000,
			CacheReadInputTokens:     1_000_000,
		},
	}
	var warnings []string
	cost := computeCost(usage, &warnings)

	// $5 input + $25 output + $6.25 cache write (5m) + $0.50 cache read = $36.75
	assert.InDelta(t, 36.75, cost, 0.001)
	assert.Empty(t, warnings)
}

func TestComputeCost_SplitCacheTiers(t *testing.T) {
	usage := map[string]*tokenUsage{
		"claude-opus-4-6": {
			CacheCreationInputTokens: 1_000_000,
			CacheCreation5mTokens:    400_000,
			CacheCreation1hTokens:    600_000,
		},
	}
	var warnings []string
	cost := computeCost(usage, &warnings)

	// 0.4M * $6.25 = $2.50  (5m tier)
	// 0.6M * $10.00 = $6.00 (1h tier)
	// total = $8.50
	assert.InDelta(t, 8.50, cost, 0.001)
	assert.Empty(t, warnings)
}

func TestComputeCostUnknownModel(t *testing.T) {
	usage := map[string]*tokenUsage{
		"claude-unknown-99": {InputTokens: 1_000_000},
	}
	var warnings []string
	cost := computeCost(usage, &warnings)

	// Falls back to sonnet pricing: $3.00 per 1M input
	assert.InDelta(t, 3.0, cost, 0.001)
	assert.Len(t, warnings, 1)
	assert.Contains(t, warnings[0], "Unknown model")
}

func TestParseSessionEndToEnd(t *testing.T) {
	provider := New()
	result, err := provider.ParseSession(context.Background(), &trace.ParseOpts{
		SessionDir: testdataDir,
		SessionID:  "test-session",
	})
	require.NoError(t, err)

	assert.Equal(t, "https://schemas.chainloop.dev/aicodingsession/0.1/ai-coding-session.schema.json", result.Schema)
	assert.Equal(t, "CHAINLOOP_AI_CODING_SESSION", result.ID)
	assert.Equal(t, "v1", result.Data.SchemaVersion)
	assert.Equal(t, "claude-code", result.Data.Agent.Name)
	assert.Equal(t, "test-session", result.Data.Session.ID)
	assert.Equal(t, "test-slug", result.Data.Session.Slug)

	assert.Equal(t, 300, result.Data.Session.DurationSeconds) // 5 minutes

	// Git context
	assert.Equal(t, "main", result.Data.GitContext.Branch)
	assert.Equal(t, "/tmp/test", result.Data.GitContext.WorkDir)

	assert.Equal(t, "claude-opus-4-6", result.Data.Model.Primary)
	assert.Contains(t, result.Data.Model.ModelsUsed, "claude-opus-4-6")
	assert.Contains(t, result.Data.Model.ModelsUsed, "claude-haiku-4-5-20251001")

	// Subagents included
	assert.Len(t, result.Data.Subagents, 1)
	assert.Equal(t, "Explore", result.Data.Subagents[0].Type)

	// Raw session data (flat map: "main" + subagent IDs)
	assert.Len(t, result.Data.RawSession["main"], 6) // 6 valid lines, 1 malformed skipped
	require.Contains(t, result.Data.RawSession, "abc123")
	assert.Len(t, result.Data.RawSession["abc123"], 2)

	// Warnings from malformed line
	assert.Len(t, result.Data.Warnings, 1)

	// Write to temp file and verify
	tmpDir := t.TempDir()
	outPath := filepath.Join(tmpDir, "output.json")
	f, err := os.Create(outPath)
	require.NoError(t, err)

	enc := json.NewEncoder(f)
	require.NoError(t, enc.Encode(result))
	require.NoError(t, f.Close())

	data, err := os.ReadFile(outPath)
	require.NoError(t, err)
	assert.Contains(t, string(data), "CHAINLOOP_AI_CODING_SESSION")
}

// TestParseSession_CraftingCompatibility verifies that the generated evidence
// can be deserialized by the upstream aicodingsession types and passes schema validation.
// This mirrors what ChainloopAICodingSessionCrafter.Craft() does when processing the file.
func TestParseSession_CraftingCompatibility(t *testing.T) {
	provider := New()
	result, err := provider.ParseSession(context.Background(), &trace.ParseOpts{
		SessionDir: testdataDir,
		SessionID:  "test-session",
	})
	require.NoError(t, err)

	// Serialize as the push flow does
	resultBytes, err := json.Marshal(result)
	require.NoError(t, err)

	// Step 1: Unmarshal into upstream envelope (same as Craft does)
	var envelope struct {
		Data json.RawMessage `json:"data"`
	}
	require.NoError(t, json.Unmarshal(resultBytes, &envelope))
	require.NotEmpty(t, envelope.Data)

	// Step 2: Verify upstream struct compatibility — this catches type mismatches
	// like the raw_session format issue (object vs array) that caused crafting failures.
	var upstreamData aicodingsession.Data
	require.NoError(t, json.Unmarshal(envelope.Data, &upstreamData), "evidence must unmarshal into upstream aicodingsession.Data")
	assert.Equal(t, "claude-code", upstreamData.Agent.Name)
	assert.Equal(t, "test-session", upstreamData.Session.ID)
	assert.Equal(t, 300, upstreamData.Session.DurationSeconds)

	// Verify raw_session survives the round-trip as a flat map
	require.Contains(t, upstreamData.RawSession, "main")
	assert.NotEmpty(t, upstreamData.RawSession["main"])

	// Verify the full evidence round-trips through upstream Evidence type
	var upstreamEvidence aicodingsession.Evidence
	require.NoError(t, json.Unmarshal(resultBytes, &upstreamEvidence), "evidence must unmarshal into upstream aicodingsession.Evidence")
	assert.Equal(t, aicodingsession.EvidenceID, upstreamEvidence.ID)
}
