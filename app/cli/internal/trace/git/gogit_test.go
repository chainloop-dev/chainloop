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
	"time"

	gogit "github.com/go-git/go-git/v6"
	"github.com/go-git/go-git/v6/config"
	"github.com/go-git/go-git/v6/plumbing/object"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGoGitClient_Context(t *testing.T) {
	cwd, err := os.Getwd()
	require.NoError(t, err)

	c := NewGoGitClient()
	ctx, err := c.Context(cwd)
	require.NoError(t, err)

	assert.NotEmpty(t, ctx.Repository, "repository should not be empty")
	assert.NotEmpty(t, ctx.Branch, "branch should not be empty")
	assert.Equal(t, cwd, ctx.WorkDir)
}

func TestGoGitClient_ContextInvalidDir(t *testing.T) {
	c := NewGoGitClient()
	ctx, err := c.Context(t.TempDir())
	require.NoError(t, err)

	assert.Empty(t, ctx.Repository)
	assert.Empty(t, ctx.Branch)
}

func TestGoGitClient_CodeChanges(t *testing.T) {
	cwd, err := os.Getwd()
	require.NoError(t, err)

	c := NewGoGitClient()
	changes, err := c.CodeChanges(cwd)
	require.NoError(t, err)

	assert.NotNil(t, changes)
	assert.GreaterOrEqual(t, changes.FilesCreated+changes.FilesModified+changes.FilesDeleted, 0)
}

func TestGoGitClient_CodeChangesInvalidDir(t *testing.T) {
	c := NewGoGitClient()
	changes, err := c.CodeChanges(t.TempDir())
	require.NoError(t, err)
	assert.NotNil(t, changes)
	assert.Empty(t, changes.Files)
}

func TestGoGitClient_CodeChangesForRange(t *testing.T) {
	// Create a temporary git repo with three commits using go-git APIs.
	dir := t.TempDir()

	repo, err := gogit.PlainInit(dir, false)
	require.NoError(t, err)

	cfg, err := repo.Config()
	require.NoError(t, err)
	cfg.Commit.GpgSign = config.NewOptBool(false)
	require.NoError(t, repo.SetConfig(cfg))

	wt, err := repo.Worktree()
	require.NoError(t, err)

	sig := &object.Signature{
		Name:  "Test",
		Email: "test@test.com",
		When:  time.Now(),
	}

	// Init commit
	require.NoError(t, os.WriteFile(dir+"/init.txt", []byte("init"), 0600))
	_, err = wt.Add("init.txt")
	require.NoError(t, err)
	_, err = wt.Commit("init", &gogit.CommitOptions{Author: sig})
	require.NoError(t, err)

	// First commit
	require.NoError(t, os.WriteFile(dir+"/a.txt", []byte("hello"), 0600))
	_, err = wt.Add("a.txt")
	require.NoError(t, err)
	first, err := wt.Commit("first", &gogit.CommitOptions{Author: sig})
	require.NoError(t, err)

	// Second commit
	require.NoError(t, os.WriteFile(dir+"/b.txt", []byte("world"), 0600))
	_, err = wt.Add("b.txt")
	require.NoError(t, err)
	second, err := wt.Commit("second", &gogit.CommitOptions{Author: sig})
	require.NoError(t, err)

	c := NewGoGitClient()
	changes, err := c.CodeChangesForRange(dir, first.String(), second.String())
	require.NoError(t, err)
	require.NotNil(t, changes)
	require.Len(t, changes.Files, 2, "range should include a.txt and b.txt")

	paths := map[string]string{}
	for _, f := range changes.Files {
		paths[f.Path] = f.Status
	}
	assert.Equal(t, "created", paths["a.txt"])
	assert.Equal(t, "created", paths["b.txt"])
}

func TestGoGitClient_CodeChangesForRangeInvalidDir(t *testing.T) {
	c := NewGoGitClient()
	changes, err := c.CodeChangesForRange(t.TempDir(), "abc", "def")
	require.NoError(t, err)
	assert.NotNil(t, changes)
	assert.Empty(t, changes.Files)
}

