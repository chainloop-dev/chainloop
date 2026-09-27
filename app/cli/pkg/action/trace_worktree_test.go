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
	"maps"
	"os"
	"path/filepath"
	"slices"
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
					require.Contains(t, sessions, sid,
						"pre-push resolves the provider from the session record")
					assert.Equal(t, mainRoot, sessions[sid].Cwd,
						"the record keeps the directory the agent runs in")

					// Pre-push copies the transcript again before parsing it,
					// from the directory in the session record. Drop the copy
					// the tool hooks made, so only that path can pass.
					require.NoError(t, os.RemoveAll(store.RawSessionDir()))
					require.NoError(t, p.CopySessionData(store, sessionLocation(sid, sessions[sid].Cwd, sessions[sid].TranscriptPath, repoRoot)))
					_, err = p.ParseSession(t.Context(), &trace.ParseOpts{SessionDir: store.RawSessionDir(), SessionID: sid})
					assert.NoError(t, err, "pre-push must find the session transcript")
				})
			}
		})
	}
}

// A Claude Code hook can run with cwd inside a git worktree while the
// transcript stays under the project directory of the directory the session
// started in. That happens for a subagent spawned with isolation "worktree"
// (its hooks carry the parent's session_id plus an agent_id) and for a main
// session that moves into a worktree. The transcript must be found through
// transcript_path, not through the cwd encoding.
func TestClaudeTranscriptFoundFromWorktreeCwd(t *testing.T) {
	// Matches the sessionId inside the transcript fixture, which is the ID
	// the parser looks up subagents by.
	const sid = "test-session"

	fixtures := "../../internal/trace/claude/testdata/"
	transcriptFixture, err := os.ReadFile(fixtures + "test-session.jsonl")
	require.NoError(t, err)
	subagentFixture, err := os.ReadFile(fixtures + "test-session/subagents/agent-abc123.jsonl")
	require.NoError(t, err)
	subagentMetaFixture, err := os.ReadFile(fixtures + "test-session/subagents/agent-abc123.meta.json")
	require.NoError(t, err)

	cases := []struct {
		name    string
		agentID string
	}{
		{name: "subagent in its own worktree", agentID: "abc123"},
		{name: "main session moved into a worktree"},
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

			// The transcripts live under the project directory of the
			// directory the session started in.
			transcriptDir := p.SessionDirForRepo(mainRoot)
			subagentsDir := filepath.Join(transcriptDir, sid, "subagents")
			require.NoError(t, os.MkdirAll(subagentsDir, 0o755))
			transcript := filepath.Join(transcriptDir, sid+".jsonl")
			require.NoError(t, os.WriteFile(transcript, transcriptFixture, 0600))
			require.NoError(t, os.WriteFile(filepath.Join(subagentsDir, "agent-abc123.jsonl"), subagentFixture, 0600))
			require.NoError(t, os.WriteFile(filepath.Join(subagentsDir, "agent-abc123.meta.json"), subagentMetaFixture, 0600))

			hook := func(cwd, event, extra string, handler func(trace.Provider, zerolog.Logger) error) {
				t.Helper()
				if tc.agentID != "" && event != "SessionStart" {
					extra += fmt.Sprintf(`,"agent_id":%q,"agent_type":"general-purpose"`, tc.agentID)
				}
				withStdin(t, fmt.Sprintf(`{"session_id":%q,"transcript_path":%q,"cwd":%q,"hook_event_name":%q%s}`,
					sid, transcript, cwd, event, extra))
				require.NoError(t, handler(p, zerolog.Nop()))
			}

			hook(mainRoot, "SessionStart", `,"source":"startup"`, HandleAgentSessionStart)

			wt := filepath.Join(mainRoot, ".claude", "worktrees", "agent-abc123")
			runGit(t, mainRoot, "worktree", "add", "-b", "feat-wt", wt)
			t.Chdir(wt)

			// From here on the hooks run with cwd inside the worktree.
			file := filepath.Join(wt, "wt.go")
			body := "package wt\n\nfunc F() {}\n"
			input := fmt.Sprintf(`,"tool_name":"Write","tool_input":{"file_path":%q,"content":%q}`, file, body)
			hook(wt, "PreToolUse", input, HandleAgentPreToolUse)
			require.NoError(t, os.WriteFile(file, []byte(body), 0600))
			hook(wt, "PostToolUse", input+fmt.Sprintf(`,"tool_response":{"type":"create","filePath":%q}`, file), HandleAgentPostToolUse)

			runGit(t, wt, "add", "wt.go")
			msgFile := filepath.Join(t.TempDir(), "COMMIT_EDITMSG")
			require.NoError(t, os.WriteFile(msgFile, []byte("feat: wt\n"), 0600))
			require.NoError(t, handleCommitMsg(msgFile, zerolog.Nop()))
			runGit(t, wt, "commit", "--no-verify", "-F", msgFile)
			require.NoError(t, handlePostCommit(t.Context(), zerolog.Nop()))

			// What pre-push sees from inside the worktree.
			store, repoRoot, err := state.Locate()
			require.NoError(t, err)

			commits, err := store.LoadAllCommitRecords()
			require.NoError(t, err)
			require.Len(t, commits, 1)
			assert.Equal(t, []string{sid}, commits[0].SessionIDs,
				"the commit is credited to the session ID the hooks carry")

			sessions, err := store.LoadAllSessionRecords()
			require.NoError(t, err)
			require.Contains(t, sessions, sid)
			assert.Equal(t, transcript, sessions[sid].TranscriptPath)

			// Drop the copy the tool hooks made, so only the pre-push
			// lookup can pass.
			require.NoError(t, os.RemoveAll(store.RawSessionDir()))
			require.NoError(t, p.CopySessionData(store, sessionLocation(sid, sessions[sid].Cwd, sessions[sid].TranscriptPath, repoRoot)))

			assert.FileExists(t, filepath.Join(state.RawSubagentDir(store.RawSessionDir(), sid), "agent-abc123.jsonl"))

			ev, err := p.ParseSession(t.Context(), &trace.ParseOpts{SessionDir: store.RawSessionDir(), SessionID: sid})
			require.NoError(t, err, "pre-push must find the session transcript")
			assert.ElementsMatch(t, []string{"main", "abc123"}, slices.Collect(maps.Keys(ev.Data.RawSession)),
				"the evidence has the main transcript and the subagent transcript")
		})
	}
}

