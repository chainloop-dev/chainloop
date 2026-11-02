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
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/chainloop-dev/chainloop/app/cli/pkg/action"
	"github.com/chainloop-dev/chainloop/internal/oauth"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/oauth2"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestAddCodeChallenge(t *testing.T) {
	loginURL, err := url.Parse("https://cp.example/auth/login?callback=http%3A%2F%2Flocalhost%3A1234%2Fauth%2Fcallback")
	require.NoError(t, err)

	verifier := addCodeChallenge(loginURL)

	q := loginURL.Query()
	assert.Equal(t, oauth2.S256ChallengeFromVerifier(verifier), q.Get(oauth.QueryParamCodeChallenge))
	assert.Equal(t, oauth.CodeChallengeMethodS256, q.Get(oauth.QueryParamCodeChallengeMethod))
	// Existing parameters are kept, and the verifier never goes in the URL
	assert.Equal(t, "http://localhost:1234/auth/callback", q.Get(oauth.QueryParamCallback))
	assert.NotContains(t, loginURL.String(), verifier)
}

// The browser fallback switches to headless after the challenge was added
func TestToHeadlessLoginURL(t *testing.T) {
	loginURL, err := url.Parse("https://cp.example/auth/login?callback=http%3A%2F%2Flocalhost%3A1234%2Fauth%2Fcallback&long-lived=true")
	require.NoError(t, err)
	addCodeChallenge(loginURL)

	toHeadlessLoginURL(loginURL)

	q := loginURL.Query()
	assert.True(t, q.Has(oauth.QueryParamCallback))
	assert.Empty(t, q.Get(oauth.QueryParamCallback))
	assert.False(t, q.Has(oauth.QueryParamCodeChallenge))
	assert.False(t, q.Has(oauth.QueryParamCodeChallengeMethod))
	assert.Equal(t, "true", q.Get(oauth.QueryParamLongLived))
}

func TestLoginCallbackToken(t *testing.T) {
	const (
		callbackURL  = "http://localhost:1234/auth/callback"
		handoffQuery = "handoff=the-code"
		legacyQuery  = "t=legacy-token"
		testVerifier = "verifier"
	)

	testCases := []struct {
		name string
		// query of the callback request
		query         string
		codeVerifier  string
		legacyAllowed bool
		exchangeErr   error
		wantToken     string
		wantErr       bool
		wantExchange  bool
		wantWarning   bool
	}{
		{
			name:         "code exchange",
			query:        handoffQuery,
			codeVerifier: testVerifier,
			wantToken:    "exchanged-token",
			wantExchange: true,
		},
		{
			name:         "failed exchange",
			query:        handoffQuery,
			codeVerifier: testVerifier,
			exchangeErr:  errors.New("invalid login code"),
			wantExchange: true,
			wantErr:      true,
		},
		{
			name:          "legacy callback while allowed",
			query:         legacyQuery,
			codeVerifier:  testVerifier,
			legacyAllowed: true,
			wantToken:     "legacy-token",
			wantWarning:   true,
		},
		{
			name:          "legacy callback while denied",
			query:         legacyQuery,
			codeVerifier:  testVerifier,
			legacyAllowed: false,
			wantErr:       true,
		},
		{
			name:      "older control plane",
			query:     legacyQuery,
			wantToken: "legacy-token",
		},
		{
			name:    "handoff from an older control plane is not expected",
			query:   handoffQuery,
			wantErr: true,
		},
		{
			name:         "no credentials",
			query:        "",
			codeVerifier: testVerifier,
			wantErr:      true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var exchanged, warned bool
			c := &loginCallback{
				callbackURL:   callbackURL,
				codeVerifier:  tc.codeVerifier,
				legacyAllowed: tc.legacyAllowed,
				exchange: func(_ context.Context, handoff, codeVerifier, cb string) (string, error) {
					exchanged = true
					assert.Equal(t, "the-code", handoff)
					assert.Equal(t, tc.codeVerifier, codeVerifier)
					assert.Equal(t, callbackURL, cb)
					return "exchanged-token", tc.exchangeErr
				},
				warn: func(string) { warned = true },
			}

			r := httptest.NewRequest(http.MethodGet, "/auth/callback?"+tc.query, nil)
			token, err := c.token(r)

			assert.Equal(t, tc.wantExchange, exchanged)
			assert.Equal(t, tc.wantWarning, warned)
			if tc.wantErr {
				assert.Error(t, err)
				assert.Empty(t, token)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tc.wantToken, token)
		})
	}
}

func TestCheckAuthToken(t *testing.T) {
	const token = "a-token"

	testCases := []struct {
		name      string
		token     string
		verifyErr error
		wantErr   string
		// whether the error renders as the CLI authentication message
		wantAuthErr bool
	}{
		{
			name:  "token accepted by the control plane",
			token: token,
		},
		{
			name:    "no token",
			token:   "",
			wantErr: "no token was entered",
		},
		{
			name:        "token rejected by the control plane",
			token:       token,
			verifyErr:   status.Error(codes.Unauthenticated, "token is invalid"),
			wantErr:     "verifying the token with the control plane",
			wantAuthErr: true,
		},
		{
			name:      "control plane error",
			token:     token,
			verifyErr: errors.New("user not found"),
			wantErr:   "verifying the token with the control plane: user not found",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var verified bool
			err := checkAuthToken(context.Background(), tc.token, func(_ context.Context, token string) error {
				verified = true
				assert.Equal(t, tc.token, token)
				return tc.verifyErr
			})

			// Only a token that was entered goes to the control plane
			assert.Equal(t, tc.token != "", verified)
			if tc.wantErr != "" {
				assert.ErrorContains(t, err, tc.wantErr)
				_, isAuthErr := action.AuthErrorMessage(err)
				assert.Equal(t, tc.wantAuthErr, isAuthErr)
				return
			}

			assert.NoError(t, err)
		})
	}
}
