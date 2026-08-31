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
	"strings"

	"github.com/chainloop-dev/chainloop/pkg/attestation/crafter/materials/aicodingsession"
)

// AILineAttribution tracks which line ranges in which files a session modified.
type AILineAttribution struct {
	SessionID string                                 `json:"session_id"`
	Files     map[string][]aicodingsession.LineRange `json:"files"`
}

// newAILineAttribution returns an AILineAttribution with an initialized Files map.
func newAILineAttribution(sessionID string) *AILineAttribution {
	return &AILineAttribution{SessionID: sessionID, Files: make(map[string][]aicodingsession.LineRange)}
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

	dec := json.NewDecoder(f)
	for dec.More() {
		var entry aiLineEntry
		if err := dec.Decode(&entry); err != nil {
			break
		}
		if entry.SessionID != "" {
			attr.SessionID = entry.SessionID
		}
		attr.Files[entry.File] = append(attr.Files[entry.File], entry.Ranges...)
	}

	return attr
}

// RecordLineRanges appends line ranges for a file to a session's attribution JSONL.
// An empty or nil ranges slice still records the file as touched (e.g., deletion-only edits).
func (s *Store) RecordLineRanges(sessionID, filePath string, ranges []aicodingsession.LineRange) error {
	path := s.aiLinesPath(sessionID)
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return fmt.Errorf("create ai-lines dir: %w", err)
	}

	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}

	if err := json.NewEncoder(f).Encode(aiLineEntry{SessionID: sessionID, File: filePath, Ranges: ranges}); err != nil {
		_ = f.Close()
		return err
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
