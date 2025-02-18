//
// Copyright 2024-2025 The Chainloop Authors.
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

package attjwtmiddleware

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	conf "github.com/chainloop-dev/chainloop/app/controlplane/internal/conf/controlplane/config/v1"
	"github.com/chainloop-dev/chainloop/app/controlplane/internal/usercontext/entities"
	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/jwt/apitoken"
	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/jwt/robotaccount"
	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/jwt/user"
	errorsAPI "github.com/go-kratos/kratos/v2/errors"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/go-kratos/kratos/v2/middleware"
	"github.com/go-kratos/kratos/v2/transport"
	"github.com/hashicorp/golang-lru/v2/expirable"

	"github.com/golang-jwt/jwt/v4"
)

const (
	// bearerWord the bearer key word for authorization
	bearerWord string = "Bearer"
	// authorizationKey holds the key used to store the JWT Token in the request tokenHeader.
	authorizationKey string = "Authorization"
	// reason holds the error reason.
	reason string = "UNAUTHORIZED"
	// RobotAccountProviderKey is the key for robot account token provider
	RobotAccountProviderKey = "robotAccountProvider"
	// APITokenProviderKey is the key for api token provider
	APITokenProviderKey = "apiTokenProvider"
	// FederatedProviderKey is the key for federated token provider
	FederatedProviderKey = "federatedProvider"
)

var (
	ErrMissingJwtToken           = errorsAPI.Unauthorized(reason, "JWT token is missing")
	ErrMissingKeyFunc            = errorsAPI.Unauthorized(reason, "keyFunc is missing")
	ErrMissingVerifyAudienceFunc = errorsAPI.Unauthorized(reason, "verifyAudienceFunc is missing")
	ErrTokenInvalid              = errorsAPI.Unauthorized(reason, "Token is invalid")
	ErrTokenExpired              = errorsAPI.Unauthorized(reason, "JWT token has expired")
	ErrTokenParseFail            = errorsAPI.Unauthorized(reason, "Fail to parse JWT token ")
	ErrUnSupportSigningMethod    = errorsAPI.Unauthorized(reason, "Wrong signing method")
	ErrWrongContext              = errorsAPI.Unauthorized(reason, "Wrong context for middleware")
)

// NewRobotAccountProvider return the configuration to validate and verify token issued for Robot Accounts
func NewRobotAccountProvider(signingSecret string) JWTOption {
	return withTokenProvider(
		RobotAccountProviderKey,
		WithClaims(func() jwt.Claims { return &robotaccount.CustomClaims{} }),
		WithVerifyAudienceFunc(func(token *jwt.Token) bool {
			claims, ok := token.Claims.(*robotaccount.CustomClaims)
			if !ok {
				return false
			}
			for _, aud := range []string{robotaccount.Audience, robotaccount.DeprecatedAudience} {
				if claims.VerifyAudience(aud, true) {
					return true
				}
			}
			return false
		}),
		WithSigningMethod(robotaccount.SigningMethod),
		WithKeyFunc(func(_ *jwt.Token) (interface{}, error) {
			// TODO: add support to multiple signing methods and keys
			return []byte(signingSecret), nil
		}),
	)
}

// NewAPITokenProvider return the configuration to validate and verify token issued for API Tokens
func NewAPITokenProvider(signingSecret string) JWTOption {
	return withTokenProvider(
		APITokenProviderKey,
		WithClaims(func() jwt.Claims { return &apitoken.CustomClaims{} }),
		WithVerifyAudienceFunc(func(token *jwt.Token) bool {
			claims, ok := token.Claims.(*apitoken.CustomClaims)
			if !ok {
				return false
			}

			return claims.VerifyAudience(apitoken.Audience, true)
		}),
		WithSigningMethod(user.SigningMethod),
		WithKeyFunc(func(_ *jwt.Token) (interface{}, error) {
			return []byte(signingSecret), nil
		}),
	)
}

type JWTAuthContext struct {
	Claims      jwt.Claims
	ProviderKey string
	Token       string
}

type authzContextKey struct{}

type JWTOption func(*options)
type TokenProviderOption func(*providerOption)

type VerifyAudienceFunc func(*jwt.Token) bool

type providerOption struct {
	providerKey        string
	signingMethod      jwt.SigningMethod
	keyFunc            jwt.Keyfunc
	claims             func() jwt.Claims
	verifyAudienceFunc VerifyAudienceFunc
}

