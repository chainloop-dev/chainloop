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

package action

import (
	"errors"
	"testing"

	"github.com/chainloop-dev/chainloop/app/cli/internal/trace"
	"github.com/chainloop-dev/chainloop/app/cli/internal/trace/state"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// recordingProvider captures what notifyPendingSessionLinks hands to the agent.
type recordingProvider struct {
	trace.Provider

	calls     int
	announced string
	err       error
}

func (p *recordingProvider) AnnounceToUser(msg string) error {
	p.calls++
	p.announced = msg

	return p.err
}

func TestNotifyPendingSessionLinks(t *testing.T) {
	const (
		link1 = "https://app.chainloop.dev/u/chainloop/sessions/ses_1"
		link2 = "https://app.chainloop.dev/u/chainloop/sessions/ses_2"
	)

	testCases := []struct {
		name            string
		saved           []string
		providerErr     error
		wantCalls       int
		wantUserMessage string
	}{
		{
			name:      "nothing pending leaves the agent alone",
			saved:     nil,
			wantCalls: 0,
		},
		{
			name:            "one link is announced with the log line's wording",
			saved:           []string{link1},
			wantCalls:       1,
			wantUserMessage: "Coding Session Available at " + link1,
		},
		{
			name:      "several links are announced one per line",
			saved:     []string{link1, link2},
			wantCalls: 1,
			wantUserMessage: "Coding Session Available at " + link1 + "\n" +
				"Coding Session Available at " + link2,
		},
		{
			name:            "a failed delivery is not fatal, and does not nag",
			saved:           []string{link1},
			providerErr:     errors.New("stdout closed"),
			wantCalls:       1,
			wantUserMessage: "Coding Session Available at " + link1,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			store := state.NewGitStore(t.TempDir())
			require.NoError(t, store.InitTraceDir())
			require.NoError(t, store.SavePendingLinks(tc.saved))

			p := &recordingProvider{err: tc.providerErr}

			// Must not panic or propagate; it only ever notifies.
			notifyPendingSessionLinks(p, store, zerolog.Nop())

			assert.Equal(t, tc.wantCalls, p.calls)
			assert.Equal(t, tc.wantUserMessage, p.announced)

			// A second hook firing must stay silent once an agent has had
			// its go, even if delivery failed: re-announcing on every later
			// shell command is worse than dropping one notification.
			before := p.calls
			notifyPendingSessionLinks(p, store, zerolog.Nop())

			assert.Equal(t, before, p.calls, "shown links must be consumed exactly once")
			assert.Empty(t, store.PendingLinks())
		})
	}
}

// TestNotifyPendingSessionLinksKeepsUnshownLinks covers the inverse of the
// consume-once rule: an agent with no way to reach the user showed nobody
// anything, so discarding the links would lose them silently.
func TestNotifyPendingSessionLinksKeepsUnshownLinks(t *testing.T) {
	const link = "https://app.chainloop.dev/u/chainloop/sessions/ses_1"

	store := state.NewGitStore(t.TempDir())
	require.NoError(t, store.InitTraceDir())
	require.NoError(t, store.SavePendingLinks([]string{link}))

	unsupported := &recordingProvider{err: trace.ErrAnnounceUnsupported}

	notifyPendingSessionLinks(unsupported, store, zerolog.Nop())

	assert.Equal(t, 1, unsupported.calls)
	assert.Equal(t, []string{link}, store.PendingLinks(), "unshown links must stay on disk")

	// The point of keeping them: an agent that can show them gets its turn
	// later, and consumes them as usual.
	capable := &recordingProvider{}

	notifyPendingSessionLinks(capable, store, zerolog.Nop())

	assert.Equal(t, 1, capable.calls, "a capable agent must still be offered the links")
	assert.Equal(t, "Coding Session Available at "+link, capable.announced)
	assert.Empty(t, store.PendingLinks(), "once shown, the links are consumed")
}
