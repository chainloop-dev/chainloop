package state

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// snapshotPath returns the path for a file snapshot: <dir>/chainloop-trace/snapshots/<session>/<path-hash>
func (s *Store) snapshotPath(sessionID, filePath string) string {
	h := sha256.Sum256([]byte(filePath))
	name := hex.EncodeToString(h[:8]) // 16-char hex, enough to avoid collisions
	return filepath.Join(s.traceDirPath(), traceDirSnapshots, sanitizeID(sessionID), name)
}

// SaveFileSnapshot stores a file's content before an AI edit.
func (s *Store) SaveFileSnapshot(sessionID, filePath string, content []byte) error {
	path := s.snapshotPath(sessionID, filePath)
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return fmt.Errorf("create snapshot dir: %w", err)
	}

	return os.WriteFile(path, content, 0600)
}

// LoadFileSnapshot loads a previously stored file snapshot.
func (s *Store) LoadFileSnapshot(sessionID, filePath string) ([]byte, error) {
	return os.ReadFile(s.snapshotPath(sessionID, filePath))
}

// DeleteFileSnapshot removes a file snapshot after it's been processed.
func (s *Store) DeleteFileSnapshot(sessionID, filePath string) {
	path := s.snapshotPath(sessionID, filePath)
	_ = os.Remove(path)
}

// shellPreSignaturePath returns the path storing the pre-command working-tree
// signature for a session: <dir>/chainloop-trace/snapshots/<session>/shell-pre.json
func (s *Store) shellPreSignaturePath(sessionID string) string {
	return filepath.Join(s.traceDirPath(), traceDirSnapshots, sanitizeID(sessionID), "shell-pre.json")
}

// SaveShellPreSignature stores the working-tree signature captured before an
// agent-run shell command, so the post-command hook can diff against it. A
// single slot per session is used; concurrent shell calls in one turn overwrite
// it (see the parallel-shell limitation).
func (s *Store) SaveShellPreSignature(sessionID string, sig map[string]string) error {
	path := s.shellPreSignaturePath(sessionID)
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return fmt.Errorf("create snapshot dir: %w", err)
	}

	data, err := json.Marshal(sig)
	if err != nil {
		return fmt.Errorf("marshal shell signature: %w", err)
	}

	return os.WriteFile(path, data, 0600)
}

// LoadShellPreSignature loads the pre-command working-tree signature for a session.
func (s *Store) LoadShellPreSignature(sessionID string) (map[string]string, error) {
	data, err := os.ReadFile(s.shellPreSignaturePath(sessionID))
	if err != nil {
		return nil, err
	}

	var sig map[string]string
	if err := json.Unmarshal(data, &sig); err != nil {
		return nil, fmt.Errorf("parse shell signature: %w", err)
	}

	return sig, nil
}

// DeleteShellPreSignature removes the pre-command signature once processed.
func (s *Store) DeleteShellPreSignature(sessionID string) {
	_ = os.Remove(s.shellPreSignaturePath(sessionID))
}
