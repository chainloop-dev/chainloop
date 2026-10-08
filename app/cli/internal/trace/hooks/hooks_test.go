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

package hooks

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInstall(t *testing.T) {
	t.Run("installs hooks in empty directory", func(t *testing.T) {
		gitDir := t.TempDir()

		_, err := Install(gitDir, false)
		require.NoError(t, err)

		for _, name := range []string{"commit-msg", "post-commit", "post-rewrite", "pre-push"} {
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

		// post-commit, post-rewrite and pre-push must NOT forward arguments
		for _, name := range []string{"post-commit", "post-rewrite", "pre-push"} {
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
		for _, name := range []string{"commit-msg", "post-commit", "post-rewrite", "pre-push"} {
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
	t.Run("installs every hook except pre-push", func(t *testing.T) {
		gitDir := t.TempDir()

		_, err := Install(gitDir, true)
		require.NoError(t, err)

		for _, name := range []string{"commit-msg", "post-commit", "post-rewrite"} {
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

	t.Run("IsInstalled reports outdated managed scripts so they get reinstalled", func(t *testing.T) {
		testCases := []struct {
			name        string
			foreignHook bool
		}{
			{name: "plain"},
			{name: "chained", foreignHook: true},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				gitDir := t.TempDir()
				hooksDir := filepath.Join(gitDir, "hooks")
				require.NoError(t, os.MkdirAll(hooksDir, 0755))
				if tc.foreignHook {
					require.NoError(t, os.WriteFile(filepath.Join(hooksDir, "pre-push"), []byte("#!/bin/sh\nexit 0\n"), 0600))
				}

				_, err := Install(gitDir, false)
				require.NoError(t, err)
				require.True(t, IsInstalled(gitDir, false))

				// A pre-push script written by an older CLI that always exits 0.
				stale := "#!/bin/sh\n" + HookMarker + "\nchainloop trace hook git pre-push\nexit 0\n"
				require.NoError(t, os.WriteFile(filepath.Join(hooksDir, "pre-push"), []byte(stale), 0600))
				assert.False(t, IsInstalled(gitDir, false))

				_, err = Install(gitDir, false)
				require.NoError(t, err)
				assert.True(t, IsInstalled(gitDir, false))

				_, err = os.Stat(filepath.Join(hooksDir, "pre-push"+hookBackupSuffix))
				assert.Equal(t, tc.foreignHook, err == nil, "backup must be kept as-is")
			})
		}
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

// TestHookGoldenFiles compares the hook scripts Install writes against the
// committed golden files in testdata/. The temporary hooks directory is
// replaced by a placeholder so the files stay stable. Run with
// UPDATE_GOLDEN=1 to regenerate them after an intentional change:
//
//	UPDATE_GOLDEN=1 go test ./app/cli/internal/trace/hooks/ -run TestHookGoldenFiles
func TestHookGoldenFiles(t *testing.T) {
	const hooksDirPlaceholder = "<hooks-dir>"

	testCases := []struct {
		hook        string
		foreignHook bool
		golden      string
	}{
		{hook: "commit-msg", golden: "testdata/commit-msg.sh"},
		{hook: "post-commit", golden: "testdata/post-commit.sh"},
		{hook: "post-rewrite", golden: "testdata/post-rewrite.sh"},
		{hook: "pre-push", golden: "testdata/pre-push.sh"},
		{hook: "pre-push", foreignHook: true, golden: "testdata/pre-push-chained.sh"},
		{hook: "post-commit", foreignHook: true, golden: "testdata/post-commit-chained.sh"},
	}

	for _, tc := range testCases {
		t.Run(filepath.Base(tc.golden), func(t *testing.T) {
			gitDir := t.TempDir()
			hooksDir := filepath.Join(gitDir, "hooks")
			require.NoError(t, os.MkdirAll(hooksDir, 0755))
			if tc.foreignHook {
				require.NoError(t, os.WriteFile(filepath.Join(hooksDir, tc.hook), []byte("#!/bin/sh\n"), 0600))
			}

			_, err := Install(gitDir, false)
			require.NoError(t, err)

			content, err := os.ReadFile(filepath.Join(hooksDir, tc.hook))
			require.NoError(t, err)
			generated := strings.ReplaceAll(string(content), hooksDir, hooksDirPlaceholder)

			if os.Getenv("UPDATE_GOLDEN") == "1" {
				//nolint:gosec // golden paths are fixed testdata paths from the table above
				require.NoError(t, os.WriteFile(tc.golden, []byte(generated), 0600))
				return
			}

			expected, err := os.ReadFile(tc.golden)
			require.NoError(t, err, "golden file missing; run UPDATE_GOLDEN=1 to generate")
			assert.Equal(t, string(expected), generated,
				"generated hook does not match golden file %s; run UPDATE_GOLDEN=1 to update", tc.golden)
		})
	}
}

// TestHookScriptsExitStatus runs each generated hook against a fake chainloop
// binary, or none at all (as in GUI git clients). Only pre-push propagates a
// chainloop failure, so requireTrace can block the push; every other hook must
// never abort the user's commit, and a missing binary never fails any hook.
func TestHookScriptsExitStatus(t *testing.T) {
	testCases := []struct {
		name          string
		hook          string
		chainloopExit int
		noBinary      bool
		foreignHook   bool
		// foreignNoExec clears the foreign hook's executable bit, which
		// exercises the failing `&&` list in the chained script.
		foreignNoExec bool
		wantErr       bool
		wantChained   bool
	}{
		{name: "pre-push blocks on chainloop failure", hook: "pre-push", chainloopExit: 1, wantErr: true},
		{name: "pre-push passes on chainloop success", hook: "pre-push", chainloopExit: 0},
		{name: "chained pre-push blocks without running the foreign hook", hook: "pre-push", chainloopExit: 1, foreignHook: true, wantErr: true},
		{name: "chained pre-push runs the foreign hook on success", hook: "pre-push", chainloopExit: 0, foreignHook: true, wantChained: true},
		{name: "commit-msg ignores chainloop failure", hook: "commit-msg", chainloopExit: 1},
		{name: "post-commit ignores chainloop failure", hook: "post-commit", chainloopExit: 1},
		{name: "post-rewrite ignores chainloop failure", hook: "post-rewrite", chainloopExit: 1},
		{name: "chained post-commit ignores chainloop failure", hook: "post-commit", chainloopExit: 1, foreignHook: true, wantChained: true},
		{name: "pre-push passes without chainloop", hook: "pre-push", noBinary: true},
		{name: "commit-msg passes without chainloop", hook: "commit-msg", noBinary: true},
		{name: "post-commit passes without chainloop", hook: "post-commit", noBinary: true},
		{name: "post-rewrite passes without chainloop", hook: "post-rewrite", noBinary: true},
		{name: "chained pre-push runs the foreign hook without chainloop", hook: "pre-push", noBinary: true, foreignHook: true, wantChained: true},
		{name: "chained post-commit passes with a non-executable foreign hook", hook: "post-commit", noBinary: true, foreignHook: true, foreignNoExec: true},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			gitDir := t.TempDir()
			hooksDir := filepath.Join(gitDir, "hooks")
			require.NoError(t, os.MkdirAll(hooksDir, 0755))

			marker := filepath.Join(t.TempDir(), "chained")
			if tc.foreignHook {
				mode := os.FileMode(0755)
				if tc.foreignNoExec {
					mode = 0600
				}
				// A redirection, not touch: PATH holds only the fake binary.
				require.NoError(t, os.WriteFile(filepath.Join(hooksDir, tc.hook),
					[]byte("#!/bin/sh\n: > \""+marker+"\"\n"), mode))
			}

			_, err := Install(gitDir, false)
			require.NoError(t, err)

			binDir := t.TempDir()
			if !tc.noBinary {
				//nolint:gosec // the fake binary must be executable
				require.NoError(t, os.WriteFile(filepath.Join(binDir, "chainloop"),
					[]byte(fmt.Sprintf("#!/bin/sh\nexit %d\n", tc.chainloopExit)), 0755))
			}

			//nolint:gosec // the hook path is derived from t.TempDir(), not from user input
			cmd := exec.Command("/bin/sh", filepath.Join(hooksDir, tc.hook), "origin")
			cmd.Env = []string{"PATH=" + binDir}
			out, err := cmd.CombinedOutput()
			if tc.wantErr {
				var exitErr *exec.ExitError
				require.ErrorAs(t, err, &exitErr, "hook output: %s", out)
				assert.Equal(t, tc.chainloopExit, exitErr.ExitCode())
			} else {
				require.NoError(t, err, "hook output: %s", out)
			}

			_, statErr := os.Stat(marker)
			assert.Equal(t, tc.wantChained, statErr == nil, "foreign hook ran")
		})
	}
}

func TestUninstall(t *testing.T) {
	t.Run("removes chainloop hooks", func(t *testing.T) {
		gitDir := t.TempDir()
		_, err := Install(gitDir, false)
		require.NoError(t, err)

		_, err = Uninstall(gitDir)
		require.NoError(t, err)

		for _, name := range []string{"commit-msg", "post-commit", "post-rewrite", "pre-push"} {
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
