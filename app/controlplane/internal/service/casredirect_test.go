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
	"context"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	pb "github.com/chainloop-dev/chainloop/app/controlplane/api/controlplane/v1"
	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/biz"
	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/jwt/user"
	"github.com/chainloop-dev/chainloop/internal/oauth"
	"github.com/go-kratos/kratos/v2/middleware"
	"github.com/go-kratos/kratos/v2/transport"
	khttp "github.com/go-kratos/kratos/v2/transport/http"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
)

// Exercise the real handoff use case without a database or an upstream OIDC provider.
type downloadHandoffRepo struct {
	row  *biz.LoginHandoff
	rows map[string]*biz.LoginHandoff
	used map[string]bool
}

func (r *downloadHandoffRepo) Create(_ context.Context, hash string, h *biz.LoginHandoff) error {
	if r.rows == nil {
		r.rows = make(map[string]*biz.LoginHandoff)
		r.used = make(map[string]bool)
	}
	r.row = h
	r.rows[hash] = h
	return nil
}

func (r *downloadHandoffRepo) Consume(_ context.Context, hash string, _ time.Time) (*biz.LoginHandoff, bool, error) {
	h, ok := r.rows[hash]
	if !ok {
		return nil, false, biz.NewErrNotFound("handoff")
	}
	consumed := !r.used[hash]
	r.used[hash] = true
	return h, consumed, nil
}

func (*downloadHandoffRepo) DeleteExpired(context.Context, time.Time) (int, error) {
	return 0, nil
}

func newDownloadTestService() (*CASRedirectService, *downloadHandoffRepo) {
	repo := &downloadHandoffRepo{}
	auth := newHandoffTestService(proto.Bool(false), nil, nil)
	auth.loginHandoffs = biz.NewLoginHandoffUseCase(repo, auth.log.Logger())
	return &CASRedirectService{auth: auth}, repo
}

// Starts a browser download, follows /auth/login, and delivers a validated OIDC login.
func startDownloadLogin(t *testing.T, svc *CASRedirectService) (string, *http.Cookie, string) {
	t.Helper()
	r := httptest.NewRequest(http.MethodGet, "/download/sha256:abc?z=2&filename=my%20file", nil)
	w := httptest.NewRecorder()
	redirected, err := svc.authenticateUser(w, r)
	require.NoError(t, err)
	require.True(t, redirected)
	require.Equal(t, http.StatusTemporaryRedirect, w.Code)
	assert.Empty(t, r.Header.Get(authorizationKey))
	assert.Equal(t, "no-store", w.Header().Get("Cache-Control"))
	assert.Equal(t, "no-referrer", w.Header().Get("Referrer-Policy"))

	login, err := url.Parse(w.Header().Get("Location"))
	require.NoError(t, err)
	assert.Equal(t, AuthLoginPath, login.Path)
	assert.Empty(t, login.Query().Get(oauth.QueryParamLegacyToken))
	assert.Empty(t, login.Query().Get(oauth.QueryParamLongLived))
	require.Len(t, w.Result().Cookies(), 1)
	verifier := w.Result().Cookies()[0]
	assert.True(t, verifier.Secure)
	assert.True(t, verifier.HttpOnly)
	assert.Equal(t, http.SameSiteLaxMode, verifier.SameSite)
	assert.Equal(t, "/download/", verifier.Path)
	assert.Positive(t, verifier.MaxAge)
	assert.True(t, oauth.VerifyS256(verifier.Value, login.Query().Get(oauth.QueryParamCodeChallenge)))
	assert.NotContains(t, login.String(), verifier.Value)

	// Even a login page that requests a long session cannot lengthen a download session.
	query := login.Query()
	query.Set(oauth.QueryParamLongLived, "true")
	login.RawQuery = query.Encode()
	loginResponse := httptest.NewRecorder()
	require.Nil(t, loginHandler(svc.auth, loginResponse, httptest.NewRequest(http.MethodGet, login.String(), nil)))
	callbackRequest := httptest.NewRequest(http.MethodGet, AuthCallbackPath, nil)
	for _, cookie := range loginResponse.Result().Cookies() {
		callbackRequest.AddCookie(cookie)
	}
	userID := uuid.NewString()
	callbackResponse := httptest.NewRecorder()
	require.Nil(t, svc.auth.deliverLogin(context.Background(), callbackResponse, callbackRequest, userID))
	callback, err := url.Parse(callbackResponse.Header().Get("Location"))
	require.NoError(t, err)
	assert.Empty(t, callback.Query().Get(oauth.QueryParamLegacyToken))
	assert.NotEmpty(t, callback.Query().Get(oauth.QueryParamHandoff))
	assert.NotEmpty(t, callback.Query().Get(queryParamDownloadFlow))
	return callback.String(), verifier, userID
}

