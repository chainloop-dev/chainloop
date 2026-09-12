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

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const sessionLink = "Coding Session Available at https://app.chainloop.dev/u/chainloop/sessions/ses_1"

// TestAnnounceToUser pins the wire shape the opencode plugin parses off the
// hook's stdout. Both channels are populated: the plugin shows "message" as a
// TUI toast and appends "relayToModel" to the shell tool's output, so a
// dismissed toast is not the only chance the user gets to see the link.
func TestAnnounceToUser(t *testing.T) {
	testCases := []struct {
		name        string
		msg         string
		wantEmitted bool
	}{
		{
			name:        "a message goes out on both channels",
			msg:         sessionLink,
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
				assert.Empty(t, out, "no message means no stdout, so the plugin has nothing to parse")
				return
			}

			var got map[string]any
			require.NoError(t, json.Unmarshal([]byte(out), &got))

			assert.Equal(t, tc.msg, got["message"], "the toast text must be the message verbatim")
			// The model needs the message verbatim to be able to repeat it.
			assert.Contains(t, got["relayToModel"], tc.msg)
		})
	}
}

// TestSystemMessage covers the session-start banner, which reaches the user as
// a toast and so goes out exactly as given — a toast supplies its own frame.
func TestSystemMessage(t *testing.T) {
	const banner = "Chainloop Trace is recording this session."

	testCases := []struct {
		name        string
		msg         string
		wantEmitted bool
	}{
		{
			name:        "the banner goes out verbatim",
			msg:         banner,
			wantEmitted: true,
		},
		{
			name:        "an empty banner emits nothing",
			msg:         "",
			wantEmitted: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			out := captureStdout(t, func() {
				require.NoError(t, New().SystemMessage(tc.msg))
			})

			if !tc.wantEmitted {
				assert.Empty(t, out)
				return
			}

			var got map[string]any
			require.NoError(t, json.Unmarshal([]byte(out), &got))

			assert.Equal(t, tc.msg, got["message"])
			// The banner is not news the model has to repeat; it is shown
			// once, at the top of the session, and nowhere else.
			assert.NotContains(t, got, "relayToModel")
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