// The session record is written by the first hook that reaches a store. When
// that hook carried no transcript_path, a later one fills it in, but it does
// not move the recorded cwd.
func TestEnsureSessionTracked_BackfillsTranscriptPath(t *testing.T) {
	const sid = "7412c0c2-1a2b-4c3d-8e9f-0a1b2c3d4e5f"
	t.Setenv("HOME", t.TempDir())

	root := chdirToResolvedGitRepo(t)
	store := state.NewGitStore(filepath.Join(root, ".git"))
	p := claude.New()

	withStdin(t, fmt.Sprintf(`{"session_id":%q,"cwd":%q,"hook_event_name":"SessionStart"}`, sid, root))
	require.NoError(t, HandleAgentSessionStart(p, zerolog.Nop()))

	rec, err := store.LoadSessionRecord(sid)
	require.NoError(t, err)
	require.NotNil(t, rec)
	require.Empty(t, rec.TranscriptPath)

	transcript := filepath.Join(p.SessionDirForRepo("/elsewhere"), sid+".jsonl")
	otherCwd := filepath.Join(root, "sub")
	withStdin(t, fmt.Sprintf(`{"session_id":%q,"transcript_path":%q,"cwd":%q,"hook_event_name":"PreToolUse","tool_name":"Bash","tool_input":{"command":"true"}}`,
		sid, transcript, otherCwd))
	require.NoError(t, HandleAgentPreToolUse(p, zerolog.Nop()))

	rec, err = store.LoadSessionRecord(sid)
	require.NoError(t, err)
	assert.Equal(t, transcript, rec.TranscriptPath, "a later hook fills in the transcript path")
	assert.Equal(t, root, rec.Cwd, "a later hook does not change the recorded cwd")
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
