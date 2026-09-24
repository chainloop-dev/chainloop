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
	"github.com/chainloop-dev/chainloop/app/cli/internal/trace/state"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// PFM-7412: a Claude Code session started in a traced repository forks its
// work into two git worktrees and commits/pushes each one as its own PR.
//
// The hook sequence below replays what Claude Code actually emits in that
// scenario (captured from a real headless session): every agent hook runs
// with cwd set to the directory the session was started in — the main
// checkout — while the Write tool targets absolute paths inside the
// worktrees. The git hooks, by contrast, run inside each worktree.
func TestClaudeSessionForkedIntoWorktrees(t *testing.T) {
	const sid = "7412a0c2-1a2b-4c3d-8e9f-0a1b2c3d4e5f"

	transcriptFixture, err := os.ReadFile("../../internal/trace/claude/testdata/test-session.jsonl")
	require.NoError(t, err)

	cases := []struct {
		name string
		// worktreePath returns where `git worktree add` puts worktree n.
		worktreePath func(mainRoot, n string) string
	}{
		{
			name:         "sibling worktrees",
			worktreePath: func(mainRoot, n string) string { return filepath.Join(filepath.Dir(mainRoot), "wt-"+n) },
		},
		{
			// Claude Code's own worktree layout.
			name:         "worktrees nested under .claude/worktrees",
			worktreePath: func(mainRoot, n string) string { return filepath.Join(mainRoot, ".claude", "worktrees", n) },
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			home := t.TempDir()
			t.Setenv("HOME", home)

			mainRoot := chdirToResolvedGitRepo(t)
			runGit(t, mainRoot, "config", "user.email", "test@test.com")
			runGit(t, mainRoot, "config", "user.name", "Test")
			runGit(t, mainRoot, "config", "commit.gpgsign", "false")
			require.NoError(t, os.WriteFile(filepath.Join(mainRoot, "init.txt"), []byte("init\n"), 0600))
			runGit(t, mainRoot, "add", "init.txt")
			runGit(t, mainRoot, "commit", "-m", "initial")

			p := claude.New()

			// Claude keeps the transcript under the project directory of the
			// cwd the session was started in, i.e. the main checkout.
			transcriptDir := p.SessionDirForRepo(mainRoot)
			require.NoError(t, os.MkdirAll(transcriptDir, 0o755))
			transcript := filepath.Join(transcriptDir, sid+".jsonl")
			require.NoError(t, os.WriteFile(transcript, transcriptFixture, 0600))

			hook := func(event, extra string, handler func(trace.Provider, zerolog.Logger) error) {
				t.Helper()
				withStdin(t, fmt.Sprintf(`{"session_id":%q,"transcript_path":%q,"cwd":%q,"hook_event_name":%q%s}`,
					sid, transcript, mainRoot, event, extra))
				require.NoError(t, handler(p, zerolog.Nop()))
			}

			hook("SessionStart", `,"source":"startup"`, HandleAgentSessionStart)

			// The agent forks the work into two worktrees, one per PR.
			worktrees := map[string]string{}
			for _, n := range []string{"a", "b"} {
				wt := tc.worktreePath(mainRoot, n)
				runGit(t, mainRoot, "worktree", "add", "-b", "feat-"+n, wt)
				worktrees[n] = wt
			}

			// The agent writes one file per worktree. Hooks still run from
			// the main checkout (Claude Code does not move the hook cwd).
			for n, wt := range worktrees {
				file := filepath.Join(wt, n+".go")
				body := fmt.Sprintf("package %s\n\nfunc F() {}\n", n)
				input := fmt.Sprintf(`,"tool_name":"Write","tool_input":{"file_path":%q,"content":%q}`, file, body)
				hook("PreToolUse", input, HandleAgentPreToolUse)
				require.NoError(t, os.WriteFile(file, []byte(body), 0600))
				hook("PostToolUse", input+fmt.Sprintf(`,"tool_response":{"type":"create","filePath":%q}`, file), HandleAgentPostToolUse)
			}

			// Each worktree is committed on its own; git runs the commit-msg
			// and post-commit hooks with cwd inside that worktree.
			for n, wt := range worktrees {
				t.Run("worktree "+n, func(t *testing.T) {
					t.Chdir(wt)

					runGit(t, wt, "add", n+".go")
					msgFile := filepath.Join(t.TempDir(), "COMMIT_EDITMSG")
					require.NoError(t, os.WriteFile(msgFile, []byte("feat: "+n+"\n"), 0600))
					require.NoError(t, handleCommitMsg(msgFile, zerolog.Nop()))

					msg, err := os.ReadFile(msgFile)
					require.NoError(t, err)
					assert.Contains(t, string(msg), state.TrailerKey+": "+sid,
						"commit-msg hook must stamp the session that wrote the file")

					runGit(t, wt, "commit", "--no-verify", "-F", msgFile)
					require.NoError(t, handlePostCommit(t.Context(), zerolog.Nop()))

					// What pre-push sees from inside the worktree.
					store, repoRoot, err := state.Locate()
					require.NoError(t, err)

					commits, err := store.LoadAllCommitRecords()
					require.NoError(t, err)
					require.Len(t, commits, 1)
					assert.Equal(t, []string{sid}, commits[0].SessionIDs,
						"commit must be credited to the session, or pre-push skips the attestation")

					sessions, err := store.LoadAllSessionRecords()
					require.NoError(t, err)
					assert.Contains(t, sessions, sid,
						"pre-push resolves the provider from the session record")

					// Pre-push refreshes the transcript before parsing it.
					_ = p.CopySessionData(store, repoRoot, sid)
					_, err = p.ParseSession(t.Context(), &trace.ParseOpts{SessionDir: store.RawSessionDir(), SessionID: sid})
					assert.NoError(t, err, "pre-push must find the session transcript")
				})
			}
		})
	}
}

// Only edits follow the file to its checkout. Claude's Read carries a
// file_path too, and reading a file in another repository must not start
// tracking the session in that repository.
func TestLocateForHook_ReadStaysWithCwd(t *testing.T) {
	const sid = "7412b0c2-1a2b-4c3d-8e9f-0a1b2c3d4e5f"

	root := chdirToResolvedGitRepo(t)
	other, err := filepath.EvalSymlinks(initTempGitRepo(t))
	require.NoError(t, err)
	file := filepath.Join(other, "README.md")
	require.NoError(t, os.WriteFile(file, []byte("hello\n"), 0600))

	withStdin(t, fmt.Sprintf(`{"session_id":%q,"cwd":%q,"hook_event_name":"PreToolUse","tool_name":"Read","tool_input":{"file_path":%q}}`, sid, root, file))
	require.NoError(t, HandleAgentPreToolUse(claude.New(), zerolog.Nop()))

	assert.True(t, state.NewGitStore(filepath.Join(root, ".git")).SessionRecordExists(sid), "session is tracked where the hook runs")
	assert.False(t, state.NewGitStore(filepath.Join(other, ".git")).SessionRecordExists(sid), "a read must not start a session in the file's repository")
}
