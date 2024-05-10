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

package attjwtmiddleware

import (
	"context"
	"errors"
	"strings"

	"github.com/chainloop-dev/chainloop/app/controlplane/internal/jwt/apitoken"
	"github.com/chainloop-dev/chainloop/app/controlplane/internal/jwt/robotaccount"
	"github.com/chainloop-dev/chainloop/app/controlplane/internal/jwt/user"
	errorsAPI "github.com/go-kratos/kratos/v2/errors"
	"github.com/go-kratos/kratos/v2/middleware"
	"github.com/go-kratos/kratos/v2/transport"
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
)

var (
	ErrMissingJwtToken        = errorsAPI.Unauthorized(reason, "JWT token is missing")
	ErrMissingKeyFunc         = errorsAPI.Unauthorized(reason, "keyFunc is missing")
	ErrTokenInvalid           = errorsAPI.Unauthorized(reason, "Token is invalid")
	ErrTokenExpired           = errorsAPI.Unauthorized(reason, "JWT token has expired")
	ErrTokenParseFail         = errorsAPI.Unauthorized(reason, "Fail to parse JWT token ")
	ErrUnSupportSigningMethod = errorsAPI.Unauthorized(reason, "Wrong signing method")
	ErrWrongContext           = errorsAPI.Unauthorized(reason, "Wrong context for middleware")
)

// NewAttestationRobotAccountProvider return the configuration to validate and verify token issued for Robot Accounts
func NewAttestationRobotAccountProvider(signingSecret string) JWTOption {
	return WithTokenProvider(
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

// NewAttestationAPITokenProvider return the configuration to validate and verify token issued for API Tokens
func NewAttestationAPITokenProvider(signingSecret string) JWTOption {
	return WithTokenProvider(
		APITokenProviderKey,
		WithVerifyAudienceFunc(func(token *jwt.Token) bool {
			claims, ok := token.Claims.(jwt.MapClaims)
			if !ok {
				return false
			}
			for _, aud := range []string{apitoken.Audience} {
				if claims.VerifyAudience(aud, true) {
					return true
				}
			}
			return false
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
	tokenProviders []providerOption
}

func WithTokenProvider(providerKey string, opts ...TokenProviderOption) JWTOption {
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

// WithAttestationTokenMiddleware creates a custom JWT middleware that configured with different token providers
// tries to run all validations from an incoming token. If you of the providers matches the expected audience
// it gets parsed and sent down to the next middleware
func WithAttestationTokenMiddleware(opts ...JWTOption) middleware.Middleware {
	o := &options{}
	for _, opt := range opts {
		opt(o)
	}
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

						return nil, err
					}

					ctx := newJWTAuthContext(ctx, JWTAuthContext{
						Claims:      tokenInfo.Claims,
						ProviderKey: provider.providerKey,
					})

					//nolint:staticcheck
					return handler(ctx, req)
				}
			}

			return nil, ErrWrongContext
		}
	}
}

// runProviderValidator runs the token parser for the given provider
func runProviderValidator(provider providerOption, jwtToken string) (*jwt.Token, error) {
	if provider.keyFunc == nil {
		return nil, ErrMissingKeyFunc
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
