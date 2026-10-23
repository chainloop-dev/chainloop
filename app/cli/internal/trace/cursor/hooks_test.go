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
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/chainloop-dev/chainloop/app/cli/internal/trace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInstallHooksCreatesFile(t *testing.T) {
	repoRoot := t.TempDir()
	p := New()

	require.NoError(t, p.InstallHooks(repoRoot))

	settingsPath := filepath.Join(repoRoot, settingsFile)
	data, err := os.ReadFile(settingsPath)
	require.NoError(t, err, "read %s", settingsPath)

	var out map[string]any
	require.NoError(t, json.Unmarshal(data, &out), "parse settings json")

	v, _ := out["version"].(float64)
	require.Equal(t, hooksSchemaVersion, int(v), "settings version")

	hooks, _ := out["hooks"].(map[string]any)
	for _, evt := range []string{eventSessionStart, eventSessionEnd, eventAfterFileEdit} {
		entries, _ := hooks[evt].([]any)
		require.NotEmpty(t, entries, "no hook entries for event %q", evt)
	}
}

func TestInstallHooksPreservesExistingUserHooks(t *testing.T) {
	repoRoot := t.TempDir()
	settingsPath := filepath.Join(repoRoot, settingsFile)
	require.NoError(t, os.MkdirAll(filepath.Dir(settingsPath), 0755))

	existing := map[string]any{
		"version": 1,
		"hooks": map[string]any{
			eventSessionStart: []any{
				map[string]any{"command": "./my-hook.sh", "timeout": 10},
			},
		},
	}
	data, err := json.MarshalIndent(existing, "", "  ")
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(settingsPath, data, 0600))

	p := New()
	require.NoError(t, p.InstallHooks(repoRoot))

	out, err := os.ReadFile(settingsPath)
	require.NoError(t, err)
	var parsed map[string]any
	require.NoError(t, json.Unmarshal(out, &parsed))

	hooks, ok := parsed["hooks"].(map[string]any)
	require.True(t, ok, "hooks is not an object")
	entries, ok := hooks[eventSessionStart].([]any)
	require.True(t, ok, "%s entries missing", eventSessionStart)
	require.Len(t, entries, 2, "expected 2 entries (user + chainloop)")

	var userCount, chainloopCount int
	for _, e := range entries {
		m, ok := e.(map[string]any)
		require.True(t, ok, "entry is not an object")
		if cmd, _ := m["command"].(string); cmd == "./my-hook.sh" {
			userCount++
		}
		if entryContainsChainloopHook(e) {
			chainloopCount++
		}
	}
	assert.Equal(t, 1, userCount, "user hook missing")
	assert.Equal(t, 1, chainloopCount, "chainloop hook missing")
}

func TestUninstallHooksLeavesUserHooks(t *testing.T) {
	repoRoot := t.TempDir()
	p := New()
	require.NoError(t, p.InstallHooks(repoRoot))

	// Add a user hook alongside chainloop's
	settingsPath := filepath.Join(repoRoot, settingsFile)
	data, err := os.ReadFile(settingsPath)
	require.NoError(t, err)
	var parsed map[string]any
	require.NoError(t, json.Unmarshal(data, &parsed))
	hooks, ok := parsed["hooks"].(map[string]any)
	require.True(t, ok, "hooks is not an object")
	hooks[eventSessionStart] = append(hooks[eventSessionStart].([]any),
		map[string]any{"command": "./user.sh", "timeout": 5})
	out, err := json.MarshalIndent(parsed, "", "  ")
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(settingsPath, out, 0600))

	require.NoError(t, p.UninstallHooks(repoRoot))

	raw, err := os.ReadFile(settingsPath)
	require.NoError(t, err, "settings file should still exist with user hook")
	var after map[string]any
	require.NoError(t, json.Unmarshal(raw, &after))
	afterHooks, ok := after["hooks"].(map[string]any)
	require.True(t, ok, "hooks is not an object")
	entries, ok := afterHooks[eventSessionStart].([]any)
	require.True(t, ok, "%s entries missing", eventSessionStart)
	require.Len(t, entries, 1, "expected 1 entry (user only)")
	m, ok := entries[0].(map[string]any)
	require.True(t, ok, "entry is not an object")
	cmd, _ := m["command"].(string)
	assert.Equal(t, "./user.sh", cmd, "user entry lost, got %+v", m)
}

