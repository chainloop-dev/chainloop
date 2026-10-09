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

package sdk

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewHTTPClientPublicTargetsOnly(t *testing.T) {
	// httptest listens on the loopback interface, so it stands in for any
	// destination a public-only client must refuse to reach.
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	testCases := []struct {
		name              string
		publicTargetsOnly bool
		wantBlocked       bool
	}{
		{name: "public-only client refuses a loopback target", publicTargetsOnly: true, wantBlocked: true},
		{name: "unrestricted client reaches a loopback target", publicTargetsOnly: false},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			client := NewHTTPClient(HTTPClientOptions{PublicTargetsOnly: tc.publicTargetsOnly})

			resp, err := client.Get(server.URL)
			if tc.wantBlocked {
				require.ErrorIs(t, err, ErrBlockedTarget)
				return
			}

			require.NoError(t, err)
			defer resp.Body.Close()
			assert.Equal(t, http.StatusOK, resp.StatusCode)
		})
	}
}

// A proxy would be the only address a public-only client connects to, leaving
// the destination unchecked, so such a client must not pick one up from the
// environment. Asserted on the transport because net/http resolves the
// environment once per process, which a test cannot change after the fact.
func TestNewHTTPClientProxyUse(t *testing.T) {
	testCases := []struct {
		name              string
		publicTargetsOnly bool
		wantProxy         bool
	}{
		{name: "public-only client ignores an environment proxy", publicTargetsOnly: true},
		{name: "unrestricted client keeps an environment proxy", wantProxy: true},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			client := NewHTTPClient(HTTPClientOptions{PublicTargetsOnly: tc.publicTargetsOnly})

			transport, ok := client.Transport.(*http.Transport)
			require.True(t, ok)

			if tc.wantProxy {
				assert.NotNil(t, transport.Proxy)
				return
			}
			assert.Nil(t, transport.Proxy)
		})
	}
}
