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

package servicelogger

import (
	"context"
	"errors"
	"testing"

	"github.com/getsentry/sentry-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newRecordingHub returns a hub tagged with org whose events are appended to events
func newRecordingHub(t *testing.T, org string, events *[]*sentry.Event) *sentry.Hub {
	t.Helper()

	client, err := sentry.NewClient(sentry.ClientOptions{
		BeforeSend: func(event *sentry.Event, _ *sentry.EventHint) *sentry.Event {
			*events = append(*events, event)
			return nil
		},
	})
	require.NoError(t, err)

	scope := sentry.NewScope()
	scope.SetTag("org.name", org)
	return sentry.NewHub(client, scope)
}

func TestLogAndMaskErrCapturesOnTheRequestHub(t *testing.T) {
	var requestEvents, globalEvents []*sentry.Event
	requestHub := newRecordingHub(t, "request-org", &requestEvents)

	// Swap the global hub for the test so that events that fall back to it are recorded
	global := sentry.CurrentHub()
	globalHub := newRecordingHub(t, "global-org", &globalEvents)
	global.BindClient(globalHub.Client())
	t.Cleanup(func() { global.BindClient(nil) })

	testCases := []struct {
		name       string
		ctx        context.Context
		wantEvents *[]*sentry.Event
		wantOrg    string
	}{
		{
			name:       "context with a request hub",
			ctx:        sentry.SetHubOnContext(context.Background(), requestHub),
			wantEvents: &requestEvents,
			wantOrg:    "request-org",
		},
		{
			name:       "context without a hub falls back to the global hub",
			ctx:        context.Background(),
			wantEvents: &globalEvents,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			requestEvents, globalEvents = requestEvents[:0], globalEvents[:0]

			require.Error(t, LogAndMaskErr(tc.ctx, errors.New("boom"), nil))

			require.Len(t, *tc.wantEvents, 1)
			assert.Equal(t, 1, len(requestEvents)+len(globalEvents), "the error must be reported on one hub only")
			assert.Equal(t, tc.wantOrg, (*tc.wantEvents)[0].Tags["org.name"])
		})
	}
}
