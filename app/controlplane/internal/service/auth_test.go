//
// Copyright 2024 The Chainloop Authors.
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

package service

import (
	"net/http"
	"net/http/httptest"
	"testing"

	conf "github.com/chainloop-dev/chainloop/app/controlplane/internal/conf/controlplane/config/v1"
	"github.com/stretchr/testify/assert"
)

func TestGetAuthURLs(t *testing.T) {
	internalServer := &conf.Server_HTTP{Addr: "1.2.3.4"}
	testCases := []struct {
		name             string
		config           *conf.Server_HTTP
		loginURLOverride string
		want             *AuthURLs
		wantErr          bool
	}{
		{
			name:   "neither external url nor externalAddr set",
			config: internalServer,
			want:   &AuthURLs{callback: "http://1.2.3.4/auth/callback", Login: "http://1.2.3.4/auth/login"},
		},
		{
			name:   "correct URL, http",
			config: &conf.Server_HTTP{Addr: "1.2.3.4", ExternalUrl: "http://foo.com"},
			want:   &AuthURLs{callback: "http://foo.com/auth/callback", Login: "http://foo.com/auth/login"},
		},
		{
			name:   "correct URL, https",
			config: &conf.Server_HTTP{Addr: "1.2.3.4", ExternalUrl: "https://foo.com"},
			want:   &AuthURLs{callback: "https://foo.com/auth/callback", Login: "https://foo.com/auth/login"},
		},
		{
			name:   "with path",
			config: &conf.Server_HTTP{Addr: "1.2.3.4", ExternalUrl: "https://foo.com/path"},
			want:   &AuthURLs{callback: "https://foo.com/path/auth/callback", Login: "https://foo.com/path/auth/login"},
		},
		{
			name:   "with port",
			config: &conf.Server_HTTP{Addr: "1.2.3.4", ExternalUrl: "https://foo.com:1234"},
			want:   &AuthURLs{callback: "https://foo.com:1234/auth/callback", Login: "https://foo.com:1234/auth/login"},
		},
		{
			name:    "invalid, missing scheme",
			config:  &conf.Server_HTTP{Addr: "1.2.3.4", ExternalUrl: "localhost.com"},
			wantErr: true,
		},
		{
			name:             "external with override",
			config:           &conf.Server_HTTP{Addr: "1.2.3.4", ExternalUrl: "https://foo.com"},
			loginURLOverride: "https://foo.override.com/auth/login",
			want:             &AuthURLs{callback: "https://foo.com/auth/callback", Login: "https://foo.override.com/auth/login", loginIsOverridden: true},
		},
		{
			name:             "internal with override",
			config:           internalServer,
			loginURLOverride: "https://foo.override.com/auth/login",
			want:             &AuthURLs{callback: "http://1.2.3.4/auth/callback", Login: "https://foo.override.com/auth/login", loginIsOverridden: true},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := getAuthURLs(tc.config, tc.loginURLOverride)
			if tc.wantErr {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)
			assert.Equal(t, tc.want, got)
		})
	}
}

func TestGetPreferredEmail(t *testing.T) {
	testCases := []struct {
		claims *upstreamOIDCclaims
		want   string
	}{
		{
			claims: &upstreamOIDCclaims{Email: ""},
			want:   "",
		},
		{
			claims: &upstreamOIDCclaims{Email: "foo@bar.com"},
			want:   "foo@bar.com",
		},
		{
			claims: &upstreamOIDCclaims{Email: "foo@bar.com", PreferredUsername: "overridden"},
			want:   "foo@bar.com",
		},
		{
			claims: &upstreamOIDCclaims{Email: "foo@bar.com", PreferredUsername: "overridden@bar.com"},
			want:   "overridden@bar.com",
		},
		{
			claims: &upstreamOIDCclaims{Email: "foo@bar.com", PreferredUsername: "overridden+22@bar.com"},
			want:   "overridden+22@bar.com",
		},
		{
			claims: &upstreamOIDCclaims{Email: "foo@bar.com", PreferredUsername: "overridden@bar.sub.com"},
			want:   "overridden@bar.sub.com",
		},
	}

	for _, tc := range testCases {
		got := tc.claims.preferredEmail()
		assert.Equal(t, tc.want, got)
	}
}

func TestCallbackAllowed(t *testing.T) {
	// mixed case on purpose, origins are matched case insensitively
	allowed := originsOf("https://app.chainloop.dev/login", "https://CP.Chainloop.dev", "https://app.chainloop.dev")

	testCases := []struct {
		name     string
		callback string
		wantErr  bool
	}{
		{name: "empty, token is rendered in a page", callback: ""},
		{name: "relative path, CAS download redirect", callback: "/download/sha256:deadbeef?foo=bar"},
		{name: "loopback with random port, CLI login", callback: "http://127.0.0.1:41337/auth/callback"},
		{name: "localhost with random port, CLI login", callback: "http://localhost:41337/auth/callback"},
		{name: "IPv6 loopback", callback: "http://[::1]:41337/auth/callback"},
		{name: "dashboard origin", callback: "https://app.chainloop.dev/login/callback?returnTo=%2Fprojects"},
		{name: "control plane origin, config declared it in a different case", callback: "https://cp.chainloop.dev/foo"},
		{name: "dashboard origin, browser sent a different case", callback: "https://APP.chainloop.dev/login/callback"},
		{name: "third party origin", callback: "https://evil.example/collect", wantErr: true},
		{name: "protocol relative", callback: "//evil.example/collect", wantErr: true},
		{name: "backslash escaping a relative path", callback: "/\\evil.example/collect", wantErr: true},
		{name: "non http scheme", callback: "javascript:alert(1)", wantErr: true},
		{name: "loopback lookalike host", callback: "http://localhost.evil.example/collect", wantErr: true},
		{name: "allowed host as a subdomain", callback: "https://app.chainloop.dev.evil.example/collect", wantErr: true},
		{name: "allowed host with a different scheme", callback: "http://app.chainloop.dev/collect", wantErr: true},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := callbackAllowed(tc.callback, allowed)
			if tc.wantErr {
				assert.Error(t, err)
				return
			}

			assert.NoError(t, err)
		})
	}
}

// The callback cookie must not be set for a destination we would refuse to redirect to
func TestLoginHandlerRejectsForeignCallback(t *testing.T) {
	svc := &AuthService{allowedCallbackOrigins: originsOf("https://app.chainloop.dev")}

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/auth/login?callback=https%3A%2F%2Fevil.example%2Fcollect&long-lived=true", nil)

	resp := loginHandler(svc, w, r)
	assert.Equal(t, http.StatusBadRequest, resp.code)
	assert.Empty(t, w.Result().Cookies())
}
