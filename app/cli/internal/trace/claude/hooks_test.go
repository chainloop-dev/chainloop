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
	"bytes"
	"encoding/json"
	"fmt"
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

func TestInstallHooks(t *testing.T) {
	provider := New()

	t.Run("creates settings file with hooks", func(t *testing.T) {
		repoRoot := t.TempDir()

		require.NoError(t, provider.InstallHooks(repoRoot))

		settings := readSettings(t, repoRoot)
		hooks := settings["hooks"].(map[string]any)

		assert.Contains(t, hooks, "SessionStart")
		assert.Contains(t, hooks, "PreToolUse")
		assert.Contains(t, hooks, "PostToolUse")

		// Verify command contents
		assertHookCommand(t, hooks, "SessionStart", "chainloop trace hook claude session-start")
		assertHookCommand(t, hooks, "UserPromptSubmit", "chainloop trace hook claude user-prompt-submit")
		assertHookCommand(t, hooks, "PreToolUse", "chainloop trace hook claude pre-tool-use")
		assertHookCommand(t, hooks, "PostToolUse", "chainloop trace hook claude post-tool-use")
		// A failed tool call fires PostToolUseFailure instead of PostToolUse,
		// and a failed shell command can still have changed files.
		assertHookCommand(t, hooks, "PostToolUseFailure", "chainloop trace hook claude post-tool-use")
	})

	t.Run("installs PreToolUse and PostToolUse with matchers", func(t *testing.T) {
		repoRoot := t.TempDir()
		require.NoError(t, provider.InstallHooks(repoRoot))

		settings := readSettings(t, repoRoot)
		hooks := settings["hooks"].(map[string]any)

		// PreToolUse should have matcher
		preEntries := hooks["PreToolUse"].([]any)
		preEntry := preEntries[0].(map[string]any)
		assert.Equal(t, "Edit|Write|MultiEdit|Bash", preEntry["matcher"])

		// PostToolUse also matches Skill, to copy the folder of a loaded skill
		postEntries := hooks["PostToolUse"].([]any)
		postEntry := postEntries[0].(map[string]any)
		assert.Equal(t, "Edit|Write|MultiEdit|Bash|Skill", postEntry["matcher"])

		// PostToolUseFailure keeps the edit matcher: a failed Skill call loads
		// no skill
		failureEntries := hooks["PostToolUseFailure"].([]any)
		failureEntry := failureEntries[0].(map[string]any)
		assert.Equal(t, "Edit|Write|MultiEdit|Bash", failureEntry["matcher"])

		// SessionStart should NOT have matcher
		startEntries := hooks["SessionStart"].([]any)
		startEntry := startEntries[0].(map[string]any)
		_, hasMatcher := startEntry["matcher"]
		assert.False(t, hasMatcher)

		// SessionEnd should NOT have matcher
		endEntries := hooks["SessionEnd"].([]any)
		endEntry := endEntries[0].(map[string]any)
		_, hasMatcher = endEntry["matcher"]
		assert.False(t, hasMatcher)
	})

	t.Run("upgrades existing hooks to add matcher", func(t *testing.T) {
		repoRoot := t.TempDir()
		settingsPath := filepath.Join(repoRoot, ".claude", "settings.json")
		require.NoError(t, os.MkdirAll(filepath.Dir(settingsPath), 0755))

		// Simulate old-format PreToolUse without matcher
		existing := map[string]any{
			"hooks": map[string]any{
				"PreToolUse": []any{
					map[string]any{
						"hooks": []any{
							map[string]any{"type": "command", "command": "chainloop trace hook claude pre-tool-use"},
						},
					},
				},
			},
		}
		data, _ := json.MarshalIndent(existing, "", "  ")
		require.NoError(t, os.WriteFile(settingsPath, data, 0600))

		// Re-install should upgrade
		require.NoError(t, provider.InstallHooks(repoRoot))

		settings := readSettings(t, repoRoot)
		hooks := settings["hooks"].(map[string]any)

		preEntries := hooks["PreToolUse"].([]any)
		require.Len(t, preEntries, 1)
		preEntry := preEntries[0].(map[string]any)
		assert.Equal(t, "Edit|Write|MultiEdit|Bash", preEntry["matcher"])
	})

	t.Run("idempotent — running twice produces same result", func(t *testing.T) {
		repoRoot := t.TempDir()

		require.NoError(t, provider.InstallHooks(repoRoot))
		require.NoError(t, provider.InstallHooks(repoRoot))

		settings := readSettings(t, repoRoot)
		hooks := settings["hooks"].(map[string]any)

		// Should have exactly one entry per event
		sessionStart := hooks["SessionStart"].([]any)
		assert.Len(t, sessionStart, 1)

		promptSubmit := hooks["UserPromptSubmit"].([]any)
		assert.Len(t, promptSubmit, 1)

		preToolUse := hooks["PreToolUse"].([]any)
		assert.Len(t, preToolUse, 1)

		postToolUse := hooks["PostToolUse"].([]any)
		assert.Len(t, postToolUse, 1)
	})

	t.Run("preserves existing settings", func(t *testing.T) {
		repoRoot := t.TempDir()
		settingsPath := filepath.Join(repoRoot, ".claude", "settings.json")
		require.NoError(t, os.MkdirAll(filepath.Dir(settingsPath), 0755))

		existing := map[string]any{
			"customSetting": "value",
			"hooks": map[string]any{
				"PostToolUse": []any{
					map[string]any{
						"matcher": "Edit|Write",
						"hooks": []any{
							map[string]any{"type": "command", "command": "format.sh"},
						},
					},
				},
			},
		}
		data, _ := json.MarshalIndent(existing, "", "  ")
		require.NoError(t, os.WriteFile(settingsPath, data, 0600))

		require.NoError(t, provider.InstallHooks(repoRoot))

		settings := readSettings(t, repoRoot)
		assert.Equal(t, "value", settings["customSetting"])

		hooks := settings["hooks"].(map[string]any)
		assert.Contains(t, hooks, "PostToolUse")
		assert.Contains(t, hooks, "SessionStart")
		assert.Contains(t, hooks, "PreToolUse")
	})
}

