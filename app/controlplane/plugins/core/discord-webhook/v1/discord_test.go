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

package discord

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
			input: map[string]interface{}{"webhook": "http://repo.io", "username": "u"},
		},
		{
			name:  "ok, only required properties",
			input: map[string]interface{}{"webhook": "http://repo.io"},
		},
		{
			name:   "not ok, empty username",
			input:  map[string]interface{}{"webhook": "http://repo.io", "username": ""},
			errMsg: "length must be >= 1, but got 0",
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

func TestNewIntegration(t *testing.T) {
	_, err := New(nil)
	assert.NoError(t, err)
}

// The Discord webhook only ever targets Discord, so a destination inside the
// deployment's own network is refused. httptest listens on the loopback
// interface, which stands in for any such destination.
func TestRegisterRejectsNonPublicWebhook(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		assert.NoError(t, json.NewEncoder(w).Encode(webhookResponse{Name: "internal service"}))
	}))
	defer server.Close()

	integration, err := New(nil)
	require.NoError(t, err)

	payload, err := json.Marshal(map[string]string{"webhook": server.URL})
	require.NoError(t, err)

	_, err = integration.Register(context.Background(), &sdk.RegistrationRequest{Payload: payload})
	assert.ErrorIs(t, err, sdk.ErrBlockedTarget)
}

func TestExecuteWebhookRejectsNonPublicURL(t *testing.T) {
	var requests int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requests++
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	err := executeWebhook(publicOnlyClient(), server.URL, "", []byte("statement"), "New Attestation Received")
	assert.ErrorIs(t, err, sdk.ErrBlockedTarget)
	assert.Zero(t, requests, "the webhook must not be reached")
}
