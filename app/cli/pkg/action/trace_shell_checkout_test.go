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
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/chainloop-dev/chainloop/app/cli/internal/trace"
	"github.com/chainloop-dev/chainloop/app/cli/internal/trace/claude"
	"github.com/chainloop-dev/chainloop/app/cli/internal/trace/cursor"
	"github.com/chainloop-dev/chainloop/app/cli/internal/trace/hooks"
	"github.com/chainloop-dev/chainloop/app/cli/internal/trace/opencode"
	"github.com/chainloop-dev/chainloop/app/cli/internal/trace/state"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// PFM-7614: a Claude Code session started in a traced checkout clones a second
// repository and works there with shell commands only: no file tool touches a
// path in the clone. Every agent hook runs from the directory the session
// started in, so the clone is found from the command text.
//
// The steps follow the subagent case in the ticket: the agent writes a file
// outside any repository, clones the repository in one Bash call, copies the
// file into the clone in a second call, and commits in a third.
func TestClaudeSessionCommitsInClonedCheckoutThroughShell(t *testing.T) {
	const sid = "7614a0c2-1a2b-4c3d-8e9f-0a1b2c3d4e5f"

	home := t.TempDir()
	setTestHome(t, home)

	// The repository that the agent clones. It is traced: it has a project
	// name, which the git hook install requires.
	upstream := resolvedTempGitRepo(t)
	configureGitUser(t, upstream)
	require.NoError(t, os.WriteFile(filepath.Join(upstream, ".chainloop.yml"), []byte("projectName: repo-b\n"), 0600))
	require.NoError(t, os.MkdirAll(filepath.Join(upstream, "docs"), 0755))
	require.NoError(t, os.WriteFile(filepath.Join(upstream, "docs", "README.md"), []byte("docs\n"), 0600))
	runGit(t, upstream, "add", ".")
	runGit(t, upstream, "commit", "-m", "initial")

	mainRoot := chdirToResolvedGitRepo(t)
	require.NoError(t, os.WriteFile(filepath.Join(mainRoot, ".chainloop.yml"), []byte("projectName: repo-a\n"), 0600))

	scratch, err := filepath.EvalSymlinks(t.TempDir())
	require.NoError(t, err)
	clone := filepath.Join(scratch, "repo-b")
	notes := filepath.Join(scratch, "notes")
	require.NoError(t, os.MkdirAll(notes, 0755))

	p := claude.New()
	hook := func(event, tool, toolUseID, toolInput string, handler func(trace.Provider, zerolog.Logger) error) {
		t.Helper()
		withStdin(t, fmt.Sprintf(`{"session_id":%q,"cwd":%q,"hook_event_name":%q,"tool_name":%q,"tool_use_id":%q,"tool_input":%s}`,
			sid, mainRoot, event, tool, toolUseID, toolInput))
		require.NoError(t, handler(p, zerolog.Nop()))
	}
	// bash replays one Bash call: the pre hook, the effect of the command,
	// then the post hook. Both hooks run from the session's directory.
	bash := func(toolUseID, command string, effect func()) {
		t.Helper()
		input := fmt.Sprintf(`{"command":%q}`, command)
		hook("PreToolUse", "Bash", toolUseID, input, HandleAgentPreToolUse)
		effect()
		hook("PostToolUse", "Bash", toolUseID, input, HandleAgentPostToolUse)
	}

	withStdin(t, fmt.Sprintf(`{"session_id":%q,"cwd":%q,"hook_event_name":"SessionStart","source":"startup"}`, sid, mainRoot))
	require.NoError(t, HandleAgentSessionStart(p, zerolog.Nop()))

	// 1. The agent writes a file in a folder that is not a git repository.
	const specBody = "# Spec\n\nline\n"
	spec := filepath.Join(notes, "spec.md")
	writeInput := fmt.Sprintf(`{"file_path":%q,"content":%q}`, spec, specBody)
	hook("PreToolUse", "Write", "toolu_w1", writeInput, HandleAgentPreToolUse)
	require.NoError(t, os.WriteFile(spec, []byte(specBody), 0600))
	hook("PostToolUse", "Write", "toolu_w1", writeInput, HandleAgentPostToolUse)

	// 2. The agent clones the repository. The clone does not exist at the
	// pre hook, so the post hook registers it.
	bash("toolu_b1", fmt.Sprintf("git clone %s %s", upstream, clone), func() {
		runGit(t, scratch, "clone", upstream, clone)
	})

	cloneGitDir := filepath.Join(clone, ".git")
	assert.True(t, hooks.IsInstalled(cloneGitDir, false), "the git hooks are installed in the clone")

	homeStore := state.NewGitStore(filepath.Join(mainRoot, ".git"))
	rec, err := homeStore.LoadSessionRecord(sid)
	require.NoError(t, err)
	require.NotNil(t, rec)
	assert.Equal(t, []string{clone}, rec.Checkouts, "the clone is registered on the session record")

	// 3. The agent copies the file into the clone with a shell command.
	bash("toolu_b2", fmt.Sprintf("cd %s && cp %s docs/", clone, spec), func() {
		require.NoError(t, os.WriteFile(filepath.Join(clone, "docs", "spec.md"), []byte(specBody), 0600))
	})

	cloneStore := state.NewGitStore(cloneGitDir)
	assert.Contains(t, cloneStore.LoadAILineAttribution(sid).Files, "docs/spec.md",
		"the shell change is recorded in the ledger of the clone")
	assert.Empty(t, homeStore.LoadAILineAttribution(sid).Files, "the session's own checkout did not change")

	// 4. The agent commits in the clone. Git runs the commit-msg and
	// post-commit hooks inside the clone.
	bash("toolu_b3", fmt.Sprintf(`cd %s && git add docs/spec.md && git commit -s -m "docs: add spec"`, clone), func() {
		t.Chdir(clone)
		defer t.Chdir(mainRoot)

		configureGitUser(t, clone)
		runGit(t, clone, "add", "docs/spec.md")
		msgFile := filepath.Join(t.TempDir(), "COMMIT_EDITMSG")
		require.NoError(t, os.WriteFile(msgFile, []byte("docs: add spec\n"), 0600))
		require.NoError(t, handleCommitMsg(msgFile, zerolog.Nop()))

		msg, err := os.ReadFile(msgFile)
		require.NoError(t, err)
		assert.Contains(t, string(msg), state.TrailerKey+": "+sid, "commit-msg stamps the session")

		runGit(t, clone, "commit", "--no-verify", "-F", msgFile)
		require.NoError(t, handlePostCommit(t.Context(), zerolog.Nop()))
	})

	// What pre-push reads in the clone.
	commits, err := cloneStore.LoadAllCommitRecords()
	require.NoError(t, err)
	require.Len(t, commits, 1)
	assert.Equal(t, []string{sid}, commits[0].SessionIDs, "the commit is credited to the session")

	sessions, err := cloneStore.LoadAllSessionRecords()
	require.NoError(t, err)
	require.Contains(t, sessions, sid, "pre-push resolves the provider from the session record of the clone")
	assert.Equal(t, mainRoot, sessions[sid].Cwd, "the record keeps the directory the agent runs in")
}