func TestBrowserDownloadHandoff(t *testing.T) {
	svc, repo := newDownloadTestService()
	callback, verifier, userID := startDownloadLogin(t, svc)
	assert.Equal(t, shortLivedDuration, repo.row.TokenTTL)
	assert.WithinDuration(t, time.Now().Add(biz.LoginHandoffTTL), repo.row.ExpiresAt, time.Second)

	// Verify that the existing middleware chain sees the internally exchanged user token.
	var internalToken string
	srv := khttp.NewServer(khttp.Middleware(func(_ middleware.Handler) middleware.Handler {
		return func(ctx context.Context, _ interface{}) (interface{}, error) {
			tr, ok := transport.FromServerContext(ctx)
			require.True(t, ok)
			assert.Equal(t, getDownloadURLOperation, tr.Operation())
			internalToken = strings.TrimPrefix(tr.RequestHeader().Get(authorizationKey), "Bearer ")
			claims := &user.CustomClaims{}
			_, err := jwt.ParseWithClaims(internalToken, claims, func(*jwt.Token) (interface{}, error) {
				return []byte(testHMACSecret), nil
			})
			require.NoError(t, err)
			assert.Equal(t, userID, claims.UserID)
			assert.WithinDuration(t, time.Now().Add(shortLivedDuration), claims.ExpiresAt.Time, time.Second)
			return &pb.GetDownloadURLResponse{Result: &pb.GetDownloadURLResponse_Result{
				Url: "https://cas.example/download/sha256:abc?t=cas-download-token",
			}}, nil
		}
	}))
	srv.Route("/").GET("/download/{digest}", svc.HTTPDownload)
	r := httptest.NewRequest(http.MethodGet, callback, nil)
	r.AddCookie(verifier)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, r)
	require.Equal(t, http.StatusFound, w.Code)
	assert.NotEmpty(t, internalToken)
	assert.NotContains(t, w.Body.String(), internalToken)
	assert.NotContains(t, w.Header().Get("Location"), internalToken)
	assert.Equal(t, "https://cas.example/download/sha256:abc?t=cas-download-token", w.Header().Get("Location"))
	assert.Equal(t, "no-store", w.Header().Get("Cache-Control"))
	assert.Equal(t, "no-referrer", w.Header().Get("Referrer-Policy"))
	require.Len(t, w.Result().Cookies(), 1)
	assert.Negative(t, w.Result().Cookies()[0].MaxAge)

	// Replaying the URL with the original verifier still fails.
	replay := httptest.NewRequest(http.MethodGet, callback, nil)
	replay.AddCookie(verifier)
	redirected, err := svc.authenticateUser(httptest.NewRecorder(), replay)
	assert.False(t, redirected)
	assert.ErrorIs(t, err, errLoginHandoffExchange)
	assert.Empty(t, replay.Header.Get(authorizationKey))
}

func TestBrowserDownloadHandoffRejected(t *testing.T) {
	for _, name := range []string{"missing cookie", "wrong verifier", "wrong digest", "wrong query", "expired", "unknown", "duplicate code", "empty code", "missing flow", "duplicate flow", "invalid flow", "wrong flow"} {
		t.Run(name, func(t *testing.T) {
			svc, repo := newDownloadTestService()
			callback, cookie, _ := startDownloadLogin(t, svc)
			u, err := url.Parse(callback)
			require.NoError(t, err)
			q := u.Query()
			clearsCookie := true
			switch name {
			case "missing cookie":
				cookie = nil
			case "wrong verifier":
				cookie.Value = strings.Repeat("x", 43)
			case "wrong digest":
				u.Path = "/download/sha256:other"
			case "wrong query":
				q.Set("filename", "other")
			case "expired":
				repo.row.ExpiresAt = time.Now().Add(-time.Second)
			case "unknown":
				q.Set(oauth.QueryParamHandoff, "unknown")
			case "duplicate code":
				q.Add(oauth.QueryParamHandoff, "other")
			case "empty code":
				q.Set(oauth.QueryParamHandoff, "")
			case "missing flow":
				q.Del(queryParamDownloadFlow)
				clearsCookie = false
			case "duplicate flow":
				q.Add(queryParamDownloadFlow, uuid.NewString())
				clearsCookie = false
			case "invalid flow":
				q.Set(queryParamDownloadFlow, "invalid")
				clearsCookie = false
			case "wrong flow":
				q.Set(queryParamDownloadFlow, uuid.NewString())
			}
			u.RawQuery = q.Encode()
			r := httptest.NewRequest(http.MethodGet, u.String(), nil)
			if cookie != nil {
				r.AddCookie(cookie)
			}
			w := httptest.NewRecorder()
			redirected, err := svc.authenticateUser(w, r)
			assert.False(t, redirected)
			assert.ErrorIs(t, err, errLoginHandoffExchange)
			assert.Empty(t, r.Header.Get(authorizationKey))
			assert.Empty(t, w.Header().Get("Location"))
			if code := q.Get(oauth.QueryParamHandoff); code != "" {
				assert.NotContains(t, err.Error(), code)
			}
			if !clearsCookie {
				// An invalid flow cannot select a cookie to clear.
				assert.Empty(t, w.Result().Cookies())
			} else {
				require.Len(t, w.Result().Cookies(), 1)
				assert.Negative(t, w.Result().Cookies()[0].MaxAge)
			}
		})
	}
}