func TestUninstallHooksDeletesFileWhenEmpty(t *testing.T) {
	repoRoot := t.TempDir()
	p := New()
	require.NoError(t, p.InstallHooks(repoRoot))
	require.NoError(t, p.UninstallHooks(repoRoot))

	settingsPath := filepath.Join(repoRoot, settingsFile)
	_, err := os.Stat(settingsPath)
	assert.True(t, os.IsNotExist(err), "expected %s to be removed, stat err: %v", settingsPath, err)
}

func TestReadHookInputCapturesCursorVersion(t *testing.T) {
	payload := []byte(`{
		"conversation_id": "conv-ver",
		"hook_event_name": "sessionStart",
		"cursor_version": "3.2.16",
		"model": "gpt-5"
	}`)

	p := New()
	in, err := p.ReadHookInput(bytes.NewReader(payload))
	require.NoError(t, err)
	assert.Equal(t, "3.2.16", in.AgentVersion, "AgentVersion")
	assert.Equal(t, "gpt-5", in.Model, "Model")
}

func TestReadHookInputAfterFileEdit(t *testing.T) {
	payload := []byte(`{
		"conversation_id": "conv-abc",
		"generation_id": "gen-1",
		"hook_event_name": "afterFileEdit",
		"model": "claude-4",
		"cursor_version": "0.48.0",
		"workspace_roots": ["/abs/path", "/abs/other"],
		"file_path": "/abs/path/file.go",
		"edits": [
			{"old_string": "foo", "new_string": "bar"},
			{"old_string": "baz", "new_string": "qux"}
		]
	}`)

	p := New()
	in, err := p.ReadHookInput(bytes.NewReader(payload))
	require.NoError(t, err)
	assert.Equal(t, "conv-abc", in.SessionID, "SessionID")
	assert.Equal(t, "afterFileEdit", in.HookEventName, "HookEventName")
	assert.Equal(t, syntheticEditToolName, in.ToolName, "ToolName")
	assert.Equal(t, "/abs/path/file.go", in.FilePath, "FilePath")
	assert.Equal(t, "/abs/path", in.Cwd, "Cwd is the first workspace root")
	require.Len(t, in.Edits, 2, "Edits len")
	assert.Equal(t, "foo", in.Edits[0].OldString, "first edit OldString")
	assert.Equal(t, "bar", in.Edits[0].NewString, "first edit NewString")
}

func TestReadHookInputSessionStart(t *testing.T) {
	payload := []byte(`{
		"conversation_id": "conv-xyz",
		"hook_event_name": "sessionStart",
		"cursor_version": "0.48.0"
	}`)

	p := New()
	in, err := p.ReadHookInput(bytes.NewReader(payload))
	require.NoError(t, err)
	assert.Equal(t, "conv-xyz", in.SessionID, "SessionID")
	assert.Empty(t, in.ToolName, "ToolName should be empty on sessionStart")
	assert.Empty(t, in.Edits, "Edits should be empty on sessionStart")
}

func TestReadHookInputFallsBackToSessionID(t *testing.T) {
	payload := []byte(`{
		"session_id": "fallback-session"
	}`)
	p := New()
	in, err := p.ReadHookInput(bytes.NewReader(payload))
	require.NoError(t, err)
	assert.Equal(t, "fallback-session", in.SessionID, "expected fallback to session_id")
}

func TestHooksInstalled(t *testing.T) {
	testCases := []struct {
		name     string
		settings string
		want     bool
		wantErr  bool
	}{
		{
			name:     "only user hooks",
			settings: `{"version":1,"hooks":{"sessionStart":[{"command":"./my-hook.sh","timeout":10}]}}`,
			want:     false,
		},
		{
			name:     "a chainloop hook next to user hooks",
			settings: `{"version":1,"hooks":{"sessionStart":[{"command":"./my-hook.sh"},{"command":"chainloop trace hook cursor session-start"}]}}`,
			want:     true,
		},
		{
			name:     "malformed settings",
			settings: `{not json`,
			wantErr:  true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			repoRoot := t.TempDir()
			require.NoError(t, os.MkdirAll(filepath.Join(repoRoot, ".cursor"), 0755))
			require.NoError(t, os.WriteFile(filepath.Join(repoRoot, settingsFile), []byte(tc.settings), 0600))

			got, err := New().HooksInstalled(repoRoot)
			if tc.wantErr {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tc.want, got)
		})
	}
}

