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

package action

import (
	"fmt"
	"reflect"
	"slices"
	"sort"

	"github.com/chainloop-dev/chainloop/app/cli/internal/trace/attribution"
	tracegit "github.com/chainloop-dev/chainloop/app/cli/internal/trace/git"
	"github.com/chainloop-dev/chainloop/app/cli/internal/trace/state"
	"github.com/rs/zerolog"
)

// scanReplacements finds the sessions whose lines the commits not pushed yet
// removed or replaced, so that the push sends their evidence again with the
// new net numbers (spec issue-3639 R-004). records are the commit records of
// the branch. A commit in no session that replaced lines of a session joins
// that session (R-010): its record gets the lines that its replacing hunks
// added, and is saved.
//
// A join can depend on another one: a human commit that replaces the lines of
// an earlier human commit joins the sessions that the earlier commit joined.
// So the commits are read oldest first, and read again until no join changes.
//
// Only files with ledger entries are blamed, because only those can hold the
// lines of a session. A file that a commit renamed is blamed under its new
// path too (R-007). It also returns the commits that it could not read, which
// must stay untracked so that the next push reads them again.
func scanReplacements(client tracegit.Client, store *state.Store, repoRoot string, records []*state.CommitRecord, ledgers map[string]*state.AILineAttribution, log zerolog.Logger) (affected map[string]struct{}, failed map[string]bool) {
	bySHA := make(map[string]*state.CommitRecord, len(records))
	var untracked []*state.CommitRecord
	for _, r := range records {
		bySHA[r.SHA] = r
		if !r.Tracked {
			untracked = append(untracked, r)
		}
	}
	sort.SliceStable(untracked, func(i, j int) bool {
		return untracked[i].Timestamp < untracked[j].Timestamp
	})

	ledgerFiles := make(map[string]bool)
	for _, l := range ledgers {
		for path := range l.Files {
			ledgerFiles[path] = true
		}
	}

	scan := &replacementScan{
		client:      client,
		repoRoot:    repoRoot,
		records:     records,
		bySHA:       bySHA,
		ledgerFiles: ledgerFiles,
		lines:       make(map[string][]tracegit.FileLines),
		blames:      make(map[[2]string][]tracegit.BlameLine),
		log:         log,
	}

	affected = make(map[string]struct{})
	failed = make(map[string]bool)
	changed := make(map[string]*state.CommitRecord)
	// Each pass can add one link to a chain of joins, so the number of
	// commits bounds the passes.
	for range len(untracked) + 1 {
		again := false
		for _, rec := range untracked {
			if failed[rec.SHA] {
				continue
			}

			joined, err := scan.replacedSessions(rec, affected)
			if err != nil {
				log.Debug().Err(err).Str("sha", rec.SHA).Msg("could not read the lines of a commit; the next push reads it again")
				failed[rec.SHA] = true
				continue
			}

			// Only a commit in no session joins one. It is recomputed on each
			// push until the commit is tracked, so that a failed push leaves
			// no stale join behind.
			if len(rec.SessionIDs) > 0 || reflect.DeepEqual(joined, rec.Joined) {
				continue
			}
			rec.Joined = joined
			changed[rec.SHA] = rec
			again = true
		}
		if !again {
			break
		}
	}

	for _, rec := range changed {
		if err := store.SaveCommitRecord(rec); err != nil {
			log.Debug().Err(err).Str("sha", rec.SHA).Msg("could not save the sessions that a commit joined")
		}
	}

	return affected, failed
}

// replacementScan reads the lines of commits and the blame of their parents,
// each one time for all the passes of scanReplacements.
type replacementScan struct {
	client   tracegit.Client
	repoRoot string
	records  []*state.CommitRecord
	bySHA    map[string]*state.CommitRecord
	// ledgerFiles holds the paths that can hold lines of a session: the
	// paths of the ledgers, and the paths that a commit renamed them to.
	ledgerFiles map[string]bool
	// pushedRenames is set once ledgerFiles holds the paths that pushed
	// commits renamed the ledger files to. renamesErr is set when the renames
	// could not be read.
	pushedRenames bool
	renamesErr    error
	lines         map[string][]tracegit.FileLines
	blames        map[[2]string][]tracegit.BlameLine
	log           zerolog.Logger
}

