package opencode

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/chainloop-dev/chainloop/pkg/attestation/crafter/materials/aicodingsession"
	"github.com/chainloop-dev/chainloop/app/cli/internal/trace"
	"github.com/chainloop-dev/chainloop/app/cli/internal/trace/state"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// sampleExport is a minimal opencode export JSON with one user message,
// two assistant messages (with tokens/cost), and a completed tool part.
const sampleExport = `{
  "info": {
    "id": "sess_abc123",
    "slug": "refactor-auth",
    "title": "Refactor auth module",
    "directory": "/home/user/project",
    "version": "0.5.0",
    "cost": 0.0523,
    "tokens": {
      "input": 12000,
      "output": 3000,
      "reasoning": 500,
      "cache": { "read": 2000, "write": 1000 }
    },
    "time": { "created": 1751702400000, "updated": 1751702700000 }
  },
  "messages": [
    {
      "info": {
        "role": "user",
        "id": "msg_001",
        "model": { "providerID": "anthropic", "modelID": "claude-sonnet-4-20250514" },
        "time": { "created": 1751702400000 }
      },
      "parts": [{ "type": "text", "id": "prt_001", "text": "Fix the auth bug" }]
    },
    {
      "info": {
        "role": "assistant",
        "id": "msg_002",
        "modelID": "claude-sonnet-4-20250514",
        "providerID": "anthropic",
        "cost": 0.0261,
        "tokens": {
          "input": 6000,
          "output": 1500,
          "reasoning": 250,
          "cache": { "read": 1000, "write": 500 }
        },
        "time": { "created": 1751702500000, "completed": 1751702550000 }
      },
      "parts": [
        { "type": "text", "id": "prt_002", "text": "I'll fix the auth bug." },
        { "type": "tool", "id": "prt_003", "tool": "edit", "state": { "status": "completed" } },
        { "type": "tool", "id": "prt_004", "tool": "read", "state": { "status": "completed" } }
      ]
    },
    {
      "info": {
        "role": "assistant",
        "id": "msg_003",
        "modelID": "claude-sonnet-4-20250514",
        "providerID": "anthropic",
        "cost": 0.0262,
        "tokens": {
          "input": 6000,
          "output": 1500,
          "reasoning": 250,
          "cache": { "read": 1000, "write": 500 }
        },
        "time": { "created": 1751702600000, "completed": 1751702700000 }
      },
      "parts": [
        { "type": "tool", "id": "prt_005", "tool": "edit", "state": { "status": "completed" } }
      ]
    }
  ]
}`

