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
	"reflect"
	"slices"

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
// Only files with ledger entries are blamed, because only those can hold the
// lines of a session. A commit or a file that cannot be read is skipped.
func scanReplacements(client tracegit.Client, store *state.Store, repoRoot string, records []*state.CommitRecord, ledgers map[string]*state.AILineAttribution, log zerolog.Logger) map[string]struct{} {
	bySHA := make(map[string]*state.CommitRecord, len(records))
	for _, r := range records {
		bySHA[r.SHA] = r
	}

	ledgerFiles := make(map[string]bool)
	for _, l := range ledgers {
		for path := range l.Files {
			ledgerFiles[path] = true
		}
	}

	affected := make(map[string]struct{})
	for _, rec := range records {
		if rec.Tracked {
			continue
		}

		joined, err := replacedSessions(client, repoRoot, rec, bySHA, ledgerFiles, affected, log)
		if err != nil {
			log.Debug().Err(err).Str("sha", rec.SHA).Msg("could not read the lines of a commit; it replaces no session lines")
			continue
		}

		// Only a commit in no session joins one. Recompute it on each push
		// until the commit is tracked, so that a failed push leaves no stale
		// join behind.
		if len(rec.SessionIDs) > 0 || reflect.DeepEqual(joined, rec.Joined) {
			continue
		}
		rec.Joined = joined
		if err := store.SaveCommitRecord(rec); err != nil {
			log.Debug().Err(err).Str("sha", rec.SHA).Msg("could not save the sessions that a commit joined")
		}
	}

	return affected
}

// replacedSessions adds to affected each session that owns a line that the
// commit removed. It returns, for a commit in no session, the sessions that
// the commit joins, with the fingerprints of the lines that its replacing
// hunks added, per file.
func replacedSessions(client tracegit.Client, repoRoot string, rec *state.CommitRecord, bySHA map[string]*state.CommitRecord, ledgerFiles map[string]bool, affected map[string]struct{}, log zerolog.Logger) (map[string]map[string][]string, error) {
	lines, err := client.CommitLines(repoRoot, rec.SHA)
	if err != nil {
		return nil, err
	}

	var joined map[string]map[string][]string
	for _, fl := range lines {
		if len(fl.Removed) == 0 || !ledgerFiles[fl.Path] {
			continue
		}

		// The removed lines are lines of the parent, so the parent's blame
		// names the commit that last wrote each one. A root commit has no
		// parent, and removes nothing.
		blame, err := client.Blame(repoRoot, rec.SHA+"^", fl.Path)
		if err != nil {
			log.Debug().Err(err).Str("sha", rec.SHA).Str("file", fl.Path).Msg("could not blame the parent of a commit")
			continue
		}

		for _, h := range fl.Hunks {
			owners := hunkOwners(h, blame, bySHA, rec.SHA, fl.Path)
			for _, sid := range owners {
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
// replacing hunks added that line.
func hunkOwners(h tracegit.Hunk, blame []tracegit.BlameLine, bySHA map[string]*state.CommitRecord, self, path string) []string {
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
			if slices.Contains(files[path], fp) && !slices.Contains(owners, sid) {
				owners = append(owners, sid)
			}
		}
	}
	slices.Sort(owners)

	return owners
}
