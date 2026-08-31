package hooks

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInstall(t *testing.T) {
	t.Run("installs hooks in empty directory", func(t *testing.T) {
		gitDir := t.TempDir()

		_, err := Install(gitDir, false)
		require.NoError(t, err)

		for _, name := range []string{"commit-msg", "post-commit", "pre-push"} {
			hookPath := filepath.Join(gitDir, "hooks", name)
			content, err := os.ReadFile(hookPath)
			require.NoError(t, err)
			assert.Contains(t, string(content), HookMarker)
			assert.Contains(t, string(content), "chainloop trace hook git "+name)

			info, err := os.Stat(hookPath)
			require.NoError(t, err)
			assert.Equal(t, os.FileMode(0755), info.Mode().Perm())
		}

		// commit-msg must forward arguments so the message file path reaches the handler
		commitMsgContent, err := os.ReadFile(filepath.Join(gitDir, "hooks", "commit-msg"))
		require.NoError(t, err)
		assert.Contains(t, string(commitMsgContent), `"$@"`)

		// post-commit and pre-push must NOT forward arguments
		for _, name := range []string{"post-commit", "pre-push"} {
			content, err := os.ReadFile(filepath.Join(gitDir, "hooks", name))
			require.NoError(t, err)
			assert.NotContains(t, string(content), `"$@"`)
		}
	})

	t.Run("backs up existing foreign hooks", func(t *testing.T) {
		gitDir := t.TempDir()
		hooksDir := filepath.Join(gitDir, "hooks")
		require.NoError(t, os.MkdirAll(hooksDir, 0755))

		foreignContent := "#!/bin/sh\necho existing hook\n"
		require.NoError(t, os.WriteFile(filepath.Join(hooksDir, "post-commit"), []byte(foreignContent), 0600))

		_, err := Install(gitDir, false)
		require.NoError(t, err)

		// Backup should exist
		backupContent, err := os.ReadFile(filepath.Join(hooksDir, "post-commit"+hookBackupSuffix))
		require.NoError(t, err)
		assert.Equal(t, foreignContent, string(backupContent))

		// New hook should chain to backup
		hookContent, err := os.ReadFile(filepath.Join(hooksDir, "post-commit"))
		require.NoError(t, err)
		assert.Contains(t, string(hookContent), HookMarker)
		assert.Contains(t, string(hookContent), hookBackupSuffix)
	})

	t.Run("overwrites existing chainloop hooks", func(t *testing.T) {
		gitDir := t.TempDir()
		hooksDir := filepath.Join(gitDir, "hooks")
		require.NoError(t, os.MkdirAll(hooksDir, 0755))

		// Write a chainloop hook
		require.NoError(t, os.WriteFile(filepath.Join(hooksDir, "pre-push"), []byte("#!/bin/sh\n"+HookMarker+"\nold version"), 0600))

		_, err := Install(gitDir, false)
		require.NoError(t, err)

		content, err := os.ReadFile(filepath.Join(hooksDir, "pre-push"))
		require.NoError(t, err)
		assert.Contains(t, string(content), HookMarker)
		assert.NotContains(t, string(content), "old version")

		// No backup should exist
		_, err = os.Stat(filepath.Join(hooksDir, "pre-push"+hookBackupSuffix))
		assert.True(t, os.IsNotExist(err))
	})

	t.Run("writes to common dir when given a worktree gitdir", func(t *testing.T) {
		// Simulate a worktree layout: main/.git is the common dir, and
		// main/.git/worktrees/wt is the per-worktree gitdir with a
		// commondir pointer back to ../...
		tmpDir := t.TempDir()
		commonDir := filepath.Join(tmpDir, "main", ".git")
		require.NoError(t, os.MkdirAll(commonDir, 0o755))

		worktreeGitDir := filepath.Join(commonDir, "worktrees", "wt")
		require.NoError(t, os.MkdirAll(worktreeGitDir, 0o755))
		require.NoError(t, os.WriteFile(
			filepath.Join(worktreeGitDir, "commondir"),
			[]byte("../..\n"),
			0o600,
		))

		hooksDir, err := Install(worktreeGitDir, false)
		require.NoError(t, err)
		assert.Equal(t, filepath.Join(commonDir, "hooks"), hooksDir)

		// Hooks must land under the common dir, not the worktree-private dir.
		for _, name := range []string{"commit-msg", "post-commit", "pre-push"} {
			content, err := os.ReadFile(filepath.Join(commonDir, "hooks", name))
			require.NoError(t, err, "hook %s should exist in common .git/hooks", name)
			assert.Contains(t, string(content), HookMarker)

			_, err = os.Stat(filepath.Join(worktreeGitDir, "hooks", name))
			assert.True(t, os.IsNotExist(err),
				"hook %s must NOT be written to worktree-private hooks dir", name)
		}
	})
}