func WithSigningMethod(method jwt.SigningMethod) TokenProviderOption {
	return func(o *providerOption) {
		o.signingMethod = method
	}
}

func WithClaims(f func() jwt.Claims) TokenProviderOption {
	return func(o *providerOption) {
		o.claims = f
	}
}

func WithKeyFunc(keyFunc jwt.Keyfunc) TokenProviderOption {
	return func(o *providerOption) {
		o.keyFunc = keyFunc
	}
}

func WithVerifyAudienceFunc(f VerifyAudienceFunc) TokenProviderOption {
	return func(o *providerOption) {
		o.verifyAudienceFunc = f
	}
}

type options struct {
	tokenProviders           []providerOption
	federatedVerificationURL string
}

func withTokenProvider(providerKey string, opts ...TokenProviderOption) JWTOption {
	op := &providerOption{
		providerKey: providerKey,
	}
	for _, opt := range opts {
		opt(op)
	}
	return func(o *options) {
		o.tokenProviders = append(o.tokenProviders, *op)
	}
}

// WithFederatedProvider adds support to ask a third party service to verify the token
// verify URL must be an API that receives a json encoded body with the following structure:
//
//	{
//		"token": "<jwt token>",
//		"org_name": "<organization name>"
//	}
//
// and returns a json with the following structure:
func WithFederatedProvider(conf *conf.FederatedVerification) JWTOption {
	return func(o *options) {
		if conf != nil && conf.GetEnabled() && conf.GetUrl() != "" {
			o.federatedVerificationURL = conf.GetUrl()
		}
	}
}

// WithJWTMulti creates a custom JWT middleware that configured with different token providers
// tries to run all validations from an incoming token. If one of the providers matches the expected audience
// it gets parsed and sent down to the next middleware. If none matches an error is returned
func WithJWTMulti(l log.Logger, opts ...JWTOption) middleware.Middleware {
	o := &options{}
	for _, opt := range opts {
		opt(o)
	}

	logger := log.NewHelper(log.With(l, "component", "jwtMiddleware"))
	if o.federatedVerificationURL != "" {
		logger.Infof("federated verification enabled, using URL: %s", o.federatedVerificationURL)
	}

	// claims cache with 10s TTL and unlimited keys
	claimsCache := expirable.NewLRU[string, *jwt.MapClaims](0, nil, time.Second*10)

	return func(handler middleware.Handler) middleware.Handler {
		return func(ctx context.Context, req interface{}) (interface{}, error) {
			if header, ok := transport.FromServerContext(ctx); ok {
				auths := strings.SplitN(header.RequestHeader().Get(authorizationKey), " ", 2)
				if len(auths) != 2 || !strings.EqualFold(auths[0], bearerWord) {
					return nil, ErrMissingJwtToken
				}
				jwtToken := auths[1]
				var (
					tokenInfo *jwt.Token
					err       error
				)

				tokenProviderLen := len(o.tokenProviders)
				for idx, provider := range o.tokenProviders {
					tokenInfo, err = runProviderValidator(provider, jwtToken)

					// Check if it's the last provider and still failed
					if err != nil {
						if idx < tokenProviderLen-1 {
							continue
						}

						// If federated verification is enabled, we try to get the information remotely
						if o.federatedVerificationURL != "" {
							// The org name might come from the header, it's optional and used to explicitly authenticate against it
							orgName, err := entities.GetOrganizationNameFromHeader(ctx)
							if err != nil {
								return nil, fmt.Errorf("error getting organization name: %w", err)
							}

							logger.Infof("calling federated provider, orgName: %s", orgName)
							claims, err := callFederatedProvider(o.federatedVerificationURL, jwtToken, orgName, claimsCache)
							if err != nil {
								logger.Errorw("msg", "error calling federated provider", "error", err)
								return nil, fmt.Errorf("couldn't authorize using the provided token")
							}

							ctx = newJWTAuthContext(ctx, JWTAuthContext{
								Claims:      claims,
								ProviderKey: FederatedProviderKey,
							})

							return handler(ctx, req)
						}

						return nil, fmt.Errorf("couldn't match JWT provider: %w", err)
					}

					// When reached this point, one match has happened meaning the auth context
					// can continue.
					ctx := newJWTAuthContext(ctx, JWTAuthContext{
						Claims:      tokenInfo.Claims,
						ProviderKey: provider.providerKey,
						Token:       jwtToken,
					})

					//nolint:staticcheck
					return handler(ctx, req)
				}
			}

			return nil, ErrWrongContext
		}
	}
}

