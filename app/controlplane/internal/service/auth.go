//
// Copyright 2023 The Chainloop Authors.
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
	"net/url"
	"time"

	pb "github.com/chainloop-dev/chainloop/app/controlplane/api/controlplane/v1"
	"github.com/chainloop-dev/chainloop/app/controlplane/internal/biz"
	"github.com/chainloop-dev/chainloop/app/controlplane/internal/conf"
	"github.com/chainloop-dev/chainloop/app/controlplane/internal/jwt"
	"github.com/chainloop-dev/chainloop/app/controlplane/internal/jwt/user"
	authenticator "github.com/chainloop-dev/chainloop/app/controlplane/internal/oidcauthenticator"
	sl "github.com/chainloop-dev/chainloop/internal/servicelogger"
	"golang.org/x/oauth2"
)

// The authentication process does the following
// 1 - Authenticate against a valid OIDC provider, currently only Google is supported
// 2 - Generate a chainloop signed JWT to be sent to the client
const cookieOauthStateName string = "oauthState"

// Deprecated: This is a legacy cookie name, it will be removed in a future release
const cookieCLICallback string = "oauthCLICallback"
const cookieCallback string = "oauthCallback"
const AuthLoginPath = "/auth/login"
const AuthCallbackPath = "/auth/callback"

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
	AuthURLs          *AuthURLs
}

func NewAuthService(userUC *biz.UserUseCase, orgUC *biz.OrganizationUseCase, mUC *biz.MembershipUseCase, authConfig *conf.Auth, serverConfig *conf.Server, opts ...NewOpt) (*AuthService, error) {
	oidcConfig := authConfig.GetOidc()
	if oidcConfig == nil {
		return nil, errors.New("oauth configuration missing")
	}

	// Craft Auth related endpoints
	authURLs := getAuthURLs(oidcConfig.RedirectUrlScheme, serverConfig)
	authInst, err := authenticator.NewOIDC(oidcConfig.Domain, oidcConfig.ClientId, oidcConfig.ClientSecret, authURLs.callback)
	if err != nil {
		return nil, fmt.Errorf("failed to create OIDC authenticator: %w", err)
	}

	return &AuthService{
		service:           newService(opts...),
		authenticator:     authInst,
		userUseCase:       userUC,
		orgUseCase:        orgUC,
		authConfig:        authConfig,
		AuthURLs:          authURLs,
		membershipUseCase: mUC,
	}, nil
}

type AuthURLs struct {
	Login, callback string
}

func getAuthURLs(urlScheme string, serverConfig *conf.Server) *AuthURLs {
	host := serverConfig.Http.Addr
	if ea := serverConfig.Http.ExternalAddr; ea != "" {
		host = ea
	}

	login := url.URL{Scheme: urlScheme, Host: host, Path: AuthLoginPath}
	callback := url.URL{Scheme: urlScheme, Host: host, Path: AuthCallbackPath}

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
	setOauthCookie(w, cookieCallback, r.URL.Query().Get("callback"))
	// TODO: Deprecated, latest CLI version uses the callback query param instead
	setOauthCookie(w, cookieCLICallback, r.URL.Query().Get("cli-callback"))

	url := svc.authenticator.AuthCodeURL(state)
	http.Redirect(w, r, url, http.StatusFound)
	return http.StatusTemporaryRedirect, nil
}

// Extract custom claims
type upstreamOIDCclaims struct {
	Email string `json:"email"`
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
	u, err := svc.userUseCase.FindOrCreateByEmail(ctx, claims.Email)
	if err != nil {
		return http.StatusInternalServerError, sl.LogAndMaskErr(err, svc.log)
	}

	// Check if the user already has an organization attached
	memberships, err := svc.membershipUseCase.ByUser(ctx, u.ID)
	if err != nil {
		return http.StatusInternalServerError, sl.LogAndMaskErr(err, svc.log)
	}

	if len(memberships) == 0 {
		// Create an org
		org, err := svc.orgUseCase.Create(ctx, "")
		if err != nil {
			return http.StatusInternalServerError, sl.LogAndMaskErr(err, svc.log)
		}

		// Create membership
		if _, err := svc.membershipUseCase.Create(ctx, org.ID, u.ID, true); err != nil {
			return http.StatusInternalServerError, sl.LogAndMaskErr(err, svc.log)
		}

		svc.log.Infow("msg", "new user associated to an org", "org_id", org.ID, "user_id", u.ID)
	}

	// Generate user token
	userToken, err := generateUserJWT(u.ID, svc.authConfig)
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
	// DEPRECATED: Remove this block in the future
	if callbackValue == "" {
		// Fallback to previous cookie that older CLIs might be sending
		callbackURLFromCookie, err = r.Cookie(cookieCLICallback)
		if err != nil {
			return http.StatusInternalServerError, sl.LogAndMaskErr(err, svc.log)
		}
		callbackValue = callbackURLFromCookie.Value
	}

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
		return nil, &errorWithCode{http.StatusUnauthorized, err}
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
func generateUserJWT(userID string, c *conf.Auth) (string, error) {
	b, err := user.NewBuilder(
		user.WithExpiration(24*time.Hour),
		user.WithIssuer(jwt.DefaultIssuer),
		user.WithKeySecret(c.GeneratedJwsHmacSecret),
	)

	if err != nil {
		return "", err
	}

	return b.GenerateJWT(userID, jwt.DefaultAudience)
}

func setOauthCookie(w http.ResponseWriter, name, value string) {
	http.SetCookie(w, &http.Cookie{Name: name, Value: value, Path: "/", Expires: time.Now().Add(5 * time.Minute)})
}

// DeleteAccount deletes an account
func (svc *AuthService) DeleteAccount(ctx context.Context, _ *pb.AuthServiceDeleteAccountRequest) (*pb.AuthServiceDeleteAccountResponse, error) {
	user, _, err := loadCurrentUserAndOrg(ctx)
	if err != nil {
		return nil, err
	}

	if err := svc.userUseCase.DeleteUser(ctx, user.ID); err != nil {
		return nil, sl.LogAndMaskErr(err, svc.log)
	}

	return &pb.AuthServiceDeleteAccountResponse{}, nil
}
