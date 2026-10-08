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

// Package spec captures the specification a coding session was built from.
//
// The agent is the resolver. Chainloop fetches nothing: the session-start hook
// hands the agent a directory, and the agent writes the text of whatever the
// task names — a ticket, a design document, an approved plan — into it, one
// file per source. The prompt-submit hook reminds it at each turn to capture a
// new or changed spec. Each push folds those files into the session evidence,
// and the session end deletes them. That is why there are no connectors, no
// credentials and no fetch failures anywhere in this package.
//
// The files live in the working tree rather than under .git because that is the
// only place an agent can reliably write: writes under .git prompt or are
// dropped, and in a git worktree the git directory sits outside the session's
// working directory entirely. A .gitignore holding "*" inside the directory
// makes it ignore itself and its contents, so nothing reaches a commit and the
// repository's own .gitignore is left alone.
package spec

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/chainloop-dev/chainloop/app/cli/internal/trace/state"
)

const (
	// dirName is the working-tree directory that parents the spec tree. It is a
	// directory rather than the .chainloop.yml file that sits beside it.
	dirName = ".chainloop"
	// specsSubdir holds one directory per session.
	specsSubdir = "specs"
	// gitignoreName and gitignoreBody make the spec tree hide itself, including
	// the ignore file, so a captured spec can never be committed by accident.
	gitignoreName = ".gitignore"
	gitignoreBody = "*\n"

	// MaxEntries bounds the evidence document against an agent that writes a
	// file per turn. The files an earlier push recorded are kept first, then
	// the oldest new ones, since what the session started from is the last
	// thing worth dropping.
	MaxEntries = 25

	// MetaSuffix names the companion file of a binary file: the name of the
	// binary file plus this suffix. It holds the role, the title and the
	// description that a binary file has no header for.
	MetaSuffix = ".meta.yaml"
)

// Dir returns the directory holding every session's captured specs.
func Dir(repoRoot string) string {
	return filepath.Join(repoRoot, dirName, specsSubdir)
}

// SessionDir returns the directory a single session's specs are written to.
//
// The session ID is agent-supplied, so it goes through the same sanitisation
// the trace state store applies: an ID carrying separators names a directory
// inside Dir rather than escaping to anywhere else in the working tree.
func SessionDir(repoRoot, sessionID string) string {
	return filepath.Join(Dir(repoRoot), state.SanitizeID(sessionID))
}

// EnsureDir creates the spec tree and, if it is absent, the .gitignore that
// hides it. An existing .gitignore is never rewritten: it is in the user's
// working tree and may have been adjusted deliberately.
//
// Only the shared parent is created, never the per-session directory. The
// agent's file-writing tool creates missing parents itself, and creating the
// session directory here would leave an empty one behind for every session that
// captures nothing — which is most of them. What must exist up front is the
// .gitignore, or the first spec written shows up in git status.
func EnsureDir(repoRoot string) error {
	if err := os.MkdirAll(Dir(repoRoot), 0755); err != nil {
		return fmt.Errorf("create spec directory: %w", err)
	}

	ignore := filepath.Join(Dir(repoRoot), gitignoreName)
	if _, err := os.Stat(ignore); err == nil {
		return nil
	} else if !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("stat spec gitignore: %w", err)
	}

	if err := os.WriteFile(ignore, []byte(gitignoreBody), 0600); err != nil {
		return fmt.Errorf("write spec gitignore: %w", err)
	}

	return nil
}

// Exists reports whether anything has been captured for a session.
//
// Deliberately cheap: the session-start hook calls it on every start to decide
// whether to repeat its instruction, so it reads a directory listing and never
// the files themselves.
func Exists(repoRoot, sessionID string) bool {
	entries, err := os.ReadDir(SessionDir(repoRoot, sessionID))
	if err != nil {
		return false
	}

	return slices.ContainsFunc(entries, isCandidate)
}

