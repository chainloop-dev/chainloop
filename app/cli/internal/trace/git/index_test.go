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
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGoGitClient_SnapshotIndexAndIndexFile_ReadStagedContent(t *testing.T) {
	// Given
	dir := t.TempDir()
	runIndexGit(t, dir, "init", "-q")
	require.NoError(t, os.WriteFile(filepath.Join(dir, "staged.txt"), []byte("staged content\n"), 0o600))
	runIndexGit(t, dir, "add", "staged.txt")

	// When
	client := NewGoGitClient()
	snapshot, err := client.SnapshotIndex(dir)
	require.NoError(t, err)
	content, err := client.IndexFile(dir, "staged.txt")
	require.NoError(t, err)

	// Then
	require.Contains(t, snapshot, "staged.txt")
	require.Equal(t, []byte("staged content\n"), content)
}

func TestExecClient_SnapshotIndexAndIndexFile_ReadSplitIndex(t *testing.T) {
	// Given
	dir := t.TempDir()
	runIndexGit(t, dir, "init", "-q")
	require.NoError(t, os.WriteFile(filepath.Join(dir, "staged.txt"), []byte("staged content\n"), 0o600))
	runIndexGit(t, dir, "add", "staged.txt")
	runIndexGit(t, dir, "update-index", "--split-index")

	// When
	client := NewGitClient()
	snapshot, err := client.SnapshotIndex(dir)
	require.NoError(t, err)
	content, err := client.IndexFile(dir, "staged.txt")
	require.NoError(t, err)

	// Then
	require.NotEmpty(t, snapshot["staged.txt"])
	require.Equal(t, []byte("staged content\n"), content)
}

func TestExecClient_SnapshotIndexAndIndexFile_ReadSparseIndex(t *testing.T) {
	// Given
	dir := t.TempDir()
	runIndexGit(t, dir, "init", "-q")
	runIndexGit(t, dir, "config", "user.email", "test@example.com")
	runIndexGit(t, dir, "config", "user.name", "Test")
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "outside"), 0o700))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "outside", "staged.txt"), []byte("staged content\n"), 0o600))
	runIndexGit(t, dir, "add", "outside/staged.txt")
	runIndexGit(t, dir, "commit", "-qm", "initial")
	runIndexGit(t, dir, "sparse-checkout", "init", "--cone", "--sparse-index")
	runIndexGit(t, dir, "sparse-checkout", "set", "inside")

	// When
	client := NewGitClient()
	snapshot, err := client.SnapshotIndex(dir)
	require.NoError(t, err)
	content, err := client.IndexFile(dir, "outside/staged.txt")
	require.NoError(t, err)

	// Then
	require.NotEmpty(t, snapshot["outside/staged.txt"])
	require.Equal(t, []byte("staged content\n"), content)
}

func runIndexGit(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	require.NoError(t, err, "git %v: %s", args, string(out))

	return strings.TrimSpace(string(out))
}