// CodeChangesForCommits aggregates a non-contiguous set of SHAs by summing
// each commit's parent..commit diff per file path. The contiguous-range
// counterpart can't do this — it would over-count commits in the gap.
func TestGoGitClient_CodeChangesForCommits(t *testing.T) {
	dir := t.TempDir()
	repo, err := gogit.PlainInit(dir, false)
	require.NoError(t, err)
	cfg, err := repo.Config()
	require.NoError(t, err)
	cfg.Commit.GpgSign = config.NewOptBool(false)
	require.NoError(t, repo.SetConfig(cfg))
	wt, err := repo.Worktree()
	require.NoError(t, err)
	sig := &object.Signature{Name: "Test", Email: "test@test.com", When: time.Now()}

	require.NoError(t, os.WriteFile(filepath.Join(dir, "init.txt"), []byte("init\n"), 0600))
	_, err = wt.Add("init.txt")
	require.NoError(t, err)
	_, err = wt.Commit("init", &gogit.CommitOptions{Author: sig})
	require.NoError(t, err)

	// commit-A: creates a.txt (one line)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "a.txt"), []byte("alpha\n"), 0600))
	_, err = wt.Add("a.txt")
	require.NoError(t, err)
	commitA, err := wt.Commit("A", &gogit.CommitOptions{Author: sig})
	require.NoError(t, err)

	// commit-B (interleaved, NOT in our session): creates b.txt
	require.NoError(t, os.WriteFile(filepath.Join(dir, "b.txt"), []byte("bravo\n"), 0600))
	_, err = wt.Add("b.txt")
	require.NoError(t, err)
	_, err = wt.Commit("B", &gogit.CommitOptions{Author: sig})
	require.NoError(t, err)

	// commit-C: extends a.txt with two more lines
	require.NoError(t, os.WriteFile(filepath.Join(dir, "a.txt"), []byte("alpha\nbeta\ngamma\n"), 0600))
	_, err = wt.Add("a.txt")
	require.NoError(t, err)
	commitC, err := wt.Commit("C", &gogit.CommitOptions{Author: sig})
	require.NoError(t, err)

	c := NewGoGitClient()

	t.Run("aggregates non-contiguous commits and excludes the gap", func(t *testing.T) {
		// Session owns commit-A and commit-C; commit-B is human or another
		// session's. Range [A..C] would over-count by including B's b.txt.
		changes, err := c.CodeChangesForCommits(dir, []string{commitA.String(), commitC.String()})
		require.NoError(t, err)
		require.Len(t, changes.Files, 1, "only a.txt belongs to the queried set")
		assert.Equal(t, "a.txt", changes.Files[0].Path)
		assert.Equal(t, 3, changes.Files[0].LinesAdded, "1 line from A + 2 lines from C")
		assert.Equal(t, 0, changes.Files[0].LinesRemoved)
		assert.Equal(t, 3, changes.LinesAdded)
	})

	t.Run("empty input returns empty result", func(t *testing.T) {
		changes, err := c.CodeChangesForCommits(dir, nil)
		require.NoError(t, err)
		assert.Empty(t, changes.Files)
		assert.Equal(t, 0, changes.LinesAdded)
	})

	t.Run("unresolvable SHA is skipped, not fatal", func(t *testing.T) {
		changes, err := c.CodeChangesForCommits(dir, []string{commitA.String(), "deadbeef00000000000000000000000000000000"})
		require.NoError(t, err)
		require.Len(t, changes.Files, 1)
		assert.Equal(t, "a.txt", changes.Files[0].Path)
	})
}

func TestGoGitClient_CommitHeadInfo(t *testing.T) {
	cwd, err := os.Getwd()
	require.NoError(t, err)

	c := NewGoGitClient()
	sha, message, err := c.CommitHeadInfo(cwd)
	require.NoError(t, err)

	assert.NotEmpty(t, sha, "SHA should not be empty")
	assert.NotEmpty(t, message, "message should not be empty")
	assert.Len(t, sha, 40, "SHA should be 40 hex characters")
}

func TestGoGitClient_CommitHeadInfoInvalidDir(t *testing.T) {
	c := NewGoGitClient()
	_, _, err := c.CommitHeadInfo(t.TempDir())
	assert.Error(t, err)
}

