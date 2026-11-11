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
	"bytes"
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"

	"github.com/chainloop-dev/chainloop/app/cli/internal/trace"
	"github.com/chainloop-dev/chainloop/app/cli/internal/trace/state"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const headerLine = `{"type":"session.start","data":{"sessionId":"` + testSessionID + `"}}` + "\n"

// copilotHome makes a Copilot home with the event log of testSessionID and
// points COPILOT_HOME at it. It returns the session-state folder.
func copilotHome(t *testing.T, content string) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("COPILOT_HOME", home)
	root := filepath.Join(home, "session-state")
	if content != "" {
		dir := filepath.Join(root, testSessionID)
		require.NoError(t, os.MkdirAll(dir, 0o755))
		require.NoError(t, os.WriteFile(filepath.Join(dir, eventsFile), []byte(content), 0o600))
	}
	return root
}

func newStore(t *testing.T) *state.Store {
	t.Helper()
	store := state.NewOutOfTreeStore(t.TempDir())
	require.NoError(t, store.SaveSessionRecord(&state.SessionRecord{SessionID: testSessionID, Provider: Name, Active: true}))
	return store
}

func copied(t *testing.T, store *state.Store) []byte {
	t.Helper()
	data, err := os.ReadFile(state.RawSessionPath(store.RawSessionDir(), testSessionID))
	if os.IsNotExist(err) {
		return nil
	}
	require.NoError(t, err)
	return data
}

func TestCopySessionData(t *testing.T) {
	p := New()

	t.Run("copies the log and saves the storage root", func(t *testing.T) {
		root := copilotHome(t, headerLine+`{"type":"user.message"}`+"\n")
		store := newStore(t)
		require.NoError(t, p.CopySessionData(store, trace.SessionLocation{SessionID: testSessionID}))
		assert.Equal(t, headerLine+`{"type":"user.message"}`+"\n", string(copied(t, store)))

		rec, err := store.LoadSessionRecord(testSessionID)
		require.NoError(t, err)
		want, err := filepath.EvalSymlinks(root)
		require.NoError(t, err)
		assert.Equal(t, want, rec.CopilotStorageRoot)
	})

	t.Run("a push with another COPILOT_HOME uses the stored root", func(t *testing.T) {
		copilotHome(t, headerLine)
		store := newStore(t)
		require.NoError(t, p.CopySessionData(store, trace.SessionLocation{SessionID: testSessionID}))

		t.Setenv("COPILOT_HOME", t.TempDir())
		rec, err := store.LoadSessionRecord(testSessionID)
		require.NoError(t, err)
		require.NoError(t, os.WriteFile(filepath.Join(rec.CopilotStorageRoot, testSessionID, eventsFile), []byte(headerLine+`{"type":"session.shutdown"}`+"\n"), 0o600))

		require.NoError(t, p.CopySessionData(store, trace.SessionLocation{SessionID: testSessionID}))
		assert.Contains(t, string(copied(t, store)), "session.shutdown")
	})

	t.Run("missing log is not an error", func(t *testing.T) {
		copilotHome(t, "")
		store := newStore(t)
		require.NoError(t, p.CopySessionData(store, trace.SessionLocation{SessionID: testSessionID}))
		assert.Nil(t, copied(t, store))
	})

	t.Run("session ID mismatch keeps the previous copy", func(t *testing.T) {
		root := copilotHome(t, headerLine)
		store := newStore(t)
		require.NoError(t, p.CopySessionData(store, trace.SessionLocation{SessionID: testSessionID}))

		other := `{"type":"session.start","data":{"sessionId":"other"}}` + "\n"
		require.NoError(t, os.WriteFile(filepath.Join(root, testSessionID, eventsFile), []byte(other), 0o600))
		require.Error(t, p.CopySessionData(store, trace.SessionLocation{SessionID: testSessionID}))
		assert.Equal(t, headerLine, string(copied(t, store)))
	})

	t.Run("transcript path outside session storage", func(t *testing.T) {
		copilotHome(t, headerLine)
		outside := filepath.Join(t.TempDir(), "events.jsonl")
		require.NoError(t, os.WriteFile(outside, []byte(headerLine), 0o600))
		store := newStore(t)
		err := p.CopySessionData(store, trace.SessionLocation{SessionID: testSessionID, TranscriptPath: outside})
		require.ErrorIs(t, err, errOutsideStorage)
		assert.Nil(t, copied(t, store))
	})

	t.Run("relative transcript path", func(t *testing.T) {
		copilotHome(t, headerLine)
		err := p.CopySessionData(newStore(t), trace.SessionLocation{SessionID: testSessionID, TranscriptPath: "events.jsonl"})
		require.Error(t, err)
	})

	t.Run("symbolic link out of session storage", func(t *testing.T) {
		root := copilotHome(t, "")
		outside := t.TempDir()
		require.NoError(t, os.WriteFile(filepath.Join(outside, eventsFile), []byte(headerLine), 0o600))
		require.NoError(t, os.MkdirAll(root, 0o755))
		require.NoError(t, os.Symlink(outside, filepath.Join(root, testSessionID)))

		store := newStore(t)
		err := p.CopySessionData(store, trace.SessionLocation{SessionID: testSessionID})
		require.ErrorIs(t, err, errOutsideStorage)
		assert.Nil(t, copied(t, store))
	})

	t.Run("a FIFO at the log path is rejected without blocking", func(t *testing.T) {
		root := copilotHome(t, "")
		dir := filepath.Join(root, testSessionID)
		require.NoError(t, os.MkdirAll(dir, 0o755))
		require.NoError(t, syscall.Mkfifo(filepath.Join(dir, eventsFile), 0o600))

		done := make(chan error, 1)
		go func() { done <- p.CopySessionData(newStore(t), trace.SessionLocation{SessionID: testSessionID}) }()
		select {
		case err := <-done:
			require.Error(t, err)
		case <-time.After(5 * time.Second):
			require.FailNow(t, "CopySessionData blocked on a FIFO")
		}
	})

	t.Run("invalid session ID", func(t *testing.T) {
		require.Error(t, p.CopySessionData(newStore(t), trace.SessionLocation{SessionID: "../x"}))
	})
}

