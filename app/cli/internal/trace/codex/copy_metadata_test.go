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
	"os"
	"path/filepath"
	"testing"

	"github.com/chainloop-dev/chainloop/app/cli/internal/trace"
	"github.com/chainloop-dev/chainloop/app/cli/internal/trace/state"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStorageRootPreservesNewerMetadata(t *testing.T) {
	store := state.NewOutOfTreeStore(t.TempDir())
	require.NoError(t, store.SaveSessionRecord(&state.SessionRecord{SessionID: testSessionID, Provider: Name, Active: true}))
	stale, err := store.LoadSessionRecord(testSessionID)
	require.NoError(t, err)
	newer := *stale
	newer.Active = false
	newer.Checkouts = []string{t.TempDir()}
	require.NoError(t, store.SaveSessionRecord(&newer))
	home := t.TempDir()
	require.NoError(t, saveStorageRoot(store, stale, home))
	got, err := store.LoadSessionRecord(testSessionID)
	require.NoError(t, err)
	assert.False(t, got.Active)
	assert.Equal(t, newer.Checkouts, got.Checkouts)
	canonicalHome, err := filepath.EvalSymlinks(home)
	require.NoError(t, err)
	assert.Equal(t, canonicalHome, got.CodexStorageRoot)
}

func TestCopyWithDamagedOptionalMetadata(t *testing.T) {
	home := t.TempDir()
	t.Setenv("CODEX_HOME", home)
	raw, err := os.ReadFile("testdata/session.jsonl")
	require.NoError(t, err)
	source := filepath.Join(home, "sessions", "session.jsonl")
	require.NoError(t, os.MkdirAll(filepath.Dir(source), 0700))
	require.NoError(t, os.WriteFile(source, raw, 0600)) //nolint:gosec // fixture copied into a private temporary Codex home
	store := state.NewOutOfTreeStore(t.TempDir())
	require.NoError(t, store.SaveSessionRecord(&state.SessionRecord{SessionID: testSessionID, Provider: Name}))
	record := filepath.Join(store.TraceDir(), "sessions", testSessionID+".json")
	require.NoError(t, os.WriteFile(record, []byte("{"), 0600))
	require.NoError(t, New().CopySessionData(store, trace.SessionLocation{SessionID: testSessionID, TranscriptPath: source}))
	got, err := os.ReadFile(state.RawSessionPath(store.RawSessionDir(), testSessionID))
	require.NoError(t, err)
	assert.Equal(t, raw, got)
	unchanged, err := os.ReadFile(record)
	require.NoError(t, err)
	assert.Equal(t, "{", string(unchanged))
}
