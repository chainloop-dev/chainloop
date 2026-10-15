//
// Copyright 2024-2026 The Chainloop Authors.
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
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/mail"
	"net/url"
	"slices"
	"strings"
	"time"
	"unicode"

	pb "github.com/chainloop-dev/chainloop/app/controlplane/api/controlplane/v1"
	conf "github.com/chainloop-dev/chainloop/app/controlplane/internal/conf/controlplane/config/v1"
	authenticator "github.com/chainloop-dev/chainloop/app/controlplane/internal/oidcauthenticator"
	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/biz"
	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/jwt"
	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/jwt/user"
	"github.com/chainloop-dev/chainloop/internal/oauth"
	sl "github.com/chainloop-dev/chainloop/pkg/servicelogger"
	kerrors "github.com/go-kratos/kratos/v2/errors"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"golang.org/x/oauth2"
)

// The authentication process does the following
// 1 - Authenticate against a valid OIDC provider, currently only Google is supported
// 2 - Generate a chainloop signed JWT to be sent to the client
const (
	// Cookie names
	cookieOauthStateName = "oauthState"
	cookieCallback       = "oauthCallback"
	cookieLongLived      = "longLived"
	cookieCodeChallenge  = "oauthCodeChallenge"

	// Auth paths
	AuthLoginPath    = "/auth/login"
	AuthCallbackPath = "/auth/callback"

	oidcErrorParam = "error_description"

	// default
	shortLivedDuration = 10 * time.Second
	// opt-in
	longLivedDuration = 7 * 24 * time.Hour
	// dev only
	devUserDuration = 30 * 24 * time.Hour
)

// legacyCallbackDeliveries counts the logins that sent the session token in the callback URL
var legacyCallbackDeliveries = promauto.NewCounter(prometheus.CounterOpts{
	Name: "chainloop_controlplane_auth_legacy_callback_deliveries_total",
	Help: "Logins that delivered the session token in the callback URL instead of a handoff code.",
})

// errLegacyCallbackDenied is shown to a user whose client does not send a code challenge
var errLegacyCallbackDenied = errors.New("this client is not supported by the server anymore, please upgrade it and log in again")

// errLoginHandoffExchange is the only error a client gets from a failed exchange, the reason goes to the server log
var errLoginHandoffExchange = kerrors.Unauthorized("LOGIN_HANDOFF_INVALID", "invalid login code, please log in again")

// loginHandoffs issues and exchanges the single-use codes a login delivers to the client callback
type loginHandoffs interface {
	Issue(ctx context.Context, userID string, tokenTTL time.Duration, codeChallenge, callback string) (string, error)
	Exchange(ctx context.Context, code, codeVerifier, callback string) (*biz.LoginHandoff, error)
}

type oauthResp struct {
	code          int
	err           error
	showErrToUser bool
	redirectURL   *url.URL
}

// NewOauthResp builds an oauthResp object without redirection
func newOauthResp(code int, err error, showErrToUser bool) *oauthResp {
	return &oauthResp{
		code:          code,
		err:           err,
		showErrToUser: showErrToUser,
	}
}

// ErrorMessage is used to provide by default a generic error message to the user
// unless showErrToUser is true
func (e *oauthResp) ErrorMessage(ctx context.Context, l *log.Helper) string {
	if e.err != nil {
		// If the error is an internal server error, log it and raise it masked
		if e.code == http.StatusInternalServerError {
			return sl.LogAndMaskErr(ctx, e.err, l).Error()
		}
		// otherwise return the error message to the user
		// or the default status text
		if e.showErrToUser {
			return e.err.Error()
		}

		return http.StatusText(e.code)
	}

	return ""
}

type oauthHandler struct {
	H   func(*AuthService, http.ResponseWriter, *http.Request) *oauthResp
	svc *AuthService
}

type AuthService struct {
	*service
	pb.UnimplementedAuthServiceServer
	// oauth info
	authenticator     *authenticator.OIDC
	authConfig        *conf.Auth
	userUseCase       *biz.UserUseCase
	orgUseCase        *biz.OrganizationUseCase
	membershipUseCase *biz.MembershipUseCase
	orgInvitesUseCase *biz.OrgInvitationUseCase
	AuthURLs          *AuthURLs
	auditorUseCase    *biz.AuditorUseCase
	loginHandoffs     loginHandoffs
	devMode           bool
	// scheme://host destinations the post-login redirect may target
	allowedCallbackOrigins []string
}