func TestUninstallHooks(t *testing.T) {
	provider := New()

	t.Run("removes chainloop entries", func(t *testing.T) {
		repoRoot := t.TempDir()

		require.NoError(t, provider.InstallHooks(repoRoot))
		require.NoError(t, provider.UninstallHooks(repoRoot))

		settingsPath := filepath.Join(repoRoot, ".claude", "settings.json")
		_, err := os.Stat(settingsPath)
		assert.True(t, os.IsNotExist(err), "empty settings file should be deleted")
	})

	t.Run("preserves other hook entries", func(t *testing.T) {
		repoRoot := t.TempDir()
		settingsPath := filepath.Join(repoRoot, ".claude", "settings.json")
		require.NoError(t, os.MkdirAll(filepath.Dir(settingsPath), 0755))

		existing := map[string]any{
			"hooks": map[string]any{
				"PostToolUse": []any{
					map[string]any{
						"hooks": []any{
							map[string]any{"type": "command", "command": "format.sh"},
						},
					},
				},
			},
		}
		data, _ := json.MarshalIndent(existing, "", "  ")
		require.NoError(t, os.WriteFile(settingsPath, data, 0600))

		require.NoError(t, provider.InstallHooks(repoRoot))
		require.NoError(t, provider.UninstallHooks(repoRoot))

		settings := readSettings(t, repoRoot)
		hooks := settings["hooks"].(map[string]any)
		assert.Contains(t, hooks, "PostToolUse")
		assert.NotContains(t, hooks, "SessionStart")
		assert.NotContains(t, hooks, "UserPromptSubmit")
		assert.NotContains(t, hooks, "PreToolUse")
		assert.NotContains(t, hooks, "PostToolUseFailure")
	})

	t.Run("noop when file does not exist", func(t *testing.T) {
		repoRoot := t.TempDir()
		assert.NoError(t, provider.UninstallHooks(repoRoot))
	})
}

