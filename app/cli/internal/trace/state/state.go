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
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	// traceDir is the directory holding all trace state. It is created
	// inside the Store's parent directory: .git/ inside a repository, or
	// the out-of-tree directory `chainloop trace run` picks outside one.
	traceDir = "chainloop-trace"

	// Subdirectories inside traceDir.
	traceInitializedFile = "initialized"
	// traceRunActiveFile is the sentinel created by `chainloop trace run`
	// for the lifetime of its wrapped command. It lets hook subprocesses
	// detect trace-run mode without depending on environment-variable
	// inheritance from the agent.
	traceRunActiveFile = "run-active"
	traceDirSessions   = "sessions"
	traceDirRaw        = "raw"
	traceDirCommits    = "commits"
	traceDirSnapshots  = "snapshots"
	traceDirAILines    = "ai-lines"

	// Per-record file extensions inside the subdirectories above.
	commitRecordExt  = ".json"
	sessionRecordExt = ".json"
	aiLinesExt       = ".jsonl"
)

// InitTraceDir creates the <dir>/chainloop-trace/ directory structure.
func (s *Store) InitTraceDir() error {
	base := s.traceDirPath()
	for _, sub := range []string{traceDirSessions, traceDirRaw, traceDirCommits, traceDirSnapshots, traceDirAILines} {
		if err := os.MkdirAll(filepath.Join(base, sub), 0755); err != nil {
			return fmt.Errorf("create %s directory: %w", sub, err)
		}
	}
	return nil
}

// WipeTraceDir clears the single-use data left over by a successful push:
// raw transcripts and per-edit file snapshots. It intentionally preserves
// commits/, ai-lines/, and sessions/ — those carry session attribution that
// survives across pushes (a rebase replays commits with the same trailers, so
// the next push can still attribute them). Bounded growth of the surviving
// subdirectories is the job of GCOrphans, called after the wipe.
func (s *Store) WipeTraceDir() error {
	base := s.traceDirPath()

	for _, sub := range []string{traceDirRaw, traceDirSnapshots} {
		if err := os.RemoveAll(filepath.Join(base, sub)); err != nil {
			return fmt.Errorf("remove %s: %w", sub, err)
		}
	}

	// Re-create empty subdirectories
	return s.InitTraceDir()
}

// GCOrphans drops trace state whose underlying commit is no longer reachable
// from any local branch. liveSHAs is the union of SHAs reachable from every
// local branch tip; the caller computes it from the git client.
//
// Two cleanups in sequence:
//
//  1. Walk commits/, drop any record whose SHA isn't in liveSHAs (post-rebase
//     orphan or commit on a deleted branch).
//  2. Walk ai-lines/ and sessions/, drop any entry whose session ID is no
//     longer referenced by a surviving CommitRecord.
//
// An empty liveSHAs is treated as a pathological signal (no local branches
// readable, or the caller fed us garbage) and the GC short-circuits — wiping
// every record on a transient git error would be far worse than skipping a
// cycle of cleanup.
func (s *Store) GCOrphans(liveSHAs map[string]bool) error {
	if len(liveSHAs) == 0 {
		return nil
	}

	commitsDir := filepath.Join(s.traceDirPath(), traceDirCommits)
	commitEntries, err := os.ReadDir(commitsDir)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("read commits dir: %w", err)
	}

	liveSessions := make(map[string]struct{})
	for _, entry := range commitEntries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), commitRecordExt) {
			continue
		}
		path := filepath.Join(commitsDir, entry.Name())
		rec, readErr := loadCommitRecord(path)
		if readErr != nil {
			// Unreadable record — drop it; we can't reason about its SHA.
			_ = os.Remove(path)
			continue
		}
		if !liveSHAs[rec.SHA] {
			_ = os.Remove(path)
			continue
		}
		for _, sid := range rec.SessionIDs {
			liveSessions[sanitizeID(sid)] = struct{}{}
		}
	}

	for _, child := range []struct {
		sub string
		ext string
	}{
		{traceDirAILines, aiLinesExt},
		{traceDirSessions, sessionRecordExt},
	} {
		dir := filepath.Join(s.traceDirPath(), child.sub)
		entries, err := os.ReadDir(dir)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				continue
			}

			return fmt.Errorf("read %s dir: %w", child.sub, err)
		}
		for _, entry := range entries {
			if entry.IsDir() || !strings.HasSuffix(entry.Name(), child.ext) {
				continue
			}
			id := strings.TrimSuffix(entry.Name(), child.ext)
			if _, alive := liveSessions[id]; !alive {
				_ = os.Remove(filepath.Join(dir, entry.Name()))
			}
		}
	}

	return nil
}

