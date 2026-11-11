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

package copilot

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/chainloop-dev/chainloop/app/cli/internal/trace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func readHookFile(t *testing.T, repo string) hookFile {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(repo, settingsFile))
	require.NoError(t, err)
	var f hookFile
	require.NoError(t, json.Unmarshal(data, &f))
	return f
}

func TestInstallHooks(t *testing.T) {
	p := New()

	t.Run("writes guarded hooks and is idempotent", func(t *testing.T) {
		repo := t.TempDir()
		require.NoError(t, p.InstallHooks(repo))
		first, err := os.ReadFile(p.SettingsFile(repo))
		require.NoError(t, err)

		f := readHookFile(t, repo)
		assert.Equal(t, 1, f.Version)
		for _, event := range []string{eventSessionStart, eventPreToolUse, eventPostToolUse, eventSessionEnd} {
			require.Len(t, f.Hooks[event], 1, event)
			entry := f.Hooks[event][0]
			assert.Equal(t, "command", entry.Type)
			assert.True(t, strings.HasPrefix(entry.Bash, guardPrefix), "hook %s must use the missing-CLI guard", event)
			assert.True(t, isChainloopEntry(entry))
		}
		assert.NotContains(t, f.Hooks, "agentStop")
		assert.NotContains(t, f.Hooks, "userPromptSubmitted")

		require.NoError(t, p.InstallHooks(repo))
		second, err := os.ReadFile(p.SettingsFile(repo))
		require.NoError(t, err)
		assert.Equal(t, first, second)
	})

	t.Run("trace run omits session end", func(t *testing.T) {
		repo := t.TempDir()
		require.NoError(t, p.InstallHooksForTraceRun(repo))
		f := readHookFile(t, repo)
		assert.NotContains(t, f.Hooks, eventSessionEnd)
		assert.Contains(t, f.Hooks, eventSessionStart)

		// A permanent install afterwards adds session end back.
		require.NoError(t, p.InstallHooks(repo))
		assert.Contains(t, readHookFile(t, repo).Hooks, eventSessionEnd)
	})

	t.Run("refuses a file with user hooks, except for trace run", func(t *testing.T) {
		repo := t.TempDir()
		path := p.SettingsFile(repo)
		require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
		user := []byte(`{"version":1,"hooks":{"sessionStart":[{"type":"command","bash":"echo hi"}]}}`)
		require.NoError(t, os.WriteFile(path, user, 0o600))

		require.Error(t, p.InstallHooks(repo))
		got, err := os.ReadFile(path)
		require.NoError(t, err)
		assert.Equal(t, user, got)

		require.NoError(t, p.InstallHooksForTraceRun(repo))
	})

	t.Run("refuses a file that is not JSON", func(t *testing.T) {
		repo := t.TempDir()
		path := p.SettingsFile(repo)
		require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
		require.NoError(t, os.WriteFile(path, []byte("not json"), 0o600))
		require.Error(t, p.InstallHooks(repo))
	})

	t.Run("leaves other hook files alone", func(t *testing.T) {
		repo := t.TempDir()
		other := filepath.Join(repo, ".github", "hooks", "mine.json")
		require.NoError(t, os.MkdirAll(filepath.Dir(other), 0o755))
		require.NoError(t, os.WriteFile(other, []byte(`{"version":1}`), 0o600))

		require.NoError(t, p.InstallHooks(repo))
		require.NoError(t, p.UninstallHooks(repo))

		got, err := os.ReadFile(other)
		require.NoError(t, err)
		assert.Equal(t, `{"version":1}`, string(got))
		assert.NoFileExists(t, p.SettingsFile(repo))
	})
}

func TestUninstallAndHooksInstalled(t *testing.T) {
	p := New()

	cases := []struct {
		name          string
		content       string
		wantInstalled bool
		wantRemoved   bool
	}{
		{name: "no file", wantInstalled: false, wantRemoved: true},
		{name: "user hooks only", content: `{"version":1,"hooks":{"sessionStart":[{"type":"command","bash":"echo hi"}]}}`},
		{
			name:          "mixed hooks",
			content:       `{"version":1,"hooks":{"sessionStart":[{"type":"command","bash":"echo hi"},{"type":"command","bash":"chainloop trace hook copilot session-start"}]}}`,
			wantInstalled: true,
		},
		{name: "not JSON", content: "{"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			repo := t.TempDir()
			path := p.SettingsFile(repo)
			if tc.content != "" {
				require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
				require.NoError(t, os.WriteFile(path, []byte(tc.content), 0o600))
			}

			installed, err := p.HooksInstalled(repo)
			require.NoError(t, err)
			assert.Equal(t, tc.wantInstalled, installed)

			require.NoError(t, p.UninstallHooks(repo))
			if tc.wantRemoved {
				assert.NoFileExists(t, path)
			} else {
				got, err := os.ReadFile(path)
				require.NoError(t, err)
				assert.Equal(t, tc.content, string(got))
			}
		})
	}

	t.Run("installed then uninstalled", func(t *testing.T) {
		repo := t.TempDir()
		require.NoError(t, p.InstallHooks(repo))
		installed, err := p.HooksInstalled(repo)
		require.NoError(t, err)
		assert.True(t, installed)

		require.NoError(t, p.UninstallHooks(repo))
		assert.NoFileExists(t, p.SettingsFile(repo))
	})
}

