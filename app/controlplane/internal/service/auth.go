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
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"net/http"
	"net/mail"
	"net/url"
	"time"

	pb "github.com/chainloop-dev/chainloop/app/controlplane/api/controlplane/v1"
	conf "github.com/chainloop-dev/chainloop/app/controlplane/internal/conf/controlplane/config/v1"
	authenticator "github.com/chainloop-dev/chainloop/app/controlplane/internal/oidcauthenticator"
	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/biz"
	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/jwt"
	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/jwt/user"
	"github.com/chainloop-dev/chainloop/internal/oauth"
	sl "github.com/chainloop-dev/chainloop/pkg/servicelogger"
	"github.com/go-kratos/kratos/v2/log"
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

	// Auth paths
	AuthLoginPath    = "/auth/login"
	AuthCallbackPath = "/auth/callback"

	// default
	shortLivedDuration = 10 * time.Second
	// opt-in
	longLivedDuration = 24 * time.Hour
	// dev only
	devUserDuration = 30 * longLivedDuration
)

type oauthHandler struct {
	H   func(*AuthService, http.ResponseWriter, *http.Request) (int, error)
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
}

func NewAuthService(userUC *biz.UserUseCase, orgUC *biz.OrganizationUseCase, mUC *biz.MembershipUseCase, inviteUC *biz.OrgInvitationUseCase, authConfig *conf.Auth, serverConfig *conf.Server, auc *biz.AuditorUseCase, opts ...NewOpt) (*AuthService, error) {
	oidcConfig := authConfig.GetOidc()
	if oidcConfig == nil {
		return nil, errors.New("oauth configuration missing")
	}

	// Craft Auth related endpoints
	authURLs, err := getAuthURLs(serverConfig.GetHttp(), authConfig.GetOidc().GetLoginUrlOverride())
	if err != nil {
		return nil, fmt.Errorf("failed to get auth URLs: %w", err)
	}

	authInst, err := authenticator.NewOIDC(oidcConfig.Domain, oidcConfig.ClientId, oidcConfig.ClientSecret, authURLs.callback)
	if err != nil {
		return nil, fmt.Errorf("failed to create OIDC authenticator: %w", err)
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
	}, nil
}

type AuthURLs struct {
	Login, callback string
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
	}

	return urls, nil
}

func craftAuthURLs(scheme, host, path string) *AuthURLs {
	base := url.URL{Scheme: scheme, Host: host, Path: path}
	login := base.JoinPath(AuthLoginPath)
	callback := base.JoinPath(AuthCallbackPath)

	return &AuthURLs{Login: login.String(), callback: callback.String()}
}

func (svc *AuthService) RegisterCallbackHandler() http.Handler {
	return oauthHandler{callbackHandler, svc}
}

func (svc *AuthService) RegisterLoginHandler() http.Handler {
	return oauthHandler{loginHandler, svc}
}

// Implement http.Handler interface
func (h oauthHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	status, err := h.H(h.svc, w, r)
	if err != nil {
		http.Error(w, http.StatusText(status), status)
	}
}

func loginHandler(svc *AuthService, w http.ResponseWriter, r *http.Request) (int, error) {
	b := make([]byte, 16)
	_, err := rand.Read(b)
	if err != nil {
		return http.StatusInternalServerError, sl.LogAndMaskErr(err, nil)
	}

	// Store a random string to check it in the oauth callback
	state := base64.URLEncoding.EncodeToString(b)
	setOauthCookie(w, cookieOauthStateName, state)

	// Store the final destination where the auth token will be pushed to, i.e the CLI
	setOauthCookie(w, cookieCallback, r.URL.Query().Get(oauth.QueryParamCallback))

	// Wether the token should be short lived or not
	setOauthCookie(w, cookieLongLived, r.URL.Query().Get(oauth.QueryParamLongLived))

	authorizationURI := svc.authenticator.AuthCodeURL(state)

	// Add the connection parameter to the authorization URL if needed
	// ?connection is useful for example in auth0 to know which connection to use
	// https://auth0.com/docs/api/authentication#login
	connectionStr := r.URL.Query().Get(oauth.QueryParamAuth0Connection)
	if connectionStr != "" {
		uri, err := url.Parse(authorizationURI)
		if err != nil {
			return http.StatusInternalServerError, sl.LogAndMaskErr(err, svc.log)
		}
		q := uri.Query()
		q.Set("connection", connectionStr)
		uri.RawQuery = q.Encode()
		authorizationURI = uri.String()
	}

	http.Redirect(w, r, authorizationURI, http.StatusFound)
	return http.StatusTemporaryRedirect, nil
}

