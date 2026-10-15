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
	"path/filepath"
	"testing"

	"github.com/chainloop-dev/chainloop/app/cli/internal/trace/state"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"
)

func TestCleanupSessionShellState_RemovesOnlyShellSnapshotsAcrossCheckouts(t *testing.T) {
	// Given
	home, homeGitDir := initGitRepo(t)
	linked := filepath.Join(t.TempDir(), "linked")
	runGit(t, home, "worktree", "add", "-b", "cleanup-linked", linked)

	const sessionID = "cleanup-session"
	rec := &state.SessionRecord{
		SessionID: sessionID,
		Provider:  providerClaudeCode,
		Cwd:       home,
		Checkouts: []string{linked},
		Active:    true,
		StartedAt: state.NowTimestamp(),
	}
	homeStore := state.NewGitStore(homeGitDir)
	require.NoError(t, homeStore.SaveSessionRecord(rec))
	linkedStore, _, err := state.LocateFrom(linked)
	require.NoError(t, err)
	require.NoError(t, linkedStore.SaveSessionRecord(rec))

	key := state.ShellCallKey{SessionID: sessionID, ToolUseID: "tool-cleanup"}
	sig := state.WorktreeSignatures{home: {"kept.txt": "hash"}}
	for _, store := range []*state.Store{homeStore, linkedStore} {
		require.NoError(t, store.SaveShellPreSignature(key, sig))
		require.NoError(t, store.SaveShellPreIndexSignature(key, sig))
	}
	require.NoError(t, homeStore.SaveFileSnapshot(sessionID, "kept.txt", []byte("keep file snapshot")))

	// When
	cleanupSessionShellState(homeStore, sessionID, zerolog.Nop())

	// Then
	for _, store := range []*state.Store{homeStore, linkedStore} {
		_, err := store.LoadShellPreSignature(key)
		require.Error(t, err)
		_, err = store.LoadShellPreIndexSignature(key)
		require.Error(t, err)
		record, err := store.LoadSessionRecord(sessionID)
		require.NoError(t, err)
		require.False(t, record.Active)
	}
	content, err := homeStore.LoadFileSnapshot(sessionID, "kept.txt")
	require.NoError(t, err)
	require.Equal(t, []byte("keep file snapshot"), content)
}