func NewAuthService(userUC *biz.UserUseCase, orgUC *biz.OrganizationUseCase, mUC *biz.MembershipUseCase, inviteUC *biz.OrgInvitationUseCase, handoffUC *biz.LoginHandoffUseCase, authConfig *conf.Auth, bootstrapConfig *conf.Bootstrap, auc *biz.AuditorUseCase, opts ...NewOpt) (*AuthService, error) {
	oidcConfig := authConfig.GetOidc()
	if oidcConfig == nil {
		return nil, errors.New("oauth configuration missing")
	}

	// Craft Auth related endpoints
	authURLs, err := getAuthURLs(bootstrapConfig.GetServer().GetHttp(), authConfig.GetOidc().GetLoginUrlOverride())
	if err != nil {
		return nil, fmt.Errorf("failed to get auth URLs: %w", err)
	}

	authInst, err := authenticator.NewOIDC(oidcConfig.Domain, oidcConfig.ClientId, oidcConfig.ClientSecret, authURLs.callback)
	if err != nil {
		return nil, fmt.Errorf("failed to create OIDC authenticator: %w", err)
	}

	extraOrigins, err := parseCallbackOrigins(bootstrapConfig.GetAllowedCallbackOrigins())
	if err != nil {
		return nil, fmt.Errorf("invalid allowed_callback_origins: %w", err)
	}

	svc := newService(opts...)
	if authConfig.DevUser != "" {
		err := generateAndLogDevUser(userUC, svc.log, authConfig)
		if err != nil {
			return nil, fmt.Errorf("failed to create development user: %w", err)
		}
	}

	return &AuthService{
		service:           svc,
		authenticator:     authInst,
		userUseCase:       userUC,
		orgUseCase:        orgUC,
		authConfig:        authConfig,
		AuthURLs:          authURLs,
		membershipUseCase: mUC,
		orgInvitesUseCase: inviteUC,
		auditorUseCase:    auc,
		loginHandoffs:     handoffUC,
		allowedCallbackOrigins: append(originsOf(authURLs.Login, bootstrapConfig.GetServer().GetHttp().GetExternalUrl(),
			bootstrapConfig.GetUiDashboardUrl()), extraOrigins...),
	}, nil
}

// parseCallbackOrigins validates the operator-provided origins. Unlike originsOf, a bad entry is an
// error, so a typo fails at start up instead of silently leaving the origin out
func parseCallbackOrigins(raw []string) ([]string, error) {
	origins := make([]string, 0, len(raw))
	for _, o := range raw {
		u, err := url.Parse(o)
		if err != nil {
			return nil, fmt.Errorf("origin %q: %w", o, err)
		}

		if u.Scheme != "http" && u.Scheme != "https" {
			return nil, fmt.Errorf("origin %q: scheme must be http or https", o)
		}

		if u.Host == "" || strings.Contains(u.Host, "*") {
			return nil, fmt.Errorf("origin %q: a host without wildcards is required", o)
		}

		if u.User != nil || (u.Path != "" && u.Path != "/") || u.RawQuery != "" || u.Fragment != "" || u.ForceQuery {
			return nil, fmt.Errorf("origin %q: must be scheme://host[:port] only", o)
		}

		origins = append(origins, originOf(u))
	}

	return origins, nil
}

var errInvalidCallback = errors.New("invalid callback URL")

// originsOf extracts the origin of the given URLs, skipping empty or malformed ones
func originsOf(urls ...string) []string {
	origins := make([]string, 0, len(urls))
	for _, raw := range urls {
		u, err := url.Parse(raw)
		if err != nil || u.Scheme == "" || u.Host == "" {
			continue
		}

		origins = append(origins, originOf(u))
	}

	return origins
}

// originOf normalizes the URL down to scheme://host. Hosts are case insensitive, so a
// config typo like "https://App.Example.com" still matches the browser-sent origin
func originOf(u *url.URL) string {
	return strings.ToLower(u.Scheme + "://" + u.Host)
}

