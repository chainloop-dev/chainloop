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

package usercontext

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/golang-jwt/jwt/v4"

	"github.com/chainloop-dev/chainloop/app/controlplane/internal/authz"
	"github.com/chainloop-dev/chainloop/app/controlplane/internal/biz"
	"github.com/chainloop-dev/chainloop/app/controlplane/internal/jwt/apitoken"
	"github.com/chainloop-dev/chainloop/app/controlplane/internal/usercontext/attjwtmiddleware"
	"github.com/go-kratos/kratos/v2/middleware"
	jwtMiddleware "github.com/go-kratos/kratos/v2/middleware/auth/jwt"
)

type APIToken struct {
	ID        string
	CreatedAt *time.Time
}

// Store the authorization subject
func WithAuthzSubject(ctx context.Context, subject string) context.Context {
	return context.WithValue(ctx, currentAuthzSubjectKey{}, subject)
}

func CurrentAuthzSubject(ctx context.Context) string {
	res := ctx.Value(currentAuthzSubjectKey{})
	if res == nil {
		return ""
	}
	return res.(string)
}

func WithCurrentAPIToken(ctx context.Context, token *APIToken) context.Context {
	return context.WithValue(ctx, currentAPITokenCtxKey{}, token)
}

func CurrentAPIToken(ctx context.Context) *APIToken {
	res := ctx.Value(currentAPITokenCtxKey{})
	if res == nil {
		return nil
	}
	return res.(*APIToken)
}

type currentAPITokenCtxKey struct{}
type currentAuthzSubjectKey struct{}

// Middleware that injects the API-Token + organization to the context
func WithCurrentAPITokenAndOrgMiddleware(apiTokenUC *biz.APITokenUseCase, orgUC *biz.OrganizationUseCase, logger *log.Helper) middleware.Middleware {
	return func(handler middleware.Handler) middleware.Handler {
		return func(ctx context.Context, req interface{}) (interface{}, error) {
			rawClaims, ok := jwtMiddleware.FromContext(ctx)
			// If not found means that there is no currentUser set in the context
			if !ok {
				logger.Warn("couldn't extract org/user, JWT parser middleware not running before this one?")
				return nil, errors.New("can't extract JWT info from the context")
			}

			genericClaims, ok := rawClaims.(jwt.MapClaims)
			if !ok {
				return nil, errors.New("error mapping the claims")
			}

			// We've received an API-token
			if genericClaims.VerifyAudience(apitoken.Audience, true) {
				var err error
				tokenID, ok := genericClaims["jti"].(string)
				if !ok || tokenID == "" {
					return nil, errors.New("error mapping the API-token claims")
				}

				ctx, err = setCurrentOrgAndAPIToken(ctx, apiTokenUC, orgUC, tokenID)
				if err != nil {
					return nil, fmt.Errorf("error setting current org and user: %w", err)
				}

				logger.Infow("msg", "[authN] processed credentials", "id", tokenID, "type", "API-token")
			}

			return handler(ctx, req)
		}
	}
}

// WithAttestationContextFromAPIToken injects the API-Token, organization + robot account to the context
func WithAttestationContextFromAPIToken(apiTokenUC *biz.APITokenUseCase, logger *log.Helper) middleware.Middleware {
	return func(handler middleware.Handler) middleware.Handler {
		return func(ctx context.Context, req interface{}) (interface{}, error) {
			authInfo, ok := attjwtmiddleware.FromJWTAuthContext(ctx)
			// If not found means that there is no currentUser set in the context
			if !ok {
				logger.Warn("couldn't extract org/user, JWT parser middleware not running before this one?")
				return nil, errors.New("can't extract JWT info from the context")
			}

			// If the token is not an API token, we don't need to do anything
			if authInfo.ProviderKey != attjwtmiddleware.APITokenProviderKey {
				return handler(ctx, req)
			}

			claims, ok := authInfo.Claims.(*apitoken.CustomClaims)
			if !ok {
				return nil, errors.New("error mapping the claims")
			}

			// We've received an API-token, double check its audience
			if !claims.VerifyAudience(apitoken.Audience, true) {
				return nil, errors.New("unexpected token, invalid audience")
			}

			tokenID := claims.ID
			if tokenID == "" {
				return nil, errors.New("error mapping the API-token claims")
			}

			ctx, err := setRobotAccountFromAPIToken(ctx, apiTokenUC, tokenID)
			if err != nil {
				return nil, fmt.Errorf("error extracting organization from APIToken: %w", err)
			}

			logger.Infow("msg", "[authN] processed credentials", "id", tokenID, "type", "API-token")

			return handler(ctx, req)
		}
	}
}

func setRobotAccountFromAPIToken(ctx context.Context, apiTokenUC *biz.APITokenUseCase, tokenID string) (context.Context, error) {
	if tokenID == "" {
		return nil, errors.New("error retrieving the key ID from the API token")
	}

	// Check that the token exists and is not revoked
	token, err := apiTokenUC.FindByID(ctx, tokenID)
	if err != nil {
		return nil, fmt.Errorf("error retrieving the API token: %w", err)
	} else if token == nil {
		return nil, errors.New("API token not found")
	}

	// Note: Expiration time does not need to be checked because that's done at the JWT
	// verification layer, which happens before this middleware is called
	if token.RevokedAt != nil {
		return nil, errors.New("API token revoked")
	}

	ctx = withRobotAccount(ctx, &RobotAccount{OrgID: token.OrganizationID.String(), ProviderKey: attjwtmiddleware.APITokenProviderKey})

	return ctx, nil
}

// Set the current organization and API-Token in the context
func setCurrentOrgAndAPIToken(ctx context.Context, apiTokenUC *biz.APITokenUseCase, orgUC *biz.OrganizationUseCase, tokenID string) (context.Context, error) {
	if tokenID == "" {
		return nil, errors.New("error retrieving the key ID from the API token")
	}

	// Check that the token exists and is not revoked
	token, err := apiTokenUC.FindByID(ctx, tokenID)
	if err != nil {
		return nil, fmt.Errorf("error retrieving the API token: %w", err)
	} else if token == nil {
		return nil, errors.New("API token not found")
	}

	// Note: Expiration time does not need to be checked because that's done at the JWT
	// verification layer, which happens before this middleware is called
	if token.RevokedAt != nil {
		return nil, errors.New("API token revoked")
	}

	// Find the associated organization
	org, err := orgUC.FindByID(ctx, token.OrganizationID.String())
	if err != nil {
		return nil, fmt.Errorf("error retrieving the organization: %w", err)
	} else if org == nil {
		return nil, errors.New("organization not found")
	}

	// Set the current organization and API-Token in the context
	ctx = WithCurrentOrg(ctx, &Org{Name: org.Name, ID: org.ID, CreatedAt: org.CreatedAt})
	ctx = WithCurrentAPIToken(ctx, &APIToken{ID: token.ID.String(), CreatedAt: token.CreatedAt})

	// Set the authorization subject that will be used to check the policies
	subjectAPIToken := authz.SubjectAPIToken{ID: token.ID.String()}
	ctx = WithAuthzSubject(ctx, subjectAPIToken.String())

	return ctx, nil
}