func TestAnnounce(t *testing.T) {
	cases := []struct {
		name string
		run  func(p *Provider) error
		want string
	}{
		{
			name: "session start banner and instruction",
			run: func(p *Provider) error {
				return p.AnnounceSessionStart(trace.SessionStartMessage{Banner: "banner", Instruction: "capture the spec"})
			},
			want: `{"type":"progress","message":"banner"}` + "\n" + `{"additionalContext":"capture the spec"}` + "\n",
		},
		{
			name: "session start banner only",
			run:  func(p *Provider) error { return p.AnnounceSessionStart(trace.SessionStartMessage{Banner: "banner"}) },
			want: `{"type":"progress","message":"banner"}` + "\n",
		},
		{
			name: "session start instruction only",
			run: func(p *Provider) error {
				return p.AnnounceSessionStart(trace.SessionStartMessage{Instruction: "capture the spec"})
			},
			want: `{"additionalContext":"capture the spec"}` + "\n",
		},
		{
			name: "multi-line banner stays one progress line",
			run: func(p *Provider) error {
				return p.AnnounceSessionStart(trace.SessionStartMessage{Banner: "line 1\nline 2"})
			},
			want: `{"type":"progress","message":"line 1\nline 2"}` + "\n",
		},
		{
			name: "empty session start",
			run:  func(p *Provider) error { return p.AnnounceSessionStart(trace.SessionStartMessage{}) },
		},
		{
			name: "prompt reminder is dropped",
			run:  func(p *Provider) error { return p.AnnouncePromptSubmit("reminder") },
		},
		{
			name: "announce to user",
			run:  func(p *Provider) error { return p.AnnounceToUser("https://example.com/s") },
			want: `{"type":"progress","message":"https://example.com/s"}` + "\n" +
				`{"additionalContext":"Tell the user the following, including any link verbatim: https://example.com/s"}` + "\n",
		},
		{
			name: "empty announce",
			run:  func(p *Provider) error { return p.AnnounceToUser("") },
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var out bytes.Buffer
			p := &Provider{out: &out}
			require.NoError(t, tc.run(p))
			assert.Equal(t, tc.want, out.String())
		})
	}

	p := New()
	assert.True(t, p.SupportsSessionStartBanner())
	assert.True(t, p.SupportsSessionStartInstruction())
	assert.False(t, p.SupportsPromptReminder())
}

func TestFileSnapshots(t *testing.T) {
	p := New()
	store := state.NewOutOfTreeStore(t.TempDir())
	file := filepath.Join(t.TempDir(), "a.txt")
	require.NoError(t, os.WriteFile(file, []byte("before\n"), 0o600))
	input := &trace.HookInput{SessionID: testSessionID, FilePath: file}

	require.NoError(t, p.CaptureFileSnapshot(store, input))
	assert.Equal(t, "before\n", string(p.ResolveBeforeContent(store, input, nil)))
	p.CleanupAfterEdit(store, input)
	assert.Nil(t, p.ResolveBeforeContent(store, input, nil))

	// A new file has no snapshot.
	missing := &trace.HookInput{SessionID: testSessionID, FilePath: filepath.Join(t.TempDir(), "new.txt")}
	require.NoError(t, p.CaptureFileSnapshot(store, missing))
	assert.Nil(t, p.ResolveBeforeContent(store, missing, nil))
}
