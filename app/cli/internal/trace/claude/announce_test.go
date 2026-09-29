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

package claude

import (
	"encoding/json"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/chainloop-dev/chainloop/app/cli/internal/trace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestAnnounceToUser pins the wire shape Claude Code expects. systemMessage
// must stay top-level: nested inside hookSpecificOutput it is silently ignored.
func TestAnnounceToUser(t *testing.T) {
	const msg = "Coding Session Available at https://app.chainloop.dev/u/chainloop/sessions/ses_1"

	testCases := []struct {
		name        string
		msg         string
		wantEmitted bool
	}{
		{
			name:        "a message goes out on both channels",
			msg:         msg,
			wantEmitted: true,
		},
		{
			name:        "nothing to say emits nothing",
			msg:         "",
			wantEmitted: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			out := captureStdout(t, func() {
				require.NoError(t, New().AnnounceToUser(tc.msg))
			})

			if !tc.wantEmitted {
				assert.Empty(t, out, "no message means no stdout, so the hook stays a no-op")
				return
			}

			var got map[string]any
			require.NoError(t, json.Unmarshal([]byte(out), &got))

			assert.Equal(t, tc.msg, got["systemMessage"], "systemMessage must be top-level")

			hookOut, ok := got["hookSpecificOutput"].(map[string]any)
			require.True(t, ok, "hookSpecificOutput must be present")
			assert.Equal(t, "PostToolUse", hookOut["hookEventName"])

			// The model needs the message verbatim to be able to repeat it.
			assert.Contains(t, hookOut["additionalContext"], tc.msg)
		})
	}
}

// TestAnnounceSessionStart pins the session-start wire shape. The two channels
// travel in one document because Claude Code parses a hook's stdout as a single
// JSON value: a second write would be discarded together with whatever it
// carried, which is why they are one call rather than two.
func TestAnnounceSessionStart(t *testing.T) {
	const (
		banner      = "Chainloop Trace is recording this session."
		instruction = "Write the specification this session is working from into /repo/.chainloop/specs/abc-123 now."
	)

	testCases := []struct {
		name            string
		msg             trace.SessionStartMessage
		wantBanner      string
		wantInstruction string
		wantEmitted     bool
	}{
		{
			name:            "both channels in one document",
			msg:             trace.SessionStartMessage{Banner: banner, Instruction: instruction},
			wantBanner:      banner,
			wantInstruction: instruction,
			wantEmitted:     true,
		},
		{
			// What the hook emits once a spec is already captured: the user
			// still gets the banner, the model is told nothing.
			name:        "a banner with nothing to instruct",
			msg:         trace.SessionStartMessage{Banner: banner},
			wantBanner:  banner,
			wantEmitted: true,
		},
		{
			name:            "an instruction with no banner",
			msg:             trace.SessionStartMessage{Instruction: instruction},
			wantInstruction: instruction,
			wantEmitted:     true,
		},
		{
			name:        "nothing to say emits nothing",
			msg:         trace.SessionStartMessage{},
			wantEmitted: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			out := captureStdout(t, func() {
				require.NoError(t, New().AnnounceSessionStart(tc.msg))
			})

			if !tc.wantEmitted {
				assert.Empty(t, out, "no message means no stdout, so the hook stays a no-op")
				return
			}

			dec := json.NewDecoder(strings.NewReader(out))

			var got map[string]any
			require.NoError(t, dec.Decode(&got))

			// The constraint this whole shape exists to respect: one document,
			// never two.
			require.ErrorIs(t, dec.Decode(&map[string]any{}), io.EOF, "stdout must carry exactly one JSON document")

			if tc.wantBanner == "" {
				assert.NotContains(t, got, "systemMessage", "nothing to show means no empty banner to render")
			} else {
				assert.Equal(t, tc.wantBanner, got["systemMessage"], "systemMessage must be top-level")
			}

			hookOut, ok := got["hookSpecificOutput"].(map[string]any)
			require.True(t, ok, "hookSpecificOutput must be present")
			assert.Equal(t, "SessionStart", hookOut["hookEventName"], "the event must be the one that fired")

			if tc.wantInstruction == "" {
				assert.NotContains(t, hookOut, "additionalContext")
				return
			}
			assert.Equal(t, tc.wantInstruction, hookOut["additionalContext"], "the model needs the instruction verbatim")
		})
	}
}

// captureStdout runs fn with os.Stdout redirected to a pipe and returns
// everything written to it. Reads to EOF rather than into a fixed buffer: a
// truncated read would corrupt the payload these tests parse as JSON.
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