// RemoveTraceDir removes the entire <dir>/chainloop-trace/ directory.
func (s *Store) RemoveTraceDir() error {
	return os.RemoveAll(s.traceDirPath())
}

// LogFilePath returns the path to the hook log file inside the trace directory.
func (s *Store) LogFilePath() string {
	return filepath.Join(s.traceDirPath(), "log.txt")
}

// AttestationStatePath returns the path to the local attestation state file
// under <dir>/chainloop-trace/. This file isolates trace attestation state
// from the default chainloop state location.
func (s *Store) AttestationStatePath() string {
	return filepath.Join(s.traceDirPath(), "attestation-state.json")
}

// IsTraceInitialized checks whether trace has been initialized in this clone.
func (s *Store) IsTraceInitialized() bool {
	_, err := os.Stat(filepath.Join(s.traceDirPath(), traceInitializedFile))
	return err == nil
}

// MarkTraceInitialized creates the sentinel file indicating trace is initialized.
func (s *Store) MarkTraceInitialized() error {
	base := s.traceDirPath()
	if err := os.MkdirAll(base, 0755); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(base, traceInitializedFile), nil, 0600)
}

// RemoveTraceInitialized removes the sentinel file.
func (s *Store) RemoveTraceInitialized() error {
	return removeIfExists(filepath.Join(s.traceDirPath(), traceInitializedFile))
}

// MarkTraceRunActive marks the trace state as belonging to a running
// `chainloop trace run` session. Removed by ClearTraceRunActive when
// the wrapped command exits.
func (s *Store) MarkTraceRunActive() error {
	base := s.traceDirPath()
	if err := os.MkdirAll(base, 0755); err != nil {
		return err
	}

	return os.WriteFile(filepath.Join(base, traceRunActiveFile), nil, 0600)
}

// ClearTraceRunActive removes the trace-run-active sentinel.
func (s *Store) ClearTraceRunActive() error {
	return removeIfExists(filepath.Join(s.traceDirPath(), traceRunActiveFile))
}

// IsTraceRunActive reports whether a `chainloop trace run` session owns
// this store's trace state.
func (s *Store) IsTraceRunActive() bool {
	_, err := os.Stat(filepath.Join(s.traceDirPath(), traceRunActiveFile))

	return err == nil
}

// ValidSessionID reports whether an agent-assigned session ID can safely name
// a file. IDs reach us from agent hook payloads and from commit-message
// trailers, and both are used to read the agent's own transcript directory, so
// anything that is not a single path element could escape it. Real IDs are
// UUID- or slug-shaped, so this rejects nothing legitimate.
//
// Writers additionally pass IDs through SanitizeID, which keeps a rejected ID
// from escaping our own state directory even if a caller skips this check.
func ValidSessionID(id string) bool {
	return id != "" && id != "." && id != ".." && filepath.Base(id) == id
}

// SanitizeID strips path separators and traversal components from an
// identifier before using it in filesystem paths. Exported so providers
// in other packages compute the same on-disk path that the writers in
// this package emit; otherwise an ID containing "/" or ".." would write
// to one filename and be read from another.
//
// A rewritten ID also gets a digest suffix, since distinct IDs ("a/x" and
// "a_x") would otherwise collapse onto one filename and merge two sessions'
// state. Separating the rewritten set is all that is needed: agent-assigned
// IDs are UUID-shaped and pass through untouched.
//
// Returning an already-safe ID verbatim makes this idempotent, which callers
// depend on — GCOrphans compares filename-derived keys against sanitized ones,
// and LoadAllAILineAttributions feeds a filename straight back in. Always
// appending the digest would double-suffix those keys and match nothing.
func SanitizeID(id string) string {
	safe := filepath.Base(strings.ReplaceAll(id, "/", "_"))
	if safe == "." || safe == ".." {
		safe = "_"
	}
	if safe == id {
		return safe
	}

	sum := sha256.Sum256([]byte(id))

	return fmt.Sprintf("%s-%x", safe, sum[:4])
}

// sanitizeID is kept as a private alias so existing call sites in this
// package read naturally; new code outside the package should use SanitizeID.
func sanitizeID(id string) string {
	return SanitizeID(id)
}

// removeIfExists removes a file, returning nil if it doesn't exist.
func removeIfExists(path string) error {
	err := os.Remove(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}

	return err
}

// NowTimestamp returns the current UTC time in RFC3339 format.
func NowTimestamp() string {
	return time.Now().UTC().Format(time.RFC3339)
}