func TestInstallSkipPrePush(t *testing.T) {
	t.Run("installs commit-msg and post-commit only", func(t *testing.T) {
		gitDir := t.TempDir()

		_, err := Install(gitDir, true)
		require.NoError(t, err)

		for _, name := range []string{"commit-msg", "post-commit"} {
			content, err := os.ReadFile(filepath.Join(gitDir, "hooks", name))
			require.NoError(t, err, "hook %s should be installed", name)
			assert.Contains(t, string(content), HookMarker)
		}

		_, err = os.Stat(filepath.Join(gitDir, "hooks", "pre-push"))
		assert.True(t, os.IsNotExist(err), "pre-push must not be installed when skipPrePush=true")
	})

	t.Run("IsInstalled with skipPrePush ignores missing pre-push", func(t *testing.T) {
		gitDir := t.TempDir()

		_, err := Install(gitDir, true)
		require.NoError(t, err)

		assert.True(t, IsInstalled(gitDir, true), "should be installed when only checking the trace-run subset")
		assert.False(t, IsInstalled(gitDir, false), "default check requires pre-push and should fail")
	})
}

func TestInstallRefusesToClobberBackup(t *testing.T) {
	gitDir := t.TempDir()
	hooksDir := filepath.Join(gitDir, "hooks")
	require.NoError(t, os.MkdirAll(hooksDir, 0755))

	// A leftover backup (crashed uninstall) plus a foreign hook: the backup is
	// the user's original and must survive.
	original := "#!/bin/sh\necho original\n"
	require.NoError(t, os.WriteFile(filepath.Join(hooksDir, "post-commit"+hookBackupSuffix), []byte(original), 0600))
	require.NoError(t, os.WriteFile(filepath.Join(hooksDir, "post-commit"), []byte("#!/bin/sh\necho newer\n"), 0600))

	_, err := Install(gitDir, false)
	require.Error(t, err)

	content, err := os.ReadFile(filepath.Join(hooksDir, "post-commit"+hookBackupSuffix))
	require.NoError(t, err)
	assert.Equal(t, original, string(content))
}

// TestHookScriptsExitZero runs each generated hook with an empty PATH (no
// chainloop binary, as in GUI git clients) and asserts it still succeeds:
// a non-zero hook aborts the user's commit.
func TestHookScriptsExitZero(t *testing.T) {
	gitDir := t.TempDir()
	hooksDir := filepath.Join(gitDir, "hooks")
	require.NoError(t, os.MkdirAll(hooksDir, 0755))

	// post-commit gets a foreign hook so the chained variant is covered too,
	// with its executable bit cleared to exercise the failing `&&` list.
	require.NoError(t, os.WriteFile(filepath.Join(hooksDir, "post-commit"), []byte("#!/bin/sh\nexit 0\n"), 0600))

	_, err := Install(gitDir, false)
	require.NoError(t, err)

	for _, name := range []string{"commit-msg", "post-commit", "pre-push"} {
		//nolint:gosec // the hook path is derived from t.TempDir(), not from user input
		cmd := exec.Command("/bin/sh", filepath.Join(hooksDir, name), "msgfile")
		cmd.Env = []string{"PATH="}
		out, err := cmd.CombinedOutput()
		assert.NoError(t, err, "hook %s must exit 0 without chainloop on PATH: %s", name, out)
	}
}