func TestGoGitClient_GeneratedMatcher(t *testing.T) {
	writeGitAttributes := func(t *testing.T, dir, content string) {
		t.Helper()
		require.NoError(t, os.WriteFile(filepath.Join(dir, ".gitattributes"), []byte(content), 0o600))
	}

	t.Run("missing .gitattributes returns no-op matcher", func(t *testing.T) {
		m := NewGoGitClient().GeneratedMatcher(t.TempDir())
		assert.False(t, m("frontend/src/pbgen/foo.ts"))
		assert.False(t, m("anything"))
	})

	t.Run("linguist-generated=true pattern matches nested paths", func(t *testing.T) {
		dir := t.TempDir()
		writeGitAttributes(t, dir, "frontend/src/pbgen/** linguist-generated=true\n")

		m := NewGoGitClient().GeneratedMatcher(dir)
		assert.True(t, m("frontend/src/pbgen/foo.ts"))
		assert.True(t, m("frontend/src/pbgen/nested/bar.ts"))
		assert.False(t, m("frontend/src/components/foo.ts"))
	})

	t.Run("bare linguist-generated is treated as set", func(t *testing.T) {
		dir := t.TempDir()
		writeGitAttributes(t, dir, "gen/** linguist-generated\n")

		m := NewGoGitClient().GeneratedMatcher(dir)
		assert.True(t, m("gen/out.pb.go"))
	})

	t.Run("linguist-generated=false override wins for nested paths", func(t *testing.T) {
		dir := t.TempDir()
		// Order matters: later rules override earlier ones for the same attribute.
		writeGitAttributes(t, dir,
			"backend/ent/** linguist-generated=true\n"+
				"backend/ent/schema/* linguist-generated=false\n",
		)

		m := NewGoGitClient().GeneratedMatcher(dir)
		assert.True(t, m("backend/ent/client.go"))
		assert.False(t, m("backend/ent/schema/user.go"))
	})

	t.Run("path not covered returns false", func(t *testing.T) {
		dir := t.TempDir()
		writeGitAttributes(t, dir, "frontend/src/pbgen/** linguist-generated=true\n")

		m := NewGoGitClient().GeneratedMatcher(dir)
		assert.False(t, m("backend/internal/service/evidence.go"))
		assert.False(t, m(""))
	})

	t.Run("unrelated attributes do not trigger generated flag", func(t *testing.T) {
		dir := t.TempDir()
		writeGitAttributes(t, dir,
			"*.go diff=golang\n"+
				"*.png binary\n",
		)

		m := NewGoGitClient().GeneratedMatcher(dir)
		assert.False(t, m("main.go"))
		assert.False(t, m("logo.png"))
	})

	t.Run("linguist-generated=true matches deeply nested paths", func(t *testing.T) {
		dir := t.TempDir()
		writeGitAttributes(t, dir, "frontend/src/pbgen/** linguist-generated=true\n")

		m := NewGoGitClient().GeneratedMatcher(dir)
		assert.True(t, m("frontend/src/pbgen/backend/v1/shared_messages_pb.ts"))
		assert.True(t, m("frontend/src/pbgen/a/b/c/d/e.ts"))
	})

	t.Run("**/*.pb.go matches generated proto Go files anywhere", func(t *testing.T) {
		dir := t.TempDir()
		writeGitAttributes(t, dir, "**/*.pb.go linguist-generated=true\n")

		m := NewGoGitClient().GeneratedMatcher(dir)
		assert.True(t, m("backend/api/gen/backend/v1/shared_messages.pb.go"))
		assert.True(t, m("backend/internal/conf/backend/config/v1/conf.pb.go"))
		assert.False(t, m("backend/internal/conf/backend/config/v1/conf.proto"))
	})
}

// TestGoGitClient_GeneratedMatcherProjectAttributes exercises the matcher against
// the real project .gitattributes, locking in the expected classification of
// representative paths under git's "last match wins" semantics.
func TestGoGitClient_GeneratedMatcherProjectAttributes(t *testing.T) {
	_, repoRoot, err := FindGitDirAndRoot()
	require.NoError(t, err)

	m := NewGoGitClient().GeneratedMatcher(repoRoot)

	// Generated by entgo, protoc and wire: must be filtered.
	assert.True(t, m("app/controlplane/pkg/data/ent/client.go"))
	assert.True(t, m("app/controlplane/pkg/data/ent/organization.go"))
	assert.True(t, m("app/controlplane/api/gen/frontend/workflowcontract/v1/crafting_schema.ts"))
	assert.True(t, m("app/controlplane/api/gen/jsonschema/foo.json"))
	assert.True(t, m("app/controlplane/api/controlplane/v1/response_messages.pb.go"))
	assert.True(t, m("app/controlplane/api/controlplane/v1/response_messages.pb.validate.go"))
	assert.True(t, m("app/controlplane/cmd/wire_gen.go"))

	// Hand-managed despite living under a generated tree: the later
	// linguist-generated=false lines win under "last match wins".
	assert.False(t, m("app/controlplane/pkg/data/ent/migrate/migrate.go"))
	assert.False(t, m("app/controlplane/pkg/data/ent/migrate/migrations/atlas.sum"))
	assert.False(t, m("app/controlplane/pkg/data/ent/schema/workflow.go"))

	// Unrelated source: not generated.
	assert.False(t, m("app/cli/cmd/root.go"))
	assert.False(t, m("pkg/attestation/crafter/crafter.go"))
	assert.False(t, m("app/cli/internal/trace/git/gogit.go"))
}

