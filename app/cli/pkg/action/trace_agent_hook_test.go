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

package action

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/chainloop-dev/chainloop/app/cli/internal/trace"
	"github.com/chainloop-dev/chainloop/app/cli/internal/trace/claude"
	"github.com/chainloop-dev/chainloop/app/cli/internal/trace/cursor"
	"github.com/chainloop-dev/chainloop/app/cli/internal/trace/hooks"
	"github.com/chainloop-dev/chainloop/app/cli/internal/trace/opencode"
	"github.com/chainloop-dev/chainloop/app/cli/internal/trace/spec"
	"github.com/chainloop-dev/chainloop/app/cli/internal/trace/state"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// initTempGitRepo creates a temp dir and runs git init in it.
func initTempGitRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	cmd := exec.Command("git", "init", dir)
	require.NoError(t, cmd.Run())
	return dir
}

func TestHandleAgentSessionStart(t *testing.T) {
	provider := claude.New()

	t.Run("captures session ID from stdin", func(t *testing.T) {
		repoDir := initTempGitRepo(t)
		gitDir := filepath.Join(repoDir, ".git")
		store := state.NewGitStore(gitDir)
		require.NoError(t, store.InitTraceDir())

		origDir, _ := os.Getwd()
		require.NoError(t, os.Chdir(repoDir))
		t.Cleanup(func() { _ = os.Chdir(origDir) })

		withStdin(t, `{"session_id":"abc-123","cwd":"/some/path"}`)

		require.NoError(t, HandleAgentSessionStart(provider, zerolog.Nop()))

		assert.True(t, store.SessionRecordExists("abc-123"))
	})

	t.Run("no-op when session already tracked", func(t *testing.T) {
		repoDir := initTempGitRepo(t)
		gitDir := filepath.Join(repoDir, ".git")
		store := state.NewGitStore(gitDir)
		require.NoError(t, store.InitTraceDir())

		origDir, _ := os.Getwd()
		require.NoError(t, os.Chdir(repoDir))
		t.Cleanup(func() { _ = os.Chdir(origDir) })

		// Pre-populate session record
		rec := &state.SessionRecord{SessionID: "abc-123", Active: true, StartedAt: "2026-03-28T00:00:00Z"}
		require.NoError(t, store.SaveSessionRecord(rec))

		withStdin(t, `{"session_id":"abc-123"}`)

		require.NoError(t, HandleAgentSessionStart(provider, zerolog.Nop()))

		assert.True(t, store.SessionRecordExists("abc-123"))
	})

	t.Run("tracks multiple sessions independently", func(t *testing.T) {
		repoDir := initTempGitRepo(t)
		gitDir := filepath.Join(repoDir, ".git")
		store := state.NewGitStore(gitDir)
		require.NoError(t, store.InitTraceDir())

		origDir, _ := os.Getwd()
		require.NoError(t, os.Chdir(repoDir))
		t.Cleanup(func() { _ = os.Chdir(origDir) })

		// Track first session
		rec := &state.SessionRecord{SessionID: "session-1", Active: true, StartedAt: "2026-03-28T00:00:00Z"}
		require.NoError(t, store.SaveSessionRecord(rec))

		// Start a new session
		withStdin(t, `{"session_id":"session-2"}`)
		require.NoError(t, HandleAgentSessionStart(provider, zerolog.Nop()))

		// Both sessions should have records
		assert.True(t, store.SessionRecordExists("session-1"))
		assert.True(t, store.SessionRecordExists("session-2"))
	})

	t.Run("outputs system message to stdout", func(t *testing.T) {
		repoDir := initTempGitRepo(t)
		gitDir := filepath.Join(repoDir, ".git")
		store := state.NewGitStore(gitDir)
		require.NoError(t, store.InitTraceDir())

		origDir, _ := os.Getwd()
		require.NoError(t, os.Chdir(repoDir))
		t.Cleanup(func() { _ = os.Chdir(origDir) })

		withStdin(t, `{"session_id":"msg-test","cwd":"/some/path"}`)
		stdout := captureStdout(t, func() {
			require.NoError(t, HandleAgentSessionStart(provider, zerolog.Nop()))
		})

		assert.Contains(t, stdout, `"systemMessage"`)
		assert.Contains(t, stdout, "Chainloop Trace is recording this session.")
	})

	t.Run("carries the banner and the spec instruction in one document", func(t *testing.T) {
		repoDir := initTempGitRepo(t)
		store := state.NewGitStore(filepath.Join(repoDir, ".git"))
		require.NoError(t, store.InitTraceDir())

		origDir, _ := os.Getwd()
		require.NoError(t, os.Chdir(repoDir))
		t.Cleanup(func() { _ = os.Chdir(origDir) })

		withStdin(t, `{"session_id":"spec-test"}`)
		stdout := captureStdout(t, func() {
			require.NoError(t, HandleAgentSessionStart(provider, zerolog.Nop()))
		})

		// Claude Code parses a hook's stdout as a single JSON value, so the two
		// channels have to share one document or one of them is lost.
		dec := json.NewDecoder(strings.NewReader(stdout))

		var got map[string]any
		require.NoError(t, dec.Decode(&got))
		require.ErrorIs(t, dec.Decode(&map[string]any{}), io.EOF, "stdout must carry exactly one JSON document")

		assert.Contains(t, got["systemMessage"], "Chainloop Trace is recording this session.")

		hookOut, ok := got["hookSpecificOutput"].(map[string]any)
		require.True(t, ok)
		assert.Contains(t, hookOut["additionalContext"], spec.SessionDir(repoDir, "spec-test"))
	})

	t.Run("stops asking for a spec once one is captured", func(t *testing.T) {
		repoDir := initTempGitRepo(t)
		store := state.NewGitStore(filepath.Join(repoDir, ".git"))
		require.NoError(t, store.InitTraceDir())

		origDir, _ := os.Getwd()
		require.NoError(t, os.Chdir(repoDir))
		t.Cleanup(func() { _ = os.Chdir(origDir) })

		require.NoError(t, spec.EnsureDir(repoDir))
		require.NoError(t, os.MkdirAll(spec.SessionDir(repoDir, "spec-done"), 0755))
		require.NoError(t, os.WriteFile(
			filepath.Join(spec.SessionDir(repoDir, "spec-done"), "ticket.md"), []byte("the ticket"), 0600))

		withStdin(t, `{"session_id":"spec-done"}`)
		stdout := captureStdout(t, func() {
			require.NoError(t, HandleAgentSessionStart(provider, zerolog.Nop()))
		})

		var got map[string]any
		require.NoError(t, json.Unmarshal([]byte(stdout), &got))

		hookOut, ok := got["hookSpecificOutput"].(map[string]any)
		require.True(t, ok)
		assert.NotContains(t, hookOut, "additionalContext", "the instruction is not worth repeating")
		assert.Contains(t, got["systemMessage"], "Chainloop Trace is recording this session.", "the banner still goes out")
	})

	t.Run("ignores malformed stdin", func(t *testing.T) {
		withStdin(t, `not json`)
		assert.NoError(t, HandleAgentSessionStart(provider, zerolog.Nop()))
	})

	t.Run("ignores empty session ID", func(t *testing.T) {
		withStdin(t, `{"session_id":""}`)
		assert.NoError(t, HandleAgentSessionStart(provider, zerolog.Nop()))
	})
}

