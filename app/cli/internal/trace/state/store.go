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
	"path/filepath"

	"github.com/chainloop-dev/chainloop/app/cli/internal/repositoryconfig"
)

// Store reads and writes the chainloop-trace state under a single parent
// directory: the .git directory inside a repository, or the out-of-tree
// directory `chainloop trace run` picks outside one. Which of the two it is
// decides whether git hooks can be installed and whether the directory itself
// is ours to remove, so the Store carries that rather than leaving callers to
// thread a second copy of the same path alongside it.
type Store struct {
	dir   string
	isGit bool
	// traceDir, when set, replaces <dir>/chainloop-trace as the trace state
	// directory. It comes from tracePath in .chainloop.yml.
	traceDir string
}

// NewGitStore returns a Store over a repository's .git directory.
// Nothing is created or validated here.
func NewGitStore(gitDir string) *Store {
	return &Store{dir: gitDir, isGit: true}
}

// NewRepoStore returns a Store over a repository's .git directory that honors
// tracePath in the repository's .chainloop.yml. tracePath moves the trace state
// directory, in every storage mode. A relative path resolves against repoRoot.
// With no tracePath, the state stays in <gitDir>/chainloop-trace. The git hooks
// stay in gitDir either way.
//
// Every git store of a repository must come from here, so the hooks, the
// pre-push and the agent hooks all read and write the same directory.
func NewRepoStore(gitDir, repoRoot string) *Store {
	store := NewGitStore(gitDir)

	cfg := repositoryconfig.FindChainloopYML(repoRoot)
	if cfg == nil || cfg.TracePath == "" {
		return store
	}

	store.traceDir = cfg.TracePath
	if !filepath.IsAbs(store.traceDir) {
		store.traceDir = filepath.Join(repoRoot, store.traceDir)
	}

	return store
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

// TraceDir returns the trace state directory: tracePath when the repository
// sets one, otherwise <dir>/chainloop-trace.
func (s *Store) TraceDir() string {
	return s.traceDirPath()
}

// traceDirPath returns the trace state directory. See TraceDir.
func (s *Store) traceDirPath() string {
	if s.traceDir != "" {
		return s.traceDir
	}

	return filepath.Join(s.dir, traceDir)
}
