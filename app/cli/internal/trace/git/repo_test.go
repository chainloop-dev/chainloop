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
	"testing"

	"github.com/chainloop-dev/chainloop/app/cli/internal/trace/hooks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFindGitDir(t *testing.T) {
	gitDir, err := FindGitDir()
	require.NoError(t, err)
	assert.NotEmpty(t, gitDir)
	assert.True(t, filepath.IsAbs(gitDir), "should return absolute path")

	// The .git directory should exist
	_, err = os.Stat(gitDir)
	assert.NoError(t, err)
}

func TestRepoRoot(t *testing.T) {
	root, err := RepoRoot()
	require.NoError(t, err)
	assert.NotEmpty(t, root)
	assert.True(t, filepath.IsAbs(root), "should return absolute path")

	// The repo root should contain a .git dir
	_, err = os.Stat(filepath.Join(root, ".git"))
	assert.NoError(t, err)
}

func TestFindGitDirAndRoot(t *testing.T) {
	gitDir, repoRoot, err := FindGitDirAndRoot()
	require.NoError(t, err)

	assert.NotEmpty(t, gitDir)
	assert.NotEmpty(t, repoRoot)
	// gitDir may differ from repoRoot/.git in worktree setups
	_, err = os.Stat(gitDir)
	assert.NoError(t, err, "gitDir should exist")
	_, err = os.Stat(repoRoot)
	assert.NoError(t, err, "repoRoot should exist")
}

func TestFindGitDirFromNonGitDir(t *testing.T) {
	// Save and restore cwd
	origDir, err := os.Getwd()
	require.NoError(t, err)
	defer func() { _ = os.Chdir(origDir) }()

	tmpDir := t.TempDir()
	require.NoError(t, os.Chdir(tmpDir))

	_, err = FindGitDir()
	assert.Error(t, err)
}

func TestFindGitDirAndRootWithCorruptConfig(t *testing.T) {
	origDir, err := os.Getwd()
	require.NoError(t, err)
	defer func() { _ = os.Chdir(origDir) }()

	repoDir := t.TempDir()
	gitDir := filepath.Join(repoDir, ".git")
	require.NoError(t, os.MkdirAll(gitDir, 0o755))

	// Invalid branch merge ref triggers go-git's strict config validation
	configContent := `[core]
	repositoryformatversion = 0
[branch "broken"]
	merge = not-a-valid-ref
`
	require.NoError(t, os.WriteFile(filepath.Join(gitDir, "config"), []byte(configContent), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(gitDir, "HEAD"), []byte("ref: refs/heads/main\n"), 0o600))

	require.NoError(t, os.Chdir(repoDir))

	foundGitDir, foundRoot, err := FindGitDirAndRoot()
	require.NoError(t, err, "should succeed despite corrupt branch config")

	// Resolve symlinks for macOS where /var -> /private/var
	expectedGitDir, err := filepath.EvalSymlinks(gitDir)
	require.NoError(t, err)
	expectedRoot, err := filepath.EvalSymlinks(repoDir)
	require.NoError(t, err)
	assert.Equal(t, expectedGitDir, foundGitDir)
	assert.Equal(t, expectedRoot, foundRoot)
}

func TestResolveGitFile(t *testing.T) {
	// Create a fake worktree setup: .git file pointing to a gitdir
	tmpDir := t.TempDir()
	realGitDir := filepath.Join(tmpDir, "real-gitdir")
	require.NoError(t, os.MkdirAll(realGitDir, 0o755))

	dotGitFile := filepath.Join(tmpDir, ".git")
	require.NoError(t, os.WriteFile(dotGitFile, []byte("gitdir: "+realGitDir+"\n"), 0o600))

	resolved, err := resolveGitFile(dotGitFile)
	require.NoError(t, err)
	assert.Equal(t, realGitDir, resolved)
}

func TestResolveGitFileRelativePath(t *testing.T) {
	tmpDir := t.TempDir()
	realGitDir := filepath.Join(tmpDir, "worktrees", "my-branch")
	require.NoError(t, os.MkdirAll(realGitDir, 0o755))

	dotGitFile := filepath.Join(tmpDir, ".git")
	require.NoError(t, os.WriteFile(dotGitFile, []byte("gitdir: worktrees/my-branch\n"), 0o600))

	resolved, err := resolveGitFile(dotGitFile)
	require.NoError(t, err)
	assert.Equal(t, realGitDir, resolved)
}

// TestFindGitDirAndRootBrokenGitFileIsNotNotFound guards the trace run
// fallback: only ErrNotARepository may be read as "no repository here", so a
// present-but-unusable .git surfaces instead of silently disabling git
// attribution.
func TestFindGitDirAndRootBrokenGitFileIsNotNotFound(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, ".git"), []byte("garbage\n"), 0o600))
	t.Chdir(dir)

	_, _, err := FindGitDirAndRoot()
	require.Error(t, err)
	assert.NotErrorIs(t, err, ErrNotARepository)
}

