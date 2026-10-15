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

func TestCommitLifecycle_BashCommitWithCleanFilterDoesNotRequeueAttribution(t *testing.T) {
	// Given
	dir, gitDir := initGitRepo(t)
	require.NoError(t, os.WriteFile(filepath.Join(dir, ".gitattributes"), []byte("filtered.txt text eol=lf\n"), 0600))
	runGit(t, dir, "add", ".gitattributes")
	runGit(t, dir, "commit", "-m", "chore: add attributes")

	store := state.NewGitStore(gitDir)
	require.NoError(t, store.InitTraceDir())

	const sessionID = "filtered-content-session"
	require.NoError(t, store.SaveSessionRecord(&state.SessionRecord{
		SessionID: sessionID,
		Provider:  providerClaudeCode,
		Active:    true,
		StartedAt: state.NowTimestamp(),
	}))

	const toolUseID = "tool-filtered-content"
	provider := beginTaggedBashCall(t,
		state.ShellCallKey{SessionID: sessionID, ToolUseID: toolUseID},
		"create filtered content, commit, push",
	)

	target := filepath.Join(dir, "filtered.txt")
	require.NoError(t, os.WriteFile(target, []byte("first\r\nsecond\r\n"), 0600))
	runGit(t, dir, "add", "filtered.txt")

	msgPath := filepath.Join(gitDir, "COMMIT_EDITMSG")
	require.NoError(t, os.WriteFile(msgPath, []byte("feat: filtered same-call commit\n"), 0600))
	require.NoError(t, handleCommitMsg(msgPath, zerolog.Nop()))
	runGit(t, dir, "commit", "-F", msgPath)
	require.NoError(t, handlePostCommit(t.Context(), zerolog.Nop()))

	// When
	withStdin(t, fmt.Sprintf(
		`{"session_id":%q,"tool_use_id":%q,"hook_event_name":"PostToolUse","tool_name":"Bash","tool_input":{"command":"create filtered content, commit, push"},"tool_response":{"stdout":"","stderr":"","interrupted":false}}`,
		sessionID, toolUseID,
	))
	require.NoError(t, HandleAgentPostToolUse(provider, zerolog.Nop()))

	// Then
	require.Empty(t, store.LoadAILineAttribution(sessionID).Pending,
		"a clean filtered working file must not be recorded again after commit")
}

func TestCommitLifecycle_BashCommitWithBinaryCRLFKeepsUnstagedAttribution(t *testing.T) {
	// Given
	dir, gitDir := initGitRepo(t)
	require.NoError(t, os.WriteFile(filepath.Join(dir, ".gitattributes"), []byte("binary.txt -text\n"), 0600))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "binary.txt"), []byte("before\n"), 0600))
	runGit(t, dir, "config", "core.autocrlf", "true")
	runGit(t, dir, "add", ".gitattributes", "binary.txt")
	runGit(t, dir, "commit", "-m", "chore: add binary content")

	store := state.NewGitStore(gitDir)
	require.NoError(t, store.InitTraceDir())

	const sessionID = "binary-crlf-session"
	require.NoError(t, store.SaveSessionRecord(&state.SessionRecord{
		SessionID: sessionID,
		Provider:  providerClaudeCode,
		Active:    true,
		StartedAt: state.NowTimestamp(),
	}))

	const toolUseID = "tool-binary-crlf"
	provider := beginTaggedBashCall(t,
		state.ShellCallKey{SessionID: sessionID, ToolUseID: toolUseID},
		"stage LF content, leave CRLF content unstaged, commit",
	)

	target := filepath.Join(dir, "binary.txt")
	require.NoError(t, os.WriteFile(target, []byte("after\n"), 0600))
	runGit(t, dir, "add", "binary.txt")
	require.NoError(t, os.WriteFile(target, []byte("after\r\n"), 0600))

	msgPath := filepath.Join(gitDir, "COMMIT_EDITMSG")
	require.NoError(t, os.WriteFile(msgPath, []byte("feat: binary same-call commit\n"), 0600))
	require.NoError(t, handleCommitMsg(msgPath, zerolog.Nop()))
	runGit(t, dir, "commit", "-F", msgPath)
	require.NoError(t, handlePostCommit(t.Context(), zerolog.Nop()))

	// When
	withStdin(t, fmt.Sprintf(
		`{"session_id":%q,"tool_use_id":%q,"hook_event_name":"PostToolUse","tool_name":"Bash","tool_input":{"command":"stage LF content, leave CRLF content unstaged, commit"},"tool_response":{"stdout":"","stderr":"","interrupted":false}}`,
		sessionID, toolUseID,
	))
	require.NoError(t, HandleAgentPostToolUse(provider, zerolog.Nop()))

	// Then
	require.Contains(t, store.LoadAILineAttribution(sessionID).Pending, "binary.txt",
		"real unstaged CRLF content must remain pending after the staged LF content is committed")
}
