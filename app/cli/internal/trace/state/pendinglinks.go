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
	"fmt"
	"os"
	"path/filepath"
	"time"
)

const (
	// pendingLinksFile holds session links produced by a successful trace
	// push, waiting for an agent hook to show them to the user. It lives
	// directly in the trace directory, which WipeTraceDir preserves.
	pendingLinksFile = "pending-links.json"

	// pendingLinksTTL bounds how long a recorded link stays announceable.
	// A push run by the user in their own terminal is never consumed by an
	// agent hook, and without an expiry the next agent command, possibly in
	// a later session, would announce a link the user has long since seen.
	pendingLinksTTL = 10 * time.Minute
)

// pendingLinks is the on-disk record. SavedAt is RFC3339 UTC, matching the
// timestamp format used by the other records in this package.
type pendingLinks struct {
	Links   []string `json:"links"`
	SavedAt string   `json:"saved_at"`
}

// SavePendingLinks records session links for an agent hook to surface to the
// user. An empty list writes nothing: there is nothing to announce, and a
// leftover empty record would only have to be cleaned up later.
func (s *Store) SavePendingLinks(links []string) error {
	if len(links) == 0 {
		return nil
	}

	data, err := json.Marshal(pendingLinks{Links: links, SavedAt: NowTimestamp()})
	if err != nil {
		return fmt.Errorf("encode pending links: %w", err)
	}

	base := s.traceDirPath()
	if err := os.MkdirAll(base, 0o755); err != nil {
		return fmt.Errorf("create trace directory: %w", err)
	}

	return os.WriteFile(filepath.Join(base, pendingLinksFile), data, 0o600)
}

// PendingLinks returns the recorded session links without consuming them, so
// a caller that turns out to be unable to show them leaves them for whoever
// can. Call ClearPendingLinks once they have actually been shown.
//
// It returns nil when there is nothing recorded, when the record has expired,
// or when it cannot be read: this feeds a cosmetic notification, and no
// failure here is worth surfacing to the caller, let alone failing an agent's
// tool call over. A record that is expired or unparseable is dropped on the
// spot, since nobody can ever use it.
func (s *Store) PendingLinks() []string {
	path := filepath.Join(s.traceDirPath(), pendingLinksFile)

	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}

	var rec pendingLinks
	if err := json.Unmarshal(data, &rec); err != nil {
		_ = removeIfExists(path)
		return nil
	}

	savedAt, err := time.Parse(time.RFC3339, rec.SavedAt)
	if err != nil || time.Since(savedAt) > pendingLinksTTL {
		_ = removeIfExists(path)
		return nil
	}

	return rec.Links
}

// ClearPendingLinks drops the record, so its links are shown at most once.
func (s *Store) ClearPendingLinks() {
	_ = removeIfExists(filepath.Join(s.traceDirPath(), pendingLinksFile))
}
