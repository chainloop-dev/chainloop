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
	"github.com/stretchr/testify/require"
)

func beginTaggedBashCall(t *testing.T, key state.ShellCallKey, command string) trace.Provider {
	t.Helper()

	cwd, err := os.Getwd()
	require.NoError(t, err)
	payload := fmt.Sprintf(
		`{"session_id":%q,"tool_use_id":%q,"cwd":%q,"hook_event_name":"PreToolUse","tool_name":"Bash","tool_input":{"command":%q}}`,
		key.SessionID, key.ToolUseID, cwd, command,
	)
	provider := claude.New()
	withStdin(t, payload)
	out := captureStdout(t, func() {
		require.NoError(t, HandleAgentPreToolUse(provider, zerolog.Nop()))
	})
	require.Contains(t, out, state.ShellCallEnv)

	token, err := state.EncodeShellCallKey(key)
	require.NoError(t, err)
	t.Setenv(state.ShellCallEnv, token)

	return provider
}

func TestCommitLifecycle_BashCommitBeforePostToolUseAddsSession(t *testing.T) {
	// Given
	dir, gitDir := initGitRepo(t)
	store := state.NewGitStore(gitDir)
	require.NoError(t, store.InitTraceDir())

	const sessionID = "same-call-session"
	require.NoError(t, store.SaveSessionRecord(&state.SessionRecord{
		SessionID: sessionID,
		Provider:  providerClaudeCode,
		Active:    true,
		StartedAt: state.NowTimestamp(),
	}))

	const toolUseID = "tool-same-call"
	key := state.ShellCallKey{SessionID: sessionID, ToolUseID: toolUseID}
	provider := beginTaggedBashCall(t, key, "edit, commit, push")

	// When
	target := filepath.Join(dir, "same-call.txt")
	require.NoError(t, os.WriteFile(target, []byte("changed by Bash\n"), 0600))
	runGit(t, dir, "add", "same-call.txt")

	msgPath := filepath.Join(gitDir, "COMMIT_EDITMSG")
	require.NoError(t, os.WriteFile(msgPath, []byte("feat: same-call commit\n"), 0600))
	require.NoError(t, handleCommitMsg(msgPath, zerolog.Nop()))

	// Then
	msg, err := os.ReadFile(msgPath)
	require.NoError(t, err)
	require.Contains(t, string(msg), "Chainloop-Trace-Sessions: "+sessionID)

	runGit(t, dir, "commit", "-F", msgPath)
	require.NoError(t, handlePostCommit(t.Context(), zerolog.Nop()))

	heads, err := store.LoadShellPreHeads(key)
	require.NoError(t, err)
	require.Equal(t, headSHA(t, dir), heads[dir], "the open Bash call must advance to the commit it just made")

	commits, err := store.LoadAllCommitRecords()
	require.NoError(t, err)
	require.Len(t, commits, 1)
	require.Equal(t, []string{sessionID}, commits[0].SessionIDs)

	postInput := fmt.Sprintf(
		`{"session_id":%q,"tool_use_id":%q,"hook_event_name":"PostToolUse","tool_name":"Bash","tool_input":{"command":"edit, commit, push"},"tool_response":{"stdout":"","stderr":"","interrupted":false}}`,
		sessionID, toolUseID,
	)
	withStdin(t, postInput)
	require.NoError(t, HandleAgentPostToolUse(provider, zerolog.Nop()))

	attr := store.LoadAILineAttribution(sessionID)
	require.Contains(t, attr.Files, "same-call.txt")
	require.False(t, attr.Legacy["same-call.txt"], "same-call attribution must use the fingerprinted edit ledger")
	require.Equal(t, 1, attr.GeneratedAdded)
	require.Empty(t, attr.Pending, "the committed file must not stay pending after the Bash call ends")
}

