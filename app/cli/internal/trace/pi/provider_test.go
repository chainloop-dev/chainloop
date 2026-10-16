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
	"os"
	"path/filepath"
	"runtime"
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
}

func TestCopySessionDataFailureRemovesStaleCopy(t *testing.T) {
	const sessionID = "pi-session-1"

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
			require.NoError(t, os.WriteFile(destination, []byte("stale\n"), 0o600))

			source := tc.transcriptPath(t)
			err := New().CopySessionData(store, trace.SessionLocation{SessionID: sessionID, TranscriptPath: source})
			require.ErrorIs(t, err, trace.ErrSessionDataNotFresh)
			if source != "" {
				assert.NotContains(t, err.Error(), source)
			}
			assert.ErrorIs(t, fileError(destination), os.ErrNotExist, "a failed refresh must not leave stale evidence")

			matches, globErr := filepath.Glob(filepath.Join(store.RawSessionDir(), ".pi-session-*"))
			require.NoError(t, globErr)
			assert.Empty(t, matches, "temporary copies must be removed")
		})
	}
}

func TestCopySessionDataInvalidatesStaleCopyWhenQuarantineFails(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("directory mode bits do not make rename fail on Windows")
	}

	const sessionID = "pi-session-1"
	store := state.NewGitStore(t.TempDir())
	destination := state.RawSessionPath(store.RawSessionDir(), sessionID)
	require.NoError(t, os.MkdirAll(filepath.Dir(destination), 0o755))
	require.NoError(t, os.WriteFile(destination, []byte("stale\n"), 0o600))
	require.NoError(t, os.Chmod(store.RawSessionDir(), 0o500))
	t.Cleanup(func() { _ = os.Chmod(store.RawSessionDir(), 0o755) })

	err := New().CopySessionData(store, trace.SessionLocation{
		SessionID:      sessionID,
		TranscriptPath: filepath.Join(t.TempDir(), "missing.jsonl"),
	})
	require.ErrorIs(t, err, trace.ErrSessionDataNotFresh)

	content, readErr := os.ReadFile(destination)
	require.NoError(t, readErr)
	assert.Empty(t, content, "an unmovable previous copy must be made unparseable")
}

func TestCopySessionDataMarksUnrecoverableStaleCopyUnsafe(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("directory mode bits do not make rename fail on Windows")
	}

	const sessionID = "pi-session-1"
	store := state.NewGitStore(t.TempDir())
	destination := state.RawSessionPath(store.RawSessionDir(), sessionID)
	require.NoError(t, os.MkdirAll(filepath.Dir(destination), 0o755))
	require.NoError(t, os.WriteFile(destination, []byte("stale\n"), 0o400))
	require.NoError(t, os.Chmod(store.RawSessionDir(), 0o500))
	t.Cleanup(func() {
		_ = os.Chmod(store.RawSessionDir(), 0o755)
		_ = os.Chmod(destination, 0o600)
	})

	err := New().CopySessionData(store, trace.SessionLocation{
		SessionID:      sessionID,
		TranscriptPath: filepath.Join(t.TempDir(), "missing.jsonl"),
	})
	require.ErrorIs(t, err, trace.ErrSessionDataNotFresh)

	content, readErr := os.ReadFile(destination)
	require.NoError(t, readErr)
	assert.Equal(t, "stale\n", string(content), "the caller must skip parsing when the provider cannot invalidate its old copy")
}

func TestCopySessionDataRejectsInvalidSessionID(t *testing.T) {
	err := New().CopySessionData(state.NewGitStore(t.TempDir()), trace.SessionLocation{
		SessionID:      "../escape",
		TranscriptPath: filepath.Join(t.TempDir(), "session.jsonl"),
	})
	assert.ErrorIs(t, err, trace.ErrSessionDataNotFresh)
	assert.ErrorContains(t, err, "copy Pi session: invalid session")
}

func fileError(path string) error {
	_, err := os.Stat(path)
	return err
}
