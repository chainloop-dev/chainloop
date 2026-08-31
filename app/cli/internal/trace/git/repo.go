package git

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ErrNotARepository reports that no .git was found from cwd up to root. It is
// the only error callers should read as "fall back to non-git mode": every
// other failure (unreadable cwd, malformed .git file) means a repository is
// there but broken, and must be surfaced.
var ErrNotARepository = errors.New("not a git repository (or any parent up to root)")

// FindGitDir returns the path to the .git directory for the current repository.
func FindGitDir() (string, error) {
	gitDir, _, err := findGitDirAndRoot()
	return gitDir, err
}

// RepoRoot returns the top-level directory of the current git repository.
func RepoRoot() (string, error) {
	_, root, err := findGitDirAndRoot()
	return root, err
}

// FindGitDirAndRoot returns both the .git directory and repo root.
func FindGitDirAndRoot() (gitDir, repoRoot string, err error) {
	return findGitDirAndRoot()
}

// findGitDirAndRoot walks up from cwd to locate .git, avoiding go-git's strict
// config validation which fails on repos with invalid branch config.
func findGitDirAndRoot() (string, string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", "", fmt.Errorf("get working directory: %w", err)
	}

	dir := cwd
	for {
		dotGit := filepath.Join(dir, ".git")
		fi, err := os.Lstat(dotGit)
		if err == nil {
			if fi.IsDir() {
				return dotGit, dir, nil
			}

			// .git is a file (worktree) — parse "gitdir: <path>"
			gitDir, err := resolveGitFile(dotGit)
			if err != nil {
				return "", "", fmt.Errorf("resolve .git file: %w", err)
			}

			return gitDir, dir, nil
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}

	return "", "", ErrNotARepository
}

// IsMergeInProgress reports whether a merge is in progress in the given .git
// directory. Detected via the presence of the MERGE_HEAD file, which git
// creates when a `git merge` cannot fast-forward and stops to let the user
// produce the merge commit.
func IsMergeInProgress(gitDir string) bool {
	_, err := os.Stat(filepath.Join(gitDir, "MERGE_HEAD"))

	return err == nil
}

// resolveGitFile reads a .git file (used in worktrees) and resolves
// the gitdir path it points to.
func resolveGitFile(dotGitFile string) (string, error) {
	data, err := os.ReadFile(dotGitFile)
	if err != nil {
		return "", fmt.Errorf("read .git file: %w", err)
	}

	line := strings.TrimSpace(string(data))
	if !strings.HasPrefix(line, "gitdir: ") {
		return "", fmt.Errorf("unexpected .git file content: %s", line)
	}

	gitDir := strings.TrimPrefix(line, "gitdir: ")
	if !filepath.IsAbs(gitDir) {
		gitDir = filepath.Join(filepath.Dir(dotGitFile), gitDir)
	}

	gitDir = filepath.Clean(gitDir)
	if _, err := os.Stat(gitDir); err != nil {
		return "", fmt.Errorf("gitdir path does not exist: %w", err)
	}

	return gitDir, nil
}