func TestCommitLifecycle_BashCommitDoesNotClaimPreviouslyStagedFile(t *testing.T) {
	// Given
	dir, gitDir := initGitRepo(t)
	store := state.NewGitStore(gitDir)
	require.NoError(t, store.InitTraceDir())

	target := filepath.Join(dir, "already-staged.txt")
	require.NoError(t, os.WriteFile(target, []byte("staged before Bash\n"), 0600))
	runGit(t, dir, "add", "already-staged.txt")
	require.NoError(t, os.WriteFile(target, []byte("human unstaged before Bash\n"), 0600))

	const sessionID = "unrelated-bash-session"
	require.NoError(t, store.SaveSessionRecord(&state.SessionRecord{
		SessionID: sessionID,
		Provider:  providerClaudeCode,
		Active:    true,
		StartedAt: state.NowTimestamp(),
	}))

	const toolUseID = "tool-pre-staged"
	provider := beginTaggedBashCall(t, state.ShellCallKey{SessionID: sessionID, ToolUseID: toolUseID}, "git commit")

	require.NoError(t, os.WriteFile(target, []byte("unstaged AI change\n"), 0600))

	// When
	msgPath := filepath.Join(gitDir, "COMMIT_EDITMSG")
	require.NoError(t, os.WriteFile(msgPath, []byte("feat: pre-staged commit\n"), 0600))
	require.NoError(t, handleCommitMsg(msgPath, zerolog.Nop()))

	// Then
	msg, err := os.ReadFile(msgPath)
	require.NoError(t, err)
	require.NotContains(t, string(msg), state.TrailerKey,
		"a Bash call must not claim the human content staged before it began")

	runGit(t, dir, "commit", "-F", msgPath)
	require.NoError(t, handlePostCommit(t.Context(), zerolog.Nop()))

	withStdin(t, fmt.Sprintf(
		`{"session_id":%q,"tool_use_id":%q,"hook_event_name":"PostToolUse","tool_name":"Bash","tool_input":{"command":"git commit"},"tool_response":{"stdout":"","stderr":"","interrupted":false}}`,
		sessionID, toolUseID,
	))
	require.NoError(t, HandleAgentPostToolUse(provider, zerolog.Nop()))

	attr := store.LoadAILineAttribution(sessionID)
	require.Contains(t, attr.Pending, "already-staged.txt",
		"the unstaged AI change must remain pending for a later commit")
}

func TestCommitLifecycle_BashCommitUsesStagedContentWhenWorkingFileIsGone(t *testing.T) {
	// Given
	dir, gitDir := initGitRepo(t)
	store := state.NewGitStore(gitDir)
	require.NoError(t, store.InitTraceDir())

	const sessionID = "staged-content-session"
	require.NoError(t, store.SaveSessionRecord(&state.SessionRecord{
		SessionID: sessionID,
		Provider:  providerClaudeCode,
		Active:    true,
		StartedAt: state.NowTimestamp(),
	}))

	beginTaggedBashCall(t,
		state.ShellCallKey{SessionID: sessionID, ToolUseID: "tool-staged-content"},
		"create, stage, remove, commit",
	)

	target := filepath.Join(dir, "staged-only.txt")
	require.NoError(t, os.WriteFile(target, []byte("committed from the index\n"), 0600))
	runGit(t, dir, "add", "staged-only.txt")
	require.NoError(t, os.Remove(target))

	// When
	msgPath := filepath.Join(gitDir, "COMMIT_EDITMSG")
	require.NoError(t, os.WriteFile(msgPath, []byte("feat: commit staged-only content\n"), 0600))
	require.NoError(t, handleCommitMsg(msgPath, zerolog.Nop()))

	// Then
	msg, err := os.ReadFile(msgPath)
	require.NoError(t, err)
	require.Contains(t, string(msg), "Chainloop-Trace-Sessions: "+sessionID)
	require.Contains(t, store.LoadAILineAttribution(sessionID).Pending, "staged-only.txt")
}