func TestReadHookInput(t *testing.T) {
	p := New()
	cwd := "/work/repo"

	cases := []struct {
		name    string
		payload string
		want    trace.HookInput
	}{
		{
			name:    "session start, CLI form",
			payload: `{"sessionId":"s-1","timestamp":1775596038764,"cwd":"/work/repo","source":"new","initialPrompt":"hi"}`,
			want:    trace.HookInput{SessionID: testHookSession, Cwd: cwd},
		},
		{
			name:    "session start, VS Code form",
			payload: `{"session_id":"s-1","hook_event_name":"SessionStart","timestamp":"2026-04-08T15:04:05.000Z","cwd":"/work/repo","transcript_path":"/home/u/.copilot/session-state/s-1/events.jsonl"}`,
			want:    trace.HookInput{SessionID: testHookSession, HookEventName: "SessionStart", Cwd: cwd, TranscriptPath: "/home/u/.copilot/session-state/s-1/events.jsonl"},
		},
		{
			name:    "post tool, create with object args",
			payload: `{"sessionId":"s-1","cwd":"/work/repo","toolName":"create","toolArgs":{"path":"/work/repo/a.md","file_text":"x"},"toolResult":{"resultType":"success"}}`,
			want:    trace.HookInput{SessionID: testHookSession, Cwd: cwd, ToolName: toolCreate, FilePath: "/work/repo/a.md"},
		},
		{
			name:    "pre tool, edit with string args and relative path",
			payload: `{"sessionId":"s-1","cwd":"/work/repo","toolName":"edit","toolArgs":"{\"path\":\"src/a.go\",\"old_str\":\"a\",\"new_str\":\"b\"}"}`,
			want:    trace.HookInput{SessionID: testHookSession, Cwd: cwd, ToolName: "edit", FilePath: "/work/repo/src/a.go"},
		},
		{
			name:    "shell tool",
			payload: `{"sessionId":"s-1","cwd":"/work/repo","toolName":"bash","toolArgs":{"command":"mkdir x","description":"d"}}`,
			want:    trace.HookInput{SessionID: testHookSession, Cwd: cwd, ToolName: toolBash, Command: "mkdir x"},
		},
		{
			name:    "VS Code tool form",
			payload: `{"session_id":"s-1","cwd":"/work/repo","tool_name":"bash","tool_input":{"command":"ls"}}`,
			want:    trace.HookInput{SessionID: testHookSession, Cwd: cwd, ToolName: toolBash, Command: "ls"},
		},
		{
			name:    "pre tool, batched calls of an older CLI",
			payload: `{"sessionId":"s-1","cwd":"/work/repo","toolCalls":[{"id":"t1","name":"report_intent","args":"{\"intent\":\"x\"}"},{"id":"t2","name":"create","args":"{\"path\":\"/work/repo/a.md\",\"file_text\":\"x\"}"},{"id":"t3","name":"edit","args":"{\"path\":\"b.md\"}"}]}`,
			want:    trace.HookInput{SessionID: testHookSession, Cwd: cwd, ToolName: toolCreate, FilePaths: []string{"/work/repo/a.md", "/work/repo/b.md"}},
		},
		{
			name:    "pre tool, batched shell call",
			payload: `{"sessionId":"s-1","cwd":"/work/repo","toolCalls":[{"id":"t1","name":"report_intent","args":"{}"},{"id":"t2","name":"bash","args":"{\"command\":\"go test\"}"}]}`,
			want:    trace.HookInput{SessionID: testHookSession, Cwd: cwd, ToolName: toolBash, Command: "go test"},
		},
		{
			name:    "other tool",
			payload: `{"sessionId":"s-1","cwd":"/work/repo","toolName":"view","toolArgs":{"path":"/work/repo/a.md"}}`,
			want:    trace.HookInput{SessionID: testHookSession, Cwd: cwd, ToolName: "view"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := p.ReadHookInput(strings.NewReader(tc.payload))
			require.NoError(t, err)
			assert.Equal(t, tc.want, *got)
		})
	}

	t.Run("invalid JSON", func(t *testing.T) {
		_, err := p.ReadHookInput(strings.NewReader("{"))
		require.Error(t, err)
	})

	t.Run("payload over the size limit", func(t *testing.T) {
		big := `{"sessionId":"s","prompt":"` + strings.Repeat("a", maxHookPayloadBytes) + `"}`
		_, err := p.ReadHookInput(strings.NewReader(big))
		require.Error(t, err)
	})
}

func TestToolLists(t *testing.T) {
	p := New()
	for _, tool := range []string{toolCreate, "edit"} {
		assert.True(t, p.IsFileWritingTool(tool), tool)
		assert.False(t, p.IsCommandTool(tool), tool)
	}
	for _, tool := range []string{toolBash, "powershell"} {
		assert.True(t, p.IsCommandTool(tool), tool)
		assert.False(t, p.IsFileWritingTool(tool), tool)
	}
	assert.False(t, p.IsFileWritingTool("view"))
	assert.False(t, p.IsCommandTool("view"))
}
