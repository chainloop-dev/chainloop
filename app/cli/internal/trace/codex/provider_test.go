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

package codex

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/chainloop-dev/chainloop/app/cli/internal/trace"
	"github.com/chainloop-dev/chainloop/app/cli/internal/trace/state"
	"github.com/chainloop-dev/chainloop/internal/schemavalidators"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const testSessionID = "sess-1"
const testModel = "gpt-5-mini"

func TestHooks(t *testing.T) {
	for _, run := range []bool{false, true} {
		t.Run(map[bool]string{false: "init", true: "run"}[run], func(t *testing.T) {
			p := New()
			root := t.TempDir()
			path := p.SettingsFile(root)
			require.NoError(t, os.MkdirAll(filepath.Dir(path), 0700))
			require.NoError(t, os.WriteFile(path, []byte(`{"description":"user settings","hooks":{"PostToolUse":[{"hooks":[{"type":"command","command":"echo user-hook"}]}]}}`), 0600))
			installed, err := p.HooksInstalled(root)
			require.NoError(t, err)
			require.False(t, installed)
			install := p.InstallHooks
			if run {
				install = p.InstallHooksForTraceRun
			}
			require.NoError(t, install(root))
			first, err := os.ReadFile(path)
			require.NoError(t, err)
			require.NoError(t, install(root))
			second, err := os.ReadFile(path)
			require.NoError(t, err)
			assert.Equal(t, first, second)
			installed, err = p.HooksInstalled(root)
			require.NoError(t, err)
			assert.True(t, installed)
			assert.Contains(t, string(first), "echo user-hook")
			assert.Contains(t, string(first), "apply_patch")
			assert.Equal(t, !run, strings.Contains(string(first), `"SessionEnd"`))
			require.NoError(t, p.UninstallHooks(root))
			left, err := os.ReadFile(path)
			require.NoError(t, err)
			assert.Contains(t, string(left), "echo user-hook")
			assert.Contains(t, string(left), "user settings")
			assert.NotContains(t, string(left), hookMarker)
			require.NoError(t, p.UninstallHooks(root))
		})
	}
}

func TestReadHookInput(t *testing.T) {
	for _, tc := range []struct {
		name, tool, command string
		paths               []string
	}{
		{"shell", "Bash", "git status", nil},
		{"patch", "apply_patch", "*** Begin Patch\n*** Update File: one.go\n@@\n-old\n+new\n*** Add File: two.go\n+new\n*** Delete File: gone.go\n*** Update File: old.go\n*** Move to: new.go\n@@\n-old\n+new\n*** End Patch", []string{"one.go", "two.go", "gone.go", "old.go", "new.go"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			raw, _ := json.Marshal(map[string]any{"session_id": testSessionID, "cwd": root, "transcript_path": "/sessions/rollout.jsonl", "tool_name": tc.tool, "tool_use_id": "call-1", "agent_id": "child-1", "model": testModel, "tool_input": map[string]string{"command": tc.command}})
			input, err := New().ReadHookInput(strings.NewReader(string(raw)))
			require.NoError(t, err)
			assert.Equal(t, testSessionID, input.SessionID)
			assert.Equal(t, "call-1", input.ToolUseID)
			assert.Equal(t, "child-1", input.AgentID)
			assert.Equal(t, testModel, input.Model)
			if tc.tool == "Bash" {
				assert.Equal(t, tc.command, input.Command)
			} else {
				var paths []string
				for _, path := range tc.paths {
					paths = append(paths, filepath.Join(root, path))
				}
				assert.Equal(t, paths, input.FilePaths)
			}
		})
	}
	_, err := New().ReadHookInput(strings.NewReader(`{`))
	assert.Error(t, err)
}

