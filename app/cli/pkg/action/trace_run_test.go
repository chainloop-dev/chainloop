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

func TestTraceRunOwnsState(t *testing.T) {
	t.Run("clean repo", func(t *testing.T) {
		assert.True(t, traceRunOwnsState(state.NewGitStore(t.TempDir())))
	})

	t.Run("live trace init install is not ours", func(t *testing.T) {
		store := state.NewGitStore(t.TempDir())
		require.NoError(t, store.MarkTraceInitialized())

		assert.False(t, traceRunOwnsState(store), "trace run must not wipe a trace init setup")
	})

	t.Run("leftover state from a dead trace run is ours", func(t *testing.T) {
		store := state.NewGitStore(t.TempDir())
		require.NoError(t, store.MarkTraceInitialized())
		require.NoError(t, store.MarkTraceRunActive())

		assert.True(t, traceRunOwnsState(store))
	})
}

// TestCleanupTraceNoGit pins the contract `chainloop trace run` relies on
// outside a git repository: an empty gitDir removes the whole out-of-tree state
// directory and never touches a hooks/ directory.
func TestCleanupTraceNoGit(t *testing.T) {
	repoRoot := t.TempDir()
	stateDir := filepath.Join(t.TempDir(), "state")
	store := state.NewOutOfTreeStore(stateDir)
	require.NoError(t, store.InitTraceDir())

	// A hooks/ directory that hooks.Uninstall would have rewritten.
	hooksDir := filepath.Join(stateDir, "hooks")
	require.NoError(t, os.MkdirAll(hooksDir, 0755))
	sentinel := filepath.Join(hooksDir, "pre-push")
	require.NoError(t, os.WriteFile(sentinel, []byte("#!/bin/sh\nexit 0\n"), 0600))

	require.NoError(t, CleanupTrace(store, repoRoot, zerolog.Nop()))

	_, err := os.Stat(filepath.Join(stateDir, "chainloop-trace"))
	assert.True(t, os.IsNotExist(err), "trace state dir should be removed")

	got, err := os.ReadFile(sentinel)
	require.NoError(t, err, "hooks/ must be left alone when there is no git dir")
	assert.Equal(t, "#!/bin/sh\nexit 0\n", string(got))
}

// TestCleanupTraceNoGitRemovesEmptyStateDir covers the tidy-up half: with
// nothing else inside, the per-directory state dir itself goes too.
func TestCleanupTraceNoGitRemovesEmptyStateDir(t *testing.T) {
	stateDir := filepath.Join(t.TempDir(), "state")
	store := state.NewOutOfTreeStore(stateDir)
	require.NoError(t, store.InitTraceDir())

	require.NoError(t, CleanupTrace(store, t.TempDir(), zerolog.Nop()))

	_, err := os.Stat(stateDir)
	assert.True(t, os.IsNotExist(err), "empty state dir should be removed")
}

// The removal above must never reach a repository: inside one the same
// directory is .git. The Store decides, so an empty .git cannot be mistaken
// for a spent out-of-tree directory.
func TestCleanupTraceKeepsGitDir(t *testing.T) {
	gitDir := filepath.Join(t.TempDir(), ".git")
	store := state.NewGitStore(gitDir)
	require.NoError(t, store.InitTraceDir())

	require.NoError(t, CleanupTrace(store, t.TempDir(), zerolog.Nop()))

	_, err := os.Stat(gitDir)
	require.NoError(t, err, ".git must survive cleanup even with nothing left inside it")
}

func TestSnapshotAndRestoreAgentSettings(t *testing.T) {
	t.Run("existing file is restored verbatim", func(t *testing.T) {
		repoRoot := t.TempDir()
		p := claude.New()
		path := p.SettingsFile(repoRoot)
		require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
		original := []byte(`{"theme":"dark","custom":42}`)
		require.NoError(t, os.WriteFile(path, original, 0o600))

		backups := snapshotAgentSettings([]trace.Provider{p}, repoRoot, zerolog.Nop())
		require.Len(t, backups, 1)

		require.NoError(t, os.WriteFile(path, []byte(`{"chainloop":"hooks"}`), 0o600))

		restoreAgentSettings(backups, zerolog.Nop())

		got, err := os.ReadFile(path)
		require.NoError(t, err)
		assert.Equal(t, original, got, "restore must produce a bit-for-bit copy of the original file")
	})

	t.Run("missing file stays missing after restore", func(t *testing.T) {
		repoRoot := t.TempDir()
		p := claude.New()
		path := p.SettingsFile(repoRoot)

		backups := snapshotAgentSettings([]trace.Provider{p}, repoRoot, zerolog.Nop())
		require.Len(t, backups, 1)

		require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
		require.NoError(t, os.WriteFile(path, []byte(`{"chainloop":"hooks"}`), 0o600))

		restoreAgentSettings(backups, zerolog.Nop())

		_, err := os.Stat(path)
		assert.True(t, os.IsNotExist(err), "restore must delete files the run created")

		_, err = os.Stat(filepath.Dir(path))
		assert.True(t, os.IsNotExist(err), "restore must also remove the parent directory if it became empty")
	})

	t.Run("pre-existing empty parent directory is preserved", func(t *testing.T) {
		repoRoot := t.TempDir()
		p := claude.New()
		path := p.SettingsFile(repoRoot)
		dir := filepath.Dir(path)

		// User pre-created an empty .claude/ directory.
		require.NoError(t, os.MkdirAll(dir, 0o755))

		backups := snapshotAgentSettings([]trace.Provider{p}, repoRoot, zerolog.Nop())
		require.Len(t, backups, 1)
		assert.True(t, backups[0].dirExisted, "snapshot must record that the parent dir existed before")

		// Simulate trace run dropping a settings file into the dir.
		require.NoError(t, os.WriteFile(path, []byte(`{"chainloop":"hooks"}`), 0o600))

		restoreAgentSettings(backups, zerolog.Nop())

		_, err := os.Stat(path)
		assert.True(t, os.IsNotExist(err), "restore must delete the settings file")
		info, err := os.Stat(dir)
		require.NoError(t, err, "restore must preserve a directory that existed before the run")
		assert.True(t, info.IsDir())
	})

	t.Run("parent directory with sibling content is preserved", func(t *testing.T) {
		repoRoot := t.TempDir()
		p := claude.New()
		path := p.SettingsFile(repoRoot)
		sibling := filepath.Join(filepath.Dir(path), "CLAUDE.md")

		backups := snapshotAgentSettings([]trace.Provider{p}, repoRoot, zerolog.Nop())
		require.Len(t, backups, 1)

		require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
		require.NoError(t, os.WriteFile(sibling, []byte(`# notes`), 0o600))
		require.NoError(t, os.WriteFile(path, []byte(`{"chainloop":"hooks"}`), 0o600))

		restoreAgentSettings(backups, zerolog.Nop())

		_, err := os.Stat(path)
		assert.True(t, os.IsNotExist(err), "restore must delete the settings file")
		_, err = os.Stat(sibling)
		assert.NoError(t, err, "restore must leave unrelated sibling files alone")
	})
}