func TestCommitLifecycle_BashCommitInLinkedCheckoutAddsSession(t *testing.T) {
	// Given
	home, homeGitDir := initGitRepo(t)
	linked := filepath.Join(t.TempDir(), "linked")
	runGit(t, home, "worktree", "add", "-b", "linked", linked)
	t.Cleanup(func() { _ = os.Chdir(home) })

	homeStore := state.NewGitStore(homeGitDir)
	require.NoError(t, homeStore.InitTraceDir())

	const sessionID = "linked-checkout-session"
	require.NoError(t, homeStore.SaveSessionRecord(&state.SessionRecord{
		SessionID: sessionID,
		Provider:  providerClaudeCode,
		Cwd:       home,
		Checkouts: []string{linked},
		Active:    true,
		StartedAt: state.NowTimestamp(),
	}))

	const toolUseID = "tool-linked"
	provider := beginTaggedBashCall(t,
		state.ShellCallKey{SessionID: sessionID, ToolUseID: toolUseID},
		"cd linked && edit, commit, push",
	)

	require.NoError(t, os.Chdir(linked))
	target := filepath.Join(linked, "linked.txt")
	require.NoError(t, os.WriteFile(target, []byte("changed in linked checkout\n"), 0600))
	runGit(t, linked, "add", "linked.txt")

	// When
	msgPath := filepath.Join(t.TempDir(), "COMMIT_EDITMSG")
	require.NoError(t, os.WriteFile(msgPath, []byte("feat: linked same-call commit\n"), 0600))
	require.NoError(t, handleCommitMsg(msgPath, zerolog.Nop()))

	// Then
	msg, err := os.ReadFile(msgPath)
	require.NoError(t, err)
	require.Contains(t, string(msg), "Chainloop-Trace-Sessions: "+sessionID)

	runGit(t, linked, "commit", "-F", msgPath)
	require.NoError(t, handlePostCommit(t.Context(), zerolog.Nop()))

	require.NoError(t, os.Chdir(home))
	withStdin(t, fmt.Sprintf(
		`{"session_id":%q,"tool_use_id":%q,"cwd":%q,"hook_event_name":"PostToolUse","tool_name":"Bash","tool_input":{"command":"cd linked && edit, commit, push"},"tool_response":{"stdout":"","stderr":"","interrupted":false}}`,
		sessionID, toolUseID, home,
	))
	require.NoError(t, HandleAgentPostToolUse(provider, zerolog.Nop()))

	linkedStore, _, err := state.LocateFrom(linked)
	require.NoError(t, err)
	commits, err := linkedStore.LoadAllCommitRecords()
	require.NoError(t, err)
	require.Len(t, commits, 1)
	require.Equal(t, []string{sessionID}, commits[0].SessionIDs)
	require.Empty(t, linkedStore.LoadAILineAttribution(sessionID).Pending)
}

func TestCommitLifecycle_StaleBashSnapshotDoesNotClaimLaterCommit(t *testing.T) {
	// Given
	dir, gitDir := initGitRepo(t)
	store := state.NewGitStore(gitDir)
	require.NoError(t, store.InitTraceDir())

	const sessionID = "abandoned-session"
	require.NoError(t, store.SaveSessionRecord(&state.SessionRecord{
		SessionID: sessionID,
		Provider:  providerClaudeCode,
		Active:    true,
		StartedAt: state.NowTimestamp(),
	}))

	beginTaggedBashCall(t,
		state.ShellCallKey{SessionID: sessionID, ToolUseID: "tool-abandoned"},
		"abandoned",
	)
	t.Setenv(state.ShellCallEnv, "")

	target := filepath.Join(dir, "later.txt")
	require.NoError(t, os.WriteFile(target, []byte("later human change\n"), 0600))
	runGit(t, dir, "add", "later.txt")

	// When
	msgPath := filepath.Join(gitDir, "COMMIT_EDITMSG")
	require.NoError(t, os.WriteFile(msgPath, []byte("feat: later commit\n"), 0600))
	require.NoError(t, handleCommitMsg(msgPath, zerolog.Nop()))

	// Then
	msg, err := os.ReadFile(msgPath)
	require.NoError(t, err)
	require.NotContains(t, string(msg), state.TrailerKey)
}