func TestReadHookInput(t *testing.T) {
	provider := New()

	t.Run("parses valid input", func(t *testing.T) {
		r := bytes.NewBufferString(`{"session_id":"abc-123","hook_event_name":"SessionStart","tool_name":"Edit","cwd":"/some/path"}`)
		input, err := provider.ReadHookInput(r)
		require.NoError(t, err)
		assert.Equal(t, "abc-123", input.SessionID)
		assert.Equal(t, "SessionStart", input.HookEventName)
		assert.Equal(t, "Edit", input.ToolName)
		assert.Equal(t, "/some/path", input.Cwd)
	})

	t.Run("extracts tool metadata", func(t *testing.T) {
		r := bytes.NewBufferString(`{
			"session_id": "abc-123",
			"hook_event_name": "PreToolUse",
			"tool_name": "Edit",
			"tool_use_id": "toolu_01ABC",
			"tool_input": {"file_path": "/some/file.go", "old_string": "foo"}
		}`)
		input, err := provider.ReadHookInput(r)
		require.NoError(t, err)
		assert.Equal(t, "abc-123", input.SessionID)
		assert.Equal(t, "PreToolUse", input.HookEventName)
		assert.Equal(t, "Edit", input.ToolName)
		assert.Equal(t, "toolu_01ABC", input.ToolUseID)
		assert.Equal(t, "/some/file.go", input.FilePath)
	})

	t.Run("extracts the shell command", func(t *testing.T) {
		r := bytes.NewBufferString(`{
			"session_id": "abc-123",
			"hook_event_name": "PreToolUse",
			"tool_name": "Bash",
			"tool_input": {"command": "cd /tmp/repo-b && git commit -m x", "description": "commit"}
		}`)
		input, err := provider.ReadHookInput(r)
		require.NoError(t, err)
		assert.Equal(t, "cd /tmp/repo-b && git commit -m x", input.Command)
		assert.Empty(t, input.FilePath)
	})

	t.Run("handles missing tool_input gracefully", func(t *testing.T) {
		r := bytes.NewBufferString(`{"session_id":"abc-123","hook_event_name":"SessionStart"}`)
		input, err := provider.ReadHookInput(r)
		require.NoError(t, err)
		assert.Equal(t, "abc-123", input.SessionID)
		assert.Equal(t, "SessionStart", input.HookEventName)
		assert.Empty(t, input.FilePath)
	})

	t.Run("flags failed tool calls", func(t *testing.T) {
		testCases := []struct {
			event      string
			wantFailed bool
		}{
			{event: "PreToolUse", wantFailed: false},
			{event: "PostToolUse", wantFailed: false},
			{event: "PostToolUseFailure", wantFailed: true},
		}

		for _, tc := range testCases {
			t.Run(tc.event, func(t *testing.T) {
				r := bytes.NewBufferString(fmt.Sprintf(`{
					"session_id": "abc-123",
					"hook_event_name": %q,
					"tool_name": "Bash",
					"tool_input": {"command": "make lint"},
					"error": "Exit code 1",
					"is_interrupt": false
				}`, tc.event))
				input, err := provider.ReadHookInput(r)
				require.NoError(t, err)
				assert.Equal(t, tc.wantFailed, input.ToolFailed)
			})
		}
	})

	t.Run("returns empty for empty session ID", func(t *testing.T) {
		r := bytes.NewBufferString(`{"session_id":""}`)
		input, err := provider.ReadHookInput(r)
		require.NoError(t, err)
		assert.Empty(t, input.SessionID)
	})

	t.Run("returns error for invalid JSON", func(t *testing.T) {
		r := bytes.NewBufferString(`not json`)
		_, err := provider.ReadHookInput(r)
		assert.Error(t, err)
	})

	t.Run("bails on Cursor-relayed payloads", func(t *testing.T) {
		// When .claude/settings.json and .cursor/hooks.json are both
		// installed and the user is running inside Cursor, Cursor's
		// runtime fires the Claude-flavored hooks too — every payload
		// it relays carries cursor_version. We must skip those so the
		// cursor provider's afterFileEdit owns the session and we don't
		// double-record line ranges.
		r := bytes.NewBufferString(`{
			"session_id": "e1cd0c12-07cb-495b-be1f-b6254bff24e8",
			"hook_event_name": "preToolUse",
			"tool_name": "Write",
			"tool_input": {"file_path": "/repo/README.md"},
			"cursor_version": "3.2.16"
		}`)
		input, err := provider.ReadHookInput(r)
		require.NoError(t, err)
		assert.Empty(t, input.SessionID, "Cursor-relayed events must yield an empty input")
		assert.Empty(t, input.FilePath)
	})
}

func TestIsFileWritingTool(t *testing.T) {
	provider := New()

	assert.True(t, provider.IsFileWritingTool("Edit"))
	assert.True(t, provider.IsFileWritingTool("Write"))
	assert.True(t, provider.IsFileWritingTool("MultiEdit"))
	assert.False(t, provider.IsFileWritingTool("Read"))
	assert.False(t, provider.IsFileWritingTool("Bash"))
	assert.False(t, provider.IsFileWritingTool(""))
}

func TestIsCommandTool(t *testing.T) {
	provider := New()

	assert.True(t, provider.IsCommandTool("Bash"))
	assert.False(t, provider.IsCommandTool("Edit"))
	assert.False(t, provider.IsCommandTool("Write"))
	assert.False(t, provider.IsCommandTool(""))
}

func TestInstallHooksForTraceRun(t *testing.T) {
	provider := New()

	t.Run("installs everything except SessionEnd", func(t *testing.T) {
		repoRoot := t.TempDir()

		require.NoError(t, provider.InstallHooksForTraceRun(repoRoot))

		settings := readSettings(t, repoRoot)
		hooks := settings["hooks"].(map[string]any)

		assert.Contains(t, hooks, "SessionStart")
		assert.Contains(t, hooks, "UserPromptSubmit")
		assert.Contains(t, hooks, "PreToolUse")
		assert.Contains(t, hooks, "PostToolUse")
		assert.Contains(t, hooks, "PostToolUseFailure")
		assert.NotContains(t, hooks, "SessionEnd", "trace run must not install SessionEnd; trace run drives end-of-session itself")
	})
}