// ReadAll returns the specs captured for a session, along with a warning for
// each file it could not read and for the entries dropped for exceeding
// MaxEntries.
//
// recorded names the files that the last push of the session stored, in the
// order that push read them. Those come first and are never the ones dropped:
// they are never more than MaxEntries, and each push puts the files of the
// push before it first, so a file stays on the list from one push to the next. The other files follow, oldest first. The modification time alone
// cannot give this order: an overwrite moves a file to the end, and the ticket
// the session started from is the file the agent is most likely to update.
//
// A session that captured nothing — by far the common case — yields no entries
// and no error. Individual files that carry nothing are skipped rather than
// recorded as empty entries. A file that cannot be read costs that file only:
// the others are still returned. The error is kept for a folder that cannot be
// read at all.
func ReadAll(repoRoot, sessionID string, recorded []string) ([]Capture, []string, error) {
	dir := SessionDir(repoRoot, sessionID)

	dirEntries, err := os.ReadDir(dir)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, nil, nil
		}

		return nil, nil, fmt.Errorf("read spec directory: %w", err)
	}

	type candidate struct {
		path    string
		modTime time.Time
		name    string
	}

	var warnings []string

	// A companion file is never a spec of its own. It is read only for the
	// binary file it is named for, and ignored when there is none.
	companions := make(map[string]string)

	candidates := make([]candidate, 0, len(dirEntries))
	for _, e := range dirEntries {
		if !isCandidate(e) {
			continue
		}

		if base, ok := strings.CutSuffix(e.Name(), MetaSuffix); ok {
			companions[base] = filepath.Join(dir, e.Name())
			continue
		}

		info, err := e.Info()
		if err != nil {
			warnings = append(warnings, notRecorded(e.Name(), err))
			continue
		}

		candidates = append(candidates, candidate{
			path:    filepath.Join(dir, e.Name()),
			modTime: info.ModTime(),
			name:    e.Name(),
		})
	}

	rank := make(map[string]int, len(recorded))
	for i, name := range recorded {
		if _, ok := rank[name]; !ok {
			rank[name] = i
		}
	}

	// Files an earlier push recorded come first, in the order they were
	// recorded. Capture order is the useful order for the rest, and the name
	// breaks ties so that two files written in the same instant still sort
	// deterministically.
	sort.Slice(candidates, func(i, j int) bool {
		ri, iRecorded := rank[candidates[i].name]
		rj, jRecorded := rank[candidates[j].name]
		switch {
		case iRecorded && jRecorded:
			return ri < rj
		case iRecorded != jRecorded:
			return iRecorded
		}

		if candidates[i].modTime.Equal(candidates[j].modTime) {
			return candidates[i].name < candidates[j].name
		}

		return candidates[i].modTime.Before(candidates[j].modTime)
	})

	var entries []Capture
	for _, c := range candidates {
		doc, err := os.ReadFile(c.path)
		if err != nil {
			warnings = append(warnings, notRecorded(c.name, err))
			continue
		}

		// An image, or a file that is not text, is something the agent copied
		// in rather than wrote. Parsing it as text would only mangle it.
		if !isText(c.name, doc) {
			capture := verbatimCapture(c.name, doc, c.modTime, isImage(c.name, doc))
			if path, ok := companions[c.name]; ok {
				// A companion file we cannot read costs its values only.
				if meta, err := os.ReadFile(path); err == nil {
					capture.MetaRaw = meta
				}
			}

			entries = append(entries, capture)
			continue
		}

		entry, warning := readText(repoRoot, c.name, doc, c.modTime)
		if warning != "" {
			warnings = append(warnings, warning)
		}
		if entry != nil {
			entries = append(entries, *entry)
		}
	}

	if len(entries) > MaxEntries {
		// Dropping entries silently removes something that would have been
		// attested, so it is reported.
		warnings = append(warnings, fmt.Sprintf("%d spec entries beyond the first %d were dropped", len(entries)-MaxEntries, MaxEntries))
		entries = entries[:MaxEntries]
	}

	return entries, warnings, nil
}

// Remove drops a session's captured specs, once they are somewhere durable. A
// session that captured nothing is not an error.
func Remove(repoRoot, sessionID string) error {
	if err := os.RemoveAll(SessionDir(repoRoot, sessionID)); err != nil {
		return fmt.Errorf("remove spec directory: %w", err)
	}

	return nil
}

// RemoveDir drops the whole spec tree, and then the .chainloop directory itself
// when nothing else is left in it. os.Remove fails with ENOTEMPTY otherwise,
// which is exactly the no-op wanted when that directory holds anything that is
// not ours.
//
// It starts by checking that .chainloop is a directory at all, which is not
// redundant: .chainloop is also the basename the CLI gives its .yaml/.yml
// config, and a plain file carrying the bare name is both something os.Remove
// would happily delete and something os.RemoveAll would fail a whole cleanup
// over, since a path below it is then not a directory.
func RemoveDir(repoRoot string) error {
	parent := filepath.Join(repoRoot, dirName)

	info, err := os.Stat(parent)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("check spec directory: %w", err)
	}
	if !info.IsDir() {
		return nil
	}

	if err := os.RemoveAll(Dir(repoRoot)); err != nil {
		return fmt.Errorf("remove spec directory: %w", err)
	}

	_ = os.Remove(parent)

	return nil
}

// isCandidate reports whether a directory entry could hold a spec. Dotfiles are
// housekeeping — the .gitignore among them — and a subdirectory is not ours to
// walk into.
func isCandidate(e fs.DirEntry) bool {
	return e.Type().IsRegular() && !strings.HasPrefix(e.Name(), ".")
}

// notRecorded is the warning for a spec file that could not be read. It goes
// into the uploaded evidence, so it names the file and the kind of failure,
// never the error text, which carries the local path to the file.
func notRecorded(name string, err error) string {
	reason := "it could not be read"
	if errors.Is(err, fs.ErrPermission) {
		reason = "permission denied"
	}

	return fmt.Sprintf("spec file %q was not recorded: %s", name, reason)
}
