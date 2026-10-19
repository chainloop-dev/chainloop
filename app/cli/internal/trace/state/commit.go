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
	"os"
	"path/filepath"
	"strings"
)

// CommitRecord holds metadata about a commit captured by the post-commit hook.
type CommitRecord struct {
	SHA        string   `json:"sha"`
	Message    string   `json:"message"`
	SessionIDs []string `json:"session_ids,omitempty"`
	Timestamp  string   `json:"timestamp"`
	// Tracked marks the record as already attested by a previous successful
	// trace push. The pre-push hook uses it to skip pushing a redundant
	// attestation when no new AI-assisted commits have been recorded since
	// the last push (e.g. a bare `git push` with no new commits, or a
	// `git tag && git push --tags` over already-pushed commits).
	Tracked bool `json:"tracked,omitempty"`
	// Joined is set on a commit in no session that replaced lines of a
	// session, and so joined that session (spec issue-3639 R-010). It maps
	// each joined session ID to the files of the commit, and each file to the
	// fingerprints of the lines that the replacing hunks added. The push
	// fills it in.
	Joined map[string]map[string][]string `json:"joined,omitempty"`
}

// AllSessionIDs returns the sessions of the commit and the sessions that it
// joined, without duplicates, in no fixed order.
func (r *CommitRecord) AllSessionIDs() []string {
	out := make([]string, 0, len(r.SessionIDs)+len(r.Joined))
	seen := make(map[string]struct{}, cap(out))
	for _, id := range r.SessionIDs {
		if _, dup := seen[id]; !dup {
			seen[id] = struct{}{}
			out = append(out, id)
		}
	}
	for id := range r.Joined {
		if _, dup := seen[id]; !dup {
			seen[id] = struct{}{}
			out = append(out, id)
		}
	}

	return out
}

// SaveCommitRecord writes a commit record to <dir>/chainloop-trace/commits/<sha>.json.
func (s *Store) SaveCommitRecord(rec *CommitRecord) error {
	dir := filepath.Join(s.traceDirPath(), traceDirCommits)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(rec, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(filepath.Join(dir, rec.SHA+".json"), data, 0600)
}

// loadCommitRecord reads a single CommitRecord file. Used by the GC pass
// where we only need to inspect each record's metadata, not load every record
// into memory at once.
func loadCommitRecord(path string) (*CommitRecord, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var rec CommitRecord
	if err := json.Unmarshal(data, &rec); err != nil {
		return nil, err
	}

	return &rec, nil
}

// LoadAllCommitRecords reads all commit records from <dir>/chainloop-trace/commits/.
func (s *Store) LoadAllCommitRecords() ([]*CommitRecord, error) {
	dir := filepath.Join(s.traceDirPath(), traceDirCommits)
	entries, err := os.ReadDir(dir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}

	var records []*CommitRecord
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		rec, err := loadCommitRecord(filepath.Join(dir, entry.Name()))
		if err != nil {
			continue
		}
		records = append(records, rec)
	}

	return records, nil
}
