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

// TestAnnounceSessionStart pins the session-start response: the Chainloop plugin reads instruction and posts it to the session as a context-only message.
// There is no channel to the user, so the banner is never emitted.
func TestAnnounceSessionStart(t *testing.T) {
	const (
		banner      = "Chainloop Trace is recording this session."
		instruction = "Write the specification this session is working from into /repo/.chainloop/specs/abc-123 now."
	)

	testCases := []struct {
		name string
		msg  trace.SessionStartMessage
		// want is the emitted document, or nil for no output at all.
		want map[string]any
	}{
		{
			name: "the instruction is emitted and the banner is not",
			msg:  trace.SessionStartMessage{Banner: banner, Instruction: instruction},
			want: map[string]any{"instruction": instruction},
		},
		{
			name: "an instruction on its own",
			msg:  trace.SessionStartMessage{Instruction: instruction},
			want: map[string]any{"instruction": instruction},
		},
		{
			// A banner has nowhere to go, so the hook stays a no-op.
			name: "a banner alone emits nothing",
			msg:  trace.SessionStartMessage{Banner: banner},
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

			var got map[string]any
			require.NoError(t, json.Unmarshal([]byte(out), &got))
			assert.Equal(t, tc.want, got)
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