func TestDownloadHeaderAndLegacyToken(t *testing.T) {
	svc, _ := newDownloadTestService()
	r := httptest.NewRequest(http.MethodGet, testDownloadBack, nil)
	r.Header.Set(authorizationKey, "Bearer header-token")
	w := httptest.NewRecorder()
	redirected, err := svc.authenticateUser(w, r)
	assert.NoError(t, err)
	assert.False(t, redirected)
	assert.Equal(t, "Bearer header-token", r.Header.Get(authorizationKey))
	assert.Empty(t, w.Result().Cookies())

	for _, query := range []string{"t=user-token", "t=", "t=user-token&handoff=code", "bad=%zz"} {
		r := httptest.NewRequest(http.MethodGet, "/download/sha256:abc?"+query, nil)
		w := httptest.NewRecorder()
		redirected, err := svc.authenticateUser(w, r)
		assert.Error(t, err)
		assert.False(t, redirected)
		assert.Empty(t, r.Header.Get(authorizationKey))
		assert.Empty(t, w.Header().Get("Location"))
	}
}

func TestDownloadVerifierCookieSecurity(t *testing.T) {
	for _, tc := range []struct {
		name    string
		devMode bool
		secure  bool
	}{
		{name: "production", secure: true},
		{name: "local HTTP development", devMode: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			svc, _ := newDownloadTestService()
			svc.auth.SetDevMode(tc.devMode)
			w := httptest.NewRecorder()
			_, err := svc.authenticateUser(w, httptest.NewRequest(http.MethodGet, testDownloadBack, nil))
			require.NoError(t, err)
			require.Len(t, w.Result().Cookies(), 1)
			assert.Equal(t, tc.secure, w.Result().Cookies()[0].Secure)
			assert.True(t, w.Result().Cookies()[0].HttpOnly)
		})
	}
}

func TestOverlappingDownloadHandoffs(t *testing.T) {
	for _, tc := range []struct {
		name      string
		order     [2]int
		failFirst bool
	}{
		{name: "first callback first", order: [2]int{0, 1}},
		{name: "second callback first", order: [2]int{1, 0}},
		{name: "failed callback leaves other flow intact", order: [2]int{0, 1}, failFirst: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			svc, _ := newDownloadTestService()
			jar, err := cookiejar.New(nil)
			require.NoError(t, err)
			origin, err := url.Parse("https://controlplane.example/download/")
			require.NoError(t, err)
			var callbacks [2]string
			var cookies [2]*http.Cookie
			var users [2]string
			for i := range callbacks {
				callbacks[i], cookies[i], users[i] = startDownloadLogin(t, svc)
				if i == 0 && tc.failFirst {
					cookies[i] = &http.Cookie{
						Name: cookies[i].Name, Value: strings.Repeat("x", 43), Path: cookies[i].Path,
						Secure: true, HttpOnly: true, SameSite: http.SameSiteLaxMode,
					}
				}
				jar.SetCookies(origin, []*http.Cookie{cookies[i]})
			}
			require.NotEqual(t, cookies[0].Name, cookies[1].Name)
			require.Len(t, jar.Cookies(origin), 2)
			for _, i := range tc.order {
				r := httptest.NewRequest(http.MethodGet, callbacks[i], nil)
				for _, cookie := range jar.Cookies(origin) {
					r.AddCookie(cookie)
				}
				w := httptest.NewRecorder()
				redirected, err := svc.authenticateUser(w, r)
				assert.False(t, redirected)
				if i == 0 && tc.failFirst {
					require.ErrorIs(t, err, errLoginHandoffExchange)
					assert.Empty(t, r.Header.Get(authorizationKey))
				} else {
					require.NoError(t, err)
					claims := &user.CustomClaims{}
					_, err := jwt.ParseWithClaims(strings.TrimPrefix(r.Header.Get(authorizationKey), "Bearer "), claims, func(*jwt.Token) (interface{}, error) {
						return []byte(testHMACSecret), nil
					})
					require.NoError(t, err)
					assert.Equal(t, users[i], claims.UserID)
				}
				require.Len(t, w.Result().Cookies(), 1)
				assert.Equal(t, cookies[i].Name, w.Result().Cookies()[0].Name)
				jar.SetCookies(origin, w.Result().Cookies())
			}
			assert.Empty(t, jar.Cookies(origin))
		})
	}
}
