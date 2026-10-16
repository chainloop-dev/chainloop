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

package opencode

import (
	"encoding/json"
	"io"
	"os"
	"testing"

	"github.com/chainloop-dev/chainloop/app/cli/internal/trace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestAnnounceSessionStart pins the session-start response: the Chainloop
// plugin posts instruction to the session as a context-only message and shows
// banner to the user, each OpenCode major on its own channel.
func TestAnnounceSessionStart(t *testing.T) {
	const (
		banner      = "Chainloop Trace is recording this session."
		instruction = "Write the specification this session is working from into /repo/.chainloop/specs/abc-123 now."
	)

	testCases := []struct {
		name string
		msg  trace.SessionStartMessage
		// want is the emitted document, or nil for no output at all.
		want *hookResponse
	}{
		{
			name: "the instruction and the banner share one document",
			msg:  trace.SessionStartMessage{Banner: banner, Instruction: instruction},
			want: &hookResponse{Instruction: instruction, Banner: banner},
		},
		{
			name: "an instruction on its own",
			msg:  trace.SessionStartMessage{Instruction: instruction},
			want: &hookResponse{Instruction: instruction},
		},
		{
			name: "a banner on its own",
			msg:  trace.SessionStartMessage{Banner: banner},
			want: &hookResponse{Banner: banner},
		},
		{
			name: "nothing to say emits nothing",
			msg:  trace.SessionStartMessage{},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			out := captureStdout(t, func() {
				require.NoError(t, New().AnnounceSessionStart(tc.msg))
			})

			if tc.want == nil {
				assert.Empty(t, out)
				return
			}

			var got hookResponse
			require.NoError(t, json.Unmarshal([]byte(out), &got))
			assert.Equal(t, *tc.want, got)
		})
	}
}

// TestAnnouncePromptSubmit pins the prompt-submit response: the plugin reads
// instruction and posts it to the session as a context-only message, as it
// does at session start.
func TestAnnouncePromptSubmit(t *testing.T) {
	const reminder = "Chainloop spec capture. Folder: /repo/.chainloop/specs/abc-123"

	testCases := []struct {
		name     string
		reminder string
		want     *hookResponse
	}{
		{name: "a reminder is emitted", reminder: reminder, want: &hookResponse{Instruction: reminder}},
		{name: "nothing to say emits nothing", reminder: ""},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			out := captureStdout(t, func() {
				require.NoError(t, New().AnnouncePromptSubmit(tc.reminder))
			})

			if tc.want == nil {
				assert.Empty(t, out)
				return
			}

			var got hookResponse
			require.NoError(t, json.Unmarshal([]byte(out), &got))
			assert.Equal(t, *tc.want, got)
		})
	}
}

// TestAnnounceToUser pins the response after a shell command: the plugin
// shows message to the user and adds relayToModel to the command result, so
// the model repeats the message in its reply.
func TestAnnounceToUser(t *testing.T) {
	const msg = "Coding Session Available at https://app.chainloop.dev/u/acme/sessions/abc-123"

	testCases := []struct {
		name string
		msg  string
		want *hookResponse
	}{
		{
			name: "a message goes out on both channels",
			msg:  msg,
			want: &hookResponse{Message: msg, RelayToModel: trace.RelayToModelInstruction + msg},
		},
		{name: "nothing to say emits nothing", msg: ""},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			out := captureStdout(t, func() {
				require.NoError(t, New().AnnounceToUser(tc.msg))
			})

			if tc.want == nil {
				assert.Empty(t, out)
				return
			}

			var got hookResponse
			require.NoError(t, json.Unmarshal([]byte(out), &got))
			assert.Equal(t, *tc.want, got)
		})
	}
}

// captureStdout runs fn and returns what it wrote to stdout.
func captureStdout(t *testing.T, fn func()) string {
	t.Helper()

	orig := os.Stdout
	r, w, err := os.Pipe()
	require.NoError(t, err)
	os.Stdout = w
	t.Cleanup(func() { os.Stdout = orig })

	fn()
	require.NoError(t, w.Close())

	out, err := io.ReadAll(r)
	require.NoError(t, err)
	require.NoError(t, r.Close())

	return string(out)
}
