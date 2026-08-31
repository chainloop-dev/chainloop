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
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExecClient_Context(t *testing.T) {
	cwd, err := os.Getwd()
	require.NoError(t, err)

	c := NewExecClient()
	ctx, err := c.Context(cwd)
	require.NoError(t, err)

	assert.NotEmpty(t, ctx.Repository, "repository should not be empty")
	assert.NotEmpty(t, ctx.Branch, "branch should not be empty")
	assert.Equal(t, cwd, ctx.WorkDir)
}

func TestExecClient_ContextInvalidDir(t *testing.T) {
	c := NewExecClient()
	ctx, err := c.Context(t.TempDir())
	require.NoError(t, err)

	assert.Empty(t, ctx.Repository)
	assert.Empty(t, ctx.Branch)
}

func TestExecClient_CodeChanges(t *testing.T) {
	cwd, err := os.Getwd()
	require.NoError(t, err)

	c := NewExecClient()
	changes, err := c.CodeChanges(cwd)
	require.NoError(t, err)

	assert.NotNil(t, changes)
	assert.GreaterOrEqual(t, changes.FilesCreated+changes.FilesModified+changes.FilesDeleted, 0)
}

func TestExecClient_CodeChangesInvalidDir(t *testing.T) {
	c := NewExecClient()
	changes, err := c.CodeChanges(t.TempDir())
	require.NoError(t, err)
	assert.NotNil(t, changes)
	assert.Empty(t, changes.Files)
}

