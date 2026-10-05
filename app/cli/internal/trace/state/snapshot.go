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

// ShellCallKey identifies the shell command that a pre-command signature
// belongs to, so the post-command hook of that command finds it.
type ShellCallKey struct {
	SessionID string
	// AgentID is empty for the main agent. Subagents share their parent's
	// session ID, so this keeps their signatures apart.
	AgentID string
	// ToolUseID is the agent's identifier for the tool call. Empty when the
	// agent does not report one. Then the agent has one slot, and its
	// overlapping commands overwrite each other's signature.
	ToolUseID string
}

// shellPreSignaturePath returns the path storing the pre-command working-tree
// signature of a shell command, under
// <dir>/chainloop-trace/snapshots/<session>/:
//   - shell-pre-call-<tool use>.json when the call has an ID;
//   - shell-pre.json for the main agent otherwise;
//   - shell-pre-<agent>.json for a subagent otherwise.
//
// A tool use ID is unique within a session, so it needs no agent qualifier.
func (s *Store) shellPreSignaturePath(key ShellCallKey) string {
	name := "shell-pre.json"
	switch {
	case key.ToolUseID != "":
		name = "shell-pre-call-" + sanitizeID(key.ToolUseID) + ".json"
	case key.AgentID != "":
		name = "shell-pre-" + sanitizeID(key.AgentID) + ".json"
	}

	return filepath.Join(s.traceDirPath(), traceDirSnapshots, sanitizeID(key.SessionID), name)
}

// WorktreeSignatures holds the working-tree signatures that a shell command
// is diffed against: checkout root → (repo-relative path → content hash). A
// command can change files in each checkout that its session edits.
type WorktreeSignatures map[string]map[string]string

// SaveShellPreSignature stores the working-tree signatures captured before an
// agent-run shell command, so the post-command hook can diff against them.
func (s *Store) SaveShellPreSignature(key ShellCallKey, sig WorktreeSignatures) error {
	path := s.shellPreSignaturePath(key)
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return fmt.Errorf("create snapshot dir: %w", err)
	}

	data, err := json.Marshal(sig)
	if err != nil {
		return fmt.Errorf("marshal shell signature: %w", err)
	}

	return os.WriteFile(path, data, 0600)
}

// LoadShellPreSignature loads the pre-command working-tree signatures of a
// shell command. A file in the earlier single-checkout format (written by an
// older CLI just before an upgrade) does not parse, and the caller skips it.
func (s *Store) LoadShellPreSignature(key ShellCallKey) (WorktreeSignatures, error) {
	data, err := os.ReadFile(s.shellPreSignaturePath(key))
	if err != nil {
		return nil, err
	}

	var sig WorktreeSignatures
	if err := json.Unmarshal(data, &sig); err != nil {
		return nil, fmt.Errorf("parse shell signature: %w", err)
	}

	return sig, nil
}

// DeleteShellPreSignature removes the pre-command signature once processed.
func (s *Store) DeleteShellPreSignature(key ShellCallKey) {
	_ = os.Remove(s.shellPreSignaturePath(key))
}
