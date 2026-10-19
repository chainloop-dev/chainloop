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
	"sort"
	"strings"
	"time"

	"github.com/chainloop-dev/chainloop/pkg/attestation/crafter/materials/aicodingsession"
)

// AILineAttribution tracks which line ranges in which files a session modified.
type AILineAttribution struct {
	SessionID string                                 `json:"session_id"`
	Files     map[string][]aicodingsession.LineRange `json:"files"`
	// Pending holds each file whose recorded ranges have not been committed
	// yet, and is the only set a new commit may be matched against. Files keeps
	// every range ever recorded because push-time enrichment needs the full
	// history, but matching against it credits a finished session on every
	// later commit that happens to touch a file it once edited.
	Pending map[string]PendingEdit `json:"-"`
	// Written holds, for each file, the fingerprint of every line that the
	// session's edits wrote. Push matches the lines of a commit against it.
	Written map[string]LineCounts `json:"-"`
	// Erased holds, for each file, the fingerprint of every line that the
	// session's edits removed. Only these removals count as AI removals
	// (R-005).
	Erased map[string]LineCounts `json:"-"`
	// Open holds, for each file, the lines that the session added or removed
	// and that no commit took yet (R-006).
	Open map[string]*OpenLines `json:"-"`
	// Legacy marks each file with an edit recorded before line fingerprints
	// existed. Its attribution falls back to the recorded ranges.
	Legacy map[string]bool `json:"-"`
	// GeneratedAdded and GeneratedRemoved are the lines that all the agent
	// edits of the session added and removed, also the ones that never
	// reached a commit (R-011).
	GeneratedAdded   int `json:"-"`
	GeneratedRemoved int `json:"-"`
}

// PendingEdit is an uncommitted edit to one file.
type PendingEdit struct {
	// At is when the edit was made. Ownership of a file that two sessions both
	// have pending goes to the later edit, which rewrote the earlier one's
	// lines.
	At time.Time
	// Seq is the edit's position in its ledger, counting from one. Retiring an
	// edit is keyed off this rather than At because wall-clock times tie on a
	// host with a coarse clock, whereas an append-only ledger's positions are
	// unique and strictly increasing. Positions from different ledgers are not
	// comparable, which is why ownership still goes by At.
	Seq int
}

// newAILineAttribution returns an AILineAttribution with initialized maps.
func newAILineAttribution(sessionID string) *AILineAttribution {
	return &AILineAttribution{
		SessionID: sessionID,
		Files:     make(map[string][]aicodingsession.LineRange),
		Pending:   make(map[string]PendingEdit),
		Written:   make(map[string]LineCounts),
		Erased:    make(map[string]LineCounts),
		Open:      make(map[string]*OpenLines),
		Legacy:    make(map[string]bool),
	}
}

// open returns the open lines of a file, creating them on first use.
func (a *AILineAttribution) open(file string) *OpenLines {
	o, ok := a.Open[file]
	if !ok {
		o = &OpenLines{Added: LineCounts{}, Removed: LineCounts{}}
		a.Open[file] = o
	}

	return o
}

// countsOf returns the LineCounts of a file in m, creating them on first use.
func countsOf(m map[string]LineCounts, file string) LineCounts {
	c, ok := m[file]
	if !ok {
		c = LineCounts{}
		m[file] = c
	}

	return c
}

// settle closes the open lines of a file that are in set.
func (a *AILineAttribution) settle(file string, set *LineSet) {
	o, ok := a.Open[file]
	if !ok || set == nil {
		return
	}
	for _, fp := range set.Added {
		o.Added.Take(fp)
	}
	for _, fp := range set.Removed {
		o.Removed.Take(fp)
	}
}

