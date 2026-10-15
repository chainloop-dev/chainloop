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

package service

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	pb "github.com/chainloop-dev/chainloop/app/controlplane/api/controlplane/v1"
	conf "github.com/chainloop-dev/chainloop/app/controlplane/internal/conf/controlplane/config/v1"
	authenticator "github.com/chainloop-dev/chainloop/app/controlplane/internal/oidcauthenticator"
	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/biz"
	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/jwt/user"
	"github.com/chainloop-dev/chainloop/internal/oauth"
	kerrors "github.com/go-kratos/kratos/v2/errors"
	"github.com/go-kratos/kratos/v2/log"
	jwt "github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/oauth2"
	"google.golang.org/protobuf/proto"
)

const (
	testHMACSecret = "test-secret"
	testCLIBack    = "http://localhost:1234/auth/callback"
	testAppBack    = "https://app.chainloop.dev/login/callback"
	// parses with no host, but browsers resolve it off-origin
	testMultiSlashBack = "////evil.example/cb"
)

var testChallenge = oauth2.S256ChallengeFromVerifier("dBjftJeZ4CVP-mB92K27uhbUJU1p1r_wW1gFWFOEjXk")

type fakeLoginHandoffs struct {
	issued struct {
		userID, challenge, callback string
		ttl                         time.Duration
	}
	exchanged   *biz.LoginHandoff
	exchangeErr error
}

func (f *fakeLoginHandoffs) Issue(_ context.Context, userID string, ttl time.Duration, challenge, callback string) (string, error) {
	f.issued.userID, f.issued.ttl, f.issued.challenge, f.issued.callback = userID, ttl, challenge, callback
	return "the-handoff-code", nil
}

func (f *fakeLoginHandoffs) Exchange(_ context.Context, _, _, _ string) (*biz.LoginHandoff, error) {
	return f.exchanged, f.exchangeErr
}

func newHandoffTestService(allowLegacy *bool, handoffs loginHandoffs, logs *bytes.Buffer) *AuthService {
	if logs == nil {
		logs = &bytes.Buffer{}
	}

	return &AuthService{
		service:                &service{log: log.NewHelper(log.NewStdLogger(logs))},
		authConfig:             &conf.Auth{GeneratedJwsHmacSecret: testHMACSecret, AllowLegacyLoginCallback: allowLegacy},
		allowedCallbackOrigins: originsOf("https://app.chainloop.dev"),
		loginHandoffs:          handoffs,
		// The login handler only needs the authorization endpoint to redirect
		authenticator: &authenticator.OIDC{Config: oauth2.Config{Endpoint: oauth2.Endpoint{AuthURL: "https://idp.example/auth"}}},
	}
}

func loginURL(params map[string]string) string {
	q := url.Values{}
	for k, v := range params {
		q.Set(k, v)
	}
	return "/auth/login?" + q.Encode()
}

func TestReturnToAllowed(t *testing.T) {
	testCases := []struct {
		name     string
		returnTo string
		wantErr  bool
	}{
		{"no returnTo", "", false},
		{"relative path", "/projects/foo?tab=1", false},
		{"root", "/", false},
		{"absolute URL", "https://evil.example/collect", true},
		{"absolute URL same origin", "https://app.chainloop.dev/projects", true},
		{"scheme-relative", "//evil.example", true},
		{"backslash", `/\evil.example`, true},
		{"backslash only", `\\evil.example`, true},
		{"tab between slashes", "/\t/evil.example", true},
		{"newline between slashes", "/\n/evil.example", true},
		{"no leading slash", "projects", true},
		{"javascript scheme", "javascript:alert(1)", true},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			callback := testAppBack
			if tc.returnTo != "" {
				callback += "?" + url.Values{oauth.QueryParamReturnTo: {tc.returnTo}}.Encode()
			}

			err := returnToAllowed(callback)
			if tc.wantErr {
				assert.ErrorIs(t, err, errInvalidReturnTo)
				return
			}
			assert.NoError(t, err)
		})
	}

	t.Run("any of several values", func(t *testing.T) {
		assert.Error(t, returnToAllowed(testAppBack+"?returnTo=%2Fok&returnTo=%2F%2Fevil.example"))
	})
}

