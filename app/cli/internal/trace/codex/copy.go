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

package codex

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/chainloop-dev/chainloop/app/cli/internal/trace"
	"github.com/chainloop-dev/chainloop/app/cli/internal/trace/state"
)

func codexHome() string {
	if home := os.Getenv("CODEX_HOME"); home != "" {
		return home
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".codex")
}

// CopySessionData refreshes a private transcript copy atomically. A missing
// transcript leaves the previous copy intact. Only Codex session storage is read.
func (p *Provider) CopySessionData(store *state.Store, loc trace.SessionLocation) error {
	if store == nil || !state.ValidSessionID(loc.SessionID) {
		return errors.New("copy Codex session: invalid session")
	}
	rec, err := store.LoadSessionRecord(loc.SessionID)
	if err != nil {
		return fmt.Errorf("copy Codex session: load session record: %w", err)
	}
	home := codexHome()
	if rec != nil && rec.CodexStorageRoot != "" {
		home = rec.CodexStorageRoot
	}
	if home == "" {
		return nil
	}
	roots := []string{filepath.Join(home, "sessions"), filepath.Join(home, "archived_sessions")}
	source := loc.TranscriptPath
	if source != "" {
		if !filepath.IsAbs(source) {
			return errors.New("copy Codex session: transcript path must be absolute")
		}
		if !insideStorage(source, roots) {
			return errors.New("copy Codex session: transcript outside Codex storage")
		}
		if _, err := os.Stat(source); errors.Is(err, os.ErrNotExist) {
			source = ""
		}
	}
	if source == "" {
		var err error
		source, err = findRollout(roots, loc.SessionID)
		if err != nil {
			return err
		}
	}

	if source == "" {
		return nil
	}
	if !insideStorage(source, roots) {
		return errors.New("copy Codex session: transcript outside Codex storage")
	}
	src, err := os.Open(source)
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
		return errors.New("copy Codex session: transcript is not a regular file")
	}
	// The rollout header, rather than the file name, owns the session identity.
	scanner := bufio.NewScanner(src)
	scanner.Buffer(make([]byte, 4096), maxHookPayloadBytes)
	if !scanner.Scan() {
		return errors.New("copy Codex session: missing rollout header")
	}
	var header struct {
		Type    string `json:"type"`
		Payload struct {
			ID string `json:"id"`
		} `json:"payload"`
	}
	if json.Unmarshal(scanner.Bytes(), &header) != nil || header.Type != "session_meta" || header.Payload.ID != loc.SessionID {
		return errors.New("copy Codex session: rollout session ID mismatch")
	}
	// Persist only after validating both storage containment and session identity.
	// Do not infer a trusted root from the hook's transcript path.
	if err := saveStorageRoot(store, rec, home); err != nil {
		return fmt.Errorf("copy Codex session: save storage root: %w", err)
	}
	if _, err := src.Seek(0, io.SeekStart); err != nil {
		return err
	}
	dir := store.RawSessionDir()
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, ".codex-session-*")
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

func saveStorageRoot(store *state.Store, rec *state.SessionRecord, home string) error {
	if rec == nil || rec.CodexStorageRoot != "" {
		return nil
	}
	root, err := filepath.EvalSymlinks(home)
	if err != nil {
		return err
	}
	root, err = filepath.Abs(root)
	if err != nil {
		return err
	}
	rec.CodexStorageRoot = root
	return store.SaveSessionRecord(rec)
}

// Resolve symlinks as well as lexical traversal before opening a hook path.
func insideStorage(path string, roots []string) bool {
	resolved, err := filepath.EvalSymlinks(path)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return false
	}
	if errors.Is(err, os.ErrNotExist) {
		resolved = filepath.Clean(path)
	}
	for _, root := range roots {
		resolvedRoot, err := filepath.EvalSymlinks(root)
		if err != nil {
			resolvedRoot = filepath.Clean(root)
		}
		rel, err := filepath.Rel(resolvedRoot, resolved)
		if err == nil && filepath.IsLocal(rel) && rel != "." {
			return true
		}
	}
	return false
}

func findRollout(roots []string, sessionID string) (string, error) {
	source := ""
	for _, root := range roots {
		err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
			if err != nil {
				if errors.Is(err, os.ErrNotExist) {
					return nil
				}
				return err
			}
			if entry.Type().IsRegular() && strings.HasSuffix(entry.Name(), "-"+sessionID+".jsonl") {
				source = path
				return fs.SkipAll
			}
			return nil
		})
		if err != nil {
			return "", fmt.Errorf("find Codex session: %w", err)
		}
		if source != "" {
			break
		}
	}
	return source, nil
}