func TestInstallHooksShellToolHooks(t *testing.T) {
	testCases := []struct {
		name           string
		install        func(p *Provider, repoRoot string) error
		wantSessionEnd bool
	}{
		{name: "install", install: (*Provider).InstallHooks, wantSessionEnd: true},
		{name: "install for trace run", install: (*Provider).InstallHooksForTraceRun},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			repoRoot := t.TempDir()
			require.NoError(t, tc.install(New(), repoRoot))

			hooks := readInstalledHooks(t, repoRoot)
			for event, command := range map[string]string{
				eventPreToolUse:  trace.GuardHookCommandWithOutput("chainloop trace hook cursor pre-tool-use", allowToolUseResponse),
				eventPostToolUse: trace.GuardHookCommand("chainloop trace hook cursor post-tool-use"),
			} {
				entries, _ := hooks[event].([]any)
				require.Len(t, entries, 1, "entries for %s", event)
				entry, _ := entries[0].(map[string]any)
				assert.Equal(t, command, entry["command"], "command for %s", event)
				assert.Equal(t, shellToolName, entry["matcher"], "matcher for %s", event)
			}

			_, hasSessionEnd := hooks[eventSessionEnd]
			assert.Equal(t, tc.wantSessionEnd, hasSessionEnd, "sessionEnd installed")
		})
	}
}

func TestInstallHooksUpdatesChainloopEntryWithoutMatcher(t *testing.T) {
	repoRoot := t.TempDir()
	settingsPath := filepath.Join(repoRoot, settingsFile)
	require.NoError(t, os.MkdirAll(filepath.Dir(settingsPath), 0755))
	require.NoError(t, os.WriteFile(settingsPath, []byte(`{"version":1,"hooks":{"preToolUse":[
		{"command":"./my-hook.sh"},
		{"command":"chainloop trace hook cursor pre-tool-use","timeout":30}
	]}}`), 0600))

	require.NoError(t, New().InstallHooks(repoRoot))

	entries, _ := readInstalledHooks(t, repoRoot)[eventPreToolUse].([]any)
	require.Len(t, entries, 2)
	user, _ := entries[0].(map[string]any)
	assert.Equal(t, "./my-hook.sh", user["command"], "user hook kept")
	chainloop, _ := entries[1].(map[string]any)
	assert.Equal(t, shellToolName, chainloop["matcher"], "matcher added to chainloop hook")
}

func TestReadHookInputShellTool(t *testing.T) {
	testCases := []struct {
		name    string
		payload string
		wantCwd string
	}{
		{
			name: "workspace root wins over the shell cwd",
			payload: `{
				"conversation_id": "conv-sh",
				"hook_event_name": "preToolUse",
				"workspace_roots": ["/abs/repo"],
				"cwd": "/abs/repo/sub",
				"tool_name": "Shell",
				"tool_use_id": "call-1",
				"tool_input": {"command": "sed -i s/a/b/ f.txt", "working_directory": "/abs/repo/sub"}
			}`,
			wantCwd: "/abs/repo",
		},
		{
			name: "shell cwd when no workspace root",
			payload: `{
				"conversation_id": "conv-sh",
				"hook_event_name": "postToolUse",
				"cwd": "/abs/repo",
				"tool_name": "Shell",
				"tool_use_id": "call-1",
				"tool_input": {"command": "sed -i s/a/b/ f.txt"},
				"tool_output": "{}"
			}`,
			wantCwd: "/abs/repo",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			in, err := New().ReadHookInput(bytes.NewReader([]byte(tc.payload)))
			require.NoError(t, err)
			assert.Equal(t, "conv-sh", in.SessionID, "SessionID")
			assert.Equal(t, shellToolName, in.ToolName, "ToolName")
			assert.Equal(t, "call-1", in.ToolUseID, "ToolUseID")
			assert.Equal(t, "sed -i s/a/b/ f.txt", in.Command, "Command")
			assert.Equal(t, tc.wantCwd, in.Cwd, "Cwd")
			assert.Empty(t, in.FilePath, "FilePath")
		})
	}
}

func TestIsCommandTool(t *testing.T) {
	testCases := []struct {
		toolName string
		want     bool
	}{
		{toolName: "Shell", want: true},
		{toolName: syntheticEditToolName},
		{toolName: "Read"},
		{toolName: ""},
	}

	for _, tc := range testCases {
		t.Run(tc.toolName, func(t *testing.T) {
			assert.Equal(t, tc.want, New().IsCommandTool(tc.toolName))
		})
	}
}