func TestLegacyCallbackAllowed(t *testing.T) {
	assert.True(t, LegacyCallbackAllowed(nil))
	assert.True(t, LegacyCallbackAllowed(&conf.Auth{}))
	assert.True(t, LegacyCallbackAllowed(&conf.Auth{AllowLegacyLoginCallback: proto.Bool(true)}))
	assert.False(t, LegacyCallbackAllowed(&conf.Auth{AllowLegacyLoginCallback: proto.Bool(false)}))
}

func TestLoginHandlerCodeChallenge(t *testing.T) {
	testCases := []struct {
		name          string
		params        map[string]string
		allowLegacy   *bool
		wantCode      int
		wantErr       string
		wantChallenge string
	}{
		{
			name:          "code exchange",
			params:        map[string]string{oauth.QueryParamCallback: testCLIBack, oauth.QueryParamCodeChallenge: testChallenge, oauth.QueryParamCodeChallengeMethod: oauth.CodeChallengeMethodS256},
			wantCode:      http.StatusTemporaryRedirect,
			wantChallenge: testChallenge,
		},
		{
			name:          "code exchange with legacy denied",
			params:        map[string]string{oauth.QueryParamCallback: testCLIBack, oauth.QueryParamCodeChallenge: testChallenge, oauth.QueryParamCodeChallengeMethod: oauth.CodeChallengeMethodS256},
			allowLegacy:   proto.Bool(false),
			wantCode:      http.StatusTemporaryRedirect,
			wantChallenge: testChallenge,
		},
		{
			name:     "legacy login allowed by default",
			params:   map[string]string{oauth.QueryParamCallback: testCLIBack},
			wantCode: http.StatusTemporaryRedirect,
		},
		{
			name:     "legacy login denied",
			params:   map[string]string{oauth.QueryParamCallback: testCLIBack},
			wantCode: http.StatusBadRequest, allowLegacy: proto.Bool(false),
			wantErr: "upgrade",
		},
		{
			name:        "headless login not affected by the legacy setting",
			params:      map[string]string{oauth.QueryParamCallback: ""},
			allowLegacy: proto.Bool(false),
			wantCode:    http.StatusTemporaryRedirect,
		},
		{
			name:        "download flow not affected by the legacy setting",
			params:      map[string]string{oauth.QueryParamCallback: "/download/sha256:abc"},
			allowLegacy: proto.Bool(false),
			wantCode:    http.StatusTemporaryRedirect,
		},
		{
			// It parses with no host, but browsers resolve it off-origin
			name:     "multi-slash callback posing as the download flow",
			params:   map[string]string{oauth.QueryParamCallback: testMultiSlashBack},
			wantCode: http.StatusBadRequest,
			wantErr:  errInvalidCallback.Error(),
		},
		{
			name:     "plain challenge method",
			params:   map[string]string{oauth.QueryParamCallback: testCLIBack, oauth.QueryParamCodeChallenge: testChallenge, oauth.QueryParamCodeChallengeMethod: "plain"},
			wantCode: http.StatusBadRequest,
			wantErr:  "unsupported code challenge method",
		},
		{
			name:     "missing challenge method",
			params:   map[string]string{oauth.QueryParamCallback: testCLIBack, oauth.QueryParamCodeChallenge: testChallenge},
			wantCode: http.StatusBadRequest,
		},
		{
			name:     "malformed challenge",
			params:   map[string]string{oauth.QueryParamCallback: testCLIBack, oauth.QueryParamCodeChallenge: "short", oauth.QueryParamCodeChallengeMethod: oauth.CodeChallengeMethodS256},
			wantCode: http.StatusBadRequest,
			wantErr:  "invalid code challenge",
		},
		{
			name:     "challenge without callback",
			params:   map[string]string{oauth.QueryParamCodeChallenge: testChallenge, oauth.QueryParamCodeChallengeMethod: oauth.CodeChallengeMethodS256},
			wantCode: http.StatusBadRequest,
		},
		{
			name:     "absolute returnTo",
			params:   map[string]string{oauth.QueryParamCallback: testAppBack + "?returnTo=https%3A%2F%2Fevil.example"},
			wantCode: http.StatusBadRequest,
			wantErr:  errInvalidReturnTo.Error(),
		},
		{
			name:     "relative returnTo",
			params:   map[string]string{oauth.QueryParamCallback: testAppBack + "?returnTo=%2Fprojects"},
			wantCode: http.StatusTemporaryRedirect,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			svc := newHandoffTestService(tc.allowLegacy, nil, nil)
			w := httptest.NewRecorder()
			r := httptest.NewRequest(http.MethodGet, loginURL(tc.params), nil)

			resp := loginHandler(svc, w, r)
			if tc.wantCode != http.StatusBadRequest {
				// The handler answered the request with the redirect
				assert.Nil(t, resp)
				assert.Equal(t, http.StatusFound, w.Code)
				assert.Equal(t, tc.wantChallenge, cookieValue(w, cookieCodeChallenge))
				return
			}

			require.NotNil(t, resp)
			assert.Equal(t, tc.wantCode, resp.code)
			assert.Contains(t, resp.ErrorMessage(context.Background(), svc.log), tc.wantErr)
			assert.Empty(t, w.Result().Cookies())
		})
	}
}

