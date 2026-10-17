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

package git

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGoGitClient_UnstagedFiles_HonorsAttributeSources(t *testing.T) {
	for _, source := range []string{"global", "local", "info"} {
		t.Run(source, func(t *testing.T) {
			dir := t.TempDir()
			home := t.TempDir()
			t.Setenv("HOME", home)
			runIndexGit(t, dir, "init", "-q")

			switch source {
			case "global":
				attributes := filepath.Join(home, "attributes")
				require.NoError(t, os.WriteFile(attributes, []byte("file.txt text eol=lf\n"), 0o600))
				config := "[core]\n\tattributesFile = ~/attributes\n"
				require.NoError(t, os.WriteFile(filepath.Join(home, ".gitconfig"), []byte(config), 0o600))
			case "local":
				globalAttributes := filepath.Join(home, "global-attributes")
				require.NoError(t, os.WriteFile(globalAttributes, []byte("file.txt -text\n"), 0o600))
				globalConfig := fmt.Sprintf("[core]\n\tattributesFile = %s\n", globalAttributes)
				require.NoError(t, os.WriteFile(filepath.Join(home, ".gitconfig"), []byte(globalConfig), 0o600))

				localAttributes := filepath.Join(dir, "local-attributes")
				require.NoError(t, os.WriteFile(localAttributes, []byte("file.txt text eol=lf\n"), 0o600))
				runIndexGit(t, dir, "config", "core.attributesFile", localAttributes)
			case "info":
				require.NoError(t, os.WriteFile(
					filepath.Join(dir, ".git", "info", "attributes"),
					[]byte("file.txt text eol=lf\n"),
					0o600,
				))
			}

			require.NoError(t, os.WriteFile(filepath.Join(dir, "file.txt"), []byte("line\r\n"), 0o600))
			runIndexGit(t, dir, "add", "file.txt")

			content, err := NewGoGitClient().IndexFile(dir, "file.txt")
			require.NoError(t, err)
			require.Equal(t, []byte("line\n"), content)

			unstaged, err := NewGoGitClient().UnstagedFiles(dir)
			require.NoError(t, err)
			require.NotContains(t, unstaged, "file.txt")
		})
	}
}

func TestGoGitClient_UnstagedFiles_MissingNestedAttributes(t *testing.T) {
	dir := t.TempDir()
	runIndexGit(t, dir, "init", "-q")
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "nested"), 0o700))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "nested", "file.txt"), []byte("content\n"), 0o600))
	runIndexGit(t, dir, "add", "nested/file.txt")

	unstaged, err := NewGoGitClient().UnstagedFiles(dir)
	require.NoError(t, err)
	require.NotContains(t, unstaged, "nested/file.txt")
}

func TestGoGitClient_UnstagedFiles_TrackedFileInIgnoredDirectory(t *testing.T) {
	dir := t.TempDir()
	runIndexGit(t, dir, "init", "-q")
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "build"), 0o700))
	require.NoError(t, os.WriteFile(filepath.Join(dir, ".gitignore"), []byte("build/\n"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "build", "tracked.txt"), []byte("tracked\n"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "build", "untracked.txt"), []byte("untracked\n"), 0o600))
	runIndexGit(t, dir, "add", ".gitignore")
	runIndexGit(t, dir, "add", "-f", "build/tracked.txt")

	snapshot, err := NewGoGitClient().SnapshotWorktree(dir)
	require.NoError(t, err)
	require.Contains(t, snapshot, "build/tracked.txt")
	require.NotContains(t, snapshot, "build/untracked.txt")

	unstaged, err := NewGoGitClient().UnstagedFiles(dir)
	require.NoError(t, err)
	require.NotContains(t, unstaged, "build/tracked.txt")

	require.NoError(t, os.WriteFile(filepath.Join(dir, "build", "tracked.txt"), []byte("changed\n"), 0o600))
	unstaged, err = NewGoGitClient().UnstagedFiles(dir)
	require.NoError(t, err)
	require.Contains(t, unstaged, "build/tracked.txt")
}

func TestGoGitClient_SnapshotWorktree_ExcludesIgnoredIntentToAdd(t *testing.T) {
	dir := t.TempDir()
	runIndexGit(t, dir, "init", "-q")
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "build"), 0o700))
	require.NoError(t, os.WriteFile(filepath.Join(dir, ".gitignore"), []byte("build/\n"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "build", "pending.txt"), []byte("pending\n"), 0o600))
	runIndexGit(t, dir, "add", ".gitignore")
	runIndexGit(t, dir, "add", "-N", "-f", "build/pending.txt")

	snapshot, err := NewGoGitClient().SnapshotWorktree(dir)
	require.NoError(t, err)
	require.NotContains(t, snapshot, "build/pending.txt")
}

func TestGoGitClient_SnapshotWorktree_DoesNotFollowEscapingSymlink(t *testing.T) {
	dir := t.TempDir()
	outside := t.TempDir()
	runIndexGit(t, dir, "init", "-q")
	require.NoError(t, os.WriteFile(filepath.Join(outside, "outside.txt"), []byte("outside\n"), 0o600))
	require.NoError(t, os.Symlink(outside, filepath.Join(dir, "link")))
	blob := writeGitObject(t, dir, "outside\n")
	writeGitIndex(t, dir, fmt.Sprintf("100644 %s 0\tlink/outside.txt\n", blob))

	snapshot, err := NewGoGitClient().SnapshotWorktree(dir)
	require.NoError(t, err)
	require.NotContains(t, snapshot, "link/outside.txt")
}

func TestGoGitClient_SnapshotWorktree_DoesNotFollowInRootSymlink(t *testing.T) {
	dir := t.TempDir()
	runIndexGit(t, dir, "init", "-q")
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "target"), 0o700))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "target", "file.txt"), []byte("inside\n"), 0o600))
	require.NoError(t, os.Symlink("target", filepath.Join(dir, "link")))
	blob := writeGitObject(t, dir, "inside\n")
	writeGitIndex(t, dir, fmt.Sprintf("100644 %s 0\tlink/file.txt\n", blob))

	snapshot, err := NewGoGitClient().SnapshotWorktree(dir)
	require.NoError(t, err)
	require.NotContains(t, snapshot, "link/file.txt")
}

func TestGoGitClient_UnstagedFiles_ChangedFilterMarksRawEqualityDirty(t *testing.T) {
	dir := t.TempDir()
	runIndexGit(t, dir, "init", "-q")
	require.NoError(t, os.WriteFile(filepath.Join(dir, ".gitattributes"), []byte("filtered.txt filter=mutate\n"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "filtered.txt"), []byte("same bytes\n"), 0o600))
	runIndexGit(t, dir, "add", ".gitattributes", "filtered.txt")
	runIndexGit(t, dir, "config", "filter.mutate.clean", "sed 's/^/clean:/'")

	unstaged, err := NewGoGitClient().UnstagedFiles(dir)
	require.NoError(t, err)
	require.Contains(t, unstaged, "filtered.txt")
}