func TestInstallHooksOnNullSettings(t *testing.T) {
	repoRoot := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(repoRoot, ".claude"), 0755))
	require.NoError(t, os.WriteFile(filepath.Join(repoRoot, settingsFile), []byte("null\n"), 0600))

	require.NoError(t, New().InstallHooks(repoRoot))
	assert.Contains(t, readSettings(t, repoRoot), "hooks")
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
			settings: `{"hooks":{"PostToolUse":[{"hooks":[{"type":"command","command":"format.sh"}]}]}}`,
			want:     false,
		},
		{
			name:     "a chainloop hook next to user hooks",
			settings: `{"hooks":{"SessionStart":[{"hooks":[{"type":"command","command":"format.sh"},{"type":"command","command":"chainloop trace hook claude session-start"}]}]}}`,
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
			require.NoError(t, os.MkdirAll(filepath.Join(repoRoot, ".claude"), 0755))
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

func readSettings(t *testing.T, repoRoot string) map[string]any {
	t.Helper()

	data, err := os.ReadFile(filepath.Join(repoRoot, ".claude", "settings.json"))
	require.NoError(t, err)

	var settings map[string]any
	require.NoError(t, json.Unmarshal(data, &settings))

	return settings
}

func assertHookCommand(t *testing.T, hooks map[string]any, event, expectedCmd string) {
	t.Helper()

	entries, ok := hooks[event].([]any)
	require.True(t, ok, "event %s should be an array", event)
	require.Len(t, entries, 1)

	entry := entries[0].(map[string]any)
	innerHooks := entry["hooks"].([]any)
	require.Len(t, innerHooks, 1)

	hook := innerHooks[0].(map[string]any)
	assert.Equal(t, "command", hook["type"])
	assert.Equal(t, trace.GuardHookCommand(expectedCmd), hook["command"])
}

// The hooks are committed to the repository, so they also run for anyone who
// opens it without the chainloop CLI installed (#3687).
func TestInstalledHooksWithoutCLI(t *testing.T) {
	shell := shellOnlyPath(t)
	root := t.TempDir()
	require.NoError(t, New().InstallHooks(root))

	for _, event := range hookEvents {
		t.Run(event.event+"/"+event.command, func(t *testing.T) {
			cmd := exec.CommandContext(t.Context(), shell.sh, "-c", trace.GuardHookCommand(event.command)) //nolint:gosec // run the installed hook to check its shell behaviour
			cmd.Env = append(os.Environ(), "PATH="+shell.bin)
			cmd.Stdin = strings.NewReader(`{"session_id":"test"}`)
			out, err := cmd.CombinedOutput()
			require.NoError(t, err, string(out))
			assert.Empty(t, out)
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
			cmd := exec.CommandContext(t.Context(), shell.sh, "-c", trace.GuardHookCommand("chainloop trace hook claude post-tool-use")) //nolint:gosec // run the installed hook against a local stub CLI
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
			assert.Equal(t, "trace hook claude post-tool-use"+"\n{\"session_id\":\"test\"}\n", string(out))
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
	assert.Equal(t, len(hookEvents), strings.Count(string(data), "command -v chainloop"))
}

func TestInstallUpgradesUnguardedHooks(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, settingsFile)
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0700))
	legacy := `{"hooks":{"PostToolUse":[{"matcher":"` + postHookToolMatcher + `","hooks":[{"type":"command","command":"chainloop trace hook claude post-tool-use"}]},{"hooks":[{"type":"command","command":"echo user-hook"}]}]}}`
	require.NoError(t, os.WriteFile(path, []byte(legacy), 0600))

	require.NoError(t, New().InstallHooks(root))
	first, err := os.ReadFile(path)
	require.NoError(t, err)
	var settings struct {
		Hooks map[string][]map[string]any `json:"hooks"`
	}
	require.NoError(t, json.Unmarshal(first, &settings))
	postToolUse := settings.Hooks[eventPostToolUse]
	require.Len(t, postToolUse, 2, "the chainloop entry is replaced, not duplicated")
	assert.Equal(t, trace.GuardHookCommand("chainloop trace hook claude post-tool-use"), chainloopHookCommand(postToolUse[0]))
	assert.NotContains(t, string(first), `"command": "chainloop trace hook claude post-tool-use"`)
	assert.Contains(t, string(first), "echo user-hook")

	// Idempotent once upgraded.
	require.NoError(t, New().InstallHooks(root))
	second, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, string(first), string(second))
}
