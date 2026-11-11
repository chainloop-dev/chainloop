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

package copilot

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/chainloop-dev/chainloop/app/cli/internal/trace"
	"github.com/chainloop-dev/chainloop/app/cli/internal/trace/state"
)

// eventsFile is the name of the session event log in a session folder.
const eventsFile = "events.jsonl"

// sessionStateRoot returns the folder where Copilot keeps its sessions:
// $COPILOT_HOME/session-state, or ~/.copilot/session-state.
func sessionStateRoot() string {
	home := os.Getenv("COPILOT_HOME")
	if home == "" {
		userHome, err := os.UserHomeDir()
		if err != nil {
			return ""
		}
		home = filepath.Join(userHome, ".copilot")
	}
	return filepath.Join(home, "session-state")
}

// CopySessionData refreshes the private copy of the session event log. The
// copy is atomic, so a failed copy keeps the previous one. A missing log is
// not an error. Only files inside the Copilot session-state folder are read.
func (p *Provider) CopySessionData(store *state.Store, loc trace.SessionLocation) error {
	if store == nil || !state.ValidSessionID(loc.SessionID) {
		return errors.New("copy Copilot session: invalid session")
	}
	rec, err := store.LoadSessionRecord(loc.SessionID)
	if err != nil {
		// The record is an optional hint. A damaged record must not stop a
		// copy that is checked against the session-state folder.
		rec = nil
	}
	root := sessionStateRoot()
	if rec != nil && rec.CopilotStorageRoot != "" {
		root = rec.CopilotStorageRoot
	}
	if root == "" {
		return nil
	}

	source := loc.TranscriptPath
	if source != "" && !filepath.IsAbs(source) {
		return errors.New("copy Copilot session: transcript path must be absolute")
	}
	if source == "" {
		source = filepath.Join(root, loc.SessionID, eventsFile)
	}
	resolved, err := resolveInsideRoot(source, root)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}

	src, err := os.Open(resolved)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	defer src.Close()
	info, err := src.Stat()
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() {
		return errors.New("copy Copilot session: transcript is not a regular file")
	}

	// The session.start record, not the path, owns the session identity.
	scanner := bufio.NewScanner(src)
	scanner.Buffer(make([]byte, 64*1024), maxHookPayloadBytes)
	if !scanner.Scan() {
		// Copilot writes session.start first. An empty log is not yet a session.
		return nil
	}
	var header struct {
		Type string `json:"type"`
		Data struct {
			SessionID string `json:"sessionId"`
		} `json:"data"`
	}
	if json.Unmarshal(scanner.Bytes(), &header) != nil || header.Type != eventSessionStartRecord || header.Data.SessionID != loc.SessionID {
		return errors.New("copy Copilot session: session ID mismatch")
	}
	if err := saveStorageRoot(store, rec, root); err != nil {
		return fmt.Errorf("copy Copilot session: save storage root: %w", err)
	}

	if _, err := src.Seek(0, io.SeekStart); err != nil {
		return err
	}
	dir := store.RawSessionDir()
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, ".copilot-session-*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	defer tmp.Close()
	if _, err := io.Copy(tmp, src); err != nil {
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), state.RawSessionPath(dir, loc.SessionID))
}

// saveStorageRoot records the checked session-state folder, so that a push
// with another COPILOT_HOME still finds the log.
func saveStorageRoot(store *state.Store, rec *state.SessionRecord, root string) error {
	if rec == nil || rec.CopilotStorageRoot != "" {
		return nil
	}
	resolved, err := filepath.EvalSymlinks(root)
	if err != nil {
		return err
	}
	resolved, err = filepath.Abs(resolved)
	if err != nil {
		return err
	}
	// Reload, so that the writes of an overlapping hook are kept.
	current, err := store.LoadSessionRecord(rec.SessionID)
	if err != nil || current == nil || current.CopilotStorageRoot != "" {
		return nil
	}
	current.CopilotStorageRoot = resolved
	return store.SaveSessionRecord(current)
}

// errOutsideStorage is returned for a transcript path outside the Copilot
// session-state folder.
var errOutsideStorage = errors.New("copy Copilot session: transcript outside Copilot session storage")

// resolveInsideRoot returns path with its symbolic links resolved, after it
// checks that path is inside root, before and after the resolution. A path
// that does not exist returns an error that wraps os.ErrNotExist.
func resolveInsideRoot(path, root string) (string, error) {
	if !within(filepath.Clean(path), filepath.Clean(root)) {
		return "", errOutsideStorage
	}
	resolved, err := filepath.EvalSymlinks(path)
	if err != nil {
		return "", err
	}
	resolvedRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		return "", err
	}
	if !within(resolved, resolvedRoot) {
		return "", errOutsideStorage
	}
	return resolved, nil
}

func within(path, root string) bool {
	rel, err := filepath.Rel(root, path)
	return err == nil && filepath.IsLocal(rel) && rel != "."
}
