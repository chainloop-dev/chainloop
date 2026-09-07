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

package webhook

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

// The generic webhook accepts any destination, so whether one inside the
// deployment's own network is reachable is up to the deployment's network
// policy. httptest listens on the loopback interface, which stands in for
// such a destination.
func TestRegisterHonoursNetworkPolicy(t *testing.T) {
	testCases := []struct {
		name        string
		netPolicy   sdk.NetworkPolicy
		wantBlocked bool
	}{
		{
			name:      "reachable by default",
			netPolicy: sdk.NetworkPolicy{},
		},
		{
			name:        "unreachable when private targets are blocked",
			netPolicy:   sdk.NetworkPolicy{BlockPrivateTargets: true},
			wantBlocked: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var requests int
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				requests++
				w.WriteHeader(http.StatusOK)
			}))
			defer server.Close()

			integration, err := New(nil, tc.netPolicy)
			require.NoError(t, err)

			payload, err := json.Marshal(map[string]string{"url": server.URL})
			require.NoError(t, err)

			_, err = integration.Register(context.Background(), &sdk.RegistrationRequest{Payload: payload})

			if tc.wantBlocked {
				assert.ErrorIs(t, err, sdk.ErrBlockedTarget)
				assert.Zero(t, requests, "the webhook must not be reached")
				return
			}

			require.NoError(t, err)
			assert.Equal(t, 1, requests)
		})
	}
}

func TestValidateURL(t *testing.T) {
	testCases := []struct {
		name    string
		url     string
		wantErr bool
	}{
		{name: "http", url: "http://example.com/hook"},
		{name: "https", url: "https://example.com/hook"},
		{name: "unsupported scheme", url: "file:///etc/passwd", wantErr: true},
		{name: "not a URL", url: "example.com", wantErr: true},
		{name: "empty", url: "", wantErr: true},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := validateURL(tc.url)
			if tc.wantErr {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)
		})
	}
}
