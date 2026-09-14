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
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/chainloop-dev/chainloop/pkg/attestation/crafter/materials/aicodingsession"
)

// AILineAttribution tracks which line ranges in which files a session modified.
type AILineAttribution struct {
	SessionID string                                 `json:"session_id"`
	Files     map[string][]aicodingsession.LineRange `json:"files"`
	// Pending maps each file whose recorded ranges have not been committed yet
	// to the time of its most recent edit, and is the only set a new commit may
	// be matched against. Files keeps every range ever recorded because
	// push-time enrichment needs the full history, but matching against it
	// credits a finished session on every later commit that happens to touch a
	// file it once edited.
	//
	// The timestamp resolves which session owns a file two of them both have
	// pending: the later edit rewrote the earlier one's lines.
	Pending map[string]time.Time `json:"-"`
}

// newAILineAttribution returns an AILineAttribution with initialized maps.
func newAILineAttribution(sessionID string) *AILineAttribution {
	return &AILineAttribution{
		SessionID: sessionID,
		Files:     make(map[string][]aicodingsession.LineRange),
		Pending:   make(map[string]time.Time),
	}
}

// aiLinesPath returns the path for a session's AI line attribution file (JSONL).
func (s *Store) aiLinesPath(sessionID string) string {
	return filepath.Join(s.traceDirPath(), traceDirAILines, sanitizeID(sessionID)+".jsonl")
}

// aiLineEntry is a single JSONL record appended by RecordLineRanges.
type aiLineEntry struct {
	// SessionID is the agent-assigned session ID. The filename carries only
	// its sanitized form, which readers cannot map back to the original.
	SessionID string                      `json:"session_id,omitempty"`
	File      string                      `json:"file"`
	Ranges    []aicodingsession.LineRange `json:"ranges"`
	// RecordedAt is the RFC3339 timestamp of the edit this entry describes,
	// with sub-second precision so two sessions editing one file can be
	// ordered. On a ConsumedBy marker it is the time of the edit being
	// retired, not when the marker was written. Entries written before this
	// field existed have none, and are never treated as pending — those
	// ledgers predate consumption tracking, so their files would otherwise
	// keep crediting their session forever.
	RecordedAt string `json:"recorded_at,omitempty"`
	// ConsumedBy is the SHA of the commit that committed the file's pending
	// ranges. Set only on marker entries appended by MarkConsumed, which
	// clear the file from Pending instead of contributing ranges.
	ConsumedBy string `json:"consumed_by,omitempty"`
}

// LoadAILineAttribution loads the AI line attribution for a session by
// consolidating all JSONL entries into a single AILineAttribution.
// Returns a zero-value struct if the file doesn't exist.
func (s *Store) LoadAILineAttribution(sessionID string) *AILineAttribution {
	attr := newAILineAttribution(sessionID)

	f, err := os.Open(s.aiLinesPath(sessionID))
	if err != nil {
		return attr
	}
	defer func() { _ = f.Close() }()

	// Entries are appended in order, so replaying them in order lets a file
	// go pending -> consumed -> pending again across successive edits and
	// commits.
	dec := json.NewDecoder(f)
	for dec.More() {
		var entry aiLineEntry
		if err := dec.Decode(&entry); err != nil {
			break
		}
		if entry.SessionID != "" {
			attr.SessionID = entry.SessionID
		}

		if entry.ConsumedBy != "" {
			// Retire only the edit the commit actually contained. A concurrent
			// RecordLineRanges can land between MarkConsumed reading the
			// pending set and appending its markers, leaving a marker behind an
			// edit it never saw; that edit's later timestamp keeps it pending.
			// A marker with no readable time predates this and retires the file
			// outright.
			consumedAt, err := time.Parse(time.RFC3339, entry.RecordedAt)
			if err != nil || !attr.Pending[entry.File].After(consumedAt) {
				delete(attr.Pending, entry.File)
			}

			continue
		}

		attr.Files[entry.File] = append(attr.Files[entry.File], entry.Ranges...)
		// An entry with no parseable timestamp is never pending: it either
		// predates consumption tracking, in which case its session is long
		// gone, or it cannot be ordered against a competing session's edit.
		if recordedAt, err := time.Parse(time.RFC3339, entry.RecordedAt); err == nil {
			attr.Pending[entry.File] = recordedAt
		}
	}

	return attr
}