func TestResolveGitFileInvalidContent(t *testing.T) {
	tmpDir := t.TempDir()
	dotGitFile := filepath.Join(tmpDir, ".git")
	require.NoError(t, os.WriteFile(dotGitFile, []byte("not a gitdir pointer\n"), 0o600))

	_, err := resolveGitFile(dotGitFile)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unexpected .git file content")
}

// TestHooksInstallFromWorktree is the integration counterpart to the
// synthetic worktree test in cli/internal/trace/hooks: it sets up a real
// linked worktree via the git binary, then verifies that hooks.Install
// given the worktree-private gitDir from FindGitDirAndRoot lands the
// hooks under the main repo's .git/hooks (the only place git reads them).
func TestHooksInstallFromWorktree(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git binary not found, skipping worktree test")
	}

	mainDir := t.TempDir()
	wtDir := filepath.Join(t.TempDir(), "linked-wt")

	run := func(args ...string) {
		cmd := exec.Command("git", args...)
		cmd.Dir = mainDir
		out, err := cmd.CombinedOutput()
		require.NoError(t, err, "git %v failed: %s", args, string(out))
	}

	run("init", "-b", "main")
	run("config", "user.email", "test@test.com")
	run("config", "user.name", "Test")
	run("commit", "--allow-empty", "-m", "init")
	run("worktree", "add", wtDir, "-b", "feature")

	origDir, err := os.Getwd()
	require.NoError(t, err)
	defer func() { _ = os.Chdir(origDir) }()
	require.NoError(t, os.Chdir(wtDir))

	worktreeGitDir, _, err := FindGitDirAndRoot()
	require.NoError(t, err)

	hooksDir, err := hooks.Install(worktreeGitDir, false)
	require.NoError(t, err)

	resolvedMainDir, err := filepath.EvalSymlinks(mainDir)
	require.NoError(t, err)
	expectedHooksDir := filepath.Join(resolvedMainDir, ".git", "hooks")
	assert.Equal(t, expectedHooksDir, hooksDir,
		"Install should resolve worktree gitDir to the main .git/hooks")

	for _, name := range []string{"commit-msg", "post-commit", "pre-push"} {
		content, err := os.ReadFile(filepath.Join(expectedHooksDir, name))
		require.NoError(t, err, "hook %s should exist in main .git/hooks", name)
		assert.Contains(t, string(content), hooks.HookMarker)

		_, err = os.Stat(filepath.Join(worktreeGitDir, "hooks", name))
		assert.True(t, os.IsNotExist(err),
			"hook %s should NOT be written to worktree-private hooks dir", name)
	}
}

// Agent hooks run from the directory the session started in, which is not
// necessarily the checkout that owns the file being edited (PFM-7412), so the
// lookup has to start from an explicit directory rather than cwd.
func TestFindGitDirAndRootFrom(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git binary not found, skipping worktree test")
	}

	mainDir, err := filepath.EvalSymlinks(t.TempDir())
	require.NoError(t, err)
	wtDir := filepath.Join(mainDir, ".claude", "worktrees", "feature")

	run := func(args ...string) {
		cmd := exec.Command("git", args...)
		cmd.Dir = mainDir
		out, err := cmd.CombinedOutput()
		require.NoError(t, err, "git %v failed: %s", args, string(out))
	}
	run("init", "-b", "main")
	run("config", "user.email", "test@test.com")
	run("config", "user.name", "Test")
	run("commit", "--allow-empty", "-m", "init")
	run("worktree", "add", wtDir, "-b", "feature")

	// cwd is the main checkout throughout, as it is for an agent hook.
	t.Chdir(mainDir)

	tests := []struct {
		name        string
		dir         string
		wantGitDir  string
		wantRoot    string
		errNotARepo bool
	}{
		{
			name:       "main checkout",
			dir:        mainDir,
			wantGitDir: filepath.Join(mainDir, ".git"),
			wantRoot:   mainDir,
		},
		{
			name:       "worktree nested in the main checkout",
			dir:        wtDir,
			wantGitDir: filepath.Join(mainDir, ".git", "worktrees", "feature"),
			wantRoot:   wtDir,
		},
		{
			// A Write creating a file in a new directory: the directory does
			// not exist yet when the pre-tool-use hook runs.
			name:       "not yet created directory inside a worktree",
			dir:        filepath.Join(wtDir, "new", "pkg"),
			wantGitDir: filepath.Join(mainDir, ".git", "worktrees", "feature"),
			wantRoot:   wtDir,
		},
		{
			name:        "outside any repository",
			dir:         t.TempDir(),
			errNotARepo: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			gitDir, root, err := FindGitDirAndRootFrom(tc.dir)
			if tc.errNotARepo {
				assert.ErrorIs(t, err, ErrNotARepository)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tc.wantGitDir, gitDir)
			assert.Equal(t, tc.wantRoot, root)
		})
	}
}