// TestHandleAgentPromptSubmit covers R-002 of Spec 003: the reminder comes at
// each user prompt, also in a resumed session whose folder holds files.
func TestHandleAgentPromptSubmit(t *testing.T) {
	const sessionID = "abc-123"

	setup := func(t *testing.T) string {
		t.Helper()
		repoDir := initTempGitRepo(t)
		require.NoError(t, state.NewGitStore(filepath.Join(repoDir, ".git")).InitTraceDir())

		origDir, _ := os.Getwd()
		require.NoError(t, os.Chdir(repoDir))
		t.Cleanup(func() { _ = os.Chdir(origDir) })

		return repoDir
	}

	t.Run("a resumed session gets the reminder with its folder", func(t *testing.T) {
		repoDir := setup(t)
		dir := spec.SessionDir(repoDir, sessionID)
		require.NoError(t, spec.EnsureDir(repoDir))
		require.NoError(t, os.MkdirAll(dir, 0755))
		require.NoError(t, os.WriteFile(filepath.Join(dir, "ticket.md"), []byte("the ticket"), 0600))

		withStdin(t, `{"session_id":"abc-123","hook_event_name":"UserPromptSubmit","prompt":"go on"}`)
		stdout := captureStdout(t, func() {
			require.NoError(t, HandleAgentPromptSubmit(claude.New(), zerolog.Nop()))
		})

		var got struct {
			HookSpecificOutput struct {
				HookEventName     string `json:"hookEventName"`
				AdditionalContext string `json:"additionalContext"`
			} `json:"hookSpecificOutput"`
		}
		require.NoError(t, json.Unmarshal([]byte(stdout), &got))
		assert.Equal(t, "UserPromptSubmit", got.HookSpecificOutput.HookEventName)
		assert.Contains(t, got.HookSpecificOutput.AdditionalContext, dir)
	})

	t.Run("an agent without the channel gets nothing", func(t *testing.T) {
		setup(t)

		withStdin(t, `{"session_id":"abc-123"}`)
		stdout := captureStdout(t, func() {
			require.NoError(t, HandleAgentPromptSubmit(cursor.New(), zerolog.Nop()))
		})

		assert.Empty(t, stdout)
	})

	t.Run("an invalid session gets nothing", func(t *testing.T) {
		setup(t)

		withStdin(t, `{"session_id":""}`)
		stdout := captureStdout(t, func() {
			require.NoError(t, HandleAgentPromptSubmit(claude.New(), zerolog.Nop()))
		})

		assert.Empty(t, stdout)
	})
}