// RecordLineRanges appends line ranges for a file to a session's attribution JSONL.
// An empty or nil ranges slice still records the file as touched (e.g., deletion-only edits).
func (s *Store) RecordLineRanges(sessionID, filePath string, ranges []aicodingsession.LineRange) error {
	return s.RecordLineRangesAt(sessionID, filePath, ranges, time.Now().UTC())
}

// RecordLineRangesAt records an edit made at a given time. Ownership of a file
// two sessions both edited is decided by which edit came last, so tests that
// exercise that need to set the times rather than hope the wall clock advances
// between two calls — it does not on a host with a coarse clock.
func (s *Store) RecordLineRangesAt(sessionID, filePath string, ranges []aicodingsession.LineRange, at time.Time) error {
	return s.appendAILineEntries(sessionID, []aiLineEntry{{
		SessionID:  sessionID,
		File:       filePath,
		Ranges:     ranges,
		RecordedAt: at.UTC().Format(time.RFC3339Nano),
	}})
}

// MarkConsumed records that the pending ranges of the given files landed in
// commit sha, so they stop crediting the session on later commits. Files the
// session has nothing pending for are ignored, which keeps the ledger from
// growing a marker per commit per file the session never touched.
func (s *Store) MarkConsumed(sessionID string, files []string, sha string) error {
	if sha == "" || len(files) == 0 {
		return nil
	}

	return s.markConsumedAt(s.LoadAILineAttribution(sessionID).Pending, sessionID, files, sha)
}

// markConsumedAt writes the retirement markers for a pending snapshot. The
// snapshot is a parameter because it is the boundary this has to be correct
// across: an edit recorded after it was taken is not part of the commit, and
// naming each marker's edit time is what keeps that edit pending.
func (s *Store) markConsumedAt(pending map[string]time.Time, sessionID string, files []string, sha string) error {
	if len(pending) == 0 {
		return nil
	}

	entries := make([]aiLineEntry, 0, len(files))
	for _, file := range files {
		editedAt, ok := pending[file]
		if !ok {
			continue
		}
		// The marker names the edit it retires, so an edit recorded after this
		// snapshot is not swept up by a commit that never contained it.
		entries = append(entries, aiLineEntry{
			SessionID:  sessionID,
			File:       file,
			RecordedAt: editedAt.Format(time.RFC3339Nano),
			ConsumedBy: sha,
		})
	}

	return s.appendAILineEntries(sessionID, entries)
}

// appendAILineEntries appends entries to a session's attribution JSONL,
// creating the file and its directory on first write.
func (s *Store) appendAILineEntries(sessionID string, entries []aiLineEntry) error {
	if len(entries) == 0 {
		return nil
	}

	path := s.aiLinesPath(sessionID)
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return fmt.Errorf("create ai-lines dir: %w", err)
	}

	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}

	enc := json.NewEncoder(f)
	for i := range entries {
		if err := enc.Encode(entries[i]); err != nil {
			_ = f.Close()

			return err
		}
	}

	return f.Close()
}

// LoadAllAILineAttributions loads AI line attribution for all sessions that have data.
func (s *Store) LoadAllAILineAttributions() ([]*AILineAttribution, error) {
	dir := filepath.Join(s.traceDirPath(), traceDirAILines)
	entries, err := os.ReadDir(dir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}

		return nil, err
	}

	var result []*AILineAttribution
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".jsonl") {
			continue
		}
		// The filename is the sanitized ID; LoadAILineAttribution replaces it
		// with the agent-assigned one recorded in the file.
		attr := s.LoadAILineAttribution(strings.TrimSuffix(entry.Name(), ".jsonl"))
		if len(attr.Files) > 0 {
			result = append(result, attr)
		}
	}

	return result, nil
}
