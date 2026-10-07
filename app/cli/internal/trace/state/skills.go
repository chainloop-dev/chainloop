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
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

const (
	// traceDirSkills holds, for each session, a copy of each skill folder
	// that the session used. A copy lives for the session, not for one push.
	traceDirSkills = "skills"
	// skillScanName records, inside a session's skills directory, how far
	// the hooks have read each transcript of the session.
	skillScanName = "scan.json"
)

// SkillCopyDir returns the directory that holds the copy of the skill folder
// dir for a session. The name is a digest of the folder path, so the path
// itself is not in the name, and two folders never share a copy.
func (s *Store) SkillCopyDir(sessionID, dir string) string {
	sum := sha256.Sum256([]byte(filepath.Clean(dir)))

	return filepath.Join(s.skillsDir(sessionID), hex.EncodeToString(sum[:16]))
}

func (s *Store) skillsDir(sessionID string) string {
	return filepath.Join(s.traceDirPath(), traceDirSkills, SanitizeID(sessionID))
}

// SkillScanOffsets returns how many bytes of each transcript of a session the
// hooks have read for skill uses, by a key that the provider chooses. A
// session that no hook has scanned has none.
func (s *Store) SkillScanOffsets(sessionID string) (map[string]int64, error) {
	data, err := os.ReadFile(filepath.Join(s.skillsDir(sessionID), skillScanName))
	if errors.Is(err, os.ErrNotExist) {
		return map[string]int64{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read skill scan offsets: %w", err)
	}

	offsets := map[string]int64{}
	if err := json.Unmarshal(data, &offsets); err != nil {
		return nil, fmt.Errorf("parse skill scan offsets: %w", err)
	}

	return offsets, nil
}

// SetSkillScanOffsets replaces the offsets of a session.
func (s *Store) SetSkillScanOffsets(sessionID string, offsets map[string]int64) error {
	if err := writeJSONAtomic(filepath.Join(s.skillsDir(sessionID), skillScanName), offsets); err != nil {
		return fmt.Errorf("write skill scan offsets: %w", err)
	}

	return nil
}

// gcSkillCopies drops the skill copies of each session that has no record
// any more. Nothing else drops them: a push after the session end still needs
// them, and the session end never runs when the agent is killed.
func (s *Store) gcSkillCopies() error {
	dir := filepath.Join(s.traceDirPath(), traceDirSkills)
	entries, err := os.ReadDir(dir)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("read %s dir: %w", traceDirSkills, err)
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		record := filepath.Join(s.traceDirPath(), traceDirSessions, entry.Name()+sessionRecordExt)
		if _, err := os.Stat(record); errors.Is(err, os.ErrNotExist) {
			_ = os.RemoveAll(filepath.Join(dir, entry.Name()))
		}
	}

	return nil
}