func TestParseExport(t *testing.T) {
	rawDir := t.TempDir()
	path := filepath.Join(rawDir, "sess_abc123.jsonl")
	require.NoError(t, os.WriteFile(path, []byte(sampleExport), 0600))

	p := New()
	result, err := p.ParseSession(context.Background(), &trace.ParseOpts{
		SessionDir: rawDir,
		SessionID:  "sess_abc123",
	})
	require.NoError(t, err)

	assert.Equal(t, "CHAINLOOP_AI_CODING_SESSION", result.ID)
	assert.Equal(t, "v1", result.Data.SchemaVersion)
	assert.Equal(t, "opencode", result.Data.Agent.Name)
	assert.Equal(t, "0.5.0", result.Data.Agent.Version)
	assert.Equal(t, "sess_abc123", result.Data.Session.ID)
	assert.Equal(t, "refactor-auth", result.Data.Session.Slug)

	// Duration: 5 minutes = 300 seconds
	assert.Equal(t, 300, result.Data.Session.DurationSeconds)

	// Model
	require.NotNil(t, result.Data.Model)
	assert.Equal(t, "claude-sonnet-4-20250514", result.Data.Model.Primary)
	assert.Equal(t, "anthropic", result.Data.Model.Provider)
	assert.Contains(t, result.Data.Model.ModelsUsed, "claude-sonnet-4-20250514")

	// Usage: session-level tokens are authoritative when present.
	require.NotNil(t, result.Data.Usage)
	assert.Equal(t, 12000, result.Data.Usage.InputTokens)
	assert.Equal(t, 3000, result.Data.Usage.OutputTokens)
	assert.Equal(t, 15000, result.Data.Usage.TotalTokens)
	assert.Equal(t, 2000, result.Data.Usage.CacheReadInputTokens)
	assert.Equal(t, 1000, result.Data.Usage.CacheCreationInputTokens)
	assert.InDelta(t, 0.0523, result.Data.Usage.EstimatedCostUSD, 0.0001)

	// Tools: 2 edits + 1 read = 3 total
	require.NotNil(t, result.Data.ToolsUsed)
	assert.Equal(t, 3, result.Data.ToolsUsed.TotalInvocations)

	got := map[string]int{}
	for _, s := range result.Data.ToolsUsed.Summary {
		got[s.ToolName] = s.InvocationCount
	}
	assert.Equal(t, 2, got["edit"])
	assert.Equal(t, 1, got["read"])

	// Highest-count tool must come first (sorted desc by count).
	assert.Equal(t, "edit", result.Data.ToolsUsed.Summary[0].ToolName)

	// Conversation
	require.NotNil(t, result.Data.Conversation)
	assert.Equal(t, 1, result.Data.Conversation.UserMessages)
	assert.Equal(t, 2, result.Data.Conversation.AssistantMessages)
	assert.Equal(t, 3, result.Data.Conversation.TotalMessages)

	// Raw session: one entry per message (3 messages = 3 entries)
	require.Contains(t, result.Data.RawSession, "main")
	assert.Len(t, result.Data.RawSession["main"], 3)

	// Verify the first entry is a user message with text content
	var firstEntry map[string]any
	require.NoError(t, json.Unmarshal(result.Data.RawSession["main"][0], &firstEntry))
	assert.Equal(t, "user", firstEntry["type"])
	assert.Equal(t, "msg_001", firstEntry["uuid"])

	msg, ok := firstEntry["message"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "user", msg["role"])

	content, ok := msg["content"].([]any)
	require.True(t, ok)
	require.Len(t, content, 1)
	block := content[0].(map[string]any)
	assert.Equal(t, "text", block["type"])
	assert.Equal(t, "Fix the auth bug", block["text"])

	// Verify the second entry is an assistant message with text + tool_use blocks
	var secondEntry map[string]any
	require.NoError(t, json.Unmarshal(result.Data.RawSession["main"][1], &secondEntry))
	assert.Equal(t, "assistant", secondEntry["type"])
	assert.Equal(t, "msg_002", secondEntry["uuid"])

	msg2, ok := secondEntry["message"].(map[string]any)
	require.True(t, ok)
	content2, ok := msg2["content"].([]any)
	require.True(t, ok)
	// 1 text block + 2 tool_use blocks
	require.Len(t, content2, 3)
	assert.Equal(t, "text", content2[0].(map[string]any)["type"])
	assert.Equal(t, "tool_use", content2[1].(map[string]any)["type"])
	assert.Equal(t, "edit", content2[1].(map[string]any)["name"])
	assert.Equal(t, "tool_use", content2[2].(map[string]any)["type"])
	assert.Equal(t, "read", content2[2].(map[string]any)["name"])
}

func TestParseExportFallsBackToMessageTokens(t *testing.T) {
	// When session-level tokens are absent, the parser sums per-message tokens.
	export := `{
  "info": {
    "id": "sess_notokens",
    "title": "No session tokens",
    "directory": "/repo",
    "version": "0.5.0",
    "time": { "created": 1751702400000, "updated": 1751702700000 }
  },
  "messages": [
    {
      "info": { "role": "assistant", "id": "m1", "modelID": "gpt-4o", "providerID": "openai", "cost": 0.01,
        "tokens": { "input": 100, "output": 50, "reasoning": 10, "cache": { "read": 5, "write": 3 } } }
    },
    {
      "info": { "role": "assistant", "id": "m2", "modelID": "gpt-4o", "providerID": "openai", "cost": 0.02,
        "tokens": { "input": 200, "output": 100, "reasoning": 20, "cache": { "read": 10, "write": 7 } } }
    }
  ]
}`

	rawDir := t.TempDir()
	path := filepath.Join(rawDir, state.SanitizeID("sess_notokens")+".jsonl")
	require.NoError(t, os.WriteFile(path, []byte(export), 0600))

	p := New()
	result, err := p.ParseSession(context.Background(), &trace.ParseOpts{
		SessionDir: rawDir,
		SessionID:  "sess_notokens",
	})
	require.NoError(t, err)

	require.NotNil(t, result.Data.Usage)
	assert.Equal(t, 300, result.Data.Usage.InputTokens)  // 100 + 200
	assert.Equal(t, 150, result.Data.Usage.OutputTokens) // 50 + 100
	assert.Equal(t, 450, result.Data.Usage.TotalTokens)
	assert.Equal(t, 15, result.Data.Usage.CacheReadInputTokens)     // 5 + 10
	assert.Equal(t, 10, result.Data.Usage.CacheCreationInputTokens) // 3 + 7
	assert.InDelta(t, 0.03, result.Data.Usage.EstimatedCostUSD, 0.0001)
}

