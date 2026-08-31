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

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// branchedRepo holds the directory and commit SHAs produced by
// setupBranchedRepo, in chronological order.
type branchedRepo struct {
	dir        string
	initialSHA string
	featureA   string
	featureB   string
	sideSHA    string
	mergeSHA   string
}

// setupBranchedRepo creates a repo with:
//   - main branch with one initial commit
//   - origin/main ref tracking that commit
//   - feature branch with two additional commits, then a merge of a side branch
func setupBranchedRepo(t *testing.T) branchedRepo {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git binary not found")
	}

	dir := t.TempDir()
	run := func(args ...string) string {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		out, err := cmd.CombinedOutput()
		require.NoError(t, err, "git %v: %s", args, string(out))

		return string(out)
	}

	run("init", "-b", "main")
	run("config", "user.email", "test@test.com")
	run("config", "user.name", "Test")
	run("config", "commit.gpgsign", "false")

	require.NoError(t, os.WriteFile(filepath.Join(dir, "init.txt"), []byte("init"), 0600))
	run("add", "init.txt")
	run("commit", "-m", "initial")
	initialSHA := strings.TrimSpace(run("rev-parse", "HEAD"))

	// Make origin/main point at the initial commit so detectBaseBranchGoGit finds it.
	run("update-ref", "refs/remotes/origin/main", "HEAD")

	// Feature branch with two commits.
	run("checkout", "-b", "feature")
	require.NoError(t, os.WriteFile(filepath.Join(dir, "a.txt"), []byte("hello"), 0600))
	run("add", "a.txt")
	run("commit", "-m", "feature a")
	featureA := strings.TrimSpace(run("rev-parse", "HEAD"))

	require.NoError(t, os.WriteFile(filepath.Join(dir, "b.txt"), []byte("world"), 0600))
	run("add", "b.txt")
	run("commit", "-m", "feature b")
	featureB := strings.TrimSpace(run("rev-parse", "HEAD"))

	// Side branch off main.
	run("branch", "side", initialSHA)
	run("checkout", "side")
	require.NoError(t, os.WriteFile(filepath.Join(dir, "c.txt"), []byte("side"), 0600))
	run("add", "c.txt")
	run("commit", "-m", "side commit")
	sideSHA := strings.TrimSpace(run("rev-parse", "HEAD"))

	// Merge side into feature with a real merge commit.
	run("checkout", "feature")
	run("merge", "--no-ff", "-m", "merge side", "side")
	mergeSHA := strings.TrimSpace(run("rev-parse", "HEAD"))

	return branchedRepo{
		dir:        dir,
		initialSHA: initialSHA,
		featureA:   featureA,
		featureB:   featureB,
		sideSHA:    sideSHA,
		mergeSHA:   mergeSHA,
	}
}

func TestGoGitClient_IsMergeCommit(t *testing.T) {
	repo := setupBranchedRepo(t)
	c := NewGoGitClient()

	t.Run("regular commit", func(t *testing.T) {
		isMerge, err := c.IsMergeCommit(repo.dir, repo.featureA)
		require.NoError(t, err)
		assert.False(t, isMerge)
	})

	t.Run("merge commit", func(t *testing.T) {
		isMerge, err := c.IsMergeCommit(repo.dir, repo.mergeSHA)
		require.NoError(t, err)
		assert.True(t, isMerge)
	})

	t.Run("unknown sha", func(t *testing.T) {
		_, err := c.IsMergeCommit(repo.dir, "0000000000000000000000000000000000000000")
		assert.Error(t, err)
	})
}

func TestGoGitClient_BranchCommitSHAs(t *testing.T) {
	repo := setupBranchedRepo(t)
	c := NewGoGitClient()

	shas, err := c.BranchCommitSHAs(repo.dir)
	require.NoError(t, err)

	// Set form: feature branch ahead of origin/main is repo.featureA, repo.featureB,
	// repo.sideSHA, repo.mergeSHA. The merge base (initial) itself is excluded.
	got := map[string]bool{}
	for _, sha := range shas {
		got[sha] = true
	}

	assert.True(t, got[repo.featureA], "repo.featureA should be in branch SHAs")
	assert.True(t, got[repo.featureB], "repo.featureB should be in branch SHAs")
	assert.True(t, got[repo.sideSHA], "repo.sideSHA should be in branch SHAs (reachable via merge)")
	assert.True(t, got[repo.mergeSHA], "repo.mergeSHA should be in branch SHAs")
	assert.False(t, got[repo.initialSHA], "merge base (initial) should be excluded")
}

func TestExecClient_IsMergeCommit(t *testing.T) {
	repo := setupBranchedRepo(t)
	c := NewExecClient()

	t.Run("regular commit", func(t *testing.T) {
		isMerge, err := c.IsMergeCommit(repo.dir, repo.featureA)
		require.NoError(t, err)
		assert.False(t, isMerge)
	})

	t.Run("merge commit", func(t *testing.T) {
		isMerge, err := c.IsMergeCommit(repo.dir, repo.mergeSHA)
		require.NoError(t, err)
		assert.True(t, isMerge)
	})
}

func TestIsMergeInProgress(t *testing.T) {
	t.Run("no merge head present", func(t *testing.T) {
		assert.False(t, IsMergeInProgress(t.TempDir()))
	})

	t.Run("merge head present", func(t *testing.T) {
		dir := t.TempDir()
		require.NoError(t, os.WriteFile(filepath.Join(dir, "MERGE_HEAD"), []byte("abc\n"), 0600))
		assert.True(t, IsMergeInProgress(dir))
	})
}

func TestExecClient_BranchCommitSHAs(t *testing.T) {
	repo := setupBranchedRepo(t)
	c := NewExecClient()

	shas, err := c.BranchCommitSHAs(repo.dir)
	require.NoError(t, err)

	got := map[string]bool{}
	for _, sha := range shas {
		got[sha] = true
	}

	assert.True(t, got[repo.featureA])
	assert.True(t, got[repo.featureB])
	assert.True(t, got[repo.sideSHA])
	assert.True(t, got[repo.mergeSHA])
	assert.False(t, got[repo.initialSHA])
}
