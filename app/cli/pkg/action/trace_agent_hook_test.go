package action

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/chainloop-dev/chainloop/app/cli/internal/trace/claude"
	"github.com/chainloop-dev/chainloop/app/cli/internal/trace/hooks"
	"github.com/chainloop-dev/chainloop/app/cli/internal/trace/opencode"
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
		assert.Contains(t, stdout, "This session will be attested by Chainloop")
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
	_, err := store.LoadShellPreSignature("ses-cmd")
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
	_, err := store.LoadShellPreSignature(sid)
	assert.Error(t, err)
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