func TestHandleAgentSessionEnd(t *testing.T) {
	provider := claude.New()

	t.Run("completes without error", func(t *testing.T) {
		repoDir := initTempGitRepo(t)
		gitDir := filepath.Join(repoDir, ".git")
		store := state.NewGitStore(gitDir)
		require.NoError(t, store.InitTraceDir())

		origDir, _ := os.Getwd()
		require.NoError(t, os.Chdir(repoDir))
		t.Cleanup(func() { _ = os.Chdir(origDir) })

		// Pre-populate session record
		rec := &state.SessionRecord{SessionID: "abc-123", Active: true, StartedAt: "2026-03-28T00:00:00Z"}
		require.NoError(t, store.SaveSessionRecord(rec))

		withStdin(t, `{"session_id":"abc-123"}`)
		require.NoError(t, HandleAgentSessionEnd(provider, zerolog.Nop()))
	})

	t.Run("deletes the spec of the ended session only", func(t *testing.T) {
		repoDir := initTempGitRepo(t)
		store := state.NewGitStore(filepath.Join(repoDir, ".git"))
		require.NoError(t, store.InitTraceDir())

		origDir, _ := os.Getwd()
		require.NoError(t, os.Chdir(repoDir))
		t.Cleanup(func() { _ = os.Chdir(origDir) })

		// The spec stays for every push of the session, so the session end is
		// the only point where it goes. Another session's spec is untouched.
		// The redacted copies of the files go with them.
		for _, id := range []string{"abc-123", "other-456"} {
			for _, dir := range []string{spec.SessionDir(repoDir, id), store.SpecRedactionDir(id)} {
				require.NoError(t, os.MkdirAll(dir, 0o755))
				require.NoError(t, os.WriteFile(filepath.Join(dir, "ticket.md"), []byte("the ticket"), 0o600))
			}
		}

		withStdin(t, `{"session_id":"abc-123"}`)
		require.NoError(t, HandleAgentSessionEnd(provider, zerolog.Nop()))

		assert.NoDirExists(t, spec.SessionDir(repoDir, "abc-123"))
		assert.NoDirExists(t, store.SpecRedactionDir("abc-123"))
		assert.FileExists(t, filepath.Join(spec.SessionDir(repoDir, "other-456"), "ticket.md"))
		assert.FileExists(t, filepath.Join(store.SpecRedactionDir("other-456"), "ticket.md"))
	})

	t.Run("marks the session inactive", func(t *testing.T) {
		repoDir := initTempGitRepo(t)
		store := state.NewGitStore(filepath.Join(repoDir, ".git"))
		require.NoError(t, store.InitTraceDir())

		origDir, _ := os.Getwd()
		require.NoError(t, os.Chdir(repoDir))
		t.Cleanup(func() { _ = os.Chdir(origDir) })

		require.NoError(t, store.SaveSessionRecord(&state.SessionRecord{
			SessionID: "abc-123", Provider: providerClaudeCode, Active: true, StartedAt: "2026-03-28T00:00:00Z",
		}))

		withStdin(t, `{"session_id":"abc-123"}`)
		require.NoError(t, HandleAgentSessionEnd(provider, zerolog.Nop()))

		rec, err := store.LoadSessionRecord("abc-123")
		require.NoError(t, err)
		require.NotNil(t, rec)
		assert.False(t, rec.Active)
		assert.Equal(t, "2026-03-28T00:00:00Z", rec.StartedAt, "ending a session must not rewrite when it began")
		assert.Equal(t, providerClaudeCode, rec.Provider)
	})

	t.Run("a resumed session goes back to active", func(t *testing.T) {
		repoDir := initTempGitRepo(t)
		store := state.NewGitStore(filepath.Join(repoDir, ".git"))
		require.NoError(t, store.InitTraceDir())

		origDir, _ := os.Getwd()
		require.NoError(t, os.Chdir(repoDir))
		t.Cleanup(func() { _ = os.Chdir(origDir) })

		require.NoError(t, store.SaveSessionRecord(&state.SessionRecord{
			SessionID:    "abc-123",
			Provider:     providerClaudeCode,
			AgentVersion: "1.2.3",
			Active:       false,
			StartedAt:    "2026-03-28T00:00:00Z",
		}))

		withStdin(t, `{"session_id":"abc-123","cwd":"/some/path"}`)
		require.NoError(t, HandleAgentSessionStart(provider, zerolog.Nop()))

		rec, err := store.LoadSessionRecord("abc-123")
		require.NoError(t, err)
		require.NotNil(t, rec)
		assert.True(t, rec.Active, "an agent can resume a session after its end hook ran")
		assert.Equal(t, "2026-03-28T00:00:00Z", rec.StartedAt)
		assert.Equal(t, "1.2.3", rec.AgentVersion, "metadata captured on the first hook must survive")
	})

	t.Run("a late tool hook does not revive an ended session", func(t *testing.T) {
		// Only session-start means "resumed". The tool hooks share
		// ensureSessionTracked, so reviving there would let one arriving after
		// session-end resurrect a session that really has finished, and the
		// next no-commit push would attest it.
		dir, gitDir := initGitRepo(t)
		store := state.NewGitStore(gitDir)
		require.NoError(t, store.InitTraceDir())

		require.NoError(t, store.SaveSessionRecord(&state.SessionRecord{
			SessionID: "abc-123", Provider: providerClaudeCode, Active: false, StartedAt: "2026-03-28T00:00:00Z",
		}))

		target := filepath.Join(dir, "late.txt")
		require.NoError(t, os.WriteFile(target, []byte("one\n"), 0600))

		withStdin(t, `{"session_id":"abc-123","tool_name":"Write","tool_input":{"file_path":"`+target+`"}}`)
		require.NoError(t, HandleAgentPostToolUse(provider, zerolog.Nop()))

		rec, err := store.LoadSessionRecord("abc-123")
		require.NoError(t, err)
		require.NotNil(t, rec)
		assert.False(t, rec.Active)
	})
}

func TestAutoInstallGitHooks(t *testing.T) {
	t.Run("installs hooks when project discoverable from yml", func(t *testing.T) {
		repoDir := initTempGitRepo(t)
		gitDir := filepath.Join(repoDir, ".git")
		store := state.NewGitStore(gitDir)

		// Create .chainloop.yml with project name
		require.NoError(t, os.WriteFile(filepath.Join(repoDir, ".chainloop.yml"), []byte("projectName: my-project\n"), 0600))

		origDir, _ := os.Getwd()
		require.NoError(t, os.Chdir(repoDir))
		t.Cleanup(func() { _ = os.Chdir(origDir) })

		autoInstallGitHooks(store, repoDir, zerolog.Nop())

		// Git hooks should be installed
		content, err := os.ReadFile(filepath.Join(gitDir, "hooks", "post-commit"))
		require.NoError(t, err)
		assert.Contains(t, string(content), hooks.HookMarker)

		// Trace should be marked as initialized
		assert.True(t, store.IsTraceInitialized())
	})

	t.Run("skips when hooks already installed", func(t *testing.T) {
		repoDir := initTempGitRepo(t)
		gitDir := filepath.Join(repoDir, ".git")
		store := state.NewGitStore(gitDir)

		// Install hooks first
		_, err := hooks.Install(gitDir, false)
		require.NoError(t, err)

		// Write a marker to verify hooks aren't reinstalled
		hookPath := filepath.Join(gitDir, "hooks", "post-commit")
		original, _ := os.ReadFile(hookPath)

		origDir, _ := os.Getwd()
		require.NoError(t, os.Chdir(repoDir))
		t.Cleanup(func() { _ = os.Chdir(origDir) })

		autoInstallGitHooks(store, repoDir, zerolog.Nop())

		// Hook content should be unchanged
		current, _ := os.ReadFile(hookPath)
		assert.Equal(t, string(original), string(current))
	})

	t.Run("skips when no project name discoverable", func(t *testing.T) {
		repoDir := initTempGitRepo(t)
		gitDir := filepath.Join(repoDir, ".git")
		store := state.NewGitStore(gitDir)

		origDir, _ := os.Getwd()
		require.NoError(t, os.Chdir(repoDir))
		t.Cleanup(func() { _ = os.Chdir(origDir) })

		autoInstallGitHooks(store, repoDir, zerolog.Nop())

		// No hooks should be installed
		_, err := os.Stat(filepath.Join(gitDir, "hooks", "post-commit"))
		assert.True(t, os.IsNotExist(err))
	})

	t.Run("skips pre-push when trace-run-active sentinel is present", func(t *testing.T) {
		repoDir := initTempGitRepo(t)
		gitDir := filepath.Join(repoDir, ".git")
		store := state.NewGitStore(gitDir)

		require.NoError(t, os.WriteFile(filepath.Join(repoDir, ".chainloop.yml"), []byte("projectName: my-project\n"), 0600))
		require.NoError(t, store.InitTraceDir())
		require.NoError(t, store.MarkTraceRunActive())

		origDir, _ := os.Getwd()
		require.NoError(t, os.Chdir(repoDir))
		t.Cleanup(func() { _ = os.Chdir(origDir) })

		autoInstallGitHooks(store, repoDir, zerolog.Nop())

		content, err := os.ReadFile(filepath.Join(gitDir, "hooks", "post-commit"))
		require.NoError(t, err)
		assert.Contains(t, string(content), hooks.HookMarker)

		_, err = os.Stat(filepath.Join(gitDir, "hooks", "pre-push"))
		assert.True(t, os.IsNotExist(err), "pre-push must not be installed under trace run mode")
	})
}