func TestUninstall(t *testing.T) {
	t.Run("removes chainloop hooks", func(t *testing.T) {
		gitDir := t.TempDir()
		_, err := Install(gitDir, false)
		require.NoError(t, err)

		_, err = Uninstall(gitDir)
		require.NoError(t, err)

		for _, name := range []string{"commit-msg", "post-commit", "pre-push"} {
			_, err := os.Stat(filepath.Join(gitDir, "hooks", name))
			assert.True(t, os.IsNotExist(err))
		}
	})

	t.Run("restores backup hooks", func(t *testing.T) {
		gitDir := t.TempDir()
		hooksDir := filepath.Join(gitDir, "hooks")
		require.NoError(t, os.MkdirAll(hooksDir, 0755))

		// Install foreign hook, then chainloop hook on top
		foreignContent := "#!/bin/sh\necho foreign\n"
		require.NoError(t, os.WriteFile(filepath.Join(hooksDir, "post-commit"), []byte(foreignContent), 0600))
		_, err := Install(gitDir, false)
		require.NoError(t, err)

		// Uninstall
		_, err = Uninstall(gitDir)
		require.NoError(t, err)

		// Foreign hook should be restored
		content, err := os.ReadFile(filepath.Join(hooksDir, "post-commit"))
		require.NoError(t, err)
		assert.Equal(t, foreignContent, string(content))
	})

	t.Run("leaves foreign hooks alone", func(t *testing.T) {
		gitDir := t.TempDir()
		hooksDir := filepath.Join(gitDir, "hooks")
		require.NoError(t, os.MkdirAll(hooksDir, 0755))

		foreignContent := "#!/bin/sh\necho untouched\n"
		require.NoError(t, os.WriteFile(filepath.Join(hooksDir, "pre-push"), []byte(foreignContent), 0600))

		_, err := Uninstall(gitDir)
		require.NoError(t, err)

		content, err := os.ReadFile(filepath.Join(hooksDir, "pre-push"))
		require.NoError(t, err)
		assert.Equal(t, foreignContent, string(content))
	})

	t.Run("noop when no hooks exist", func(t *testing.T) {
		gitDir := t.TempDir()
		_, err := Uninstall(gitDir)
		assert.NoError(t, err)
	})
}

func TestResolveCommonDir(t *testing.T) {
	t.Run("absent commondir file returns gitDir unchanged", func(t *testing.T) {
		gitDir := t.TempDir()

		resolved, err := resolveCommonDir(gitDir)
		require.NoError(t, err)
		assert.Equal(t, gitDir, resolved)
	})

	t.Run("absolute commondir path", func(t *testing.T) {
		tmpDir := t.TempDir()
		commonDir := filepath.Join(tmpDir, "main", ".git")
		require.NoError(t, os.MkdirAll(commonDir, 0o755))

		worktreeGitDir := filepath.Join(commonDir, "worktrees", "wt")
		require.NoError(t, os.MkdirAll(worktreeGitDir, 0o755))
		require.NoError(t, os.WriteFile(
			filepath.Join(worktreeGitDir, "commondir"),
			[]byte(commonDir+"\n"),
			0o600,
		))

		resolved, err := resolveCommonDir(worktreeGitDir)
		require.NoError(t, err)
		assert.Equal(t, commonDir, resolved)
	})

	t.Run("relative commondir path", func(t *testing.T) {
		tmpDir := t.TempDir()
		commonDir := filepath.Join(tmpDir, "main", ".git")
		require.NoError(t, os.MkdirAll(commonDir, 0o755))

		// git writes "../.." here: from .git/worktrees/<name> up to main .git.
		worktreeGitDir := filepath.Join(commonDir, "worktrees", "wt")
		require.NoError(t, os.MkdirAll(worktreeGitDir, 0o755))
		require.NoError(t, os.WriteFile(
			filepath.Join(worktreeGitDir, "commondir"),
			[]byte("../..\n"),
			0o600,
		))

		resolved, err := resolveCommonDir(worktreeGitDir)
		require.NoError(t, err)
		assert.Equal(t, commonDir, resolved)
	})

	t.Run("empty commondir file is rejected", func(t *testing.T) {
		gitDir := t.TempDir()
		require.NoError(t, os.WriteFile(filepath.Join(gitDir, "commondir"), []byte("\n"), 0o600))

		_, err := resolveCommonDir(gitDir)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "commondir file is empty")
	})
}