// applyEdit adds the lines of a fingerprinted edit to the session's history
// and to its open lines. A removal of an open line of the session closes that
// line: it never reached a commit, so no commit can remove it. In the same
// way, writing back an open removal closes it: the line is as it was
// committed, so no commit can add it.
func (a *AILineAttribution) applyEdit(entry *aiLineEntry) {
	lines := entry.Lines
	countsOf(a.Written, entry.File).Add(lines.Added...)
	countsOf(a.Erased, entry.File).Add(lines.Removed...)

	o := a.open(entry.File)
	for _, fp := range lines.Removed {
		if !o.Added.Take(fp) {
			o.Removed.Add(fp)
		}
	}
	for _, fp := range lines.Added {
		if !o.Removed.Take(fp) {
			o.Added.Add(fp)
		}
	}

	a.GeneratedAdded += entry.LinesAdded
	a.GeneratedRemoved += entry.LinesRemoved
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
	// RecordedAt is the RFC3339 timestamp of the edit, with sub-second
	// precision so two sessions editing one file can be ordered. Entries
	// written before this field existed have none, and are never treated as
	// pending — those ledgers predate consumption tracking, so their files
	// would otherwise keep crediting their session forever.
	RecordedAt string `json:"recorded_at,omitempty"`
	// ConsumedBy is the SHA of the commit that committed the file's pending
	// ranges. Set only on marker entries appended by MarkCommitted, which
	// clear the file from Pending instead of contributing ranges.
	ConsumedBy string `json:"consumed_by,omitempty"`
	// ConsumedSeq is the ledger position of the edit a marker retires. It
	// bounds the marker to the snapshot MarkCommitted read, so an edit appended
	// afterwards — which the commit cannot have contained — keeps its pending
	// state. Zero on a marker written before this field existed, which retires
	// the file outright.
	ConsumedSeq int `json:"consumed_seq,omitempty"`
	// Lines holds the fingerprints of the lines that an edit added and
	// removed. Nil on an edit recorded before line fingerprints existed, and
	// on markers.
	Lines *LineSet `json:"lines,omitempty"`
	// LinesAdded and LinesRemoved are the lines that an edit added and
	// removed, as the line diff counts them (R-011).
	LinesAdded   int `json:"lines_added,omitempty"`
	LinesRemoved int `json:"lines_removed,omitempty"`
	// Settled holds the fingerprints of open lines that are closed: on a
	// marker, the lines that the commit took or that are gone from the file;
	// on an entry without ConsumedBy, the lines that changed outside the
	// session's edits (R-002).
	Settled *LineSet `json:"settled,omitempty"`
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
	//
	// A legacy edit (no fingerprints) is pending until a marker retires the
	// whole file. A fingerprinted file is pending while it has open lines,
	// with the time of its last edit.
	legacyPending := make(map[string]PendingEdit)
	lastEdit := make(map[string]PendingEdit)
	seq := 0
	dec := json.NewDecoder(f)
	for dec.More() {
		var entry aiLineEntry
		if err := dec.Decode(&entry); err != nil {
			break
		}
		seq++
		if entry.SessionID != "" {
			attr.SessionID = entry.SessionID
		}

		if entry.ConsumedBy != "" {
			// Retire only the edit the commit actually contained. A concurrent
			// RecordLineRanges can land between MarkCommitted reading the
			// pending set and appending its markers, leaving a marker behind an
			// edit it never saw; that edit sits past the marker's snapshot, so
			// it keeps its pending state.
			cur, pending := legacyPending[entry.File]
			if pending && (entry.ConsumedSeq == 0 || cur.Seq <= entry.ConsumedSeq) {
				delete(legacyPending, entry.File)
			}
			attr.settle(entry.File, entry.Settled)

			continue
		}

		if entry.Settled != nil {
			attr.settle(entry.File, entry.Settled)

			continue
		}

		attr.Files[entry.File] = append(attr.Files[entry.File], entry.Ranges...)
		if entry.Lines != nil {
			attr.applyEdit(&entry)
		} else {
			attr.Legacy[entry.File] = true
		}

		// An entry with no parseable timestamp is never pending: it either
		// predates consumption tracking, in which case its session is long
		// gone, or it cannot be ordered against a competing session's edit.
		recordedAt, err := time.Parse(time.RFC3339, entry.RecordedAt)
		if err != nil {
			continue
		}
		edit := PendingEdit{At: recordedAt, Seq: seq}
		lastEdit[entry.File] = edit
		if entry.Lines == nil {
			legacyPending[entry.File] = edit
		}
	}

	for file, edit := range lastEdit {
		_, legacy := legacyPending[file]
		if legacy || !attr.Open[file].isEmpty() {
			attr.Pending[file] = edit
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

// RecordEdit appends one fingerprinted agent edit of a file to a session's
// ledger.
func (s *Store) RecordEdit(sessionID, filePath string, edit EditRecord) error {
	return s.RecordEditAt(sessionID, filePath, edit, time.Now().UTC())
}

// RecordEditAt records an edit made at a given time. See RecordLineRangesAt
// for why tests set the time.
func (s *Store) RecordEditAt(sessionID, filePath string, edit EditRecord, at time.Time) error {
	lines := edit.Lines

	return s.appendAILineEntries(sessionID, []aiLineEntry{{
		SessionID:    sessionID,
		File:         filePath,
		Ranges:       edit.Ranges,
		RecordedAt:   at.UTC().Format(time.RFC3339Nano),
		Lines:        &lines,
		LinesAdded:   edit.AddedCount,
		LinesRemoved: edit.RemovedCount,
	}})
}

// SettleLines closes open lines of a file that changed outside the session's
// own edits (R-002): a human, or another session, rewrote or removed them. An
// empty set records nothing.
func (s *Store) SettleLines(sessionID, filePath string, lines LineSet) error {
	if lines.IsEmpty() {
		return nil
	}

	return s.appendAILineEntries(sessionID, []aiLineEntry{{
		SessionID:  sessionID,
		File:       filePath,
		RecordedAt: time.Now().UTC().Format(time.RFC3339Nano),
		Settled:    &lines,
	}})
}

// MarkCommitted records that commit sha landed pending work of a session, so
// that work stops crediting the session on later commits. settled holds, for
// each file of the commit, the open lines that the commit took or that are
// gone from the file; the other open lines stay pending (R-006). A legacy file
// of the commit is retired as a whole. Files the session has nothing pending
// for are ignored, which keeps the ledger from growing a marker per commit per
// file the session never touched.
func (s *Store) MarkCommitted(sessionID, sha string, settled map[string]LineSet) error {
	if sha == "" || len(settled) == 0 {
		return nil
	}

	return s.markCommittedAt(s.LoadAILineAttribution(sessionID).Pending, sessionID, sha, settled)
}

// markCommittedAt writes the markers for a pending snapshot. The snapshot is a
// parameter because it is the boundary this has to be correct across: an edit
// recorded after it was taken is not part of the commit, and pinning each
// marker to the position of the edit it retires is what keeps that edit
// pending.
func (s *Store) markCommittedAt(pending map[string]PendingEdit, sessionID, sha string, settled map[string]LineSet) error {
	if len(pending) == 0 {
		return nil
	}

	files := make([]string, 0, len(settled))
	for file := range settled {
		files = append(files, file)
	}
	sort.Strings(files)

	entries := make([]aiLineEntry, 0, len(files))
	for _, file := range files {
		edit, ok := pending[file]
		if !ok {
			continue
		}
		set := settled[file]
		entries = append(entries, aiLineEntry{
			SessionID:   sessionID,
			File:        file,
			RecordedAt:  edit.At.Format(time.RFC3339Nano),
			ConsumedBy:  sha,
			ConsumedSeq: edit.Seq,
			Settled:     &set,
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
