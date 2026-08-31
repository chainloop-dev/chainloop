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

import "path/filepath"

// Store reads and writes the chainloop-trace state under a single parent
// directory: the .git directory inside a repository, or the out-of-tree
// directory `chainloop trace run` picks outside one. Which of the two it is
// decides whether git hooks can be installed and whether the directory itself
// is ours to remove, so the Store carries that rather than leaving callers to
// thread a second copy of the same path alongside it.
type Store struct {
	dir   string
	isGit bool
}

// NewGitStore returns a Store over a repository's .git directory.
// Nothing is created or validated here.
func NewGitStore(gitDir string) *Store {
	return &Store{dir: gitDir, isGit: true}
}

// NewOutOfTreeStore returns a Store over the directory that holds trace state
// outside a repository, as returned by NonGitDir.
func NewOutOfTreeStore(dir string) *Store {
	return &Store{dir: dir}
}

// IsGit reports whether the state lives inside a repository's .git directory.
func (s *Store) IsGit() bool {
	return s.isGit
}

// GitDir returns the repository's .git directory, and "" outside a repository.
// Callers that install or remove git hooks must take the path from here, so a
// store that is not a .git directory cannot be mistaken for one.
func (s *Store) GitDir() string {
	if !s.isGit {
		return ""
	}

	return s.dir
}

// Dir returns the parent directory, one level above chainloop-trace/.
//
// Only `trace run`'s cleanup needs it, to drop the empty out-of-tree directory
// after a non-git run. Inside a repository the same path is .git and must never
// be removed, so callers guarding on IsGit are the only legitimate users.
func (s *Store) Dir() string {
	return s.dir
}

// traceDirPath returns the path to <dir>/chainloop-trace/.
func (s *Store) traceDirPath() string {
	return filepath.Join(s.dir, traceDir)
}
