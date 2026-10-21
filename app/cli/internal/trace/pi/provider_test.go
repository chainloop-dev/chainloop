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

package pi

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/chainloop-dev/chainloop/app/cli/internal/trace"
	"github.com/chainloop-dev/chainloop/app/cli/internal/trace/state"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestProviderName(t *testing.T) {
	assert.Equal(t, Name, New().Name())
}

func TestCopySessionDataUsesReportedPath(t *testing.T) {
	const sessionID = "pi-session-1"
	source := filepath.Join(t.TempDir(), "non-default", "session.jsonl")
	require.NoError(t, os.MkdirAll(filepath.Dir(source), 0o755))
	require.NoError(t, os.WriteFile(source, []byte("original\n"), 0o600))

	store := state.NewGitStore(t.TempDir())
	require.NoError(t, New().CopySessionData(store, trace.SessionLocation{
		SessionID:      sessionID,
		Cwd:            filepath.Join(t.TempDir(), "ignored"),
		TranscriptPath: source,
	}))

	destination := state.RawSessionPath(store.RawSessionDir(), sessionID)
	copied, err := os.ReadFile(destination)
	require.NoError(t, err)
	assert.Equal(t, "original\n", string(copied))

	require.NoError(t, os.WriteFile(source, []byte("changed\n"), 0o600))
	copied, err = os.ReadFile(destination)
	require.NoError(t, err)
	assert.Equal(t, "original\n", string(copied), "the store parses its private copy, not the live session")

	require.NoError(t, New().CopySessionData(store, trace.SessionLocation{
		SessionID:      sessionID,
		TranscriptPath: source,
	}))
	copied, err = os.ReadFile(destination)
	require.NoError(t, err)
	assert.Equal(t, "changed\n", string(copied), "a successful refresh replaces the stored copy")
}

func TestCopySessionDataFailureKeepsBestAvailableCopy(t *testing.T) {
	const sessionID = "pi-session-1"
	previous := []byte(`{"type":"session","version":3,"id":"pi-session-1","timestamp":"2026-01-02T03:04:00Z","cwd":"/work/repo"}` + "\n")

	for _, tc := range []struct {
		name           string
		transcriptPath func(t *testing.T) string
	}{
		{name: "missing reported path", transcriptPath: func(*testing.T) string { return "" }},
		{name: "missing source", transcriptPath: func(t *testing.T) string { return filepath.Join(t.TempDir(), "private", "missing.jsonl") }},
		{name: "unreadable source", transcriptPath: func(t *testing.T) string { return t.TempDir() }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			store := state.NewGitStore(t.TempDir())
			destination := state.RawSessionPath(store.RawSessionDir(), sessionID)
			require.NoError(t, os.MkdirAll(filepath.Dir(destination), 0o755))
			require.NoError(t, os.WriteFile(destination, previous, 0o600))

			source := tc.transcriptPath(t)
			err := New().CopySessionData(store, trace.SessionLocation{SessionID: sessionID, TranscriptPath: source})
			require.Error(t, err)
			if source != "" {
				assert.NotContains(t, err.Error(), source)
			}
			copied, readErr := os.ReadFile(destination)
			require.NoError(t, readErr)
			assert.Equal(t, previous, copied, "a failed refresh must preserve the last usable copy")

			matches, globErr := filepath.Glob(filepath.Join(store.RawSessionDir(), ".pi-session-*"))
			require.NoError(t, globErr)
			assert.Empty(t, matches, "temporary copies must be removed")
			result, parseErr := New().ParseSession(context.Background(), &trace.ParseOpts{
				SessionDir: store.RawSessionDir(),
				SessionID:  sessionID,
			})
			require.NoError(t, parseErr)
			assert.Equal(t, sessionID, result.Data.Session.ID)
		})
	}
}

func TestCopySessionDataRejectsInvalidSessionID(t *testing.T) {
	err := New().CopySessionData(state.NewGitStore(t.TempDir()), trace.SessionLocation{
		SessionID:      "../escape",
		TranscriptPath: filepath.Join(t.TempDir(), "session.jsonl"),
	})
	require.Error(t, err)
	assert.ErrorContains(t, err, "copy Pi session: invalid session")
}
