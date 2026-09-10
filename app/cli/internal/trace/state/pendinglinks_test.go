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
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTakePendingLinks(t *testing.T) {
	links := []string{
		"https://app.chainloop.dev/u/chainloop/sessions/ses_1",
		"https://app.chainloop.dev/u/chainloop/sessions/ses_2",
	}

	testCases := []struct {
		name string
		// age of the saved record; zero means "just written"
		age   time.Duration
		links []string
		want  []string
	}{
		{
			name:  "an empty list is never saved, so nothing comes back",
			links: nil,
			want:  nil,
		},
		{
			name:  "fresh record is returned",
			links: links,
			want:  links,
		},
		{
			name:  "record just inside the TTL is returned",
			age:   pendingLinksTTL - time.Minute,
			links: links,
			want:  links,
		},
		{
			name:  "record past the TTL is discarded",
			age:   pendingLinksTTL + time.Minute,
			links: links,
			want:  nil,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			store := NewGitStore(t.TempDir())
			require.NoError(t, store.InitTraceDir())

			require.NoError(t, store.SavePendingLinks(tc.links))
			if tc.age > 0 {
				ageRecord(t, store, tc.age)
			}

			got := store.TakePendingLinks()
			assert.Equal(t, tc.want, got)

			// Whatever the outcome, nothing is left behind to announce twice.
			assert.Empty(t, store.TakePendingLinks(), "links must be consumed exactly once")
		})
	}
}

// ageRecord rewrites the saved record with a timestamp shifted into the past,
// so TTL behaviour is exercised without sleeping.
func ageRecord(t *testing.T, store *Store, age time.Duration) {
	t.Helper()

	path := filepath.Join(store.traceDirPath(), pendingLinksFile)
	raw, err := os.ReadFile(path)
	require.NoError(t, err)

	rec := pendingLinks{}
	require.NoError(t, json.Unmarshal(raw, &rec))
	rec.SavedAt = time.Now().UTC().Add(-age).Format(time.RFC3339)

	out, err := json.Marshal(rec)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(path, out, 0o600))
}

// TestPendingLinksSurviveWipe pins the assumption the agent-notification
// handoff rests on: the push saves the links and then wipes single-use trace
// state, so a wipe that also cleared this record would silently swallow every
// notification.
func TestPendingLinksSurviveWipe(t *testing.T) {
	store := NewGitStore(t.TempDir())
	require.NoError(t, store.InitTraceDir())

	links := []string{"https://app.chainloop.dev/u/chainloop/sessions/ses_1"}
	require.NoError(t, store.SavePendingLinks(links))

	require.NoError(t, store.WipeTraceDir())

	assert.Equal(t, links, store.TakePendingLinks(), "links must outlive the post-push wipe")
}

func TestTakePendingLinksIgnoresCorruptRecord(t *testing.T) {
	store := NewGitStore(t.TempDir())
	require.NoError(t, store.InitTraceDir())

	path := filepath.Join(store.traceDirPath(), pendingLinksFile)
	require.NoError(t, os.WriteFile(path, []byte("{not json"), 0o600))

	assert.Empty(t, store.TakePendingLinks(), "a corrupt record must not surface links")
	assert.NoFileExists(t, path, "a corrupt record must still be cleared")
}