// Extract custom claims
type upstreamOIDCclaims struct {
	Email string `json:"email"`
	// This value is present  in the case of Microsoft Entra IDp
	// It might show the user's proxy email used during login
	// https://learn.microsoft.com/en-us/entra/identity/authentication/howto-authentication-use-email-signin
	// https://learn.microsoft.com/en-us/entra/identity-platform/id-token-claims-reference
	PreferredUsername string `json:"preferred_username"`
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

type errorWithCode struct {
	code int
	error
}

func callbackHandler(svc *AuthService, w http.ResponseWriter, r *http.Request) (int, error) {
	ctx := context.Background()
	// Get information from google OIDC token
	claims, errWithCode := extractUserInfoFromToken(ctx, svc, r)
	if errWithCode != nil {
		return errWithCode.code, sl.LogAndMaskErr(errWithCode.error, svc.log)
	}

	// Create user if needed
	u, err := svc.userUseCase.FindOrCreateByEmail(ctx, claims.preferredEmail())
	if err != nil {
		return http.StatusInternalServerError, sl.LogAndMaskErr(err, svc.log)
	}

	// Accept any pending invites
	if err := svc.orgInvitesUseCase.AcceptPendingInvitations(ctx, u.Email); err != nil {
		return http.StatusInternalServerError, sl.LogAndMaskErr(err, svc.log)
	}

	// Set the expiration
	expiration := shortLivedDuration
	longLived, err := r.Cookie(cookieLongLived)
	if err != nil {
		return http.StatusInternalServerError, sl.LogAndMaskErr(err, svc.log)
	}

	if longLived.Value == "true" {
		expiration = longLivedDuration
	}

	// Generate user token
	userToken, err := generateUserJWT(u.ID, svc.authConfig.GeneratedJwsHmacSecret, expiration)
	if err != nil {
		return http.StatusInternalServerError, sl.LogAndMaskErr(err, svc.log)
	}

	// Either redirect or render the token if fallback is specified
	// Callback URL from the cookie
	callbackURLFromCookie, err := r.Cookie(cookieCallback)
	if err != nil {
		return http.StatusInternalServerError, sl.LogAndMaskErr(err, svc.log)
	}

	callbackValue := callbackURLFromCookie.Value

	// There is no callback, just render the token
	if callbackValue == "" {
		fmt.Fprintf(w, "copy this token and paste it in your terminal window\n\n%s", userToken)
		return http.StatusOK, nil
	}

	// Redirect to the callback URL
	callbackURL, err := crafCallbackURL(callbackValue, userToken)
	if err != nil {
		return http.StatusInternalServerError, sl.LogAndMaskErr(err, svc.log)
	}

	http.Redirect(w, r, callbackURL, http.StatusFound)
	return http.StatusTemporaryRedirect, nil
}

func crafCallbackURL(callback, userToken string) (string, error) {
	callbackURL, err := url.Parse(callback)
	if err != nil {
		return "", fmt.Errorf("invalid callback URL: %w", err)
	}

	q := callbackURL.Query()
	q.Set("t", userToken)
	callbackURL.RawQuery = q.Encode()

	return callbackURL.String(), nil
}

// Returns the claims from the OIDC token received during the OIDC callback
func extractUserInfoFromToken(ctx context.Context, svc *AuthService, r *http.Request) (*upstreamOIDCclaims, *errorWithCode) {
	cookieState, err := r.Cookie(cookieOauthStateName)
	if err != nil {
		return nil, &errorWithCode{http.StatusUnauthorized, fmt.Errorf("retrieving cookie %s: %w", cookieOauthStateName, err)}
	}

	if r.URL.Query().Get("state") != cookieState.Value {
		return nil, &errorWithCode{http.StatusUnauthorized, errors.New("oauth state does not match")}
	}

	code := r.URL.Query().Get("code")
	// Use the custom HTTP client when requesting a token.
	httpClient := &http.Client{Timeout: 2 * time.Second}
	ctx = context.WithValue(ctx, oauth2.HTTPClient, httpClient)

	// Exchange the code for a token
	oauth2Token, err := svc.authenticator.Exchange(ctx, code)
	if err != nil {
		return nil, &errorWithCode{http.StatusUnauthorized, err}
	}

	// It's a valid Oauth2 token
	if !oauth2Token.Valid() {
		return nil, &errorWithCode{http.StatusUnauthorized, errors.New("retrieved invalid Token")}
	}

	// Parse and verify ID token content and signature
	idToken, err := svc.authenticator.VerifyIDToken(ctx, oauth2Token)
	if err != nil {
		return nil, &errorWithCode{http.StatusInternalServerError, err}
	}

	var claims *upstreamOIDCclaims
	if err := idToken.Claims(&claims); err != nil {
		return nil, &errorWithCode{http.StatusInternalServerError, err}
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

func setOauthCookie(w http.ResponseWriter, name, value string) {
	http.SetCookie(w, &http.Cookie{Name: name, Value: value, Path: "/", Expires: time.Now().Add(5 * time.Minute)})
}

func generateAndLogDevUser(userUC *biz.UserUseCase, log *log.Helper, authConfig *conf.Auth) error {
	// Create user if needed
	u, err := userUC.FindOrCreateByEmail(context.Background(), authConfig.DevUser)
	if err != nil {
		return sl.LogAndMaskErr(err, log)
	}

	// Generate user token
	userToken, err := generateUserJWT(u.ID, authConfig.GeneratedJwsHmacSecret, devUserDuration)
	if err != nil {
		return sl.LogAndMaskErr(err, log)
	}

	log.Info("******************* DEVELOPMENT USER TOKEN *******************")
	log.Infof("Use chainloop 'auth login --skip-browser' and paste this token to start a headless session: %s", userToken)

	return nil
}

// DeleteAccount deletes an account
func (svc *AuthService) DeleteAccount(ctx context.Context, _ *pb.AuthServiceDeleteAccountRequest) (*pb.AuthServiceDeleteAccountResponse, error) {
	user, err := requireCurrentUser(ctx)
	if err != nil {
		return nil, err
	}

	if err := svc.userUseCase.DeleteUser(ctx, user.ID); err != nil {
		return nil, sl.LogAndMaskErr(err, svc.log)
	}

	return &pb.AuthServiceDeleteAccountResponse{}, nil
}