// OpenCode and Cursor send the command line of a shell call too, each in its
// own payload. A checkout that exists before the call is registered at the
// pre hook, so the changes that the same call makes there are recorded in its
// ledger.
func TestShellCommandInOtherCheckout(t *testing.T) {
	const sid = "ses_7614"

	cases := []struct {
		name     string
		provider trace.Provider
		// start, pre and post return the payloads of the session start and
		// of the hooks before and after one shell call of command.
		start     string
		pre, post func(command string) string
	}{
		{
			name:     "opencode",
			provider: opencode.New(),
			start:    fmt.Sprintf(`{"session_id":%q,"hook_event_name":"session.created"}`, sid),
			pre: func(command string) string {
				return fmt.Sprintf(`{"session_id":%q,"hook_event_name":"tool.execute.before","tool_name":"bash","tool_use_id":"call_01","command":%q}`, sid, command)
			},
			post: func(command string) string {
				return fmt.Sprintf(`{"session_id":%q,"hook_event_name":"tool.execute.after","tool_name":"bash","tool_use_id":"call_01","command":%q}`, sid, command)
			},
		},
		{
			name:     "cursor",
			provider: cursor.New(),
			start:    fmt.Sprintf(`{"conversation_id":%q,"hook_event_name":"sessionStart"}`, sid),
			pre: func(command string) string {
				return fmt.Sprintf(`{"conversation_id":%q,"hook_event_name":"preToolUse","tool_name":"Shell","tool_use_id":"call-1","tool_input":{"command":%q}}`, sid, command)
			},
			post: func(command string) string {
				return fmt.Sprintf(`{"conversation_id":%q,"hook_event_name":"postToolUse","tool_name":"Shell","tool_use_id":"call-1","tool_input":{"command":%q},"tool_output":"{}"}`, sid, command)
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			mainRoot := chdirToResolvedGitRepo(t)
			other := resolvedTempGitRepo(t)
			require.NoError(t, os.WriteFile(filepath.Join(other, ".chainloop.yml"), []byte("projectName: repo-b\n"), 0600))

			p := tc.provider
			command := fmt.Sprintf("cd %s && sed -i '' s/a/b/ notes.md", other)

			withStdin(t, tc.start)
			captureStdout(t, func() { require.NoError(t, HandleAgentSessionStart(p, zerolog.Nop())) })

			withStdin(t, tc.pre(command))
			captureStdout(t, func() { require.NoError(t, HandleAgentPreToolUse(p, zerolog.Nop())) })
			require.NoError(t, os.WriteFile(filepath.Join(other, "notes.md"), []byte("b\n"), 0600))
			withStdin(t, tc.post(command))
			captureStdout(t, func() { require.NoError(t, HandleAgentPostToolUse(p, zerolog.Nop())) })

			otherGitDir := filepath.Join(other, ".git")
			assert.True(t, hooks.IsInstalled(otherGitDir, false), "the git hooks are installed in the other checkout")

			rec, err := state.NewGitStore(filepath.Join(mainRoot, ".git")).LoadSessionRecord(sid)
			require.NoError(t, err)
			require.NotNil(t, rec)
			assert.Equal(t, []string{other}, rec.Checkouts)

			otherStore := state.NewGitStore(otherGitDir)
			assert.True(t, otherStore.SessionRecordExists(sid))
			assert.Contains(t, otherStore.LoadAILineAttribution(sid).Files, "notes.md",
				"the change of the same call is recorded in the other checkout")
		})
	}
}

func configureGitUser(t *testing.T, dir string) {
	t.Helper()
	runGit(t, dir, "config", "user.email", "test@test.com")
	runGit(t, dir, "config", "user.name", "Test")
	runGit(t, dir, "config", "commit.gpgsign", "false")
}
