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
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGoGitClient_SnapshotIndex_ResolutionChangesSignature(t *testing.T) {
	// Given
	dir := t.TempDir()
	runIndexGit(t, dir, "init", "-q")
	runIndexGit(t, dir, "read-tree", "--empty")
	base := writeGitObject(t, dir, "base\n")
	ours := writeGitObject(t, dir, "ours\n")
	theirs := writeGitObject(t, dir, "theirs\n")
	writeGitIndex(t, dir, fmt.Sprintf(
		"100644 %s 1\tconflict.txt\n100644 %s 2\tconflict.txt\n100644 %s 3\tconflict.txt\n",
		base, ours, theirs,
	))

	client := NewGoGitClient()
	before, err := client.SnapshotIndex(dir)
	require.NoError(t, err)

	runIndexGit(t, dir, "read-tree", "--empty")
	writeGitIndex(t, dir, fmt.Sprintf("100644 %s 0\tconflict.txt\n", theirs))

	// When
	after, err := client.SnapshotIndex(dir)
	require.NoError(t, err)

	// Then
	require.Contains(t, before, "conflict.txt")
	require.Contains(t, after, "conflict.txt")
	require.NotEqual(t, before["conflict.txt"], after["conflict.txt"],
		"resolving to an existing conflict stage must change the index signature")
}

func TestGoGitClient_IndexFile_IgnoresReplacementObjects(t *testing.T) {
	// Given
	dir := t.TempDir()
	runIndexGit(t, dir, "init", "-q")
	runIndexGit(t, dir, "read-tree", "--empty")
	indexed := writeGitObject(t, dir, "indexed content\n")
	replacement := writeGitObject(t, dir, "replacement content\n")
	writeGitIndex(t, dir, fmt.Sprintf("100644 %s 0\tindexed.txt\n", indexed))
	runIndexGit(t, dir, "replace", indexed, replacement)

	// When
	content, err := NewGoGitClient().IndexFile(dir, "indexed.txt")
	require.NoError(t, err)

	// Then
	require.Equal(t, []byte("indexed content\n"), content,
		"staged content must come from the exact object recorded in the index")
}

func TestGoGitClient_StagedFiles_ExcludesIntentToAdd(t *testing.T) {
	dir := t.TempDir()
	runIndexGit(t, dir, "init", "-q")
	require.NoError(t, os.WriteFile(filepath.Join(dir, "pending.txt"), []byte("pending\n"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "staged.txt"), nil, 0o600))
	runIndexGit(t, dir, "add", "-N", "pending.txt")
	runIndexGit(t, dir, "add", "staged.txt")

	files, err := NewGoGitClient().StagedFiles(dir)
	require.NoError(t, err)
	require.Equal(t, []string{"staged.txt"}, files)
}

func runIndexGit(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	require.NoError(t, err, "git %v: %s", args, string(out))

	return strings.TrimSpace(string(out))
}

func writeGitIndex(t *testing.T, dir, entries string) {
	t.Helper()
	cmd := exec.Command("git", "update-index", "--index-info")
	cmd.Dir = dir
	cmd.Stdin = strings.NewReader(entries)
	out, err := cmd.CombinedOutput()
	require.NoError(t, err, "git update-index: %s", string(out))
}

func writeGitObject(t *testing.T, dir, content string) string {
	t.Helper()
	cmd := exec.Command("git", "hash-object", "-w", "--stdin")
	cmd.Dir = dir
	cmd.Stdin = strings.NewReader(content)
	out, err := cmd.CombinedOutput()
	require.NoError(t, err, "git hash-object: %s", string(out))

	return strings.TrimSpace(string(out))
}
