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

package cursor

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/chainloop-dev/chainloop/app/cli/internal/trace"
	"github.com/chainloop-dev/chainloop/app/cli/internal/trace/state"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const sampleJSONL = `{"role":"user","message":{"content":[{"type":"text","text":"hello"}]}}
{"role":"assistant","message":{"content":[{"type":"text","text":"hi"}]}}
{"role":"assistant","message":{"content":[{"type":"text","text":"anything else?"}]}}
`

func TestParseSessionMessageCounts(t *testing.T) {
	rawDir := t.TempDir()
	path := filepath.Join(rawDir, "session-1.jsonl")
	require.NoError(t, os.WriteFile(path, []byte(sampleJSONL), 0600))

	p := New()
	result, err := p.ParseSession(context.Background(), &trace.ParseOpts{
		SessionDir: rawDir,
		SessionID:  "session-1",
	})
	require.NoError(t, err)

	assert.Equal(t, 1, result.Data.Conversation.UserMessages, "UserMessages")
	assert.Equal(t, 2, result.Data.Conversation.AssistantMessages, "AssistantMessages")
	assert.Equal(t, agentName, result.Data.Agent.Name, "Agent.Name")
	assert.Equal(t, "session-1", result.Data.Session.ID, "Session.ID")
	assert.NotEmpty(t, result.Data.Warnings, "expected at least one warning about missing token usage")
}

func TestParseSessionMissingFile(t *testing.T) {
	p := New()
	_, err := p.ParseSession(context.Background(), &trace.ParseOpts{
		SessionDir: t.TempDir(),
		SessionID:  "does-not-exist",
	})
	require.Error(t, err, "expected error for missing transcript")
}

// toolUseJSONL mirrors the shape observed in real Cursor transcripts:
// assistant messages can mix text blocks with tool_use blocks, and the
// same tool name can appear multiple times across records.
const toolUseJSONL = `{"role":"user","message":{"content":[{"type":"text","text":"do work"}]}}
{"role":"assistant","message":{"content":[{"type":"text","text":"reading"},{"type":"tool_use","name":"ReadFile","input":{"path":"/x"}},{"type":"tool_use","name":"ReadFile","input":{"path":"/y"}}]}}
{"role":"assistant","message":{"content":[{"type":"tool_use","name":"ApplyPatch","input":"*** Begin Patch ***"}]}}
{"role":"assistant","message":{"content":[{"type":"tool_use","name":"Grep","input":{"pattern":"foo"}},{"type":"tool_use","name":"ReadFile","input":{"path":"/z"}}]}}
`

func TestParseSessionPropagatesAgentVersion(t *testing.T) {
	rawDir := t.TempDir()
	path := filepath.Join(rawDir, "ver-1.jsonl")
	require.NoError(t, os.WriteFile(path, []byte(sampleJSONL), 0600))

	p := New()
	result, err := p.ParseSession(context.Background(), &trace.ParseOpts{
		SessionDir:   rawDir,
		SessionID:    "ver-1",
		AgentVersion: "3.2.16",
		Model:        "gpt-5",
	})
	require.NoError(t, err)

	assert.Equal(t, "3.2.16", result.Data.Agent.Version, "Agent.Version")
	require.NotNil(t, result.Data.Model, "Model")
	assert.Equal(t, "gpt-5", result.Data.Model.Primary, "Model.Primary")
}

func TestParseSessionExtractsToolUsage(t *testing.T) {
	rawDir := t.TempDir()
	path := filepath.Join(rawDir, "tools-1.jsonl")
	require.NoError(t, os.WriteFile(path, []byte(toolUseJSONL), 0600))

	p := New()
	result, err := p.ParseSession(context.Background(), &trace.ParseOpts{
		SessionDir: rawDir,
		SessionID:  "tools-1",
	})
	require.NoError(t, err)

	tools := result.Data.ToolsUsed
	require.NotNil(t, tools, "ToolsUsed is nil; expected populated summary")
	assert.Equal(t, 5, tools.TotalInvocations, "TotalInvocations")

	got := map[string]int{}
	for _, s := range tools.Summary {
		got[s.ToolName] = s.InvocationCount
	}
	want := map[string]int{"ReadFile": 3, "ApplyPatch": 1, "Grep": 1}
	for name, count := range want {
		assert.Equal(t, count, got[name], "tool %q", name)
	}

	// Highest-count tool must come first (sorted desc by count).
	require.NotEmpty(t, tools.Summary)
	assert.Equal(t, "ReadFile", tools.Summary[0].ToolName, "expected ReadFile first in summary")
}

func TestCopySessionDataFromFlat(t *testing.T) {
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)

	repoRoot := "/tmp/myrepo"
	sanitized := sanitizeRepoPath(repoRoot)
	src := filepath.Join(homeDir, ".cursor", "projects", sanitized, "agent-transcripts")
	require.NoError(t, os.MkdirAll(src, 0755))
	srcFile := filepath.Join(src, "conv-1.jsonl")
	require.NoError(t, os.WriteFile(srcFile, []byte(sampleJSONL), 0600))

	gitDir := filepath.Join(t.TempDir(), ".git")
	require.NoError(t, os.MkdirAll(gitDir, 0755))

	p := New()
	require.NoError(t, p.CopySessionData(state.NewGitStore(gitDir), repoRoot, "conv-1"))

	dst := filepath.Join(gitDir, "chainloop-trace", "raw", "conv-1.jsonl")
	got, err := os.ReadFile(dst)
	require.NoError(t, err, "read destination")
	require.Equal(t, sampleJSONL, string(got), "destination content mismatch")
}

func TestCopySessionDataFromNested(t *testing.T) {
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)

	repoRoot := "/tmp/myrepo"
	sanitized := sanitizeRepoPath(repoRoot)
	nested := filepath.Join(homeDir, ".cursor", "projects", sanitized, "agent-transcripts", "conv-2")
	require.NoError(t, os.MkdirAll(nested, 0755))
	srcFile := filepath.Join(nested, "conv-2.jsonl")
	require.NoError(t, os.WriteFile(srcFile, []byte(sampleJSONL), 0600))

	gitDir := filepath.Join(t.TempDir(), ".git")
	require.NoError(t, os.MkdirAll(gitDir, 0755))

	p := New()
	require.NoError(t, p.CopySessionData(state.NewGitStore(gitDir), repoRoot, "conv-2"))

	dst := filepath.Join(gitDir, "chainloop-trace", "raw", "conv-2.jsonl")
	_, err := os.Stat(dst)
	require.NoError(t, err, "destination missing")
}

func TestParseCursorJSONLSkipsMalformedLines(t *testing.T) {
	path := filepath.Join(t.TempDir(), "session-1.jsonl")
	// Trailing line is truncated, as happens when Cursor is still writing.
	content := sampleJSONL + "   \n" + `{"role":"assistant","message":{"con`
	require.NoError(t, os.WriteFile(path, []byte(content), 0600))

	parsed, err := parseCursorJSONL(path)
	require.NoError(t, err)

	// An invalid raw record fails the evidence encoder and aborts the attestation.
	_, err = json.Marshal(parsed.RawRecords)
	assert.NoErrorf(t, err, "marshal RawRecords (got %d records, want 3)", len(parsed.RawRecords))
}