func cookieValue(w *httptest.ResponseRecorder, name string) string {
	for _, c := range w.Result().Cookies() {
		if c.Name == name {
			return c.Value
		}
	}
	return ""
}

func TestDeliverLogin(t *testing.T) {
	userID := uuid.NewString()

	testCases := []struct {
		name          string
		callback      string
		challenge     string
		allowLegacy   *bool
		wantCode      int
		wantHandoff   bool
		wantToken     bool
		wantLegacyInc float64
	}{
		{name: "code exchange", callback: testCLIBack, challenge: testChallenge, wantCode: http.StatusTemporaryRedirect, wantHandoff: true},
		{name: "code exchange with legacy denied", callback: testCLIBack, challenge: testChallenge, allowLegacy: proto.Bool(false), wantCode: http.StatusTemporaryRedirect, wantHandoff: true},
		{name: "legacy login", callback: testCLIBack, wantCode: http.StatusTemporaryRedirect, wantToken: true, wantLegacyInc: 1},
		{name: "legacy login denied", callback: testCLIBack, allowLegacy: proto.Bool(false), wantCode: http.StatusBadRequest},
		{name: "download flow", callback: "/download/sha256:abc", allowLegacy: proto.Bool(false), wantCode: http.StatusTemporaryRedirect, wantToken: true},
		{name: "multi-slash callback cookie", callback: testMultiSlashBack, allowLegacy: proto.Bool(false), wantCode: http.StatusBadRequest},
		{name: "multi-slash callback cookie with legacy allowed", callback: testMultiSlashBack, wantCode: http.StatusBadRequest},
		{name: "headless", callback: "", wantCode: http.StatusOK},
		{name: "off-origin returnTo", callback: testAppBack + "?returnTo=%2F%2Fevil.example", challenge: testChallenge, wantCode: http.StatusBadRequest},
		{name: "foreign callback", callback: "https://evil.example/cb", challenge: testChallenge, wantCode: http.StatusBadRequest},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			handoffs := &fakeLoginHandoffs{}
			svc := newHandoffTestService(tc.allowLegacy, handoffs, nil)

			r := httptest.NewRequest(http.MethodGet, "/auth/callback", nil)
			r.AddCookie(&http.Cookie{Name: cookieLongLived, Value: "true"})     //nolint:gosec // cookies of a test request
			r.AddCookie(&http.Cookie{Name: cookieCallback, Value: tc.callback}) //nolint:gosec // cookies of a test request
			if tc.challenge != "" {
				r.AddCookie(&http.Cookie{Name: cookieCodeChallenge, Value: tc.challenge}) //nolint:gosec // cookies of a test request
			}
			w := httptest.NewRecorder()

			before := testutil.ToFloat64(legacyCallbackDeliveries)
			resp := svc.deliverLogin(context.Background(), w, r, userID)
			assert.Equal(t, tc.wantLegacyInc, testutil.ToFloat64(legacyCallbackDeliveries)-before)

			// The handler answers a delivered login itself and returns nothing
			if tc.wantCode == http.StatusOK {
				assert.Nil(t, resp)
				assert.Equal(t, http.StatusOK, w.Code)
				return
			}

			if tc.wantCode != http.StatusTemporaryRedirect {
				require.NotNil(t, resp)
				assert.Equal(t, tc.wantCode, resp.code)
				assert.Empty(t, w.Header().Get("Location"))
				return
			}

			assert.Nil(t, resp)
			assert.Equal(t, http.StatusFound, w.Code)

			location, err := url.Parse(w.Header().Get("Location"))
			require.NoError(t, err)
			q := location.Query()

			if tc.wantHandoff {
				assert.Equal(t, "the-handoff-code", q.Get(oauth.QueryParamHandoff))
				assert.Empty(t, q.Get(oauth.QueryParamLegacyToken))
				assert.Equal(t, userID, handoffs.issued.userID)
				assert.Equal(t, longLivedDuration, handoffs.issued.ttl)
				assert.Equal(t, tc.challenge, handoffs.issued.challenge)
				assert.Equal(t, tc.callback, handoffs.issued.callback)
			}

			if tc.wantToken {
				assert.NotEmpty(t, q.Get(oauth.QueryParamLegacyToken))
				assert.Empty(t, q.Get(oauth.QueryParamHandoff))
			}
		})
	}
}

