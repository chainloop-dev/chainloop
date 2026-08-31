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
