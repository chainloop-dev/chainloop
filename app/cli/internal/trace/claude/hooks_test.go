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
	"os"
	"path/filepath"
	"testing"

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
		assertHookCommand(t, hooks, "PreToolUse", "chainloop trace hook claude pre-tool-use")
		assertHookCommand(t, hooks, "PostToolUse", "chainloop trace hook claude post-tool-use")
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

		// PostToolUse should have matcher
		postEntries := hooks["PostToolUse"].([]any)
		postEntry := postEntries[0].(map[string]any)
		assert.Equal(t, "Edit|Write|MultiEdit|Bash", postEntry["matcher"])

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
		assert.NotContains(t, hooks, "PreToolUse")
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
	})

	t.Run("extracts tool metadata", func(t *testing.T) {
		r := bytes.NewBufferString(`{
			"session_id": "abc-123",
			"hook_event_name": "PreToolUse",
			"tool_name": "Edit",
			"tool_input": {"file_path": "/some/file.go", "old_string": "foo"}
		}`)
		input, err := provider.ReadHookInput(r)
		require.NoError(t, err)
		assert.Equal(t, "abc-123", input.SessionID)
		assert.Equal(t, "PreToolUse", input.HookEventName)
		assert.Equal(t, "Edit", input.ToolName)
		assert.Equal(t, "/some/file.go", input.FilePath)
	})

	t.Run("handles missing tool_input gracefully", func(t *testing.T) {
		r := bytes.NewBufferString(`{"session_id":"abc-123","hook_event_name":"SessionStart"}`)
		input, err := provider.ReadHookInput(r)
		require.NoError(t, err)
		assert.Equal(t, "abc-123", input.SessionID)
		assert.Equal(t, "SessionStart", input.HookEventName)
		assert.Empty(t, input.FilePath)
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
		assert.Contains(t, hooks, "PreToolUse")
		assert.Contains(t, hooks, "PostToolUse")
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
	assert.Equal(t, expectedCmd, hook["command"])
}
