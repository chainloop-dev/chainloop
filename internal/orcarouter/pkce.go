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
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"
)

// Origins and paths of the login flow. Authentication and inference are
// different origins, and the exchange path is not below /v1.
const (
	// EnvAuthBaseURL overrides the authentication origin.
	EnvAuthBaseURL = "ORCA_AUTH_BASE_URL"
	// EnvAPIBaseURL overrides the inference origin.
	EnvAPIBaseURL = "ORCA_API_BASE_URL"
	// EnvSharedBaseURL sets one origin for both, as a self-hosted deployment
	// may do. The explicit overrides win.
	EnvSharedBaseURL = "ORCA_BASE_URL"
	// AuthPath is the consent screen.
	AuthPath = "/auth"
	// ExchangePath is where an auth code becomes an API key.
	ExchangePath = "/api/v1/auth/keys"
	// DefaultScope is the scope the login asks for.
	DefaultScope = "api"
	// MaxResponseBytes bounds what the exchange may return.
	MaxResponseBytes = 1 << 20
)

// ErrAccessDenied is returned when the user declines on the consent screen.
var ErrAccessDenied = errors.New("the authorization was denied")

// ErrStateMismatch is returned when the callback state does not match the one
// that this attempt sent.
var ErrStateMismatch = errors.New("the callback state does not match this authorization attempt")

// ErrExchangeRejected is returned when the exchange refuses the code. A code is
// single use with a ten minute lifetime; any refusal needs a new attempt.
var ErrExchangeRejected = errors.New("the authorization code was rejected")

// ErrScopeDowngrade is returned when the granted scope is narrower than the
// scope the caller needs.
var ErrScopeDowngrade = errors.New("the granted scope is narrower than the requested scope")

// ErrNeedsReauth is returned by the request path when the service rejects a
// durable key. The key is not refreshable; a new login is required.
var ErrNeedsReauth = errors.New("the OrcaRouter credential was rejected; authorize again")

// AuthOrigin returns the authentication origin: the explicit override first,
// then the shared base, then the public default.
func AuthOrigin() string {
	return resolveBase(EnvAuthBaseURL)
}

// APIOrigin returns the inference origin, with the same precedence.
func APIOrigin() string {
	return resolveBase(EnvAPIBaseURL)
}

// resolveBase reads a documented override and normalizes it.
func resolveBase(explicit string) string {
	base := os.Getenv(explicit)
	if base == "" {
		base = os.Getenv(EnvSharedBaseURL)
	}
	if base == "" {
		if explicit == EnvAuthBaseURL {
			return AuthBaseURL
		}
		return BaseURL
	}
	return NormalizeBase(base)
}

// NormalizeBase trims a trailing slash from a configured origin.
func NormalizeBase(base string) string {
	return strings.TrimRight(strings.TrimSpace(base), "/")
}

// ValidateOrigin requires HTTPS for a remote origin and permits plain HTTP only
// for loopback, where a development server and the local callback live.
func ValidateOrigin(raw string) error {
	u, err := url.Parse(raw)
	if err != nil {
		return fmt.Errorf("parse origin: %w", err)
	}
	if u.Scheme == "https" && u.Host != "" {
		return nil
	}
	if u.Scheme == "http" && isLoopbackHost(u.Hostname()) {
		return nil
	}
	return fmt.Errorf("origin %q must be https, or http on loopback", raw)
}