// captureStdout captures stdout output during fn execution.
func captureStdout(t *testing.T, fn func()) string {
	t.Helper()

	origStdout := os.Stdout
	r, w, err := os.Pipe()
	require.NoError(t, err)
	os.Stdout = w
	defer func() { os.Stdout = origStdout }()

	fn()

	_ = w.Close()

	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)

	return buf.String()
}

// withStdin replaces os.Stdin with a reader containing the given content for the test duration.
func withStdin(t *testing.T, content string) {
	t.Helper()

	origStdin := os.Stdin
	r, w, _ := os.Pipe()
	_, _ = io.Copy(w, bytes.NewBufferString(content))
	_ = w.Close()
	os.Stdin = r
	t.Cleanup(func() { os.Stdin = origStdin })
}

func TestHandleAgentPostToolUse_DeletionAttribution(t *testing.T) {
	dir, gitDir := initGitRepo(t)
	store := state.NewGitStore(gitDir)
	require.NoError(t, store.InitTraceDir())

	target := filepath.Join(dir, "victim.txt")
	require.NoError(t, os.WriteFile(target, []byte("line 1\nline 2\nline 3\n"), 0600))

	p := opencode.New()

	withStdin(t, `{"session_id":"ses-delete-test","hook_event_name":"tool.execute.before","tool_name":"apply_patch","file_path":"`+target+`"}`)
	require.NoError(t, HandleAgentPreToolUse(p, zerolog.Nop()))

	snap, err := store.LoadFileSnapshot("ses-delete-test", target)
	require.NoError(t, err)
	assert.Equal(t, "line 1\nline 2\nline 3\n", string(snap))

	require.NoError(t, os.Remove(target))

	withStdin(t, `{"session_id":"ses-delete-test","hook_event_name":"tool.execute.after","tool_name":"apply_patch","file_path":"`+target+`"}`)
	require.NoError(t, HandleAgentPostToolUse(p, zerolog.Nop()))

	attr := store.LoadAILineAttribution("ses-delete-test")
	assert.Contains(t, attr.Files, "victim.txt",
		"deleted file must have ai-lines attribution recorded")
	assert.Empty(t, attr.Files["victim.txt"],
		"deleted file must have nil ranges (no lines in after)")
}

func TestHandleAgentPostToolUse_UpdateAttribution(t *testing.T) {
	dir, gitDir := initGitRepo(t)
	store := state.NewGitStore(gitDir)
	require.NoError(t, store.InitTraceDir())

	target := filepath.Join(dir, "updated.txt")
	require.NoError(t, os.WriteFile(target, []byte("old line\n"), 0600))

	p := opencode.New()

	withStdin(t, `{"session_id":"ses-update-test","hook_event_name":"tool.execute.before","tool_name":"apply_patch","file_path":"`+target+`"}`)
	require.NoError(t, HandleAgentPreToolUse(p, zerolog.Nop()))

	require.NoError(t, os.WriteFile(target, []byte("new line\n"), 0600))

	withStdin(t, `{"session_id":"ses-update-test","hook_event_name":"tool.execute.after","tool_name":"apply_patch","file_path":"`+target+`"}`)
	require.NoError(t, HandleAgentPostToolUse(p, zerolog.Nop()))

	attr := store.LoadAILineAttribution("ses-update-test")
	assert.Contains(t, attr.Files, "updated.txt")
	ranges := attr.Files["updated.txt"]
	require.NotEmpty(t, ranges)
	assert.Equal(t, 1, ranges[0].Start)
	assert.Equal(t, 1, ranges[0].End)
}

func TestHandleAgentPostToolUse_AddFileAttribution(t *testing.T) {
	dir, gitDir := initGitRepo(t)
	store := state.NewGitStore(gitDir)
	require.NoError(t, store.InitTraceDir())

	target := filepath.Join(dir, "newfile.txt")
	p := opencode.New()

	withStdin(t, `{"session_id":"ses-add-test","hook_event_name":"tool.execute.before","tool_name":"apply_patch","file_path":"`+target+`"}`)
	require.NoError(t, HandleAgentPreToolUse(p, zerolog.Nop()))

	require.NoError(t, os.WriteFile(target, []byte("brand\nnew\ncontent\n"), 0600))

	withStdin(t, `{"session_id":"ses-add-test","hook_event_name":"tool.execute.after","tool_name":"apply_patch","file_path":"`+target+`"}`)
	require.NoError(t, HandleAgentPostToolUse(p, zerolog.Nop()))

	attr := store.LoadAILineAttribution("ses-add-test")
	assert.Contains(t, attr.Files, "newfile.txt")
	ranges := attr.Files["newfile.txt"]
	require.NotEmpty(t, ranges)
	assert.Equal(t, 1, ranges[0].Start)
	assert.Equal(t, 3, ranges[0].End)
}

