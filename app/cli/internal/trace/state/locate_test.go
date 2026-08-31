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

package state

import (
	"os"
	"path/filepath"
	"testing"

	tracegit "github.com/chainloop-dev/chainloop/app/cli/internal/trace/git"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// isolateCacheDir points os.UserCacheDir at a temp directory so non-git state
// tests never touch the developer's real cache. HOME covers macOS, where
// os.UserCacheDir derives from it; XDG_CACHE_HOME covers Linux.
func isolateCacheDir(t *testing.T) {
	t.Helper()

	cache := t.TempDir()
	t.Setenv("HOME", cache)
	t.Setenv("XDG_CACHE_HOME", cache)
}

// TestNonGitDirSymlink guards the failure mode that would silently stop
// all recording: `trace run` and its hook subprocesses reaching the same
// directory by different names must agree on one state directory.
func TestNonGitDirSymlink(t *testing.T) {
	isolateCacheDir(t)

	realDir := t.TempDir()
	link := filepath.Join(t.TempDir(), "link")
	require.NoError(t, os.Symlink(realDir, link))

	viaReal, err := NonGitDir(realDir)
	require.NoError(t, err)
	viaLink, err := NonGitDir(link)
	require.NoError(t, err)

	assert.Equal(t, viaReal, viaLink, "a symlink must resolve to the same state dir as its target")
	assert.True(t, filepath.IsAbs(viaReal), "should return an absolute path")
}

func TestNonGitDirIsPerDirectory(t *testing.T) {
	isolateCacheDir(t)

	first, err := NonGitDir(t.TempDir())
	require.NoError(t, err)
	second, err := NonGitDir(t.TempDir())
	require.NoError(t, err)

	assert.NotEqual(t, first, second, "two working directories must not share a state dir")
}

func TestLocate(t *testing.T) {
	t.Run("inside a git repository returns .git", func(t *testing.T) {
		isolateCacheDir(t)

		dir := t.TempDir()
		require.NoError(t, os.MkdirAll(filepath.Join(dir, ".git"), 0755))
		t.Chdir(dir)

		store, root, err := Locate()
		require.NoError(t, err)
		assert.True(t, store.IsGit())
		assert.Equal(t, filepath.Join(root, ".git"), store.GitDir())
	})

	t.Run("no repository and no active run errors", func(t *testing.T) {
		isolateCacheDir(t)
		t.Chdir(t.TempDir())

		_, _, err := Locate()
		assert.Error(t, err)
	})

	t.Run("no repository binds to the active run in cwd", func(t *testing.T) {
		isolateCacheDir(t)

		dir := t.TempDir()
		t.Chdir(dir)

		want, err := NonGitDir(dir)
		require.NoError(t, err)
		wantStore := NewOutOfTreeStore(want)
		require.NoError(t, wantStore.InitTraceDir())
		require.NoError(t, wantStore.MarkTraceRunActive())

		store, root, err := Locate()
		require.NoError(t, err)
		assert.False(t, store.IsGit())
		assert.Empty(t, store.GitDir(), "an out-of-tree store must not pass for a .git directory")
		assert.Equal(t, want, store.Dir())
		assert.Equal(t, resolveDir(dir), root)
	})

	t.Run("no repository walks up to an ancestor's active run", func(t *testing.T) {
		isolateCacheDir(t)

		dir := t.TempDir()
		nested := filepath.Join(dir, "a", "b")
		require.NoError(t, os.MkdirAll(nested, 0755))

		want, err := NonGitDir(dir)
		require.NoError(t, err)
		wantStore := NewOutOfTreeStore(want)
		require.NoError(t, wantStore.InitTraceDir())
		require.NoError(t, wantStore.MarkTraceRunActive())

		t.Chdir(nested)

		store, root, err := Locate()
		require.NoError(t, err)
		assert.False(t, store.IsGit())
		assert.Empty(t, store.GitDir(), "an out-of-tree store must not pass for a .git directory")
		assert.Equal(t, want, store.Dir())
		assert.Equal(t, resolveDir(dir), root)
	})

	// A broken .git must surface, not silently degrade to non-git mode: the
	// clone's commits would then be recorded against an unrelated state dir.
	t.Run("a broken .git is not treated as no repository", func(t *testing.T) {
		isolateCacheDir(t)

		dir := t.TempDir()
		require.NoError(t, os.WriteFile(filepath.Join(dir, ".git"), []byte("gitdir: /nope/missing\n"), 0600))
		t.Chdir(dir)

		// An active run in the same directory is what Locate would wrongly
		// bind to if it read every git error as "no repository".
		nonGit, err := NonGitDir(dir)
		require.NoError(t, err)
		require.NoError(t, NewOutOfTreeStore(nonGit).MarkTraceRunActive())

		_, _, err = Locate()
		require.Error(t, err)
		assert.NotErrorIs(t, err, tracegit.ErrNotARepository)
	})

	t.Run("a state dir without the sentinel is ignored", func(t *testing.T) {
		isolateCacheDir(t)

		dir := t.TempDir()
		t.Chdir(dir)

		// Leftovers from a crashed run that already cleared the sentinel
		// must not capture an unrelated invocation.
		stale, err := NonGitDir(dir)
		require.NoError(t, err)
		require.NoError(t, NewOutOfTreeStore(stale).InitTraceDir())

		_, _, err = Locate()
		assert.Error(t, err)
	})
}
