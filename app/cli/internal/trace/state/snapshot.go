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
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ShellCallEnv identifies the Bash call inherited by Git commit hooks.
const ShellCallEnv = "CHAINLOOP_TRACE_SHELL_CALL"

// FileSnapshotKey identifies the edit whose pre-edit content was captured.
// ToolUseID is optional so providers without call IDs retain the prior
// session-and-path key.
type FileSnapshotKey struct {
	SessionID string
	FilePath  string
	ToolUseID string
}

// snapshotPath returns the path for a file snapshot: <dir>/chainloop-trace/snapshots/<session>/<key-hash>.
func (s *Store) snapshotPath(key FileSnapshotKey) string {
	identity := key.FilePath
	if key.ToolUseID != "" {
		identity += "\x00" + key.ToolUseID
	}
	h := sha256.Sum256([]byte(identity))
	name := hex.EncodeToString(h[:8]) // 16-char hex, enough to avoid collisions
	return filepath.Join(s.traceDirPath(), traceDirSnapshots, sanitizeID(key.SessionID), name)
}

// SaveFileSnapshot stores a file's content before an AI edit.
func (s *Store) SaveFileSnapshot(key FileSnapshotKey, content []byte) error {
	path := s.snapshotPath(key)
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return fmt.Errorf("create snapshot dir: %w", err)
	}

	return os.WriteFile(path, content, 0600)
}

// LoadFileSnapshot loads a previously stored file snapshot.
func (s *Store) LoadFileSnapshot(key FileSnapshotKey) ([]byte, error) {
	content, err := os.ReadFile(s.snapshotPath(key))
	if err == nil || key.ToolUseID == "" || !errors.Is(err, os.ErrNotExist) {
		return content, err
	}

	key.ToolUseID = ""
	return os.ReadFile(s.snapshotPath(key))
}

// DeleteFileSnapshot removes a file snapshot after it's been processed.
func (s *Store) DeleteFileSnapshot(key FileSnapshotKey) {
	_ = os.Remove(s.snapshotPath(key))
	if key.ToolUseID != "" {
		key.ToolUseID = ""
		_ = os.Remove(s.snapshotPath(key))
	}
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

// EncodeShellCallKey serializes the call identity for its shell environment.
func EncodeShellCallKey(key ShellCallKey) (string, error) {
	data, err := json.Marshal(key)
	if err != nil {
		return "", fmt.Errorf("marshal shell call key: %w", err)
	}

	return base64.RawURLEncoding.EncodeToString(data), nil
}

// DecodeShellCallKey restores a call identity from its shell environment.
func DecodeShellCallKey(encoded string) (ShellCallKey, error) {
	data, err := base64.RawURLEncoding.DecodeString(encoded)
	if err != nil {
		return ShellCallKey{}, fmt.Errorf("decode shell call key: %w", err)
	}

	var key ShellCallKey
	if err := json.Unmarshal(data, &key); err != nil {
		return ShellCallKey{}, fmt.Errorf("parse shell call key: %w", err)
	}
	if !ValidSessionID(key.SessionID) {
		return ShellCallKey{}, fmt.Errorf("invalid shell call session ID")
	}

	return key, nil
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
	return s.shellSignaturePath("shell-pre", key)
}

func (s *Store) shellPreIndexSignaturePath(key ShellCallKey) string {
	return s.shellSignaturePath("shell-index", key)
}

func (s *Store) shellSignaturePath(prefix string, key ShellCallKey) string {
	name := prefix + commitRecordExt
	switch {
	case key.ToolUseID != "":
		name = prefix + "-call-" + sanitizeID(key.ToolUseID) + commitRecordExt
	case key.AgentID != "":
		name = prefix + "-" + sanitizeID(key.AgentID) + commitRecordExt
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
	return s.saveShellSignature(s.shellPreSignaturePath(key), sig)
}

// SaveShellPreIndexSignature stores the staged index signature captured before
// an agent-run shell command.
func (s *Store) SaveShellPreIndexSignature(key ShellCallKey, sig WorktreeSignatures) error {
	return s.saveShellSignature(s.shellPreIndexSignaturePath(key), sig)
}

func (s *Store) saveShellSignature(path string, sig WorktreeSignatures) error {
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

// LoadShellPreIndexSignature loads the staged index signature captured before
// the command. It is absent when the index could not be decoded.
func (s *Store) LoadShellPreIndexSignature(key ShellCallKey) (WorktreeSignatures, error) {
	data, err := os.ReadFile(s.shellPreIndexSignaturePath(key))
	if err != nil {
		return nil, err
	}

	var sig WorktreeSignatures
	if err := json.Unmarshal(data, &sig); err != nil {
		return nil, fmt.Errorf("parse shell index signature: %w", err)
	}

	return sig, nil
}

// DeleteShellPreSignature removes the pre-command signature, and the HEAD
// commit and index signature saved with it, once processed.
func (s *Store) DeleteShellPreSignature(key ShellCallKey) {
	_ = os.Remove(s.shellPreSignaturePath(key))
	_ = os.Remove(s.shellPreIndexSignaturePath(key))
	_ = os.Remove(s.shellPreHeadsPath(key))
}

// shellPreHeadsPath returns the path storing the HEAD commit of each checkout
// before a shell command, next to the command's signature.
func (s *Store) shellPreHeadsPath(key ShellCallKey) string {
	return strings.TrimSuffix(s.shellPreSignaturePath(key), ".json") + ".heads.json"
}

// SaveShellPreHeads stores the HEAD commit of each checkout (checkout root →
// SHA) before an agent-run shell command. The signature holds hashes only, so
// the file at that commit stands in for the content before the command, also
// when the command commits its own change.
func (s *Store) SaveShellPreHeads(key ShellCallKey, heads map[string]string) error {
	path := s.shellPreHeadsPath(key)
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return fmt.Errorf("create snapshot dir: %w", err)
	}

	data, err := json.Marshal(heads)
	if err != nil {
		return fmt.Errorf("marshal shell heads: %w", err)
	}

	return os.WriteFile(path, data, 0600)
}

// LoadShellPreHeads loads the HEAD commits saved before a shell command. A
// command whose pre hook ran on an older CLI has none, and the error then
// wraps os.ErrNotExist.
func (s *Store) LoadShellPreHeads(key ShellCallKey) (map[string]string, error) {
	data, err := os.ReadFile(s.shellPreHeadsPath(key))
	if err != nil {
		return nil, err
	}

	var heads map[string]string
	if err := json.Unmarshal(data, &heads); err != nil {
		return nil, fmt.Errorf("parse shell heads: %w", err)
	}

	return heads, nil
}
