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
