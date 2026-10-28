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

package orcarouter

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeAuthServer is a stand-in for the OrcaRouter consent exchange. It records
// what it was sent so the tests can assert the path, the verifier, and the
// method, and it never receives a real credential.
type fakeAuthServer struct {
	*httptest.Server
	mu       sync.Mutex
	requests []exchangeRecord
	// respond lets a test override the reply.
	respond func(w http.ResponseWriter, body map[string]string)
	// verifier is the verifier the server expects, learned from the challenge
	// the client sent at authorize time.
	challenge string
}

type exchangeRecord struct {
	path   string
	method string
	code   string
	verif  string
	kmeth  string
}

func newFakeAuthServer(t *testing.T) *fakeAuthServer {
	t.Helper()
	f := &fakeAuthServer{}
	f.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]string
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, "bad body", http.StatusBadRequest)
			return
		}
		f.mu.Lock()
		f.requests = append(f.requests, exchangeRecord{
			path: r.URL.Path, method: r.Method,
			code: body["code"], verif: body["code_verifier"], kmeth: body["code_challenge_method"],
		})
		respond := f.respond
		challenge := f.challenge
		f.mu.Unlock()

		if respond != nil {
			respond(w, body)
			return
		}
		// When a test set a challenge, enforce it the way the real service
		// does before minting a key.
		if challenge != "" {
			sum := sha256.Sum256([]byte(body["code_verifier"]))
			if base64.RawURLEncoding.EncodeToString(sum[:]) != challenge {
				http.Error(w, `{"error":"invalid_grant"}`, http.StatusForbidden)
				return
			}
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"key":"sk-orca-test-minted-key","user_id":"42","scope":"api"}`))
	}))
	t.Cleanup(f.Close)
	return f
}

// runConnect drives Connect with a fake browser: it reads the authorize URL,
// extracts the callback, and delivers the code the way the consent screen
// would. deliver lets a test send a different query.
func runConnect(t *testing.T, authBase string, deliver func(callback url.URL, state string) string) (Credential, error) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	t.Cleanup(cancel)

	return Connect(ctx, ConnectOptions{
		AuthBase: authBase,
		AppName:  "Chainloop Test",
		Client:   &http.Client{Timeout: 5 * time.Second},
		OpenBrowser: func(authorizeURL string) error {
			u, err := url.Parse(authorizeURL)
			if err != nil {
				return err
			}
			q := u.Query()
			require.Equal(t, "S256", q.Get("code_challenge_method"))
			require.NotEmpty(t, q.Get("code_challenge"))
			require.NotEmpty(t, q.Get("state"))
			require.Equal(t, "Chainloop Test", q.Get("app_name"))
			require.Equal(t, "api", q.Get("scope"))

			callback, err := url.Parse(q.Get("callback_url"))
			if err != nil {
				return err
			}
			target := deliver(*callback, q.Get("state"))
			resp, err := http.Get(target)
			if err != nil {
				return err
			}
			defer func() { _ = resp.Body.Close() }()
			return nil
		},
	})
}

func TestConnectSuccess(t *testing.T) {
	f := newFakeAuthServer(t)

	// The authorize URL carries only the challenge; the real verifier is what
	// the exchange presents. Recompute the challenge from the exchange body
	// afterwards, so the fake server can enforce the binding.
	cred, err := runConnect(t, f.URL, func(callback url.URL, state string) string {
		callback.RawQuery = url.Values{"code": {"test-code"}, "state": {state}}.Encode()
		return callback.String()
	})
	require.NoError(t, err)

	assert.Equal(t, "sk-orca-test-minted-key", cred.Key)
	assert.Equal(t, SourcePKCE, cred.Source)
	assert.Equal(t, "42", cred.UserID)
	assert.Equal(t, "api", cred.Scope)

	f.mu.Lock()
	defer f.mu.Unlock()
	require.Len(t, f.requests, 1)
	assert.Equal(t, ExchangePath, f.requests[0].path)
	assert.Equal(t, "/api/v1/auth/keys", f.requests[0].path)
	assert.Equal(t, http.MethodPost, f.requests[0].method)
	assert.Equal(t, "test-code", f.requests[0].code)
	assert.Equal(t, "S256", f.requests[0].kmeth)
	assert.NotEmpty(t, f.requests[0].verif)
}

func TestConnectChallengeEnforcedByServer(t *testing.T) {
	f := newFakeAuthServer(t)
	f.mu.Lock()
	f.challenge = "not-the-challenge-of-the-verifier"
	f.mu.Unlock()

	_, err := runConnect(t, f.URL, func(callback url.URL, state string) string {
		callback.RawQuery = url.Values{"code": {"test-code"}, "state": {state}}.Encode()
		return callback.String()
	})
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrExchangeRejected)
}

func TestConnectDenied(t *testing.T) {
	f := newFakeAuthServer(t)

	_, err := runConnect(t, f.URL, func(callback url.URL, state string) string {
		callback.RawQuery = url.Values{"error": {"access_denied"}, "state": {state}}.Encode()
		return callback.String()
	})
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrAccessDenied)
	// A denial stops before the exchange.
	f.mu.Lock()
	defer f.mu.Unlock()
	assert.Empty(t, f.requests)
}

func TestConnectStateMismatch(t *testing.T) {
	f := newFakeAuthServer(t)

	_, err := runConnect(t, f.URL, func(callback url.URL, _ string) string {
		callback.RawQuery = url.Values{"code": {"test-code"}, "state": {"someone-elses-state"}}.Encode()
		return callback.String()
	})
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrStateMismatch)
	f.mu.Lock()
	defer f.mu.Unlock()
	assert.Empty(t, f.requests, "a callback from another attempt must never be exchanged")
}

func TestConnectTimeout(t *testing.T) {
	f := newFakeAuthServer(t)

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	_, err := Connect(ctx, ConnectOptions{
		AuthBase:    f.URL,
		Client:      &http.Client{Timeout: time.Second},
		OpenBrowser: func(string) error { return nil },
	})
	require.Error(t, err)
	assert.ErrorIs(t, err, context.DeadlineExceeded)
}

func TestConnectExchangeCodes(t *testing.T) {
	cases := []struct {
		name   string
		status int
		want   error
	}{
		{name: "expired or reused code", status: http.StatusForbidden, want: ErrExchangeRejected},
		{name: "bad method", status: http.StatusBadRequest, want: ErrExchangeRejected},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := newFakeAuthServer(t)
			f.mu.Lock()
			f.respond = func(w http.ResponseWriter, _ map[string]string) {
				http.Error(w, `{"error":"invalid_grant"}`, tc.status)
			}
			f.mu.Unlock()

			_, err := runConnect(t, f.URL, func(callback url.URL, state string) string {
				callback.RawQuery = url.Values{"code": {"used-code"}, "state": {state}}.Encode()
				return callback.String()
			})
			require.Error(t, err)
			assert.ErrorIs(t, err, tc.want)
		})
	}
}

func TestConnectRateLimited(t *testing.T) {
	f := newFakeAuthServer(t)
	f.mu.Lock()
	f.respond = func(w http.ResponseWriter, _ map[string]string) {
		http.Error(w, `{"error":"rate_limited"}`, http.StatusTooManyRequests)
	}
	f.mu.Unlock()

	_, err := runConnect(t, f.URL, func(callback url.URL, state string) string {
		callback.RawQuery = url.Values{"code": {"test-code"}, "state": {state}}.Encode()
		return callback.String()
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "429")
}

func TestConnectScopeDowngrade(t *testing.T) {
	f := newFakeAuthServer(t)
	f.mu.Lock()
	f.respond = func(w http.ResponseWriter, _ map[string]string) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"key":"sk-orca-narrow","user_id":"42","scope":"connector"}`))
	}
	f.mu.Unlock()

	_, err := runConnect(t, f.URL, func(callback url.URL, state string) string {
		callback.RawQuery = url.Values{"code": {"test-code"}, "state": {state}}.Encode()
		return callback.String()
	})
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrScopeDowngrade)
}