func TestHandleAgentCommandTool_AttributesShellFileChanges(t *testing.T) {
	dir, gitDir := initGitRepo(t)
	store := state.NewGitStore(gitDir)
	require.NoError(t, store.InitTraceDir())

	p := opencode.New() // opencode's "bash" is a command tool

	// pre-command: snapshot the worktree before the shell command runs.
	withStdin(t, `{"session_id":"ses-cmd","hook_event_name":"tool.execute.before","tool_name":"bash"}`)
	require.NoError(t, HandleAgentPreToolUse(p, zerolog.Nop()))

	// Simulate the shell command creating a file, modifying a tracked one, and
	// creating an empty file (e.g. `touch marker`).
	require.NoError(t, os.WriteFile(filepath.Join(dir, "generated.txt"), []byte("a\nb\nc\n"), 0600))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "init.txt"), []byte("init changed\n"), 0600))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "marker"), nil, 0600))

	// post-command: diff the worktree and attribute the changes to the AI.
	withStdin(t, `{"session_id":"ses-cmd","hook_event_name":"tool.execute.after","tool_name":"bash"}`)
	require.NoError(t, HandleAgentPostToolUse(p, zerolog.Nop()))

	attr := store.LoadAILineAttribution("ses-cmd")

	// Created file: whole-file range recorded (repo-relative key, no symlink drift).
	require.Contains(t, attr.Files, "generated.txt")
	require.NotEmpty(t, attr.Files["generated.txt"])
	assert.Equal(t, 1, attr.Files["generated.txt"][0].Start)
	assert.Equal(t, 3, attr.Files["generated.txt"][0].End)

	// Modified tracked file is also captured.
	assert.Contains(t, attr.Files, "init.txt")

	// Empty file created by the command is recorded (AI-touched, no ranges).
	assert.Contains(t, attr.Files, "marker")
	assert.Empty(t, attr.Files["marker"])

	// The pre-command signature is cleaned up afterwards.
	_, err := store.LoadShellPreSignature(state.ShellCallKey{SessionID: "ses-cmd"})
	assert.Error(t, err)
}

// TestHandleAgentClaudeCodeSession drives the pre/post-tool-use handlers with
// the real JSON payloads Claude Code pipes to its hooks, across a full session:
// SessionStart → Write (new file) → Edit (modify it) → Bash (generate a file).
// It asserts the AI-line attribution map ends up crediting every file the agent
// touched — including the one produced only by the shell command, which is the
// PFM-6684 regression.
func TestHandleAgentClaudeCodeSession(t *testing.T) {
	root := chdirToResolvedGitRepo(t)
	gitDir := filepath.Join(root, ".git")
	store := state.NewGitStore(gitDir)
	require.NoError(t, store.InitTraceDir())

	p := claude.New()
	const sid = "b1f4e0c2-1a2b-4c3d-8e9f-0a1b2c3d4e5f"
	tp := filepath.Join(root, ".claude", "transcript.jsonl")

	// 1. SessionStart — Claude sends session_id, transcript_path, cwd, source.
	withStdin(t, fmt.Sprintf(
		`{"session_id":%q,"transcript_path":%q,"cwd":%q,"hook_event_name":"SessionStart","source":"startup"}`,
		sid, tp, root))
	require.NoError(t, HandleAgentSessionStart(p, zerolog.Nop()))
	require.True(t, store.SessionRecordExists(sid), "SessionStart should record the session")

	// 2. Write a brand-new file. Claude fires PreToolUse (file does not exist
	//    yet), performs the write, then PostToolUse with a tool_response.
	apiGo := filepath.Join(root, "api.go")
	writeInput := fmt.Sprintf(`"tool_name":"Write","tool_input":{"file_path":%q,"content":"package api\n\nfunc A() {}\n"}`, apiGo)
	withStdin(t, fmt.Sprintf(`{"session_id":%q,"hook_event_name":"PreToolUse",%s}`, sid, writeInput))
	require.NoError(t, HandleAgentPreToolUse(p, zerolog.Nop()))

	require.NoError(t, os.WriteFile(apiGo, []byte("package api\n\nfunc A() {}\n"), 0600)) // the Write tool's effect

	withStdin(t, fmt.Sprintf(`{"session_id":%q,"hook_event_name":"PostToolUse",%s,"tool_response":{"type":"create","filePath":%q}}`, sid, writeInput, apiGo))
	require.NoError(t, HandleAgentPostToolUse(p, zerolog.Nop()))

	// 3. Edit the file the agent just wrote (append a function).
	editInput := fmt.Sprintf(`"tool_name":"Edit","tool_input":{"file_path":%q,"old_string":"func A() {}","new_string":"func A() {}\n\nfunc B() {}"}`, apiGo)
	withStdin(t, fmt.Sprintf(`{"session_id":%q,"hook_event_name":"PreToolUse",%s}`, sid, editInput))
	require.NoError(t, HandleAgentPreToolUse(p, zerolog.Nop()))

	require.NoError(t, os.WriteFile(apiGo, []byte("package api\n\nfunc A() {}\n\nfunc B() {}\n"), 0600)) // the Edit tool's effect

	withStdin(t, fmt.Sprintf(`{"session_id":%q,"hook_event_name":"PostToolUse",%s,"tool_response":{"filePath":%q}}`, sid, editInput, apiGo))
	require.NoError(t, HandleAgentPostToolUse(p, zerolog.Nop()))

	// 4. Run a Bash command that generates a file. Bash payloads carry a
	//    "command" in tool_input and never a file_path — this is the path that
	//    previously misattributed the generated file to "human".
	bashInput := `"tool_name":"Bash","tool_input":{"command":"protoc --go_out=. api.proto","description":"generate bindings"}`
	withStdin(t, fmt.Sprintf(`{"session_id":%q,"hook_event_name":"PreToolUse",%s}`, sid, bashInput))
	require.NoError(t, HandleAgentPreToolUse(p, zerolog.Nop()))

	require.NoError(t, os.MkdirAll(filepath.Join(root, "gen"), 0755))
	require.NoError(t, os.WriteFile(filepath.Join(root, "gen", "api.pb.go"), []byte("// generated\npackage gen\n\ntype Msg struct{}\n"), 0600)) // the command's effect

	withStdin(t, fmt.Sprintf(`{"session_id":%q,"hook_event_name":"PostToolUse",%s,"tool_response":{"stdout":"","stderr":"","interrupted":false}}`, sid, bashInput))
	require.NoError(t, HandleAgentPostToolUse(p, zerolog.Nop()))

	// Attribution: the Write/Edit file and the Bash-generated file are all AI.
	attr := store.LoadAILineAttribution(sid)

	require.Contains(t, attr.Files, "api.go", "Write/Edit file must be recorded")
	require.NotEmpty(t, attr.Files["api.go"])

	require.Contains(t, attr.Files, "gen/api.pb.go", "shell-generated file must be attributed to AI (PFM-6684)")
	genRanges := attr.Files["gen/api.pb.go"]
	require.NotEmpty(t, genRanges)
	assert.Equal(t, 1, genRanges[0].Start)
	assert.Equal(t, 4, genRanges[0].End) // whole 4-line generated file

	// The pre-command signature is cleaned up after the Bash post hook.
	_, err := store.LoadShellPreSignature(state.ShellCallKey{SessionID: sid})
	assert.Error(t, err)
}

