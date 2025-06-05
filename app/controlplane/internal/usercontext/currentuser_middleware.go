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

package usercontext

import (
	"context"
	"errors"
	"fmt"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/golang-jwt/jwt/v4"

	"github.com/chainloop-dev/chainloop/app/controlplane/internal/usercontext/entities"
	"github.com/chainloop-dev/chainloop/app/controlplane/internal/usercontext/multijwtmiddleware"
	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/authz"
	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/biz"
	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/jwt/user"
	"github.com/go-kratos/kratos/v2/middleware"
)

// Middleware that injects the current user + organization to the context
func WithCurrentUserMiddleware(userUseCase biz.UserOrgFinder, logger *log.Helper) middleware.Middleware {
	return func(handler middleware.Handler) middleware.Handler {
		return func(ctx context.Context, req interface{}) (interface{}, error) {
			authInfo, ok := multijwtmiddleware.FromJWTAuthContext(ctx)
			// If not found means that there is no currentUser set in the context
			if !ok {
				logger.Warn("couldn't extract user, JWT parser middleware not running before this one?")
				return nil, errors.New("can't extract JWT info from the context")
			}

			if authInfo.ProviderKey != multijwtmiddleware.UserTokenProviderKey {
				return handler(ctx, req)
			}

			genericClaims, ok := authInfo.Claims.(jwt.MapClaims)
			if !ok {
				return nil, errors.New("error mapping the claims")
			}

			// Check wether the token is for a user or an API-token and handle accordingly
			// We've received a token for a user
			if genericClaims.VerifyAudience(user.Audience, true) {
				userID, ok := genericClaims["user_id"].(string)
				if !ok || userID == "" {
					return nil, errors.New("error mapping the user claims")
				}

				var err error
				ctx, err = setCurrentUser(ctx, userUseCase, userID, logger)
				if err != nil {
					return nil, fmt.Errorf("error setting current user: %w", err)
				}

				logger.Infow("msg", "[authN] processed credentials", "id", userID, "type", "user")
			}

			return handler(ctx, req)
		}
	}
}

// Find the user by its ID and sets it on the context
func setCurrentUser(ctx context.Context, userUC biz.UserOrgFinder, userID string, logger *log.Helper) (context.Context, error) {
	u, err := userUC.FindByID(ctx, userID)
	if err != nil {
		return nil, err
	}

	if u == nil {
		logger.Warnf("user with id %s not found", userID)
		return nil, errors.New("user not found")
	}

	return entities.WithCurrentUser(ctx, &entities.User{Email: u.Email, ID: u.ID, CreatedAt: u.CreatedAt}), nil
}

// Middleware that injects the current user + organization to the context during the attestation process
// it leverages the existing middlewares to set the current user and organization
// but with a skipping behavior since that's the one required by the attMiddleware multi-selector
func WithAttestationContextFromUser(userUC *biz.UserUseCase, logger *log.Helper) middleware.Middleware {
	return func(handler middleware.Handler) middleware.Handler {
		return func(ctx context.Context, req interface{}) (interface{}, error) {
			// If the token is not an user token, we don't need to do anything
			// note that this middleware is called by a multi-middleware that works in cascade mode
			authInfo, ok := multijwtmiddleware.FromJWTAuthContext(ctx)
			// If not found means that there is no currentUser set in the context
			if !ok {
				logger.Warn("couldn't extract org/user, JWT parser middleware not running before this one?")
				return nil, errors.New("can't extract JWT info from the context")
			}

			if authInfo.ProviderKey != multijwtmiddleware.UserTokenProviderKey {
				return handler(ctx, req)
			}

			// We received a user token during the attestation process
			// 1 - Set the current user from the user token
			// 2 - Set the current organization for the user token from the DB, header or default
			// 3 - Check if the user has permissions to perform attestations in the organization
			// 4 - Set the robot account
			// NOTE: we reuse the existing middlewares to set the current user and organization by wrapping the call
			// Now we can load the organization using the other middleware we have set
			return WithCurrentUserMiddleware(userUC, logger)(func(ctx context.Context, req any) (any, error) {
				return WithCurrentOrganizationMiddleware(userUC, logger)(func(ctx context.Context, req any) (any, error) {
					org := entities.CurrentOrg(ctx)
					if org == nil {
						return nil, errors.New("organization not found")
					}

					subject := CurrentAuthzSubject(ctx)
					if subject == "" {
						return nil, errors.New("missing authorization subject")
					}

					// Load the authorization subject from the context which might be related to a currentUser or an APItoken
					// TODO: move to authz middleware once we add support for all the tokens
					// for now in that middleware we are not mapping admins nor owners to a specific role
					if subject != string(authz.RoleAdmin) && subject != string(authz.RoleOwner) {
						return nil, fmt.Errorf("your user doesn't have permissions to perform attestations in this organization, role=%s, orgID=%s", subject, org.ID)
					}

					ctx = withAPIToken(ctx, &APIToken{OrgID: org.ID, ProviderKey: multijwtmiddleware.UserTokenProviderKey})
					logger.Infow("msg", "[authN] processed credentials", "type", multijwtmiddleware.UserTokenProviderKey)

					return handler(ctx, req)
				})(ctx, req)
			})(ctx, req)
		}
	}
}
