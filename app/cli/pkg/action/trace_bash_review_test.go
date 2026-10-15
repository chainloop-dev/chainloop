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

	"github.com/chainloop-dev/chainloop/app/cli/internal/trace/state"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"
)

func TestCommitLifecycle_BashCommitOnlyHonorsGitIndexFile(t *testing.T) {
	// Given
	dir, gitDir := initGitRepo(t)
	store := state.NewGitStore(gitDir)
	require.NoError(t, store.InitTraceDir())

	const sessionID = "commit-only-session"
	require.NoError(t, store.SaveSessionRecord(&state.SessionRecord{
		SessionID: sessionID,
		Provider:  providerClaudeCode,
		Active:    true,
		StartedAt: state.NowTimestamp(),
	}))

	beginTaggedBashCall(t,
		state.ShellCallKey{SessionID: sessionID, ToolUseID: "tool-commit-only"},
		"edit and commit --only",
	)

	require.NoError(t, os.WriteFile(filepath.Join(dir, "init.txt"), []byte("changed through temporary index\n"), 0600))
	t.Setenv("GIT_INDEX_FILE", filepath.Join(t.TempDir(), "index"))
	runGit(t, dir, "read-tree", "HEAD")
	runGit(t, dir, "add", "init.txt")

	// When
	msgPath := filepath.Join(gitDir, "COMMIT_EDITMSG")
	require.NoError(t, os.WriteFile(msgPath, []byte("feat: commit only\n"), 0600))
	require.NoError(t, handleCommitMsg(msgPath, zerolog.Nop()))

	// Then
	msg, err := os.ReadFile(msgPath)
	require.NoError(t, err)
	require.Contains(t, string(msg), "Chainloop-Trace-Sessions: "+sessionID)
	require.Contains(t, store.LoadAILineAttribution(sessionID).Pending, "init.txt")
}

func TestCommitLifecycle_BashCommitWithRecreatedDeletedFileKeepsPending(t *testing.T) {
	// Given
	dir, gitDir := initGitRepo(t)
	store := state.NewGitStore(gitDir)
	require.NoError(t, store.InitTraceDir())

	const sessionID = "recreated-deletion-session"
	require.NoError(t, store.SaveSessionRecord(&state.SessionRecord{
		SessionID: sessionID,
		Provider:  providerClaudeCode,
		Active:    true,
		StartedAt: state.NowTimestamp(),
	}))

	const toolUseID = "tool-recreated-deletion"
	provider := beginTaggedBashCall(t,
		state.ShellCallKey{SessionID: sessionID, ToolUseID: toolUseID},
		"delete, recreate, commit",
	)

	runGit(t, dir, "rm", "init.txt")
	require.NoError(t, os.WriteFile(filepath.Join(dir, "init.txt"), []byte("init"), 0600))

	msgPath := filepath.Join(gitDir, "COMMIT_EDITMSG")
	require.NoError(t, os.WriteFile(msgPath, []byte("feat: delete tracked file\n"), 0600))
	require.NoError(t, handleCommitMsg(msgPath, zerolog.Nop()))
	runGit(t, dir, "commit", "-F", msgPath)
	require.NoError(t, handlePostCommit(t.Context(), zerolog.Nop()))

	// When
	withStdin(t, fmt.Sprintf(
		`{"session_id":%q,"tool_use_id":%q,"hook_event_name":"PostToolUse","tool_name":"Bash","tool_input":{"command":"delete, recreate, commit"},"tool_response":{"stdout":"","stderr":"","interrupted":false}}`,
		sessionID, toolUseID,
	))
	require.NoError(t, HandleAgentPostToolUse(provider, zerolog.Nop()))

	// Then
	require.Contains(t, store.LoadAILineAttribution(sessionID).Pending, "init.txt",
		"recreated content must remain pending after the staged deletion is committed")
}