// A Claude Code subagent shares its parent's session_id. When the parent and
// a subagent run shell commands that overlap in one store, each must keep its
// own pre-command signature, or the first post hook deletes the other's and
// the second command's file changes are lost.
func TestHandleAgentCommandTool_ConcurrentSubagent(t *testing.T) {
	root := chdirToResolvedGitRepo(t)
	store := state.NewGitStore(filepath.Join(root, ".git"))
	require.NoError(t, store.InitTraceDir())

	p := claude.New()
	const sid = "c0c0e0c2-1a2b-4c3d-8e9f-0a1b2c3d4e5f"
	const agentID = "afd65659e2015d48d"

	bash := func(event, extra string, handler func(trace.Provider, zerolog.Logger) error) {
		t.Helper()
		withStdin(t, fmt.Sprintf(`{"session_id":%q,"cwd":%q,"hook_event_name":%q,"tool_name":"Bash","tool_input":{"command":"gen"}%s}`,
			sid, root, event, extra))
		require.NoError(t, handler(p, zerolog.Nop()))
	}
	sub := fmt.Sprintf(`,"agent_id":%q,"agent_type":"general-purpose"`, agentID)

	bash("PreToolUse", "", HandleAgentPreToolUse)  // parent starts a command
	bash("PreToolUse", sub, HandleAgentPreToolUse) // subagent starts a command

	require.NoError(t, os.WriteFile(filepath.Join(root, "sub.txt"), []byte("sub\n"), 0600))
	bash("PostToolUse", sub, HandleAgentPostToolUse) // subagent finishes first

	require.NoError(t, os.WriteFile(filepath.Join(root, "parent.txt"), []byte("parent\n"), 0600))
	bash("PostToolUse", "", HandleAgentPostToolUse) // parent finishes

	attr := store.LoadAILineAttribution(sid)
	assert.Contains(t, attr.Files, "sub.txt")
	assert.Contains(t, attr.Files, "parent.txt", "the parent's command must keep its own pre-command signature")

	for _, id := range []string{"", agentID} {
		_, err := store.LoadShellPreSignature(state.ShellCallKey{SessionID: sid, AgentID: id})
		assert.Error(t, err, "signature for agent %q is cleaned up", id)
	}
}

// One agent can run shell commands that overlap (for example, a background
// command and a foreground one). Each call carries its own tool_use_id, so
// each must keep its own pre-command signature: the second pre hook must not
// overwrite the first, and the first post hook must not delete the second's.
func TestHandleAgentCommandTool_OverlappingCallsOfOneAgent(t *testing.T) {
	root := chdirToResolvedGitRepo(t)
	store := state.NewGitStore(filepath.Join(root, ".git"))
	require.NoError(t, store.InitTraceDir())

	p := claude.New()
	const sid = "e0e0e0c2-1a2b-4c3d-8e9f-0a1b2c3d4e5f"

	bash := func(event, toolUseID string, handler func(trace.Provider, zerolog.Logger) error) {
		t.Helper()
		withStdin(t, fmt.Sprintf(`{"session_id":%q,"cwd":%q,"hook_event_name":%q,"tool_name":"Bash","tool_use_id":%q,"tool_input":{"command":"gen"}}`,
			sid, root, event, toolUseID))
		require.NoError(t, handler(p, zerolog.Nop()))
	}

	bash("PreToolUse", "toolu_first", HandleAgentPreToolUse)
	require.NoError(t, os.WriteFile(filepath.Join(root, "first.txt"), []byte("first\n"), 0600))

	// The second command starts after the first one wrote its file. With one
	// slot per agent, its snapshot replaces the first one and already
	// contains first.txt.
	bash("PreToolUse", "toolu_second", HandleAgentPreToolUse)
	require.NoError(t, os.WriteFile(filepath.Join(root, "second.txt"), []byte("second\n"), 0600))

	bash("PostToolUse", "toolu_first", HandleAgentPostToolUse)
	bash("PostToolUse", "toolu_second", HandleAgentPostToolUse)

	attr := store.LoadAILineAttribution(sid)
	assert.Contains(t, attr.Files, "first.txt", "the first command keeps its own pre-command signature")
	assert.Contains(t, attr.Files, "second.txt", "the second command keeps its own pre-command signature")

	for _, id := range []string{"toolu_first", "toolu_second"} {
		_, err := store.LoadShellPreSignature(state.ShellCallKey{SessionID: sid, ToolUseID: id})
		assert.Error(t, err, "signature for call %q is cleaned up", id)
	}
}

