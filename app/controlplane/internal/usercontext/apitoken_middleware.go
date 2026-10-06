//
// Copyright 2023-2026 The Chainloop Authors.
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
	"github.com/golang-jwt/jwt/v5"

	"github.com/chainloop-dev/chainloop/app/controlplane/internal/usercontext/attjwtmiddleware"
	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/authz"
	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/biz"
	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/jwt/apitoken"
	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/usercontext/entities"
	"github.com/chainloop-dev/chainloop/pkg/otelx"
	"github.com/go-kratos/kratos/v2/middleware"
	jwtMiddleware "github.com/go-kratos/kratos/v2/middleware/auth/jwt"
)

var apiTokenTracer = otelx.Tracer("chainloop-controlplane", "middleware/apitoken")

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

type currentAuthzSubjectKey struct{}

// Middleware that injects the API-Token + organization to the context
func WithCurrentAPITokenAndOrgMiddleware(apiTokenUC *biz.APITokenUseCase, orgUC *biz.OrganizationUseCase, logger *log.Helper) middleware.Middleware {
	return func(handler middleware.Handler) middleware.Handler {
		return func(ctx context.Context, req interface{}) (interface{}, error) {
			ctx, span := otelx.Start(ctx, apiTokenTracer, "WithCurrentAPITokenAndOrgMiddleware")
			defer span.End()

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
			if claimsHaveAudience(genericClaims, apitoken.Audience) {
				claims, err := apitoken.ClaimsFromMap(genericClaims)
				if err != nil {
					// This control plane never signs a claim of the wrong type. The log line never
					// includes the raw token or the claims map.
					id, _ := genericClaims["jti"].(string)
					logger.Errorw("msg", "[authN] API token claims do not decode", "id", id, "error", err)

					return nil, errors.New("error mapping the API-token claims")
				}

				if claims.ID == "" {
					return nil, errors.New("error mapping the API-token claims")
				}

				ctx, _, err = setCurrentOrgAndAPIToken(ctx, apiTokenUC, orgUC, claims, logger)
				if err != nil {
					return nil, fmt.Errorf("error setting current org and user: %w", err)
				}

				// legacy_claims marks the tokens minted before the scope claims, to plan the end of
				// their support
				logger.Infow("msg", "[authN] processed credentials", "id", claims.ID, "type", "API-token", "projectID", claims.ProjectID, "legacy_claims", !claims.HasScopeClaims())
			}

			return handler(ctx, req)
		}
	}
}

// WithAttestationContextFromAPIToken injects the API-Token, organization + robot account to the context
func WithAttestationContextFromAPIToken(apiTokenUC *biz.APITokenUseCase, orgUC *biz.OrganizationUseCase, logger *log.Helper) middleware.Middleware {
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
			if !claimsHaveAudience(claims, apitoken.Audience) {
				return nil, errors.New("unexpected token, invalid audience")
			}

			tokenID := claims.ID
			if tokenID == "" {
				return nil, errors.New("error mapping the API-token claims")
			}

			ctx, token, err := setCurrentOrgAndAPIToken(ctx, apiTokenUC, orgUC, claims, logger)
			if err != nil {
				return nil, fmt.Errorf("error setting current org and user: %w", err)
			}

			// The robot account comes from the row that VerifyClaims checked.
			ctx = WithRobotAccount(ctx, &RobotAccount{OrgID: token.OrganizationID.String(), ProviderKey: attjwtmiddleware.APITokenProviderKey})

			logger.Infow("msg", "[authN] processed credentials", "id", tokenID, "type", "API-token", "legacy_claims", !claims.HasScopeClaims())

			return handler(ctx, req)
		}
	}
}