// holdsSessionLines reports whether a file of the parent can hold lines of a
// session. A rename in a commit that a push already read is not seen in the
// commits that the scan reads, so on the first file that is not a ledger
// file, the paths at HEAD of the files of the pushed session commits are
// added. An error means the renames could not be read, so the commit must be
// read again on the next push.
func (s *replacementScan) holdsSessionLines(path string) (bool, error) {
	if s.ledgerFiles[path] {
		return true, nil
	}
	if !s.pushedRenames {
		s.pushedRenames = true
		s.renamesErr = s.addPushedRenames()
	}

	return s.ledgerFiles[path], s.renamesErr
}

// addPushedRenames adds to ledgerFiles the paths at HEAD of the ledger files
// that the pushed session commits changed.
func (s *replacementScan) addPushedRenames() error {
	for _, r := range s.records {
		if !r.Tracked || len(r.AllSessionIDs()) == 0 {
			continue
		}
		renames, err := s.client.Renames(s.repoRoot, r.SHA, "HEAD")
		if err != nil {
			return fmt.Errorf("read the renames since %s: %w", r.SHA, err)
		}
		for from, to := range renames {
			if s.ledgerFiles[from] {
				s.ledgerFiles[to] = true
			}
		}
	}

	return nil
}

// replacedSessions adds to affected each session that owns a line that the
// commit removed. It returns, for a commit in no session, the sessions that
// the commit joins, with the fingerprints of the lines that its replacing
// hunks added, per file. An error means the commit could not be read.
func (s *replacementScan) replacedSessions(rec *state.CommitRecord, affected map[string]struct{}) (map[string]map[string][]string, error) {
	lines, ok := s.lines[rec.SHA]
	if !ok {
		var err error
		if lines, err = s.client.CommitLines(s.repoRoot, rec.SHA); err != nil {
			return nil, err
		}
		s.lines[rec.SHA] = lines
	}

	var joined map[string]map[string][]string
	for _, fl := range lines {
		// The parent holds the file under its old path.
		parentPath := fl.Path
		if fl.OldPath != "" {
			parentPath = fl.OldPath
			// The commits are read oldest first, so a later commit that
			// changes the file under its new path is blamed too.
			if s.ledgerFiles[fl.OldPath] {
				s.ledgerFiles[fl.Path] = true
			}
		}

		// A root commit removes nothing, so its parent is never blamed.
		if len(fl.Removed) == 0 {
			continue
		}
		holds, err := s.holdsSessionLines(parentPath)
		if err != nil {
			return nil, err
		}
		if !holds {
			continue
		}

		// The removed lines are lines of the parent, so the parent's blame
		// names the commit that last wrote each one.
		key := [2]string{rec.SHA, parentPath}
		blame, ok := s.blames[key]
		if !ok {
			var err error
			if blame, err = s.client.Blame(s.repoRoot, rec.SHA+"^", parentPath); err != nil {
				return nil, err
			}
			s.blames[key] = blame
		}

		for _, h := range fl.Hunks {
			for _, sid := range hunkOwners(h, blame, s.bySHA, rec.SHA) {
				affected[sid] = struct{}{}
				if len(rec.SessionIDs) > 0 {
					continue
				}

				if joined == nil {
					joined = make(map[string]map[string][]string)
				}
				if joined[sid] == nil {
					joined[sid] = make(map[string][]string)
				}
				for _, l := range h.Added {
					joined[sid][fl.Path] = append(joined[sid][fl.Path], attribution.Fingerprint(l))
				}
			}
		}
	}

	return joined, nil
}

// hunkOwners returns the sessions that own a line that the hunk removed, by
// the record of the commit that last wrote the line. A commit owns its line
// for each of its sessions, and for each session it joined when its
// replacing hunks added that line. The join holds the line under the path of
// the file at that commit, which a later commit can have renamed, so the
// line is looked up in each file of the join.
func hunkOwners(h tracegit.Hunk, blame []tracegit.BlameLine, bySHA map[string]*state.CommitRecord, self string) []string {
	var owners []string
	for i, text := range h.Removed {
		idx := h.OldStart - 1 + i
		if idx < 0 || idx >= len(blame) {
			continue
		}
		owner := bySHA[blame[idx].SHA]
		if owner == nil || owner.SHA == self {
			continue
		}

		for _, sid := range owner.SessionIDs {
			if !slices.Contains(owners, sid) {
				owners = append(owners, sid)
			}
		}
		fp := attribution.Fingerprint(text)
		for sid, files := range owner.Joined {
			if slices.Contains(owners, sid) {
				continue
			}
			for _, fps := range files {
				if slices.Contains(fps, fp) {
					owners = append(owners, sid)
					break
				}
			}
		}
	}
	slices.Sort(owners)

	return owners
}