func TestParseSession(t *testing.T) {
	dir := t.TempDir()
	raw, err := os.ReadFile("testdata/session.jsonl")
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(state.RawSessionPath(dir, testSessionID), raw, 0600)) //nolint:gosec // temporary fixture path and a fixed session ID
	evidence, err := New().ParseSession(context.Background(), &trace.ParseOpts{SessionDir: dir, SessionID: testSessionID})
	require.NoError(t, err)
	d := evidence.Data
	assert.Equal(t, "v2", d.SchemaVersion)
	assert.Equal(t, Name, d.Agent.Name)
	assert.Equal(t, "0.163.0", d.Agent.Version)
	assert.Equal(t, 60, d.Session.DurationSeconds)
	assert.Equal(t, testModel, d.Model.Primary)
	assert.Equal(t, "openai", d.Model.Provider)
	assert.Equal(t, 900, d.Usage.InputTokens)
	assert.Equal(t, 400, d.Usage.OutputTokens)
	assert.Equal(t, 1300, d.Usage.TotalTokens)
	assert.Equal(t, 1000, d.Usage.CacheReadInputTokens)
	assert.Equal(t, 100, d.Usage.CacheCreationInputTokens)
	assert.InDelta(t, 0.0011, d.Usage.EstimatedCostUSD, 0.00001)
	assert.Equal(t, 2, d.Conversation.TotalMessages)
	assert.Equal(t, 2, d.ToolsUsed.TotalInvocations)
	assert.Equal(t, 1, d.Conversation.UserMessages)
	assert.Equal(t, 1, d.Conversation.AssistantMessages)
	assert.Len(t, d.RawSession["main"], 12)
	assert.Empty(t, d.Warnings)
	doc, err := json.Marshal(d)
	require.NoError(t, err)
	var data map[string]any
	require.NoError(t, json.Unmarshal(doc, &data))
	require.NoError(t, schemavalidators.ValidateAICodingSession(data, schemavalidators.AICodingSessionVersion0_1))
	assert.Equal(t, "v2", data["schema_version"])
	// Verify the actual serialized evidence keeps both cache categories while
	// input and total follow the same convention as Claude.
	usage, ok := data["usage"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, float64(900), usage["input_tokens"])
	assert.Equal(t, float64(1300), usage["total_tokens"])
	assert.Equal(t, float64(1000), usage["cache_read_input_tokens"])
	assert.Equal(t, float64(100), usage["cache_creation_input_tokens"])
}

func TestCopySessionData(t *testing.T) {
	home := t.TempDir()
	t.Setenv("CODEX_HOME", home)
	p := New()
	store := state.NewOutOfTreeStore(t.TempDir())
	raw, err := os.ReadFile("testdata/session.jsonl")
	require.NoError(t, err)
	path := filepath.Join(home, "sessions", "2026", "10", "21", "rollout-time-sess-1.jsonl")
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0700))
	require.NoError(t, os.WriteFile(path, raw, 0600)) //nolint:gosec // temporary Codex session fixture
	for _, reported := range []string{path, "", filepath.Join(home, "sessions", "moved.jsonl")} {
		require.NoError(t, p.CopySessionData(store, trace.SessionLocation{SessionID: testSessionID, TranscriptPath: reported}))
		got, err := os.ReadFile(state.RawSessionPath(store.RawSessionDir(), testSessionID))
		require.NoError(t, err)
		assert.Equal(t, raw, got)
	}
	outside := filepath.Join(t.TempDir(), "rollout-time-sess-1.jsonl")
	require.NoError(t, os.WriteFile(outside, raw, 0600)) //nolint:gosec // deliberately outside storage to verify rejection
	assert.Error(t, p.CopySessionData(store, trace.SessionLocation{SessionID: testSessionID, TranscriptPath: outside}))
	link := filepath.Join(home, "sessions", "rollout-link-sess-1.jsonl")
	require.NoError(t, os.Symlink(outside, link))
	assert.Error(t, p.CopySessionData(store, trace.SessionLocation{SessionID: testSessionID, TranscriptPath: link}))
	require.NoError(t, p.CopySessionData(store, trace.SessionLocation{SessionID: "missing"}))
	assert.Error(t, p.CopySessionData(store, trace.SessionLocation{SessionID: "../escape"}))
}

func TestUninstallKeepsMixedUserHookGroup(t *testing.T) {
	p := New()
	root := t.TempDir()
	path := p.SettingsFile(root)
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0700))
	require.NoError(t, os.WriteFile(path, []byte(`{"hooks":{"PostToolUse":[{"matcher":"Bash","hooks":[{"type":"command","command":"chainloop trace hook codex post-tool-use"},{"type":"command","command":"echo keep-me"}]}]}}`), 0600))
	require.NoError(t, p.InstallHooks(root))
	require.NoError(t, p.UninstallHooks(root))
	raw, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Contains(t, string(raw), "echo keep-me")
	assert.NotContains(t, string(raw), hookMarker)
}