func readInstalledHooks(t *testing.T, repoRoot string) map[string]any {
	t.Helper()

	data, err := os.ReadFile(filepath.Join(repoRoot, settingsFile))
	require.NoError(t, err)

	var out map[string]any
	require.NoError(t, json.Unmarshal(data, &out))
	hooks, _ := out["hooks"].(map[string]any)

	return hooks
}

// The hooks are committed to the repository, so they also run for anyone who
// opens it without the chainloop CLI installed (#3687).
func TestInstalledHooksWithoutCLI(t *testing.T) {
	shell := shellOnlyPath(t)
	root := t.TempDir()
	require.NoError(t, New().InstallHooks(root))

	for _, event := range cursorHookEvents {
		t.Run(event.event+"/"+event.command, func(t *testing.T) {
			cmd := exec.CommandContext(t.Context(), shell.sh, "-c", event.guardedCommand()) //nolint:gosec // run the installed hook to check its shell behaviour
			cmd.Env = append(os.Environ(), "PATH="+shell.bin)
			cmd.Stdin = strings.NewReader(`{"session_id":"test"}`)
			out, err := cmd.CombinedOutput()
			require.NoError(t, err, string(out))
			if event.event != eventPreToolUse {
				assert.Empty(t, out)
				return
			}
			// Cursor blocks the shell command unless preToolUse answers.
			var resp struct {
				Permission string `json:"permission"`
			}
			require.NoError(t, json.Unmarshal(out, &resp), string(out))
			assert.Equal(t, "allow", resp.Permission)
		})
	}
	assertInstalledCommandsGuarded(t, root)
}

func TestInstalledHookPreservesIOAndExitStatus(t *testing.T) {
	shell := shellOnlyPath(t)
	stub := "#!/bin/sh\nIFS= read -r payload\nprintf '%s\\n%s\\n' \"$*\" \"$payload\"\nexit \"$HOOK_TEST_EXIT_CODE\"\n"
	require.NoError(t, os.WriteFile(filepath.Join(shell.bin, "chainloop"), []byte(stub), 0700)) //nolint:gosec // executable stub CLI in a temporary test directory

	for _, code := range []string{"0", "37"} {
		t.Run("exit_"+code, func(t *testing.T) {
			cmd := exec.CommandContext(t.Context(), shell.sh, "-c", trace.GuardHookCommand("chainloop trace hook cursor post-tool-use")) //nolint:gosec // run the installed hook against a local stub CLI
			cmd.Env = append(os.Environ(), "PATH="+shell.bin, "HOOK_TEST_EXIT_CODE="+code)
			cmd.Stdin = strings.NewReader("{\"session_id\":\"test\"}\n")
			out, err := cmd.Output()
			if code == "0" {
				require.NoError(t, err)
			} else {
				var exitErr *exec.ExitError
				require.ErrorAs(t, err, &exitErr)
				assert.Equal(t, 37, exitErr.ExitCode())
			}
			assert.Equal(t, "trace hook cursor post-tool-use"+"\n{\"session_id\":\"test\"}\n", string(out))
		})
	}
}

type shellPath struct{ sh, bin string }

// shellOnlyPath returns a PATH directory holding only sh, so chainloop is missing.
func shellOnlyPath(t *testing.T) shellPath {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("the guard is a POSIX sh command")
	}
	sh, err := exec.LookPath("sh")
	require.NoError(t, err)
	bin := t.TempDir()
	require.NoError(t, os.Symlink(sh, filepath.Join(bin, "sh")))
	return shellPath{sh: sh, bin: bin}
}

func assertInstalledCommandsGuarded(t *testing.T, root string) {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(root, settingsFile))
	require.NoError(t, err)
	assert.Equal(t, len(cursorHookEvents), strings.Count(string(data), "command -v chainloop"))
}

func TestInstallUpgradesUnguardedHooks(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, settingsFile)
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0700))
	legacy := `{"version":1,"hooks":{"postToolUse":[{"command":"./my-hook.sh"},{"command":"chainloop trace hook cursor post-tool-use","matcher":"` + shellToolName + `"}]}}`
	require.NoError(t, os.WriteFile(path, []byte(legacy), 0600))

	require.NoError(t, New().InstallHooks(root))
	first, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, 1, strings.Count(string(first), "exec chainloop trace hook cursor post-tool-use"))
	assert.NotContains(t, string(first), `"command": "chainloop trace hook cursor post-tool-use"`)
	assert.Contains(t, string(first), "./my-hook.sh")

	// Idempotent once upgraded.
	require.NoError(t, New().InstallHooks(root))
	second, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, string(first), string(second))
}