func TestExchangeUsesAuthOriginOnly(t *testing.T) {
	var gotPath, gotHost string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath, gotHost = r.URL.Path, r.Host
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"key":"sk-orca-x","scope":"api"}`))
	}))
	defer srv.Close()

	_, err := Exchange(context.Background(), srv.Client(), srv.URL, "code", "verifier", "api")
	require.NoError(t, err)
	assert.Equal(t, "/api/v1/auth/keys", gotPath)
	assert.False(t, strings.HasPrefix(gotPath, "/v1/"), "the exchange path is not below /v1")
	assert.NotContains(t, gotHost, "api.orcarouter.ai", "the exchange must not go to the inference origin")
}

func TestAuthorizeURL(t *testing.T) {
	raw := AuthorizeURL("https://www.orcarouter.ai", "http://127.0.0.1:5000/cb", "CHALLENGE", "STATE", "Chainloop", "api")
	u, err := url.Parse(raw)
	require.NoError(t, err)
	assert.Equal(t, "/auth", u.Path)
	q := u.Query()
	assert.Equal(t, "http://127.0.0.1:5000/cb", q.Get("callback_url"))
	assert.Equal(t, "CHALLENGE", q.Get("code_challenge"))
	assert.Equal(t, "S256", q.Get("code_challenge_method"))
	assert.Equal(t, "STATE", q.Get("state"))
	assert.Equal(t, "Chainloop", q.Get("app_name"))
	assert.Equal(t, "api", q.Get("scope"))
	assert.NotContains(t, raw, "verifier")
}

func TestVerifierAndStateAreFresh(t *testing.T) {
	a, err := newVerifier()
	require.NoError(t, err)
	b, err := newVerifier()
	require.NoError(t, err)
	assert.NotEqual(t, a, b)
	assert.Len(t, a, 43)

	s1, err := newState()
	require.NoError(t, err)
	s2, err := newState()
	require.NoError(t, err)
	assert.NotEqual(t, s1, s2)

	// The challenge is the unpadded base64url SHA-256 of the verifier.
	sum := sha256.Sum256([]byte(a))
	assert.Equal(t, base64.RawURLEncoding.EncodeToString(sum[:]), challengeFor(a))
	assert.False(t, strings.Contains(challengeFor(a), "="))
	assert.Len(t, challengeFor(a), 43)
}

func TestResolveOrigins(t *testing.T) {
	t.Setenv(EnvSharedBaseURL, "")
	t.Setenv(EnvAuthBaseURL, "")
	t.Setenv(EnvAPIBaseURL, "")
	assert.Equal(t, "https://www.orcarouter.ai", AuthOrigin())
	assert.Equal(t, "https://api.orcarouter.ai/v1", APIOrigin())

	// A shared self-hosted base covers both, and the explicit overrides win.
	t.Setenv(EnvSharedBaseURL, "https://orca.internal/")
	assert.Equal(t, "https://orca.internal", AuthOrigin())
	assert.Equal(t, "https://orca.internal", APIOrigin())

	t.Setenv(EnvAuthBaseURL, "https://login.internal")
	t.Setenv(EnvAPIBaseURL, "https://relay.internal/v1")
	assert.Equal(t, "https://login.internal", AuthOrigin())
	assert.Equal(t, "https://relay.internal/v1", APIOrigin())
}

func TestValidateOrigin(t *testing.T) {
	cases := []struct {
		name    string
		origin  string
		wantErr bool
	}{
		{name: "https remote", origin: "https://www.orcarouter.ai"},
		{name: "http loopback", origin: "http://127.0.0.1:8080"},
		{name: "http localhost", origin: "http://localhost:1234"},
		{name: "http remote", origin: "http://evil.example.com", wantErr: true},
		{name: "no scheme", origin: "www.orcarouter.ai", wantErr: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateOrigin(tc.origin)
			if tc.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
		})
	}
}

// TestConnectDoesNotLeakVerifier proves the verifier never rides in the URL,
// which is what the tests above read.
func TestConnectDoesNotLeakVerifier(t *testing.T) {
	f := newFakeAuthServer(t)
	var seenAuthorizeURL string

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	_, err := Connect(ctx, ConnectOptions{
		AuthBase: f.URL,
		Client:   &http.Client{Timeout: 5 * time.Second},
		OpenBrowser: func(authorizeURL string) error {
			seenAuthorizeURL = authorizeURL
			u, _ := url.Parse(authorizeURL)
			callback, _ := url.Parse(u.Query().Get("callback_url"))
			state := u.Query().Get("state")
			callback.RawQuery = url.Values{"code": {"c"}, "state": {state}}.Encode()
			resp, err := http.Get(callback.String())
			if err != nil {
				return err
			}
			defer func() { _ = resp.Body.Close() }()
			return nil
		},
	})
	require.NoError(t, err)

	f.mu.Lock()
	defer f.mu.Unlock()
	require.Len(t, f.requests, 1)
	verifier := f.requests[0].verif
	require.NotEmpty(t, verifier)
	assert.NotContains(t, seenAuthorizeURL, verifier, "the verifier must never leave the process")
	assert.NotContains(t, fmt.Sprintf("%v", f.requests), "sk-orca-", "the server record holds no key")
}
