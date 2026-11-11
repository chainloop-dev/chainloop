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

package authenticator

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/chainloop-dev/chainloop/pkg/errorsx"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/oauth2"
)

func TestNewOIDCDiscoveryFailureRecordsStack(t *testing.T) {
	srv := httptest.NewServer(http.NotFoundHandler())
	t.Cleanup(srv.Close)

	_, err := NewOIDC(srv.URL, "client", "secret", "http://localhost/callback")
	require.Error(t, err)
	assert.True(t, errorsx.HasStack(err))
}

func TestVerifyIDTokenErrorsRecordStack(t *testing.T) {
	testCases := []struct {
		name  string
		token *oauth2.Token
	}{
		{name: "missing id_token", token: &oauth2.Token{}},
		{name: "invalid id_token", token: (&oauth2.Token{}).WithExtra(map[string]any{"id_token": "not-a-jwt"})},
	}

	srv := newDiscoveryServer(t)
	a, err := NewOIDC(srv.URL, "client", "secret", "http://localhost/callback")
	require.NoError(t, err)

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := a.VerifyIDToken(context.Background(), tc.token)
			require.Error(t, err)
			assert.True(t, errorsx.HasStack(err))
		})
	}
}

// newDiscoveryServer serves a minimal OIDC discovery document for its own URL.
func newDiscoveryServer(t *testing.T) *httptest.Server {
	t.Helper()

	var srv *httptest.Server
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/.well-known/openid-configuration" {
			http.NotFound(w, r)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"issuer":"` + srv.URL + `","authorization_endpoint":"` + srv.URL + `/auth","token_endpoint":"` + srv.URL + `/token","jwks_uri":"` + srv.URL + `/keys"}`))
	}))
	t.Cleanup(srv.Close)

	return srv
}
