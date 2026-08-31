package cursor

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

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
