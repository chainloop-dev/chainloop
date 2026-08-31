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

package state

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	tracegit "github.com/chainloop-dev/chainloop/app/cli/internal/trace/git"
)

// Locate returns a Store over the directory that parents chainloop-trace
// state, plus the working-tree root. Inside a git repository those are the .git
// directory and the repo root; outside one, they are the NonGitDir of the
// nearest ancestor of cwd with an active `chainloop trace run`, and that
// ancestor. The returned Store reports which of the two it is via IsGit.
//
// Constructing the Store here is deliberate: callers never hold a bare state
// path they could confuse with the repo root returned alongside it.
//
// The non-git branch probes the run-active sentinel rather than the mere
// existence of a state directory, so one left behind by a crashed run is not
// bound to by an unrelated later invocation. Only ErrNotARepository reaches
// that branch: a broken .git means a repository is there and unusable, and
// binding such a clone to an ancestor's out-of-tree state would silently
// record its commits nowhere.
func Locate() (store *Store, root string, err error) {
	gitDir, repoRoot, err := tracegit.FindGitDirAndRoot()
	switch {
	case err == nil:
		return NewGitStore(gitDir), repoRoot, nil
	case !errors.Is(err, tracegit.ErrNotARepository):
		return nil, "", err
	}

	cwd, err := os.Getwd()
	if err != nil {
		return nil, "", fmt.Errorf("get working directory: %w", err)
	}

	base, err := nonGitBase()
	if err != nil {
		return nil, "", err
	}

	for dir := resolveDir(cwd); ; {
		candidate := NewOutOfTreeStore(filepath.Join(base, hashDir(dir)))
		if candidate.IsTraceRunActive() {
			return candidate, dir, nil
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}

	return nil, "", fmt.Errorf("not a git repository and no active chainloop trace run found from %q up to root", cwd)
}

// NonGitDir returns the out-of-tree directory that parents trace state for a
// working directory with no git repository, under <user cache>/chainloop/trace/.
//
// The path is derived from dir alone so that hook subprocesses recompute it
// instead of inheriting it: a hook is a grandchild of `chainloop trace run`
// (run -> agent -> hook), and an agent that rewrites the environment would
// otherwise send the hook to a different directory where it would silently
// record nothing. The user cache dir is $HOME-derived, which survives that far
// better than $TMPDIR, and it keeps live session state away from /tmp reapers.
func NonGitDir(dir string) (string, error) {
	base, err := nonGitBase()
	if err != nil {
		return "", err
	}

	return filepath.Join(base, hashDir(dir)), nil
}

// nonGitBase returns the parent of every non-git trace state directory.
// An error here means $HOME (and $XDG_CACHE_HOME) are unset — the one case that
// must not be papered over with a fallback path, since the run and its hooks
// would then disagree on where state lives.
func nonGitBase() (string, error) {
	cacheDir, err := os.UserCacheDir()
	if err != nil {
		return "", fmt.Errorf("locate user cache directory: %w", err)
	}

	return filepath.Join(cacheDir, "chainloop", "trace"), nil
}

// hashDir returns the state directory name for a working directory. Symlinks
// are resolved first so two spellings of the same directory (e.g. /tmp/x and
// /private/tmp/x on macOS) map to one state directory.
func hashDir(dir string) string {
	sum := sha256.Sum256([]byte(resolveDir(dir)))

	return hex.EncodeToString(sum[:])[:16]
}

// resolveDir returns dir as an absolute, symlink-free path, degrading to the
// closest approximation it can compute. Duplicated from the config package,
// which cannot import this one.
func resolveDir(dir string) string {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return dir
	}

	resolved, err := filepath.EvalSymlinks(abs)
	if err != nil {
		return abs
	}

	return resolved
}