func TestParseSessionWarnings(t *testing.T) {
	raw, err := os.ReadFile("testdata/session.jsonl")
	require.NoError(t, err)
	for _, tc := range []struct {
		name    string
		data    string
		wantErr bool
		warning string
	}{
		{name: "truncated tail", data: string(raw) + `{"type":`, warning: "invalid Codex rollout record"},
		{name: "wrong session", data: strings.ReplaceAll(string(raw), testSessionID, "other"), wantErr: true},
		{name: "no header", data: `{"type":"event_msg","timestamp":"2026-10-21T10:00:00Z","payload":{"type":"other"}}`, wantErr: true},
		{name: "unknown model", data: strings.ReplaceAll(string(raw), testModel, "future-model"), warning: "unknown Codex model"},
		{name: "no usage", data: strings.ReplaceAll(string(raw), "token_count", "other"), warning: "no token usage"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			require.NoError(t, os.WriteFile(state.RawSessionPath(dir, testSessionID), []byte(tc.data), 0600))
			e, err := New().ParseSession(t.Context(), &trace.ParseOpts{SessionDir: dir, SessionID: testSessionID})
			if tc.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Contains(t, strings.Join(e.Data.Warnings, "\n"), tc.warning)
		})
	}
}

func TestTraceRunOmitsPreviouslyInstalledEndHook(t *testing.T) {
	root := t.TempDir()
	p := New()
	require.NoError(t, p.InstallHooks(root))
	require.NoError(t, p.InstallHooksForTraceRun(root))
	raw, err := os.ReadFile(p.SettingsFile(root))
	require.NoError(t, err)
	assert.NotContains(t, string(raw), `"SessionEnd"`)
}

func TestUsageAcrossModels(t *testing.T) {
	dir := t.TempDir()
	raw, err := os.ReadFile("testdata/session.jsonl")
	require.NoError(t, err)
	// The second model owns only the increment after the first cumulative snapshot.
	extra := `{"timestamp":"2026-10-21T10:01:00Z","type":"token_usage_record","payload":{"thread_token_usage":{"input_tokens":2000,"cached_input_tokens":1000,"cache_write_input_tokens":100,"output_tokens":400,"total_tokens":2400}}}
{"timestamp":"2026-10-21T10:01:01Z","type":"turn_context","payload":{"model":"gpt-5"}}
{"timestamp":"2026-10-21T10:01:02Z","type":"response_item","payload":{"type":"message","role":"assistant","content":[]}}
{"timestamp":"2026-10-21T10:01:03Z","type":"response_item","payload":{"type":"message","role":"assistant","content":[]}}
{"timestamp":"2026-10-21T10:01:04Z","type":"token_usage_record","payload":{"usage":{"input_tokens":2000,"output_tokens":404,"total_tokens":2404},"thread_token_usage":{"input_tokens":4000,"cached_input_tokens":2000,"cache_write_input_tokens":200,"output_tokens":804,"total_tokens":4804}}}
{"timestamp":"2026-10-21T10:01:04Z","type":"event_msg","payload":{"type":"token_count","info":{"total_token_usage":{"input_tokens":4000,"cached_input_tokens":2000,"cache_write_input_tokens":200,"output_tokens":804,"total_tokens":4804}}}}
`
	raw = append(raw, extra...)
	require.NoError(t, os.WriteFile(state.RawSessionPath(dir, testSessionID), raw, 0600)) //nolint:gosec // temporary fixture and a fixed session ID
	e, err := New().ParseSession(t.Context(), &trace.ParseOpts{SessionDir: dir, SessionID: testSessionID})
	require.NoError(t, err)
	assert.Equal(t, []string{"gpt-5", testModel}, e.Data.Model.ModelsUsed)
	assert.Equal(t, "gpt-5", e.Data.Model.Primary)
	assert.Equal(t, 1800, e.Data.Usage.InputTokens)
	assert.Equal(t, 2604, e.Data.Usage.TotalTokens)
	assert.InDelta(t, 0.0065, e.Data.Usage.EstimatedCostUSD, 0.00001)
}

func TestNativeRollout(t *testing.T) {
	raw, err := os.ReadFile("testdata/native-session.jsonl")
	require.NoError(t, err)
	lines := strings.SplitAfter(string(raw), "\n")
	for _, tc := range []struct {
		name, data string
	}{
		{"before mirrored event", strings.Join(lines[:6], "")},
		{"with mirrored event", string(raw)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			require.NoError(t, os.WriteFile(state.RawSessionPath(dir, testSessionID), []byte(tc.data), 0600))
			e, err := New().ParseSession(t.Context(), &trace.ParseOpts{SessionDir: dir, SessionID: testSessionID})
			require.NoError(t, err)
			require.NotNil(t, e.Data.Usage)
			assert.Equal(t, 3, e.Data.Usage.InputTokens)
			assert.Equal(t, 128, e.Data.Usage.OutputTokens)
			assert.Equal(t, 131, e.Data.Usage.TotalTokens)
			assert.Equal(t, 19061, e.Data.Usage.CacheCreationInputTokens)
			assert.InDelta(t, 0.0489, e.Data.Usage.EstimatedCostUSD, 0.00001)
			assert.NotEmpty(t, e.Data.Model.Primary)
			assert.NotEmpty(t, e.Data.ToolsUsed.Summary)
			assert.Empty(t, e.Data.Warnings)
		})
	}
}