// TestGoGitClient_Worktree exercises all Client methods from a linked git worktree.
// Setup uses exec for worktree creation (go-git doesn't support creating linked worktrees),
// then verifies the GoGitClient can correctly read from the worktree.
func TestGoGitClient_Worktree(t *testing.T) {
	// Skip if git binary is not available (needed for worktree setup only)
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git binary not found, skipping worktree test")
	}

	// --- Setup: create main repo with origin remote and a linked worktree ---
	mainDir := t.TempDir()
	wtDir := filepath.Join(t.TempDir(), "linked-wt")

	// Initialize main repo using exec (to respect system default branch name)
	run := func(args ...string) string {
		cmd := exec.Command("git", args...)
		cmd.Dir = mainDir
		out, err := cmd.CombinedOutput()
		require.NoError(t, err, "git %v failed: %s", args, string(out))
		return strings.TrimSpace(string(out))
	}

	run("init", "-b", "main")
	run("config", "user.email", "test@test.com")
	run("config", "user.name", "Test")

	require.NoError(t, os.WriteFile(filepath.Join(mainDir, "init.txt"), []byte("init"), 0600))
	run("add", "init.txt")
	run("commit", "-m", "initial commit")

	// Add a fake origin remote so Context() can read the remote URL
	run("remote", "add", "origin", "https://github.com/example/repo.git")
	// Create origin/main ref so merge-base detection works
	run("update-ref", "refs/remotes/origin/main", "HEAD")

	// Create a feature branch and add commits
	run("checkout", "-b", "feature")

	require.NoError(t, os.WriteFile(filepath.Join(mainDir, "a.txt"), []byte("hello"), 0600))
	run("add", "a.txt")
	run("commit", "-m", "first feature commit")
	firstHash := run("rev-parse", "HEAD")

	require.NoError(t, os.WriteFile(filepath.Join(mainDir, "b.txt"), []byte("world"), 0600))
	run("add", "b.txt")
	run("commit", "-m", "second feature commit")
	secondHash := run("rev-parse", "HEAD")

	// Switch main repo back to main before creating the linked worktree
	run("checkout", "main")

	// Create a linked worktree on the feature branch (requires exec)
	cmd := exec.Command("git", "worktree", "add", wtDir, "feature")
	cmd.Dir = mainDir
	out, err := cmd.CombinedOutput()
	require.NoError(t, err, "git worktree add failed: %s", string(out))

	// --- Test: all GoGitClient methods from the linked worktree ---
	c := NewGoGitClient()

	t.Run("Context", func(t *testing.T) {
		ctx, err := c.Context(wtDir)
		require.NoError(t, err)

		assert.Equal(t, "https://github.com/example/repo.git", ctx.Repository)
		assert.Equal(t, "feature", ctx.Branch)
		assert.Equal(t, wtDir, ctx.WorkDir)
		assert.Equal(t, 0, ctx.CommitCount, "Context should not collect commits (overridden per-session)")
		assert.Empty(t, ctx.CommitStart)
		assert.Empty(t, ctx.CommitEnd)
	})

	t.Run("CommitHeadInfo", func(t *testing.T) {
		sha, message, err := c.CommitHeadInfo(wtDir)
		require.NoError(t, err)

		assert.Equal(t, secondHash, sha)
		assert.Equal(t, "second feature commit", message)
	})

	t.Run("CodeChangesForRange", func(t *testing.T) {
		changes, err := c.CodeChangesForRange(wtDir, firstHash, secondHash)
		require.NoError(t, err)
		require.Len(t, changes.Files, 2, "range should include a.txt and b.txt")

		paths := map[string]string{}
		for _, f := range changes.Files {
			paths[f.Path] = f.Status
		}
		assert.Equal(t, "created", paths["a.txt"])
		assert.Equal(t, "created", paths["b.txt"])
		assert.Greater(t, changes.LinesAdded, 0)
	})

	t.Run("CodeChanges", func(t *testing.T) {
		changes, err := c.CodeChanges(wtDir)
		require.NoError(t, err)
		assert.NotNil(t, changes)
		assert.GreaterOrEqual(t, changes.FilesCreated, 2)
	})

	t.Run("FindGitDirAndRoot", func(t *testing.T) {
		// Save and restore cwd
		origDir, err := os.Getwd()
		require.NoError(t, err)
		defer func() { _ = os.Chdir(origDir) }()

		require.NoError(t, os.Chdir(wtDir))

		gitDir, repoRoot, err := FindGitDirAndRoot()
		require.NoError(t, err)

		// Resolve symlinks for comparison (macOS /var -> /private/var)
		resolvedWtDir, err := filepath.EvalSymlinks(wtDir)
		require.NoError(t, err)
		resolvedMainDir, err := filepath.EvalSymlinks(mainDir)
		require.NoError(t, err)

		assert.Equal(t, resolvedWtDir, repoRoot, "repoRoot should be the worktree directory")
		// gitDir should point to the worktree-specific git dir inside the main repo's .git/worktrees/
		assert.Contains(t, gitDir, "worktrees", "gitDir should be inside .git/worktrees/")
		assert.True(t, strings.HasPrefix(gitDir, resolvedMainDir), "gitDir should be under the main repo")

		// The gitDir should actually exist
		_, err = os.Stat(gitDir)
		assert.NoError(t, err, "gitDir should exist on disk")
	})
}