// callFederatedProvider calls the federated provider to verify the token
// it returns the claims of the token if the token is valid and verified
func callFederatedProvider(verifyURL string, jwtToken, orgName string, cache *expirable.LRU[string, *jwt.MapClaims]) (*jwt.MapClaims, error) {
	cacheKey := fmt.Sprintf("%s:%s", jwtToken, orgName)
	if claims, ok := cache.Get(cacheKey); ok {
		return claims, nil
	}

	client := &http.Client{}
	reqBody := &bytes.Buffer{}
	err := json.NewEncoder(reqBody).Encode(map[string]string{
		"token":    jwtToken,
		"org_name": orgName,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to encode request body: %w", err)
	}

	req, err := http.NewRequest(http.MethodPost, verifyURL, reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	var response struct {
		IssuerURL  string `json:"issuerUrl"`
		Repository string `json:"repository"`
		OrgID      string `json:"orgId"`
		OrgName    string `json:"orgName"`
		// error message
		Message   string `json:"message"`
		ErrorCode int    `json:"code"`
	}

	if err := json.Unmarshal(respBody, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d, errorCode: %d, error: %s", resp.StatusCode, response.ErrorCode, response.Message)
	}

	claims := &jwt.MapClaims{
		"iss":        response.IssuerURL,
		"repository": response.Repository,
		"orgId":      response.OrgID,
		"orgName":    response.OrgName,
	}

	cache.Add(cacheKey, claims)

	return claims, nil
}

// runProviderValidator runs the token parser for the given provider. Main logic of the code is taken from:
// https://github.com/go-kratos/kratos/blob/d0d5761f9ca89271231f23e1aad452362c3c09f9/middleware/auth/jwt/jwt.go#L86
// The main differences are:
//   - Always tries to parse with claims. The code is the one in charge of populating empty claims if not passed.
//   - Given a custom providerOption, if the token is valid and verified it tries to match its audience with any included
//     on such provider to check the token is expected by at least one provider.
//
// The information return by the function is the actual decoded jwt.Token ready to be operated with.
func runProviderValidator(provider providerOption, jwtToken string) (*jwt.Token, error) {
	if provider.keyFunc == nil {
		return nil, ErrMissingKeyFunc
	}

	if provider.verifyAudienceFunc == nil {
		return nil, ErrMissingVerifyAudienceFunc
	}

	var (
		tokenInfo  *jwt.Token
		err        error
		claimsFunc = func() jwt.Claims { return jwt.MapClaims{} }
	)
	if provider.claims != nil {
		claimsFunc = provider.claims
	}

	tokenInfo, err = jwt.ParseWithClaims(jwtToken, claimsFunc(), provider.keyFunc)

	if err != nil {
		var ve *jwt.ValidationError
		ok := errors.As(err, &ve)
		if !ok {
			return nil, errorsAPI.Unauthorized(reason, err.Error())
		}
		if ve.Errors&jwt.ValidationErrorMalformed != 0 {
			return nil, ErrTokenInvalid
		}
		if ve.Errors&(jwt.ValidationErrorExpired|jwt.ValidationErrorNotValidYet) != 0 {
			return nil, ErrTokenExpired
		}
		if ve.Inner != nil {
			return nil, ve.Inner
		}
		return nil, ErrTokenParseFail
	}
	if !tokenInfo.Valid {
		return nil, ErrTokenInvalid
	}
	if tokenInfo.Method != provider.signingMethod {
		return nil, ErrUnSupportSigningMethod
	}

	// Once the token is valid and verified, let's check for its audience. If the token's audience matches
	// the one on the provider, we return the token information meaning there is a match and that we can continue.
	// On the other hand if the verification fails, we continue with the list of the providers if any, otherwise we fail.
	if provider.verifyAudienceFunc(tokenInfo) {
		return tokenInfo, nil
	}

	return nil, errors.New("unexpected token, invalid audience")
}

// newJWTAuthContext put auth info into context
func newJWTAuthContext(ctx context.Context, authContext JWTAuthContext) context.Context {
	return context.WithValue(ctx, authzContextKey{}, authContext)
}

// FromJWTAuthContext extract JWTAuthContext from context
func FromJWTAuthContext(ctx context.Context) (authContext JWTAuthContext, ok bool) {
	authContext, ok = ctx.Value(authzContextKey{}).(JWTAuthContext)
	return
}