func TestCopySessionDataPreservesStorageRoot(t *testing.T) {
	for _, env := range []string{"unset", "different home"} {
		t.Run(env, func(t *testing.T) {
			home := t.TempDir()
			t.Setenv("CODEX_HOME", home)
			store := state.NewOutOfTreeStore(t.TempDir())
			require.NoError(t, store.SaveSessionRecord(&state.SessionRecord{SessionID: testSessionID, Provider: Name, Active: true}))
			raw, err := os.ReadFile("testdata/session.jsonl")
			require.NoError(t, err)
			path := filepath.Join(home, "sessions", "rollout-time-"+testSessionID+".jsonl")
			require.NoError(t, os.MkdirAll(filepath.Dir(path), 0700))
			require.NoError(t, os.WriteFile(path, raw, 0600)) //nolint:gosec // temporary Codex storage fixture and a fixed session ID
			loc := trace.SessionLocation{SessionID: testSessionID, TranscriptPath: path}
			require.NoError(t, New().CopySessionData(store, loc))
			rec, err := store.LoadSessionRecord(testSessionID)
			require.NoError(t, err)
			canonicalHome, err := filepath.EvalSymlinks(home)
			require.NoError(t, err)
			assert.Equal(t, canonicalHome, rec.CodexStorageRoot)
			assert.True(t, rec.Active)

			updated := append(append([]byte(nil), raw...), []byte("\n{\"new_record\":true}\n")...)
			require.NoError(t, os.WriteFile(path, updated, 0600)) //nolint:gosec // temporary Codex storage fixture and a fixed session ID
			if env == "unset" {
				require.NoError(t, os.Unsetenv("CODEX_HOME"))
			} else {
				t.Setenv("CODEX_HOME", t.TempDir())
			}
			// A new provider instance represents a later Git hook process.
			require.NoError(t, New().CopySessionData(store, loc))
			got, err := os.ReadFile(state.RawSessionPath(store.RawSessionDir(), testSessionID))
			require.NoError(t, err)
			assert.Equal(t, updated, got)

			archived := filepath.Join(home, "archived_sessions", filepath.Base(path))
			require.NoError(t, os.MkdirAll(filepath.Dir(archived), 0700))
			require.NoError(t, os.Rename(path, archived))
			require.NoError(t, New().CopySessionData(store, loc))
			got, err = os.ReadFile(state.RawSessionPath(store.RawSessionDir(), testSessionID))
			require.NoError(t, err)
			assert.Equal(t, updated, got)

			outside := filepath.Join(t.TempDir(), "outside.jsonl")
			require.NoError(t, os.WriteFile(outside, raw, 0600)) //nolint:gosec // deliberately outside storage to verify rejection
			assert.Error(t, New().CopySessionData(store, trace.SessionLocation{SessionID: testSessionID, TranscriptPath: outside}))
			link := filepath.Join(home, "sessions", "escaped.jsonl")
			require.NoError(t, os.Symlink(outside, link))
			assert.Error(t, New().CopySessionData(store, trace.SessionLocation{SessionID: testSessionID, TranscriptPath: link}))
		})
	}
}

func TestEstimateCostDateSuffix(t *testing.T) {
	for _, tc := range []struct {
		model   string
		cost    float64
		warning bool
	}{
		{"gpt-6-luna", 0.1, false},
		{"gpt-6-luna-2026-10-21", 0.1, false},
		{"openai/gpt-6-luna-2024-02-29", 0.1, false},
		{"gpt-6-luna-foo-123456", 0.25, true},
		{"gpt-6-luna-2026-02-29", 0.25, true},
		{"gpt-6-luna-2026-13-21", 0.25, true},
		{"gpt-6-luna-2026-1-21", 0.25, true},
	} {
		t.Run(tc.model, func(t *testing.T) {
			var warnings []string
			assert.InDelta(t, tc.cost, estimateCost(tc.model, tokenUsage{Input: 1_000_000}, &warnings), 0.00001)
			if tc.warning {
				require.Len(t, warnings, 1)
				assert.Contains(t, warnings[0], "unknown Codex model")
			} else {
				assert.Empty(t, warnings)
			}
		})
	}
}