// setCurrentOrgAndAPIToken loads the token's row and checks it against the signed claims. Then it
// puts the organization and the token in the context, and returns the row. The claims fix the
// token's scope, organization, project and workflow. The row must match them. The policies, the
// product project list and the revocation come only from the row.
func setCurrentOrgAndAPIToken(ctx context.Context, apiTokenUC *biz.APITokenUseCase, orgUC *biz.OrganizationUseCase, claims *apitoken.CustomClaims, logger *log.Helper) (context.Context, *biz.APIToken, error) {
	if claims == nil || claims.ID == "" {
		return nil, nil, errors.New("error retrieving the key ID from the API token")
	}

	// Check that the token exists and is not revoked
	token, err := apiTokenUC.FindByID(ctx, claims.ID)
	if err != nil {
		return nil, nil, fmt.Errorf("error retrieving the API token: %w", err)
	} else if token == nil {
		return nil, nil, errors.New("API token not found")
	}

	if err := token.VerifyClaims(claims); err != nil {
		// A row should never disagree with its signed claims. If it does, something wrote the row
		// incorrectly. The log line gives the reason and never includes the raw JWT. The caller
		// learns only that the token could not be verified.
		if errors.Is(err, biz.ErrAPITokenClaimsMismatch) {
			logger.Errorw("msg", "[authN] API token row disagrees with its signed claims", "id", claims.ID, "error", err)
			return nil, nil, biz.ErrAPITokenClaimsMismatch
		}

		return nil, nil, err
	}

	// Note: Expiration time does not need to be checked because that's done at the JWT
	// verification layer, which happens before this middleware is called
	if token.RevokedAt != nil {
		return nil, nil, errors.New("API token revoked")
	}

	// Handle instance admin tokens
	// An instance token has no organization of its own: it takes the one named in the header.
	if token.IsInstanceScoped() {
		// Check if org name provided in header
		orgName, _ := entities.GetOrganizationNameFromHeader(ctx)
		if orgName != "" {
			// Load organization from header
			org, err := orgUC.FindByName(ctx, orgName)
			if err != nil {
				return nil, nil, fmt.Errorf("error retrieving the organization: %w", err)
			} else if org == nil {
				return nil, nil, errors.New("organization not found")
			}

			ctx = entities.WithCurrentOrg(ctx, &entities.Org{Name: org.Name, ID: org.ID, CreatedAt: org.CreatedAt, Suspended: org.Suspended})
		}
		// If no org header, org context remains unset, operations will either:
		// 1. Work without org context
		// 2. Fail appropriately if they require org context
	} else {
		org, err := orgUC.FindByID(ctx, token.OrganizationID.String())
		if err != nil {
			return nil, nil, fmt.Errorf("error retrieving the organization: %w", err)
		} else if org == nil {
			return nil, nil, errors.New("organization not found")
		}

		// Set the current organization in the context
		ctx = entities.WithCurrentOrg(ctx, &entities.Org{Name: org.Name, ID: org.ID, CreatedAt: org.CreatedAt, Suspended: org.Suspended})
	}

	// Every value here comes from the row. VerifyClaims checked the scope, project and workflow
	// against the signed claims. The policies, the project list and the system flag come only from
	// the row.
	ctx = entities.WithCurrentAPIToken(ctx, &entities.APIToken{
		ID:           token.ID.String(),
		Name:         token.Name,
		CreatedAt:    token.CreatedAt,
		Token:        token.JWT,
		ProjectID:    token.ProjectID,
		ProjectName:  token.ProjectName,
		WorkflowID:   token.WorkflowID,
		WorkflowName: token.WorkflowName,
		Scope:        token.Scope,
		ScopeID:      token.ScopeID,
		ProjectIDs:   token.ProjectIDs,
		Policies:     token.Policies,
		IsSystem:     token.IsSystem,
	})

	// Set the authorization subject that will be used to check the policies
	subjectAPIToken := authz.SubjectAPIToken{ID: token.ID.String()}
	ctx = WithAuthzSubject(ctx, subjectAPIToken.String())

	return ctx, token, nil
}

func WithAPITokenUsageUpdater(apiTokenUC *biz.APITokenUseCase, logger *log.Helper) middleware.Middleware {
	return func(handler middleware.Handler) middleware.Handler {
		return func(ctx context.Context, req interface{}) (interface{}, error) {
			if token := entities.CurrentAPIToken(ctx); token != nil {
				if err := apiTokenUC.UpdateLastUsedAt(ctx, token.ID); err != nil {
					logger.Warnw("msg", "failed to record API token usage", "error", err)
				}
			}

			return handler(ctx, req)
		}
	}
}