// A session can run in one checkout and edit another one: it writes files
// there with a file tool, and it changes more files there with shell commands
// like `cd <other checkout> && …`. Shell hooks run in the session's
// directory, so they must also snapshot every checkout that the session
// edited with a file tool, and record each change in the ledger of the
// checkout that holds the file.
func TestHandleAgentCommandTool_OtherCheckout(t *testing.T) {
	home := chdirToResolvedGitRepo(t)
	other := resolvedTempGitRepo(t)
	untouched := resolvedTempGitRepo(t)

	homeStore := state.NewGitStore(filepath.Join(home, ".git"))
	otherStore := state.NewGitStore(filepath.Join(other, ".git"))
	untouchedStore := state.NewGitStore(filepath.Join(untouched, ".git"))

	p := claude.New()
	const sid = "f0f0e0c2-1a2b-4c3d-8e9f-0a1b2c3d4e5f"

	hook := func(event, tool, toolUseID, toolInput string, handler func(trace.Provider, zerolog.Logger) error) {
		t.Helper()
		withStdin(t, fmt.Sprintf(`{"session_id":%q,"cwd":%q,"hook_event_name":%q,"tool_name":%q,"tool_use_id":%q,"tool_input":%s}`,
			sid, home, event, tool, toolUseID, toolInput))
		require.NoError(t, handler(p, zerolog.Nop()))
	}

	withStdin(t, fmt.Sprintf(`{"session_id":%q,"cwd":%q,"hook_event_name":"SessionStart","source":"startup"}`, sid, home))
	require.NoError(t, HandleAgentSessionStart(p, zerolog.Nop()))

	// 1. The agent writes a file in the other checkout, twice.
	modal := filepath.Join(other, "modal.tsx")
	writeInput := fmt.Sprintf(`{"file_path":%q,"content":"export {}\n"}`, modal)
	for _, id := range []string{"toolu_w1", "toolu_w2"} {
		hook("PreToolUse", "Write", id, writeInput, HandleAgentPreToolUse)
		require.NoError(t, os.WriteFile(modal, []byte("export {}\n"), 0600))
		hook("PostToolUse", "Write", id, writeInput, HandleAgentPostToolUse)
	}

	rec, err := homeStore.LoadSessionRecord(sid)
	require.NoError(t, err)
	require.NotNil(t, rec)
	assert.Equal(t, []string{other}, rec.Checkouts, "the other checkout is registered once on the session record")

	// 2. A shell command changes files in the other checkout and in a
	// checkout that no file tool touched.
	bashInput := fmt.Sprintf(`{"command":"cd %s && python3 gen.py"}`, other)
	hook("PreToolUse", "Bash", "toolu_b1", bashInput, HandleAgentPreToolUse)
	require.NoError(t, os.WriteFile(filepath.Join(other, "init.txt"), []byte("changed\n"), 0600))
	require.NoError(t, os.MkdirAll(filepath.Join(other, "sheet"), 0755))
	require.NoError(t, os.WriteFile(filepath.Join(other, "sheet", "list.tsx"), []byte("a\nb\n"), 0600))
	require.NoError(t, os.WriteFile(filepath.Join(untouched, "w.go"), []byte("package w\n"), 0600))
	hook("PostToolUse", "Bash", "toolu_b1", bashInput, HandleAgentPostToolUse)

	otherAttr := otherStore.LoadAILineAttribution(sid)
	assert.Contains(t, otherAttr.Files, "modal.tsx", "the file tool edit is recorded")
	assert.Contains(t, otherAttr.Files, "init.txt", "a shell edit in the other checkout is recorded there")
	require.Contains(t, otherAttr.Files, "sheet/list.tsx", "paths are relative to the checkout that holds the file")
	assert.Equal(t, 2, otherAttr.Files["sheet/list.tsx"][0].End)

	homeAttr := homeStore.LoadAILineAttribution(sid)
	assert.Empty(t, homeAttr.Files, "the session's own checkout did not change")

	// Accepted gap: without a file tool edit there, a checkout is not
	// snapshotted.
	assert.Empty(t, untouchedStore.LoadAILineAttribution(sid).Files)
	assert.False(t, untouchedStore.SessionRecordExists(sid))

	_, err = homeStore.LoadShellPreSignature(state.ShellCallKey{SessionID: sid, ToolUseID: "toolu_b1"})
	assert.Error(t, err, "the pre-command signature is cleaned up")
}

// The checkout list lives on the session record of the session's own
// checkout. When that record is missing (the hooks were installed partway
// through the session), registration must not create one: creating it would
// also copy the transcripts into that checkout.
func TestHandleAgentFileTool_OtherCheckoutWithoutHomeRecord(t *testing.T) {
	home := chdirToResolvedGitRepo(t)
	other := resolvedTempGitRepo(t)
	homeStore := state.NewGitStore(filepath.Join(home, ".git"))

	p := claude.New()
	const sid = "a0a0e0c2-1a2b-4c3d-8e9f-0a1b2c3d4e5f"

	file := filepath.Join(other, "x.go")
	writeInput := fmt.Sprintf(`"tool_name":"Write","tool_input":{"file_path":%q,"content":"package x\n"}`, file)
	withStdin(t, fmt.Sprintf(`{"session_id":%q,"cwd":%q,"hook_event_name":"PreToolUse",%s}`, sid, home, writeInput))
	require.NoError(t, HandleAgentPreToolUse(p, zerolog.Nop()))
	require.NoError(t, os.WriteFile(file, []byte("package x\n"), 0600))
	withStdin(t, fmt.Sprintf(`{"session_id":%q,"cwd":%q,"hook_event_name":"PostToolUse",%s}`, sid, home, writeInput))
	require.NoError(t, HandleAgentPostToolUse(p, zerolog.Nop()))

	assert.False(t, homeStore.SessionRecordExists(sid), "registration does not create the home record")
	assert.Contains(t, state.NewGitStore(filepath.Join(other, ".git")).LoadAILineAttribution(sid).Files, "x.go")
}