// headerCounter counts the WriteHeader calls, a second one is what net/http logs as "superfluous"
type headerCounter struct {
	*httptest.ResponseRecorder
	writes int
}

func (h *headerCounter) WriteHeader(code int) {
	h.writes++
	h.ResponseRecorder.WriteHeader(code)
}

// A handler that already answered the request must not get a second response from ServeHTTP
func TestOauthHandlerAnswersOnce(t *testing.T) {
	loginRequest := func() *http.Request {
		return httptest.NewRequest(http.MethodGet, loginURL(map[string]string{oauth.QueryParamCallback: testCLIBack}), nil)
	}

	deliverRequest := func(callback, challenge string) *http.Request {
		r := httptest.NewRequest(http.MethodGet, "/auth/callback", nil)
		r.AddCookie(&http.Cookie{Name: cookieLongLived, Value: "true"})  //nolint:gosec // cookies of a test request
		r.AddCookie(&http.Cookie{Name: cookieCallback, Value: callback}) //nolint:gosec // cookies of a test request
		if challenge != "" {
			r.AddCookie(&http.Cookie{Name: cookieCodeChallenge, Value: challenge}) //nolint:gosec // cookies of a test request
		}
		return r
	}

	deliver := func(svc *AuthService, w http.ResponseWriter, r *http.Request) *oauthResp {
		return svc.deliverLogin(context.Background(), w, r, uuid.NewString())
	}

	testCases := []struct {
		name     string
		handler  func(*AuthService, http.ResponseWriter, *http.Request) *oauthResp
		request  *http.Request
		wantCode int
	}{
		{name: "login redirect", handler: loginHandler, request: loginRequest(), wantCode: http.StatusFound},
		{name: "handoff redirect", handler: deliver, request: deliverRequest(testCLIBack, testChallenge), wantCode: http.StatusFound},
		{name: "legacy token redirect", handler: deliver, request: deliverRequest(testCLIBack, ""), wantCode: http.StatusFound},
		{name: "headless token page", handler: deliver, request: deliverRequest("", ""), wantCode: http.StatusOK},
		{name: "rejected callback", handler: deliver, request: deliverRequest(testMultiSlashBack, ""), wantCode: http.StatusBadRequest},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			svc := newHandoffTestService(nil, &fakeLoginHandoffs{}, nil)
			w := &headerCounter{ResponseRecorder: httptest.NewRecorder()}

			oauthHandler{tc.handler, svc}.ServeHTTP(w, tc.request)

			assert.Equal(t, tc.wantCode, w.Code)
			// A page body written with no explicit header counts as none
			assert.LessOrEqual(t, w.writes, 1)
		})
	}
}