func TestParseExportEmptySession(t *testing.T) {
	export := `{
  "info": {
    "id": "sess_empty",
    "title": "Empty",
    "directory": "/repo",
    "version": "0.5.0",
    "time": { "created": 1751702400000, "updated": 1751702400000 }
  },
  "messages": []
}`

	rawDir := t.TempDir()
	path := filepath.Join(rawDir, state.SanitizeID("sess_empty")+".jsonl")
	require.NoError(t, os.WriteFile(path, []byte(export), 0600))

	p := New()
	result, err := p.ParseSession(context.Background(), &trace.ParseOpts{
		SessionDir: rawDir,
		SessionID:  "sess_empty",
	})
	require.NoError(t, err)

	assert.Equal(t, "opencode", result.Data.Agent.Name)
	assert.Equal(t, "sess_empty", result.Data.Session.ID)
	assert.Equal(t, 0, result.Data.Session.DurationSeconds)
	require.NotNil(t, result.Data.Usage)
	assert.Equal(t, 0, result.Data.Usage.InputTokens)
	require.NotNil(t, result.Data.ToolsUsed)
	assert.Equal(t, 0, result.Data.ToolsUsed.TotalInvocations)
	require.NotNil(t, result.Data.Conversation)
	assert.Equal(t, 0, result.Data.Conversation.TotalMessages)
}

func TestParseExportMissingFile(t *testing.T) {
	p := New()
	_, err := p.ParseSession(context.Background(), &trace.ParseOpts{
		SessionDir: t.TempDir(),
		SessionID:  "does-not-exist",
	})
	assert.Error(t, err)
}

func TestParseExportInvalidJSON(t *testing.T) {
	rawDir := t.TempDir()
	path := filepath.Join(rawDir, state.SanitizeID("bad")+".jsonl")
	require.NoError(t, os.WriteFile(path, []byte("not json"), 0600))

	p := New()
	_, err := p.ParseSession(context.Background(), &trace.ParseOpts{
		SessionDir: rawDir,
		SessionID:  "bad",
	})
	assert.Error(t, err)
}

func TestParseExportCraftingCompatibility(t *testing.T) {
	rawDir := t.TempDir()
	path := filepath.Join(rawDir, "sess_abc123.jsonl")
	require.NoError(t, os.WriteFile(path, []byte(sampleExport), 0600))

	p := New()
	result, err := p.ParseSession(context.Background(), &trace.ParseOpts{
		SessionDir: rawDir,
		SessionID:  "sess_abc123",
	})
	require.NoError(t, err)

	resultBytes, err := json.Marshal(result)
	require.NoError(t, err)

	var envelope struct {
		Data json.RawMessage `json:"data"`
	}
	require.NoError(t, json.Unmarshal(resultBytes, &envelope))
	require.NotEmpty(t, envelope.Data)

	var upstreamData aicodingsession.Data
	require.NoError(t, json.Unmarshal(envelope.Data, &upstreamData))
	assert.Equal(t, "opencode", upstreamData.Agent.Name)
	assert.Equal(t, "sess_abc123", upstreamData.Session.ID)
	require.Contains(t, upstreamData.RawSession, "main")
	assert.NotEmpty(t, upstreamData.RawSession["main"])
}

func TestFormatEpochMs(t *testing.T) {
	assert.Equal(t, "2025-07-05T08:00:00Z", formatEpochMs(1751702400000))
	assert.Equal(t, "", formatEpochMs(0))
	assert.Equal(t, "", formatEpochMs(-1))
}