// isLoopbackHost reports whether a host names this machine.
func isLoopbackHost(host string) bool {
	if host == "localhost" {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

// newVerifier returns a fresh PKCE verifier from the cryptographic RNG.
func newVerifier() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generate code verifier: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// newState returns a fresh opaque state value.
func newState() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generate state: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// challengeFor returns the S256 challenge of a verifier, unpadded base64url.
func challengeFor(verifier string) string {
	sum := sha256.Sum256([]byte(verifier))
	return base64.RawURLEncoding.EncodeToString(sum[:])
}

// AuthorizeURL builds the consent URL for one attempt.
func AuthorizeURL(authBase, callbackURL, challenge, state, appName, scope string) string {
	u, err := url.Parse(NormalizeBase(authBase) + AuthPath)
	if err != nil {
		// The callers validate the origin before reaching here; fall back to
		// the public default rather than emitting a broken URL.
		u, _ = url.Parse(AuthBaseURL + AuthPath)
	}
	q := u.Query()
	q.Set("callback_url", callbackURL)
	q.Set("code_challenge", challenge)
	q.Set("code_challenge_method", "S256")
	q.Set("state", state)
	q.Set("app_name", appName)
	if scope != "" {
		q.Set("scope", scope)
	}
	u.RawQuery = q.Encode()
	return u.String()
}

// exchangeResponse is the body of a successful exchange.
type exchangeResponse struct {
	Key    string `json:"key"`
	UserID string `json:"user_id"`
	Scope  string `json:"scope"`
}

// ConnectOptions configures a login attempt.
type ConnectOptions struct {
	// AuthBase is the authentication origin. Empty uses AuthOrigin().
	AuthBase string
	// AppName is shown on the consent screen.
	AppName string
	// Scope is the scope to ask for. Empty uses DefaultScope.
	Scope string
	// Client is the HTTP client. Empty uses a client with a bounded timeout.
	Client *http.Client
	// OpenBrowser opens the consent URL. Empty uses the platform opener.
	OpenBrowser func(url string) error
	// Timeout bounds the wait for the user. Empty uses two minutes.
	Timeout time.Duration
}

// Connect runs the loopback redirect flow (Flow A) and returns the minted API
// key. A CLI on a developer machine has a browser and can listen on loopback,
// so this flow needs no code copied by hand. The verifier and state are fresh
// for every attempt, and only the S256 challenge travels in the URL.
func Connect(ctx context.Context, opts ConnectOptions) (Credential, error) {
	authBase := opts.AuthBase
	if authBase == "" {
		authBase = AuthOrigin()
	}
	authBase = NormalizeBase(authBase)
	if err := ValidateOrigin(authBase); err != nil {
		return Credential{}, err
	}

	appName := opts.AppName
	if appName == "" {
		appName = "Chainloop"
	}
	scope := opts.Scope
	if scope == "" {
		scope = DefaultScope
	}
	client := opts.Client
	if client == nil {
		client = &http.Client{Timeout: 30 * time.Second}
	}
	timeout := opts.Timeout
	if timeout == 0 {
		timeout = 2 * time.Minute
	}

	verifier, err := newVerifier()
	if err != nil {
		return Credential{}, err
	}
	state, err := newState()
	if err != nil {
		return Credential{}, err
	}

	// Listen first, so the callback URL is known before the browser opens and
	// no approval can race the listener.
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return Credential{}, fmt.Errorf("listen on loopback: %w", err)
	}
	defer func() { _ = listener.Close() }()

	port := listener.Addr().(*net.TCPAddr).Port
	callbackURL := fmt.Sprintf("http://127.0.0.1:%d/cb", port)

	results := make(chan callbackResult, 1)
	server := &http.Server{
		Handler:           callbackHandler(state, results),
		ReadHeaderTimeout: 10 * time.Second,
	}
	go func() { _ = server.Serve(listener) }()
	defer func() { _ = server.Close() }()

	authorizeURL := AuthorizeURL(authBase, callbackURL, challengeFor(verifier), state, appName, scope)
	open := opts.OpenBrowser
	if open == nil {
		open = openBrowser
	}
	if err := open(authorizeURL); err != nil {
		return Credential{}, fmt.Errorf("open the authorization page: %w", err)
	}

	waitCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	var code string
	select {
	case <-waitCtx.Done():
		return Credential{}, fmt.Errorf("waiting for authorization: %w", waitCtx.Err())
	case res := <-results:
		if res.err != nil {
			return Credential{}, res.err
		}
		code = res.code
	}

	return Exchange(ctx, client, authBase, code, verifier, scope)
}

// callbackResult is what the loopback handler delivers: a code, or the reason
// there is none.
type callbackResult struct {
	code string
	err  error
}

// callbackHandler serves the loopback callback. It compares the state in
// constant time before anything that uses the code, so a callback another page
// dropped here cannot be redeemed, and it always answers the browser so the
// user is never left on a blank page.
func callbackHandler(state string, results chan<- callbackResult) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/cb" {
			http.NotFound(w, r)
			return
		}

		q := r.URL.Query()
		send := func(res callbackResult) {
			select {
			case results <- res:
			default:
			}
		}

		// Constant-time state comparison comes before anything that uses the
		// code, so a callback another page dropped here cannot be redeemed.
		if subtle.ConstantTimeCompare([]byte(q.Get("state")), []byte(state)) != 1 {
			http.Error(w, "this authorization response does not belong to this login attempt", http.StatusBadRequest)
			send(callbackResult{err: ErrStateMismatch})
			return
		}

		if e := q.Get("error"); e != "" {
			http.Error(w, "authorization failed; you can close this tab", http.StatusForbidden)
			send(callbackResult{err: fmt.Errorf("%w: %s", ErrAccessDenied, e)})
			return
		}

		code := q.Get("code")
		if code == "" {
			http.Error(w, "the callback carried no code; you can close this tab", http.StatusBadRequest)
			send(callbackResult{err: ErrExchangeRejected})
			return
		}

		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = io.WriteString(w, "<p>Connected. You can close this tab and return to the terminal.</p>")
		send(callbackResult{code: code})
	})
}

