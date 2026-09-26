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

package cmd

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/chainloop-dev/chainloop/app/cli/internal/telemetry"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// captureClient records what the flush command hands to the telemetry layer, so the
// tests can assert on the exact event that would reach PostHog.
type captureClient struct {
	eventName string
	id        string
	tags      telemetry.Tags
	calls     int
}

func (c *captureClient) TrackEvent(_ context.Context, eventName string, id string, tags telemetry.Tags) error {
	c.eventName = eventName
	c.id = id
	c.tags = tags
	c.calls++
	return nil
}

func TestRunTelemetryFlush(t *testing.T) {
	const payload = `{
		"command": "attestation push",
		"duration_ms": 4321,
		"success": true,
		"cp_url": "api.cp.chainloop.dev:443",
		"organization_name": "my-org",
		"token_type": "AUTH_TYPE_API_TOKEN",
		"user_id": "6f2e4a1b-0000-4a4a-9a1e-8a6b4c2d1e0f",
		"org_id": "3b8f3b1e-0b7a-4a4a-9a1e-8a6b4c2d1e0f"
	}`

	client := &captureClient{}
	require.NoError(t, runTelemetryFlush(strings.NewReader(payload), client))

	assert.Equal(t, 1, client.calls)
	assert.Equal(t, "command_executed", client.eventName)
	assert.Equal(t, "6f2e4a1b-0000-4a4a-9a1e-8a6b4c2d1e0f", client.id)

	assert.Equal(t, "attestation push", client.tags["command"])
	assert.Equal(t, testOrgName, client.tags["organization_name"])
	assert.Equal(t, "AUTH_TYPE_API_TOKEN", client.tags["token_type"])
	assert.Equal(t, "api.cp.chainloop.dev:443", client.tags["cp_installation_url"])
	assert.Equal(t, "cli", client.tags["chainloop_source"])

	// The whole point of the change: the duration must reach PostHog as a number, not a
	// string, or averages and percentiles are not expressible in an insight.
	assert.Equal(t, int64(4321), client.tags["duration_ms"])
	assert.Equal(t, true, client.tags["success"])
	assert.NotContains(t, client.tags, "error_kind")

	// Derived in the child rather than carried in the payload.
	assert.Contains(t, client.tags, "os")
	assert.Contains(t, client.tags, "arch")
	assert.Contains(t, client.tags, "ci")
	assert.NotEmpty(t, client.tags["cp_url_hash"])
}

func TestRunTelemetryFlushFailedCommand(t *testing.T) {
	const payload = `{
		"command": "workflow list",
		"duration_ms": 12,
		"success": false,
		"error_kind": "Unavailable",
		"cp_url": "api.cp.chainloop.dev:443"
	}`

	client := &captureClient{}
	require.NoError(t, runTelemetryFlush(strings.NewReader(payload), client))

	assert.Equal(t, false, client.tags["success"])
	assert.Equal(t, "Unavailable", client.tags["error_kind"])
	assert.Equal(t, int64(12), client.tags["duration_ms"])

	// No identity in the payload: the event still goes out, keyed on the machine.
	assert.NotContains(t, client.tags, "user_id")
	assert.NotContains(t, client.tags, "org_id")
	assert.NotContains(t, client.tags, "organization_name")
}

func TestRunTelemetryFlushRejectsBadPayloads(t *testing.T) {
	testCases := []struct {
		name    string
		payload string
	}{
		{
			name:    "empty input",
			payload: "",
		},
		{
			name:    "not json",
			payload: "workflow list 4321",
		},
		{
			name: "unknown field from a newer parent",
			// Decoding strictly means a version skew fails loudly in the debug log
			// instead of silently reporting a partial event.
			payload: `{"command":"workflow list","duration_ms":1,"success":true,"something_new":"x"}`,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			client := &captureClient{}
			assert.Error(t, runTelemetryFlush(strings.NewReader(tc.payload), client))
			assert.Zero(t, client.calls)
		})
	}
}

// TestTelemetryRoundTrip pins the parent and the child to the same schema: whatever the
// parent encodes, the child must decode and report without loss.
func TestTelemetryRoundTrip(t *testing.T) {
	sent := completedCommand{
		Command:          "attestation push",
		DurationMs:       9876,
		Success:          false,
		ErrorKind:        "PermissionDenied",
		ControlPlaneURL:  "localhost:9000",
		OrganizationName: testOrgName,
		TokenType:        "AUTH_TYPE_USER",
		UserID:           "user-id",
		OrgID:            "org-id",
	}

	encoded, err := json.Marshal(sent)
	require.NoError(t, err)

	client := &captureClient{}
	require.NoError(t, runTelemetryFlush(strings.NewReader(string(encoded)), client))

	assert.Equal(t, sent.Command, client.tags["command"])
	assert.Equal(t, sent.DurationMs, client.tags["duration_ms"])
	assert.Equal(t, sent.Success, client.tags["success"])
	assert.Equal(t, sent.ErrorKind, client.tags["error_kind"])
	assert.Equal(t, sent.OrganizationName, client.tags["organization_name"])
	assert.Equal(t, sent.UserID, client.id)
}
