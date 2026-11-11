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

	"github.com/chainloop-dev/chainloop/app/cli/internal/trace/attribution"
	tracegit "github.com/chainloop-dev/chainloop/app/cli/internal/trace/git"
	"github.com/chainloop-dev/chainloop/app/cli/internal/trace/state"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"
)

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
	require.NoError(t, os.WriteFile(filepath.Join(dir, "init.txt"), []byte("init"), 0o600))

	msgPath := filepath.Join(gitDir, "COMMIT_EDITMSG")
	require.NoError(t, os.WriteFile(msgPath, []byte("feat: delete tracked file\n"), 0o600))
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

func TestCheckpointActiveShellAttribution_BatchReadFailurePreservesReadablePaths(t *testing.T) {
	tests := []struct {
		name            string
		removeFromIndex bool
	}{
		{name: "indexed paths"},
		{name: "pre-staged deletions", removeFromIndex: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Given
			dir, gitDir := initGitRepo(t)
			require.NoError(t, os.WriteFile(filepath.Join(dir, "good.txt"), []byte("unchanged human line\n"), 0o600))
			require.NoError(t, os.WriteFile(filepath.Join(dir, "broken.txt"), []byte("distinct old content\n"), 0o600))
			runGit(t, dir, "add", "good.txt", "broken.txt")
			runGit(t, dir, "commit", "-m", "base files")

			client := tracegit.NewGoGitClient()
			index, err := client.SnapshotIndex(dir)
			require.NoError(t, err)
			brokenHash := index["broken.txt"]
			require.Greater(t, len(brokenHash), 2)
			if tt.removeFromIndex {
				// Keep the working files while staging their deletion before Bash.
				runGit(t, dir, "rm", "--cached", "--", "good.txt", "broken.txt")
			}

			store := state.NewGitStore(gitDir)
			require.NoError(t, store.InitTraceDir())
			const sessionID = "batch-read-failure-session"
			require.NoError(t, store.SaveSessionRecord(&state.SessionRecord{
				SessionID: sessionID,
				Provider:  providerClaudeCode,
				Active:    true,
				StartedAt: state.NowTimestamp(),
			}))
			key := state.ShellCallKey{SessionID: sessionID, ToolUseID: "batch-read-failure-call"}
			beginTaggedBashCall(t, key, "edit and stage files")
			heads, err := store.LoadShellPreHeads(key)
			require.NoError(t, err)
			require.NotEmpty(t, heads[dir])

			require.NoError(t, os.WriteFile(filepath.Join(dir, "good.txt"), []byte("unchanged human line\nnew AI line\n"), 0o600))
			require.NoError(t, os.WriteFile(filepath.Join(dir, "broken.txt"), []byte("replacement content\n"), 0o600))
			runGit(t, dir, "add", "good.txt", "broken.txt")

			brokenObject := filepath.Join(gitDir, "objects", brokenHash[:2], brokenHash[2:])
			require.NoError(t, os.Chmod(brokenObject, 0o600))
			require.NoError(t, os.WriteFile(brokenObject, []byte("invalid compressed object"), 0o600))
			t.Setenv("PATH", "")
			contents, err := client.FilesAt(dir, heads[dir], []string{"good.txt", "broken.txt"})
			require.Error(t, err)
			require.Nil(t, contents)

			// When
			require.NoError(t, checkpointActiveShellAttribution(store, dir, []string{"good.txt", "broken.txt"}))

			// Then
			open := store.LoadAILineAttribution(sessionID).Open["good.txt"]
			require.NotNil(t, open)
			require.Equal(t, state.LineCounts{attribution.Fingerprint("new AI line"): 1}, open.Added,
				"a batch failure must not credit unchanged content in a readable file")
		})
	}
}

func TestCommitLifecycle_BashRestageDoesNotClaimPreStagedHumanLines(t *testing.T) {
	// Given
	dir, gitDir := initGitRepo(t)
	target := filepath.Join(dir, "shared.txt")
	require.NoError(t, os.WriteFile(target, []byte("base line\n"), 0o600))
	runGit(t, dir, "add", "shared.txt")
	runGit(t, dir, "commit", "-m", "base file")
	require.NoError(t, os.WriteFile(target, []byte("base line\nhuman staged line\n"), 0o600))
	runGit(t, dir, "add", "shared.txt")

	store := state.NewGitStore(gitDir)
	require.NoError(t, store.InitTraceDir())
	const sessionID = "restaged-human-session"
	require.NoError(t, store.SaveSessionRecord(&state.SessionRecord{
		SessionID: sessionID,
		Provider:  providerClaudeCode,
		Active:    true,
		StartedAt: state.NowTimestamp(),
	}))
	beginTaggedBashCall(t,
		state.ShellCallKey{SessionID: sessionID, ToolUseID: "restaged-human-call"},
		"append one line and restage the file",
	)

	// When
	require.NoError(t, os.WriteFile(target, []byte("base line\nhuman staged line\nAI added line\n"), 0o600))
	runGit(t, dir, "add", "shared.txt")
	msg := filepath.Join(gitDir, "COMMIT_EDITMSG")
	require.NoError(t, os.WriteFile(msg, []byte("feat: restage edited file\n"), 0o600))
	require.NoError(t, handleCommitMsg(msg, zerolog.Nop()))

	// Then
	attr := store.LoadAILineAttribution(sessionID)
	open := attr.Open["shared.txt"]
	require.NotNil(t, open)
	require.Equal(t, state.LineCounts{attribution.Fingerprint("AI added line"): 1}, open.Added,
		"restaging must not credit the earlier human addition to Bash")
	added, _ := attr.GeneratedLines(nil)
	require.Equal(t, 1, added)
}