func TestExchangeHandoff(t *testing.T) {
	userID := uuid.New()
	req := &pb.AuthServiceExchangeHandoffRequest{Handoff: "code", CodeVerifier: "verifier", Callback: testCLIBack}

	t.Run("success", func(t *testing.T) {
		svc := newHandoffTestService(nil, &fakeLoginHandoffs{exchanged: &biz.LoginHandoff{UserID: userID, TokenTTL: time.Hour}}, nil)
		resp, err := svc.ExchangeHandoff(context.Background(), req)
		require.NoError(t, err)

		claims := &user.CustomClaims{}
		_, err = jwt.ParseWithClaims(resp.GetToken(), claims, func(_ *jwt.Token) (any, error) {
			return []byte(testHMACSecret), nil
		})
		require.NoError(t, err)
		assert.Equal(t, userID.String(), claims.UserID)
		assert.WithinDuration(t, time.Now().Add(time.Hour), claims.ExpiresAt.Time, time.Minute)
	})

	reasons := []string{
		biz.LoginHandoffReasonUnknown,
		biz.LoginHandoffReasonExpired,
		biz.LoginHandoffReasonAlreadyUsed,
		biz.LoginHandoffReasonWrongVerifier,
		biz.LoginHandoffReasonWrongCallback,
	}

	for _, reason := range reasons {
		t.Run(reason, func(t *testing.T) {
			var logs bytes.Buffer
			svc := newHandoffTestService(nil, &fakeLoginHandoffs{exchangeErr: biz.ErrLoginHandoffExchange{Reason: reason}}, &logs)

			_, err := svc.ExchangeHandoff(context.Background(), req)
			// Same error for every reason, the reason only goes to the server log
			assert.Equal(t, errLoginHandoffExchange, err)
			assert.True(t, kerrors.IsUnauthorized(err))
			assert.NotContains(t, err.Error(), reason)
			assert.Contains(t, logs.String(), reason)
		})
	}

	unexpected := []struct {
		name string
		err  error
	}{
		{"repository error", errors.New("db down")},
		{"cancelled", context.Canceled},
	}

	for _, tc := range unexpected {
		t.Run(tc.name, func(t *testing.T) {
			var logs bytes.Buffer
			svc := newHandoffTestService(nil, &fakeLoginHandoffs{exchangeErr: tc.err}, &logs)

			_, err := svc.ExchangeHandoff(context.Background(), req)
			// Unexpected failures are indistinguishable from the rest for the client, the cause only goes to the server log
			assert.Equal(t, errLoginHandoffExchange, err)
			assert.Contains(t, logs.String(), tc.err.Error())
		})
	}
}

func TestInfozLoginFlags(t *testing.T) {
	testCases := []struct {
		name        string
		allowLegacy *bool
		want        bool
	}{
		{"legacy allowed by default", nil, true},
		{"legacy allowed", proto.Bool(true), true},
		{"legacy denied", proto.Bool(false), false},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			svc := NewStatusService("http://login", "v1", nil, &conf.Bootstrap{Auth: &conf.Auth{AllowLegacyLoginCallback: tc.allowLegacy}})
			resp, err := svc.Infoz(context.Background(), &pb.InfozRequest{})
			require.NoError(t, err)
			assert.True(t, resp.GetLoginCodeExchangeSupported())
			assert.Equal(t, tc.want, resp.GetLoginLegacyCallbackAllowed())
		})
	}
}
