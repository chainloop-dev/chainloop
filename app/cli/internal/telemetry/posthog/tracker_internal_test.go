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

package posthog

import (
	"context"
	"testing"

	"github.com/chainloop-dev/chainloop/app/cli/internal/telemetry"
	"github.com/posthog/posthog-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// recordingClient captures the messages the Tracker enqueues, in order, so the tests
// can assert on the exact payload that would reach PostHog.
type recordingClient struct {
	messages []posthog.Message
	closed   bool
}

func (r *recordingClient) Enqueue(msg posthog.Message) error {
	r.messages = append(r.messages, msg)
	return nil
}

func (r *recordingClient) Close() error {
	r.closed = true
	return nil
}

const (
	testCPHash    = "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
	testOrgID     = "3b8f3b1e-0b7a-4a4a-9a1e-8a6b4c2d1e0f"
	testUserID    = "9f1c2b3a-4d5e-6f70-8192-a3b4c5d6e7f8"
	testMachineID = "machine-1234"

	tagCPURLHash = "cp_url_hash"
	tagMachineID = "machine_id"
	tagTokenType = "token_type"
	tagCI        = "ci"

	groupCPInstallation = "cp_installation"

	tagFalse = "false"
	tagTrue  = "true"
)

func TestTrackEventGroupsAndAlias(t *testing.T) {
	testCases := []struct {
		name string
		// id is the distinct ID the CommandTracker resolved for the event.
		id             string
		tags           telemetry.Tags
		wantGroups     posthog.Groups
		wantAliasFor   string
		wantNoGroups   bool
		wantAliasCount int
	}{
		{
			// Unauthenticated run: no token parsed, so the machine ID is the distinct ID
			// and there is nothing to alias it to.
			name: "unauthenticated, installation group only",
			id:   testMachineID,
			tags: telemetry.Tags{
				tagCPURLHash: testCPHash,
				tagMachineID: testMachineID,
				tagCI:        tagFalse,
			},
			wantGroups: posthog.Groups{groupCPInstallation: testCPHash},
		},
		{
			// An API token carries an org ID. Both groups have to travel on the same event:
			// dropping cp_installation here is what made the group keys look inconsistent
			// across CLI versions.
			name: "api token in CI, both groups and no alias",
			id:   testOrgID,
			tags: telemetry.Tags{
				tagCPURLHash: testCPHash,
				"org_id":     testOrgID,
				tagMachineID: testMachineID,
				tagTokenType: "AUTH_TYPE_API_TOKEN",
				tagCI:        tagTrue,
			},
			wantGroups: posthog.Groups{
				groupCPInstallation: testCPHash,
				"organization":      testOrgID,
			},
		},
		{
			// The only case where stitching a pre-login machine identity onto an account
			// is meaningful.
			name: "interactive user token, aliased",
			id:   testUserID,
			tags: telemetry.Tags{
				tagCPURLHash: testCPHash,
				tagMachineID: testMachineID,
				tagTokenType: "AUTH_TYPE_USER",
				tagCI:        tagFalse,
			},
			wantGroups:     posthog.Groups{groupCPInstallation: testCPHash},
			wantAliasFor:   testMachineID,
			wantAliasCount: 1,
		},
		{
			// A user token used from CI is still a shared identity across ephemeral machines.
			name: "user token in CI, not aliased",
			id:   testUserID,
			tags: telemetry.Tags{
				tagCPURLHash: testCPHash,
				tagMachineID: testMachineID,
				tagTokenType: "AUTH_TYPE_USER",
				tagCI:        tagTrue,
			},
			wantGroups: posthog.Groups{groupCPInstallation: testCPHash},
		},
		{
			// Federated tokens resolve to the issuer URL, which is identical for every
			// customer, so aliasing merges unrelated runners into a single person.
			name: "federated token, not aliased",
			id:   "https://token.actions.githubusercontent.com",
			tags: telemetry.Tags{
				tagCPURLHash: testCPHash,
				tagMachineID: testMachineID,
				tagTokenType: "AUTH_TYPE_FEDERATED",
				tagCI:        tagTrue,
			},
			wantGroups: posthog.Groups{groupCPInstallation: testCPHash},
		},
		{
			name: "no group tags, no groups sent",
			id:   testMachineID,
			tags: telemetry.Tags{
				tagMachineID: testMachineID,
				tagCI:        tagFalse,
			},
			wantNoGroups: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			client := &recordingClient{}
			tracker := &Tracker{client: client}

			err := tracker.TrackEvent(context.Background(), "command_executed", tc.id, tc.tags)
			require.NoError(t, err)
			assert.True(t, client.closed, "the client must be closed so the batch is flushed")

			var captures []posthog.Capture
			var aliases []posthog.Alias
			for _, msg := range client.messages {
				switch m := msg.(type) {
				case posthog.Capture:
					captures = append(captures, m)
				case posthog.Alias:
					aliases = append(aliases, m)
				default:
					require.Failf(t, "unexpected message type", "%T", msg)
				}
			}

			require.Len(t, captures, 1)
			if tc.wantNoGroups {
				assert.Nil(t, captures[0].Groups)
			} else {
				assert.Equal(t, tc.wantGroups, captures[0].Groups)
			}

			require.Len(t, aliases, tc.wantAliasCount)
			if tc.wantAliasCount > 0 {
				assert.Equal(t, tc.id, aliases[0].DistinctId)
				assert.Equal(t, tc.wantAliasFor, aliases[0].Alias)
				// The alias has to be enqueued before the event it stitches.
				assert.IsType(t, posthog.Alias{}, client.messages[0])
			}
		})
	}
}