// Exchange trades an auth code for an API key. It posts to the authentication
// origin at /api/v1/auth/keys, never to the inference origin.
func Exchange(ctx context.Context, client *http.Client, authBase, code, verifier, requestedScope string) (Credential, error) {
	if code == "" {
		return Credential{}, ErrExchangeRejected
	}
	if client == nil {
		client = &http.Client{Timeout: 30 * time.Second}
	}
	authBase = NormalizeBase(authBase)
	if err := ValidateOrigin(authBase); err != nil {
		return Credential{}, err
	}

	body := struct {
		Code                string `json:"code"`
		CodeVerifier        string `json:"code_verifier"`
		CodeChallengeMethod string `json:"code_challenge_method"`
	}{Code: code, CodeVerifier: verifier, CodeChallengeMethod: "S256"}
	payload, err := json.Marshal(body)
	if err != nil {
		return Credential{}, fmt.Errorf("encode exchange request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, authBase+ExchangePath, strings.NewReader(string(payload)))
	if err != nil {
		return Credential{}, fmt.Errorf("build exchange request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		// The error text of a transport failure can carry the URL but never
		// the body, and the verifier is never in either.
		return Credential{}, fmt.Errorf("exchange the authorization code: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	raw, err := io.ReadAll(io.LimitReader(resp.Body, MaxResponseBytes))
	if err != nil {
		return Credential{}, fmt.Errorf("read the exchange response: %w", err)
	}

	switch resp.StatusCode {
	case http.StatusOK:
	case http.StatusBadRequest, http.StatusForbidden:
		return Credential{}, fmt.Errorf("%w: status %d", ErrExchangeRejected, resp.StatusCode)
	case http.StatusTooManyRequests:
		return Credential{}, fmt.Errorf("OrcaRouter rate limited the login; wait and try again (status 429)")
	default:
		return Credential{}, fmt.Errorf("exchange the authorization code: status %d", resp.StatusCode)
	}

	var decoded exchangeResponse
	if err := json.Unmarshal(raw, &decoded); err != nil {
		return Credential{}, fmt.Errorf("decode the exchange response: %w", err)
	}
	if decoded.Key == "" {
		return Credential{}, ErrExchangeRejected
	}

	cred := Credential{Key: decoded.Key, Source: SourcePKCE, UserID: decoded.UserID, Scope: decoded.Scope}
	if requestedScope != "" && decoded.Scope != "" && decoded.Scope != requestedScope {
		return Credential{}, fmt.Errorf("%w: granted %q, requested %q", ErrScopeDowngrade, decoded.Scope, requestedScope)
	}
	return cred, nil
}

// openBrowser opens a URL with the desktop opener of the platform.
func openBrowser(rawURL string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", rawURL) //nolint:gosec // the URL is one we built
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", rawURL) //nolint:gosec // the URL is one we built
	default:
		cmd = exec.Command("xdg-open", rawURL) //nolint:gosec // the URL is one we built
	}
	if err := cmd.Start(); err != nil {
		return err
	}
	return cmd.Process.Release()
}