func TestExecClient_CodeChangesForRange(t *testing.T) {
	// Create a temporary git repo with three commits so startSHA is never the
	// root commit (startSHA^ must exist for the range to work).
	dir := t.TempDir()
	run := func(args ...string) string { return gitOutput(dir, args...) }

	run("init", "-b", "main")
	run("config", "user.email", "test@test.com")
	run("config", "user.name", "Test")

	require.NoError(t, os.WriteFile(dir+"/init.txt", []byte("init"), 0600))
	run("add", "init.txt")
	run("commit", "-m", "init")

	require.NoError(t, os.WriteFile(dir+"/a.txt", []byte("hello"), 0600))
	run("add", "a.txt")
	run("commit", "-m", "first")
	first := run("rev-parse", "HEAD")

	require.NoError(t, os.WriteFile(dir+"/b.txt", []byte("world"), 0600))
	run("add", "b.txt")
	run("commit", "-m", "second")
	second := run("rev-parse", "HEAD")

	c := NewExecClient()
	changes, err := c.CodeChangesForRange(dir, first, second)
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

func TestExecClient_CodeChangesForRangeInvalidDir(t *testing.T) {
	c := NewExecClient()
	changes, err := c.CodeChangesForRange(t.TempDir(), "abc", "def")
	require.NoError(t, err)
	assert.NotNil(t, changes)
	assert.Empty(t, changes.Files)
}

func TestExecClient_CodeChangesForCommits(t *testing.T) {
	dir := t.TempDir()
	run := func(args ...string) string { return gitOutput(dir, args...) }

	run("init", "-b", "main")
	run("config", "user.email", "test@test.com")
	run("config", "user.name", "Test")
	run("config", "commit.gpgsign", "false")

	require.NoError(t, os.WriteFile(dir+"/init.txt", []byte("init\n"), 0600))
	run("add", "init.txt")
	run("commit", "-m", "init")

	// commit-A: creates a.txt with one line
	require.NoError(t, os.WriteFile(dir+"/a.txt", []byte("alpha\n"), 0600))
	run("add", "a.txt")
	run("commit", "-m", "A")
	commitA := run("rev-parse", "HEAD")

	// commit-B (interleaved, NOT in our session): creates b.txt
	require.NoError(t, os.WriteFile(dir+"/b.txt", []byte("bravo\n"), 0600))
	run("add", "b.txt")
	run("commit", "-m", "B")

	// commit-C: extends a.txt with two more lines
	require.NoError(t, os.WriteFile(dir+"/a.txt", []byte("alpha\nbeta\ngamma\n"), 0600))
	run("add", "a.txt")
	run("commit", "-m", "C")
	commitC := run("rev-parse", "HEAD")

	c := NewExecClient()

	t.Run("aggregates non-contiguous commits and excludes the gap", func(t *testing.T) {
		changes, err := c.CodeChangesForCommits(dir, []string{commitA, commitC})
		require.NoError(t, err)
		require.Len(t, changes.Files, 1, "only a.txt belongs to the queried set")
		assert.Equal(t, "a.txt", changes.Files[0].Path)
		assert.Equal(t, 3, changes.Files[0].LinesAdded, "1 line from A + 2 lines from C")
		assert.Equal(t, 3, changes.LinesAdded)
	})

	t.Run("empty input returns empty result", func(t *testing.T) {
		changes, err := c.CodeChangesForCommits(dir, nil)
		require.NoError(t, err)
		assert.Empty(t, changes.Files)
	})

	t.Run("root commit diffs against the empty tree", func(t *testing.T) {
		// Fresh repo; a root commit has no parent, so `sha^` doesn't resolve.
		// CodeChangesForCommits must fall back to the empty tree so the root
		// commit's full content shows up as additions.
		rootDir := t.TempDir()
		runRoot := func(args ...string) string { return gitOutput(rootDir, args...) }
		runRoot("init", "-b", "main")
		runRoot("config", "user.email", "test@test.com")
		runRoot("config", "user.name", "Test")
		runRoot("config", "commit.gpgsign", "false")

		require.NoError(t, os.WriteFile(rootDir+"/seed.txt", []byte("seed1\nseed2\nseed3\n"), 0600))
		runRoot("add", "seed.txt")
		runRoot("commit", "-m", "root")
		rootSHA := runRoot("rev-parse", "HEAD")

		changes, err := c.CodeChangesForCommits(rootDir, []string{rootSHA})
		require.NoError(t, err)
		require.Len(t, changes.Files, 1, "root commit's content must show up as additions")
		assert.Equal(t, "seed.txt", changes.Files[0].Path)
		assert.Equal(t, 3, changes.Files[0].LinesAdded)
	})
}

func TestExecClient_GeneratedMatcher(t *testing.T) {
	// Set up a real git repo so `git check-attr` has a working environment.
	dir := t.TempDir()
	run := func(args ...string) string { return gitOutput(dir, args...) }

	run("init", "-b", "main")
	run("config", "user.email", "test@test.com")
	run("config", "user.name", "Test")

	require.NoError(t, os.WriteFile(
		filepath.Join(dir, ".gitattributes"),
		[]byte(
			"frontend/src/pbgen/** linguist-generated=true\n"+
				"gen/** linguist-generated\n"+
				"docs/** linguist-generated=false\n",
		),
		0o600,
	))

	c := NewExecClient()
	m := c.GeneratedMatcher(dir)

	t.Run("linguist-generated=true matches", func(t *testing.T) {
		assert.True(t, m("frontend/src/pbgen/foo.ts"))
		assert.True(t, m("frontend/src/pbgen/nested/bar.ts"))
		assert.False(t, m("frontend/src/components/foo.ts"))
	})

	t.Run("bare linguist-generated is treated as set", func(t *testing.T) {
		assert.True(t, m("gen/out.pb.go"))
	})

	t.Run("linguist-generated=false returns false", func(t *testing.T) {
		assert.False(t, m("docs/manual.md"))
	})

	t.Run("path not covered by any rule returns false", func(t *testing.T) {
		assert.False(t, m("backend/internal/service/evidence.go"))
	})

	t.Run("empty path returns false", func(t *testing.T) {
		assert.False(t, m(""))
	})
}

func TestExecClient_GeneratedMatcherNonRepo(t *testing.T) {
	// Non-git directory: `git check-attr` fails; matcher must degrade to false.
	c := NewExecClient()
	m := c.GeneratedMatcher(t.TempDir())
	assert.False(t, m("anything"))
	assert.False(t, m("foo/bar.go"))
}

func TestMapGitStatus(t *testing.T) {
	assert.Equal(t, "created", mapGitStatus("A"))
	assert.Equal(t, "deleted", mapGitStatus("D"))
	assert.Equal(t, "renamed", mapGitStatus("R100"))
	assert.Equal(t, "modified", mapGitStatus("M"))
	assert.Equal(t, "modified", mapGitStatus(""))
}