// resolvedTempGitRepo creates a git repo and returns its symlink-resolved
// root, without changing the working directory.
func resolvedTempGitRepo(t *testing.T) string {
	t.Helper()

	resolved, err := filepath.EvalSymlinks(initTempGitRepo(t))
	require.NoError(t, err)

	return resolved
}

// A shell command that fails still changes the files it wrote before it
// failed. Claude Code reports such a call through PostToolUseFailure, not
// PostToolUse, so the handler must attribute the command's changes from that
// payload too.
func TestHandleAgentCommandTool_FailedCommand(t *testing.T) {
	root := chdirToResolvedGitRepo(t)
	store := state.NewGitStore(filepath.Join(root, ".git"))
	require.NoError(t, store.InitTraceDir())

	p := claude.New()
	const sid = "d1e2f3a4-1a2b-4c3d-8e9f-0a1b2c3d4e5f"
	bashInput := `"tool_name":"Bash","tool_input":{"command":"python3 gen.py && npx eslint ."}`

	withStdin(t, fmt.Sprintf(`{"session_id":%q,"cwd":%q,"hook_event_name":"PreToolUse",%s}`, sid, root, bashInput))
	require.NoError(t, HandleAgentPreToolUse(p, zerolog.Nop()))

	// The command writes three files, then its linter step fails.
	for _, name := range []string{"colors.ts", "check.tsx", "card.tsx"} {
		require.NoError(t, os.WriteFile(filepath.Join(root, name), []byte("export {}\n"), 0600))
	}

	// A link left by an earlier push must wait for a successful command: the
	// link announcement is a PostToolUse response, which does not match the
	// failure event.
	const link = "https://app.chainloop.dev/u/org/sessions/ses_1"
	require.NoError(t, store.SavePendingLinks([]string{link}))

	withStdin(t, fmt.Sprintf(`{"session_id":%q,"cwd":%q,"hook_event_name":"PostToolUseFailure",%s,"error":"Exit code 1","is_interrupt":false}`, sid, root, bashInput))
	out := captureStdout(t, func() {
		require.NoError(t, HandleAgentPostToolUse(p, zerolog.Nop()))
	})

	attr := store.LoadAILineAttribution(sid)
	for _, name := range []string{"colors.ts", "check.tsx", "card.tsx"} {
		assert.Contains(t, attr.Files, name, "a file written by a failed command is AI-made")
	}

	_, err := store.LoadShellPreSignature(state.ShellCallKey{SessionID: sid})
	assert.Error(t, err, "the pre-command signature is cleaned up")

	assert.Empty(t, out, "a failed tool call gets no hook response")
	assert.Equal(t, []string{link}, store.PendingLinks(), "the link stays for the next successful command")
}

// chdirToResolvedGitRepo creates a git repo, chdirs into its symlink-resolved
// path, and returns that canonical root. Using the resolved path mirrors the
// canonical absolute paths Claude Code passes in hook payloads and keeps
// filepath.Rel keys stable on macOS (where /var is a symlink to /private/var).
func chdirToResolvedGitRepo(t *testing.T) string {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git binary not found")
	}

	dir := initTempGitRepo(t)
	resolved, err := filepath.EvalSymlinks(dir)
	require.NoError(t, err)

	cwd, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(resolved))
	t.Cleanup(func() { _ = os.Chdir(cwd) })

	return resolved
}

func TestHandleAgentPostToolUse_EmptyFilePathSkips(t *testing.T) {
	_, gitDir := initGitRepo(t)
	store := state.NewGitStore(gitDir)
	require.NoError(t, store.InitTraceDir())

	p := opencode.New()

	withStdin(t, `{"session_id":"ses-empty-path","hook_event_name":"tool.execute.after","tool_name":"apply_patch","file_path":""}`)
	require.NoError(t, HandleAgentPostToolUse(p, zerolog.Nop()))

	attr := store.LoadAILineAttribution("ses-empty-path")
	assert.Empty(t, attr.Files)
}

func TestHandleAgentPostToolUse_NonFileWritingToolSkips(t *testing.T) {
	_, gitDir := initGitRepo(t)
	store := state.NewGitStore(gitDir)
	require.NoError(t, store.InitTraceDir())

	p := opencode.New()

	withStdin(t, `{"session_id":"ses-non-writing","hook_event_name":"tool.execute.after","tool_name":"bash","file_path":"/some/path"}`)
	require.NoError(t, HandleAgentPostToolUse(p, zerolog.Nop()))

	attr := store.LoadAILineAttribution("ses-non-writing")
	assert.Empty(t, attr.Files)
}

func TestHandleAgentPreToolUse_EmptyFilePathSkips(t *testing.T) {
	_, gitDir := initGitRepo(t)
	store := state.NewGitStore(gitDir)
	require.NoError(t, store.InitTraceDir())

	p := opencode.New()

	withStdin(t, `{"session_id":"ses-pre-empty","hook_event_name":"tool.execute.before","tool_name":"apply_patch","file_path":""}`)
	require.NoError(t, HandleAgentPreToolUse(p, zerolog.Nop()))
}

func TestHandleAgentPreToolUse_NonFileWritingToolSkips(t *testing.T) {
	_, gitDir := initGitRepo(t)
	store := state.NewGitStore(gitDir)
	require.NoError(t, store.InitTraceDir())

	p := opencode.New()

	withStdin(t, `{"session_id":"ses-pre-non-writing","hook_event_name":"tool.execute.before","tool_name":"bash","file_path":"/some/path"}`)
	require.NoError(t, HandleAgentPreToolUse(p, zerolog.Nop()))
}
