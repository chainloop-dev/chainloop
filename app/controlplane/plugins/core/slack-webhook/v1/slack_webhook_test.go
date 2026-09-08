//
// Copyright 2023-2026 The Chainloop Authors.
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

package slack

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/chainloop-dev/chainloop/app/controlplane/plugins/sdk/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidateRegistrationInput(t *testing.T) {
	testCases := []struct {
		name   string
		input  map[string]interface{}
		errMsg string
	}{
		{
			name:   "not ok, missing required property",
			input:  map[string]interface{}{},
			errMsg: "missing properties: 'webhook'",
		},
		{
			name:   "not ok, random properties",
			input:  map[string]interface{}{"foo": "bar"},
			errMsg: "additionalProperties 'foo' not allowed",
		},
		{
			name:  "ok, all properties",
			input: map[string]interface{}{"webhook": "http://repo.io"},
		},
		{
			name:  "ok, webhook with path",
			input: map[string]interface{}{"webhook": "http://repo/foo/bar"},
		},
		{
			name:   "not ok, invalid webhook, missing protocol",
			input:  map[string]interface{}{"webhook": "repo.io"},
			errMsg: "is not valid 'uri'",
		},
		{
			name:   "not ok, empty webhook",
			input:  map[string]interface{}{"webhook": ""},
			errMsg: "is not valid 'uri'",
		},
	}

	integration, err := New(nil)
	require.NoError(t, err)

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			payload, err := json.Marshal(tc.input)
			require.NoError(t, err)

			err = sdk.ValidateRegistrationRequest(integration, payload)
			if tc.errMsg != "" {
				assert.ErrorContains(t, err, tc.errMsg)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestRenderContent(t *testing.T) {
	testCases := []struct {
		name     string
		input    *templateContent
		expected string
	}{
		{
			name: "all fields",
			input: &templateContent{
				WorkflowRunID:   "deadbeef",
				WorkflowName:    "test",
				WorkflowProject: "project",
				RunnerLink:      "http://runner.io",
			},
			expected: `New attestation received!
- Workflow: project/test
- Workflow Run: deadbeef
- Link to runner: http://runner.io`,
		},
		{
			name: "no runner link",
			input: &templateContent{
				WorkflowRunID:   "deadbeef",
				WorkflowName:    "test",
				WorkflowProject: "project",
			},
			expected: `New attestation received!
- Workflow: project/test
- Workflow Run: deadbeef`,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			actual := renderContent(tc.input)
			assert.Equal(t, tc.expected, actual)
		})
	}
}

func TestNewIntegration(t *testing.T) {
	_, err := New(nil)
	assert.NoError(t, err)
}

// The Slack webhook only ever targets Slack, so a destination inside the
// deployment's own network is refused. httptest listens on the loopback
// interface, which stands in for any such destination.
func TestExecuteWebhookRejectsNonPublicURL(t *testing.T) {
	var requests int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requests++
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	err := executeWebhook(publicOnlyClient(), server.URL, "New attestation received")
	assert.ErrorIs(t, err, sdk.ErrBlockedTarget)
	assert.Zero(t, requests, "the webhook must not be reached")
}

func TestRegisterRejectsNonPublicWebhook(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	integration, err := New(nil)
	require.NoError(t, err)

	payload, err := json.Marshal(map[string]string{"webhook": server.URL})
	require.NoError(t, err)

	_, err = integration.Register(context.Background(), &sdk.RegistrationRequest{Payload: payload})
	assert.ErrorIs(t, err, sdk.ErrBlockedTarget)
}