func TestGoGitClient_SnapshotWorktree(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(root, "keep.txt"), []byte("hello\n"), 0600))
	require.NoError(t, os.MkdirAll(filepath.Join(root, "sub"), 0755))
	require.NoError(t, os.WriteFile(filepath.Join(root, "sub", "nested.go"), []byte("package sub\n"), 0600))
	require.NoError(t, os.WriteFile(filepath.Join(root, ".gitignore"), []byte("ignored.txt\nbuild/\n"), 0600))
	require.NoError(t, os.WriteFile(filepath.Join(root, "ignored.txt"), []byte("secret\n"), 0600))
	require.NoError(t, os.MkdirAll(filepath.Join(root, "build"), 0755))
	require.NoError(t, os.WriteFile(filepath.Join(root, "build", "out.bin"), []byte("artifact\n"), 0600))
	// A .git directory must always be skipped, even without a real repo.
	require.NoError(t, os.MkdirAll(filepath.Join(root, ".git"), 0755))
	require.NoError(t, os.WriteFile(filepath.Join(root, ".git", "config"), []byte("[core]\n"), 0600))

	c := NewGoGitClient()
	sig, err := c.SnapshotWorktree(root)
	require.NoError(t, err)

	// Non-ignored files present (forward-slash keys), ignored + .git excluded.
	assert.Contains(t, sig, "keep.txt")
	assert.Contains(t, sig, "sub/nested.go")
	assert.Contains(t, sig, ".gitignore")
	assert.NotContains(t, sig, "ignored.txt")
	assert.NotContains(t, sig, "build/out.bin")
	assert.NotContains(t, sig, ".git/config")

	// Modifying a file changes its signature; adding one adds a key.
	require.NoError(t, os.WriteFile(filepath.Join(root, "keep.txt"), []byte("hello world\n"), 0600))
	require.NoError(t, os.WriteFile(filepath.Join(root, "new.txt"), []byte("new\n"), 0600))
	sig2, err := c.SnapshotWorktree(root)
	require.NoError(t, err)
	assert.NotEqual(t, sig["keep.txt"], sig2["keep.txt"])
	assert.Contains(t, sig2, "new.txt")
}

func TestGoGitClient_SnapshotWorktree_TrackedIgnoredFile(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git binary not found")
	}
	root := t.TempDir()
	run := func(args ...string) {
		cmd := exec.Command("git", args...)
		cmd.Dir = root
		out, err := cmd.CombinedOutput()
		require.NoError(t, err, "git %v: %s", args, out)
	}
	run("init", "-b", "main")
	run("config", "user.email", "t@t.com")
	run("config", "user.name", "t")

	// *.gen is ignored, but one .gen file is force-added and tracked.
	require.NoError(t, os.WriteFile(filepath.Join(root, ".gitignore"), []byte("*.gen\n"), 0600))
	require.NoError(t, os.WriteFile(filepath.Join(root, "tracked.gen"), []byte("tracked\n"), 0600))
	require.NoError(t, os.WriteFile(filepath.Join(root, "untracked.gen"), []byte("untracked\n"), 0600))
	require.NoError(t, os.WriteFile(filepath.Join(root, "normal.txt"), []byte("normal\n"), 0600))
	run("add", "-f", "tracked.gen")
	run("add", "normal.txt", ".gitignore")
	run("commit", "-m", "init")

	sig, err := NewGoGitClient().SnapshotWorktree(root)
	require.NoError(t, err)

	assert.Contains(t, sig, "tracked.gen", "force-added tracked file must be snapshotted despite matching .gitignore")
	assert.Contains(t, sig, "normal.txt")
	assert.NotContains(t, sig, "untracked.gen", "untracked ignored file must be excluded")
}