// callbackAllowed rejects post-login redirect targets that would hand the user JWT over to
// a third party. Relative paths (CAS download redirect) and loopback (CLI login) are always
// allowed, anything else must match a known origin.
func callbackAllowed(callback string, allowedOrigins []string) error {
	if callback == "" {
		return nil
	}

	// Browsers fold "\" into "/", so "/\evil.example" would escape a seemingly relative path
	if strings.Contains(callback, `\`) {
		return errInvalidCallback
	}

	u, err := url.Parse(callback)
	if err != nil {
		return errInvalidCallback
	}

	// Relative path. Both parts must be empty, otherwise "//evil.example" would pass as a path
	if u.Scheme == "" && u.Host == "" {
		return nil
	}

	if u.Scheme != "http" && u.Scheme != "https" {
		return errInvalidCallback
	}

	if host := u.Hostname(); host == "localhost" || net.ParseIP(host).IsLoopback() {
		return nil
	}

	if slices.Contains(allowedOrigins, originOf(u)) {
		return nil
	}

	return fmt.Errorf("callback URL not allowed: %s", originOf(u))
}

var errInvalidReturnTo = errors.New("invalid post-login redirect, it must be a relative path")

// returnToAllowed rejects a callback whose post-login redirect is not a relative path, so the
// destination stays on the origin of the callback
func returnToAllowed(callback string) error {
	if callback == "" {
		return nil
	}

	u, err := url.Parse(callback)
	if err != nil {
		return errInvalidCallback
	}

	for _, v := range u.Query()[oauth.QueryParamReturnTo] {
		if v != "" && !isRelativePath(v) {
			return errInvalidReturnTo
		}
	}

	return nil
}

// isRelativePath accepts only a path that starts with a single "/"
func isRelativePath(p string) bool {
	// "//host" is scheme-relative, and browsers fold "\" into "/"
	if !strings.HasPrefix(p, "/") || strings.HasPrefix(p, "//") || strings.Contains(p, `\`) {
		return false
	}

	// Browsers strip tabs and newlines, so "/<tab>/host" would become "//host"
	if strings.ContainsFunc(p, unicode.IsControl) {
		return false
	}

	u, err := url.Parse(p)
	return err == nil && u.Scheme == "" && u.Host == ""
}

// LegacyCallbackAllowed tells if a login with no code challenge still gets the token in the callback URL
func LegacyCallbackAllowed(authConfig *conf.Auth) bool {
	// Phase 1 of the deprecation: allowed unless the operator denies it
	return authConfig == nil || authConfig.AllowLegacyLoginCallback == nil || authConfig.GetAllowLegacyLoginCallback()
}

// checkLegacyCallback rejects a login that would put the token in an external callback URL when the
// operator denied it. A relative callback is the same-origin download flow, which is not affected
func (svc *AuthService) checkLegacyCallback(callback, codeChallenge string) error {
	if callback == "" || codeChallenge != "" || isRelativeRef(callback) || LegacyCallbackAllowed(svc.authConfig) {
		return nil
	}

	return errLegacyCallbackDenied
}

// isRelativeRef tells if the URL has no scheme and no host
func isRelativeRef(raw string) bool {
	u, err := url.Parse(raw)
	return err == nil && u.Scheme == "" && u.Host == ""
}

// codeChallengeFromQuery returns the S256 code challenge of a login request, if any
func codeChallengeFromQuery(q url.Values) (string, error) {
	challenge := q.Get(oauth.QueryParamCodeChallenge)
	if challenge == "" {
		return "", nil
	}

	if q.Get(oauth.QueryParamCallback) == "" {
		return "", errors.New("a code challenge requires a callback")
	}

	if q.Get(oauth.QueryParamCodeChallengeMethod) != oauth.CodeChallengeMethodS256 {
		return "", fmt.Errorf("unsupported code challenge method, only %s is supported", oauth.CodeChallengeMethodS256)
	}

	if !oauth.ValidS256Challenge(challenge) {
		return "", errors.New("invalid code challenge")
	}

	return challenge, nil
}

type AuthURLs struct {
	Login, callback   string
	loginIsOverridden bool
}

// urlScheme is deprecated, now it will be inferred from the serverConfig externalURL
func getAuthURLs(serverConfig *conf.Server_HTTP, loginURLOverride string) (*AuthURLs, error) {
	host := serverConfig.Addr

	// Calculate it based on local server configuration
	urls := craftAuthURLs("http", host, "")

	// If needed, use the external URL
	if ea := serverConfig.GetExternalUrl(); ea != "" {
		// x must be a valid absolute URI (via RFC 3986)
		url, err := url.ParseRequestURI(ea)
		if err != nil {
			return nil, fmt.Errorf("validation error: %w", err)
		}

		urls = craftAuthURLs(url.Scheme, url.Host, url.Path)
	}

	// Override the login URL if needed
	if loginURLOverride != "" {
		urls.Login = loginURLOverride
		// denote it's been overridden
		urls.loginIsOverridden = true
	}

	return urls, nil
}

func craftAuthURLs(scheme, host, path string) *AuthURLs {
	base := url.URL{Scheme: scheme, Host: host, Path: path}
	login := base.JoinPath(AuthLoginPath)
	callback := base.JoinPath(AuthCallbackPath)

	return &AuthURLs{Login: login.String(), callback: callback.String()}
}

// SetDevMode configures the service to relax secure cookie settings for local development.
func (svc *AuthService) SetDevMode(devMode bool) {
	svc.devMode = devMode
}

func (svc *AuthService) RegisterCallbackHandler() http.Handler {
	return oauthHandler{callbackHandler, svc}
}

func (svc *AuthService) RegisterLoginHandler() http.Handler {
	return oauthHandler{loginHandler, svc}
}

// Implement http.Handler interface
func (h oauthHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if resp := h.H(h.svc, w, r); resp != nil {
		if resp.redirectURL != nil {
			http.Redirect(w, r, resp.redirectURL.String(), http.StatusTemporaryRedirect)
			return
		}
		http.Error(w, resp.ErrorMessage(r.Context(), h.svc.log), resp.code)
	}
}

func loginHandler(svc *AuthService, w http.ResponseWriter, r *http.Request) *oauthResp {
	// The final destination where the auth token will be pushed to, i.e the CLI.
	// Rejected upfront so a crafted callback never starts the OIDC dance
	query := r.URL.Query()
	callback := query.Get(oauth.QueryParamCallback)
	if err := callbackAllowed(callback, svc.allowedCallbackOrigins); err != nil {
		return newOauthResp(http.StatusBadRequest, err, true)
	}

	if err := returnToAllowed(callback); err != nil {
		return newOauthResp(http.StatusBadRequest, err, true)
	}

	// A client that sends a code challenge gets a handoff code instead of the token
	codeChallenge, err := codeChallengeFromQuery(query)
	if err != nil {
		return newOauthResp(http.StatusBadRequest, err, true)
	}

	if err := svc.checkLegacyCallback(callback, codeChallenge); err != nil {
		return newOauthResp(http.StatusBadRequest, err, true)
	}

	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return newOauthResp(http.StatusInternalServerError, fmt.Errorf("failed to generate random string: %w", err), false)
	}

	// Store a random string to check it in the oauth callback
	state := base64.URLEncoding.EncodeToString(b)
	svc.setOauthCookie(w, cookieOauthStateName, state)

	svc.setOauthCookie(w, cookieCallback, callback)
	svc.setOauthCookie(w, cookieCodeChallenge, codeChallenge)

	// Wether the token should be short lived or not
	svc.setOauthCookie(w, cookieLongLived, query.Get(oauth.QueryParamLongLived))

	authorizationURI := svc.authenticator.AuthCodeURL(state)

	// Add the connection parameter to the authorization URL if needed
	// ?connection is useful for example in auth0 to know which connection to use
	// https://auth0.com/docs/api/authentication#login
	connectionStr := query.Get(oauth.QueryParamAuth0Connection)
	if connectionStr != "" {
		uri, err := url.Parse(authorizationURI)
		if err != nil {
			return newOauthResp(http.StatusInternalServerError, fmt.Errorf("failed to parse authorization URI: %w", err), false)
		}
		q := uri.Query()
		q.Set("connection", connectionStr)
		uri.RawQuery = q.Encode()
		authorizationURI = uri.String()
	}

	http.Redirect(w, r, authorizationURI, http.StatusFound)
	return newOauthResp(http.StatusTemporaryRedirect, nil, false)
}

// Extract custom claims
type upstreamOIDCclaims struct {
	Email string `json:"email"`
	// This value is present  in the case of Microsoft Entra IDp
	// It might show the user's proxy email used during login
	// https://learn.microsoft.com/en-us/entra/identity/authentication/howto-authentication-use-email-signin
	// https://learn.microsoft.com/en-us/entra/identity-platform/id-token-claims-reference
	PreferredUsername string   `json:"preferred_username"`
	FamilyName        string   `json:"family_name"`
	GivenName         string   `json:"given_name"`
	Groups            []string `json:"groups"`
}

// Will retrieve the email from the preferred username if it's a valid email
// or fallback to the email field
func (c *upstreamOIDCclaims) preferredEmail() string {
	if c.PreferredUsername != "" {
		// validate that this is an email since according to the Entra spec this might be a phone or username
		if _, err := mail.ParseAddress(c.PreferredUsername); err == nil {
			return c.PreferredUsername
		}
	}

	return c.Email
}

func callbackHandler(svc *AuthService, w http.ResponseWriter, r *http.Request) *oauthResp {
	ctx := context.Background()
	// if OIDC provider returns an error, show the error to the user
	if desc := r.URL.Query().Get(oidcErrorParam); desc != "" {
		// Do not redirect if there is no dedicated login page
		if !svc.AuthURLs.loginIsOverridden {
			return newOauthResp(http.StatusUnauthorized, errors.New(desc), true)
		}

		// redirect to the login page to and show it to the user
		redirectURL, err := url.Parse(svc.AuthURLs.Login)
		if err != nil {
			return newOauthResp(http.StatusInternalServerError, fmt.Errorf("failed to redirect to login: %w", err), false)
		}

		q := redirectURL.Query()
		q.Set(oidcErrorParam, desc)
		redirectURL.RawQuery = q.Encode()

		return &oauthResp{http.StatusUnauthorized, nil, true, redirectURL}
	}

	// Get information from google OIDC token
	claims, errWithCode := extractUserInfoFromToken(ctx, svc, r)
	if errWithCode != nil {
		return newOauthResp(errWithCode.code, errWithCode.err, errWithCode.showErrToUser)
	}

	// Create user if needed
	u, err := svc.userUseCase.UpsertByEmail(ctx, claims.preferredEmail(), &biz.UpsertByEmailOpts{
		FirstName: &claims.GivenName,
		LastName:  &claims.FamilyName,
		SSOGroups: claims.Groups,
		Source:    "oidc",
	})
	if err != nil {
		return newOauthResp(http.StatusInternalServerError, fmt.Errorf("failed to find or create user: %w", err), false)
	}

	// Accept any pending invites
	if err := svc.orgInvitesUseCase.AcceptPendingInvitations(ctx, u.Email); err != nil {
		return newOauthResp(http.StatusInternalServerError, fmt.Errorf("failed to accept pending invitations: %w", err), false)
	}

	return svc.deliverLogin(ctx, w, r, u.ID)
}

// deliverLogin hands the session of the user over to the client that started the login: a
// handoff code when it sent a code challenge, otherwise the token itself
func (svc *AuthService) deliverLogin(ctx context.Context, w http.ResponseWriter, r *http.Request, userID string) *oauthResp {
	// Set the expiration
	expiration := shortLivedDuration
	longLived, err := r.Cookie(cookieLongLived)
	if err != nil {
		return newOauthResp(http.StatusInternalServerError, fmt.Errorf("failed to get long lived cookie: %w", err), false)
	}

	if longLived.Value == "true" {
		expiration = longLivedDuration
	}

	// Either redirect or render the token if fallback is specified
	// Callback URL from the cookie
	callbackURLFromCookie, err := r.Cookie(cookieCallback)
	if err != nil {
		return newOauthResp(http.StatusInternalServerError, fmt.Errorf("failed to get callback URL from cookie: %w", err), false)
	}

	callbackValue := callbackURLFromCookie.Value

	// There is no callback, render the token in an HTML page with a copy button
	if callbackValue == "" {
		userToken, err := generateUserJWT(userID, svc.authConfig.GeneratedJwsHmacSecret, expiration)
		if err != nil {
			return newOauthResp(http.StatusInternalServerError, fmt.Errorf("failed to generate user token: %w", err), false)
		}

		if err := renderTokenPage(w, userToken); err != nil {
			return newOauthResp(http.StatusInternalServerError, err, false)
		}
		return newOauthResp(http.StatusOK, nil, false)
	}

	// Redirect to the callback URL. The cookie was validated at login time, re-check it here
	// so a stale or tampered cookie can't turn this into an open redirect either
	if err := callbackAllowed(callbackValue, svc.allowedCallbackOrigins); err != nil {
		return newOauthResp(http.StatusBadRequest, err, true)
	}

	if err := returnToAllowed(callbackValue); err != nil {
		return newOauthResp(http.StatusBadRequest, err, true)
	}

	// A login started before an upgrade has no challenge cookie, it is a legacy login
	var codeChallenge string
	if c, err := r.Cookie(cookieCodeChallenge); err == nil {
		codeChallenge = c.Value
	}

	if err := svc.checkLegacyCallback(callbackValue, codeChallenge); err != nil {
		return newOauthResp(http.StatusBadRequest, err, true)
	}

	var callbackURL string
	if codeChallenge != "" {
		handoff, err := svc.loginHandoffs.Issue(ctx, userID, expiration, codeChallenge, callbackValue)
		if err != nil {
			return newOauthResp(http.StatusInternalServerError, fmt.Errorf("failed to issue login handoff code: %w", err), false)
		}

		callbackURL, err = crafCallbackURL(callbackValue, oauth.QueryParamHandoff, handoff)
		if err != nil {
			return newOauthResp(http.StatusInternalServerError, fmt.Errorf("failed to craft callback URL: %w", err), false)
		}
	} else {
		userToken, err := generateUserJWT(userID, svc.authConfig.GeneratedJwsHmacSecret, expiration)
		if err != nil {
			return newOauthResp(http.StatusInternalServerError, fmt.Errorf("failed to generate user token: %w", err), false)
		}

		// The same-origin download flow (relative callback) stays on the token callback, it is not part of the deprecation
		if !isRelativeRef(callbackValue) {
			legacyCallbackDeliveries.Inc()
			svc.log.WithContext(ctx).Warnw("msg", "login delivered the token in the callback URL, the client does not use the code exchange",
				"callback_origin", strings.Join(originsOf(callbackValue), ""))
		}

		callbackURL, err = crafCallbackURL(callbackValue, oauth.QueryParamLegacyToken, userToken)
		if err != nil {
			return newOauthResp(http.StatusInternalServerError, fmt.Errorf("failed to craft callback URL: %w", err), false)
		}
	}

	setTokenLeakHeaders(w)
	http.Redirect(w, r, callbackURL, http.StatusFound)
	return newOauthResp(http.StatusTemporaryRedirect, nil, false)
}

func crafCallbackURL(callback, param, value string) (string, error) {
	callbackURL, err := url.Parse(callback)
	if err != nil {
		return "", fmt.Errorf("invalid callback URL: %w", err)
	}

	q := callbackURL.Query()
	q.Set(param, value)
	callbackURL.RawQuery = q.Encode()

	return callbackURL.String(), nil
}

// Returns the claims from the OIDC token received during the OIDC callback
func extractUserInfoFromToken(ctx context.Context, svc *AuthService, r *http.Request) (*upstreamOIDCclaims, *oauthResp) {
	cookieState, err := r.Cookie(cookieOauthStateName)
	// if the cookie is not found, it likely means the authentication process has expired
	if err != nil {
		return nil, newOauthResp(http.StatusUnauthorized, errors.New("the authentication process has expired, please try again"), true)
	}

	if r.URL.Query().Get("state") != cookieState.Value {
		return nil, newOauthResp(http.StatusUnauthorized, errors.New("the authentication was invalid, please try again"), true)
	}

	code := r.URL.Query().Get("code")
	// Use the custom HTTP client when requesting a token.
	httpClient := &http.Client{Timeout: 2 * time.Second}
	ctx = context.WithValue(ctx, oauth2.HTTPClient, httpClient)

	// Exchange the code for a token
	oauth2Token, err := svc.authenticator.Exchange(ctx, code)
	if err != nil {
		return nil, newOauthResp(http.StatusUnauthorized, err, false)
	}

	// It's a valid Oauth2 token
	if !oauth2Token.Valid() {
		return nil, newOauthResp(http.StatusUnauthorized, errors.New("retrieved invalid Token"), false)
	}

	// Parse and verify ID token content and signature
	idToken, err := svc.authenticator.VerifyIDToken(ctx, oauth2Token)
	if err != nil {
		return nil, newOauthResp(http.StatusInternalServerError, err, false)
	}

	var claims *upstreamOIDCclaims
	if err := idToken.Claims(&claims); err != nil {
		return nil, newOauthResp(http.StatusInternalServerError, err, false)
	}

	return claims, nil
}

// Take an upstream token from Google and generates a temporary Chainloop JWT
func generateUserJWT(userID, passphrase string, expiration time.Duration) (string, error) {
	b, err := user.NewBuilder(
		user.WithExpiration(expiration),
		user.WithIssuer(jwt.DefaultIssuer),
		user.WithKeySecret(passphrase),
	)

	if err != nil {
		return "", err
	}

	return b.GenerateJWT(userID)
}

func (svc *AuthService) setOauthCookie(w http.ResponseWriter, name, value string) {
	cookie := &http.Cookie{
		Name:     name,
		Value:    value,
		Path:     "/",
		Expires:  time.Now().Add(10 * time.Minute),
		HttpOnly: true,
	}

	if !svc.devMode {
		cookie.Secure = true
		cookie.SameSite = http.SameSiteLaxMode
	}

	http.SetCookie(w, cookie)
}

func generateAndLogDevUser(userUC *biz.UserUseCase, log *log.Helper, authConfig *conf.Auth) error {
	// Runs at startup, outside of any request
	ctx := context.Background()
	// Create user if needed
	u, err := userUC.UpsertByEmail(ctx, authConfig.DevUser, nil)
	if err != nil {
		return sl.LogAndMaskErr(ctx, err, log)
	}

	// Generate user token
	userToken, err := generateUserJWT(u.ID, authConfig.GeneratedJwsHmacSecret, devUserDuration)
	if err != nil {
		return sl.LogAndMaskErr(ctx, err, log)
	}

	log.Info("******************* DEVELOPMENT USER TOKEN *******************")
	log.Infof("Use chainloop 'auth login --skip-browser' and paste this token to start a headless session: %s", userToken)

	return nil
}

// ExchangeHandoff trades a login handoff code for the session token. Every failure returns the
// same error, the specific reason only goes to the server log
func (svc *AuthService) ExchangeHandoff(ctx context.Context, req *pb.AuthServiceExchangeHandoffRequest) (*pb.AuthServiceExchangeHandoffResponse, error) {
	h, err := svc.loginHandoffs.Exchange(ctx, req.GetHandoff(), req.GetCodeVerifier(), req.GetCallback())
	if err != nil {
		var exchangeErr biz.ErrLoginHandoffExchange
		if errors.As(err, &exchangeErr) {
			svc.log.WithContext(ctx).Warnw("msg", "login handoff exchange failed", "reason", exchangeErr.Reason)
			return nil, errLoginHandoffExchange
		}

		return nil, handleUseCaseErr(ctx, err, svc.log)
	}

	token, err := generateUserJWT(h.UserID.String(), svc.authConfig.GeneratedJwsHmacSecret, h.TokenTTL)
	if err != nil {
		return nil, sl.LogAndMaskErr(ctx, fmt.Errorf("generating user token: %w", err), svc.log)
	}

	return &pb.AuthServiceExchangeHandoffResponse{Token: token}, nil
}

// DeleteAccount deletes an account
func (svc *AuthService) DeleteAccount(ctx context.Context, _ *pb.AuthServiceDeleteAccountRequest) (*pb.AuthServiceDeleteAccountResponse, error) {
	user, err := requireCurrentUser(ctx)
	if err != nil {
		return nil, err
	}

	if err := svc.userUseCase.DeleteUser(ctx, user.ID); err != nil {
		return nil, handleUseCaseErr(ctx, err, svc.log)
	}

	return &pb.AuthServiceDeleteAccountResponse{}, nil
}
