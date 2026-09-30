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

package usercontext

import (
	"context"
	"fmt"
	"io"
	"testing"
	"time"

	"github.com/chainloop-dev/chainloop/app/controlplane/internal/usercontext/attjwtmiddleware"
	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/authz"
	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/biz"
	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/biz/mocks"
	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/jwt/apitoken"
	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/usercontext/entities"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/go-kratos/kratos/v2/middleware"
	jwtmiddleware "github.com/go-kratos/kratos/v2/middleware/auth/jwt"
	"github.com/go-kratos/kratos/v2/transport"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

type middlewareTestCase struct {
	name          string
	receivedToken bool
	audience      string
	tokenExists   bool
	tokenRevoked  bool
	orgExist      bool
	// workflowIDClaim, if non-empty, is the workflow_id claim included on the JWT
	workflowIDClaim string
	// tokenWorkflowID, if set, is the workflow_id stored on the DB row
	tokenWorkflowID *uuid.UUID
	// the middleware logic got skipped
	skipped         bool
	wantErr         bool
	wantErrContains string
}

func TestWithCurrentAPITokenAndOrgMiddleware(t *testing.T) {
	logger := log.NewHelper(log.NewStdLogger(io.Discard))
	matchingWorkflowID := uuid.New()
	otherWorkflowID := uuid.New()
	testCases := []middlewareTestCase{
		{
			name:          "invalid audience", // in this case it gets ignored
			receivedToken: true,
			audience:      "another-aud",
			skipped:       true,
		},
		{
			name:          "token and org exists",
			receivedToken: true,
			audience:      apitoken.Audience,
			tokenExists:   true,
			orgExist:      true,
			wantErr:       false,
		},
		{
			name:          "token revoked",
			receivedToken: true,
			audience:      apitoken.Audience,
			tokenExists:   true,
			tokenRevoked:  true,
			wantErr:       true,
		},
		{
			name:          "token does not exist",
			receivedToken: true,
			audience:      apitoken.Audience,
			tokenExists:   false,
			wantErr:       true,
		},
		{
			name:          "org does not exist",
			receivedToken: true,
			audience:      apitoken.Audience,
			tokenExists:   true,
			wantErr:       true,
		},
		{
			name:          "no token received",
			receivedToken: false,
			audience:      apitoken.Audience,
			wantErr:       true,
		},
		{
			name:            "workflow claim matches DB row",
			receivedToken:   true,
			audience:        apitoken.Audience,
			tokenExists:     true,
			orgExist:        true,
			workflowIDClaim: matchingWorkflowID.String(),
			tokenWorkflowID: &matchingWorkflowID,
		},
		{
			name:            "workflow claim does not match DB row",
			receivedToken:   true,
			audience:        apitoken.Audience,
			tokenExists:     true,
			workflowIDClaim: matchingWorkflowID.String(),
			tokenWorkflowID: &otherWorkflowID,
			wantErr:         true,
			wantErrContains: "workflow mismatch",
		},
		{
			name:            "workflow claim present but DB row has none",
			receivedToken:   true,
			audience:        apitoken.Audience,
			tokenExists:     true,
			workflowIDClaim: matchingWorkflowID.String(),
			wantErr:         true,
			wantErrContains: "workflow mismatch",
		},
	}

	for _, tc := range testCases {
		wantOrgID := uuid.New()
		wantOrg := &biz.Organization{ID: wantOrgID.String()}
		wantToken := &biz.APIToken{ID: uuid.New(), OrganizationID: wantOrgID}

		t.Run(tc.name, func(t *testing.T) {
			apiTokenRepo := mocks.NewAPITokenRepo(t)
			orgRepo := mocks.NewOrganizationRepo(t)
			apiTokenUC, err := biz.NewAPITokenUseCase(apiTokenRepo, &biz.APITokenJWTConfig{SymmetricHmacKey: "test"}, nil, nil, nil, nil)
			require.NoError(t, err)
			orgUC := biz.NewOrganizationUseCase(orgRepo, nil, nil, nil, nil, nil, nil)
			require.NoError(t, err)

			ctx := context.Background()
			if tc.receivedToken {
				c := jwt.MapClaims{
					"aud": tc.audience,
					"jti": wantToken.ID.String(),
				}
				if tc.workflowIDClaim != "" {
					c["workflow_id"] = tc.workflowIDClaim
				}

				ctx = jwtmiddleware.NewContext(ctx, c)
			}

			if tc.tokenExists {
				if tc.tokenRevoked {
					wantToken.RevokedAt = toTimePtr(time.Now())
				}
				if tc.tokenWorkflowID != nil {
					wantToken.WorkflowID = tc.tokenWorkflowID
				}

				apiTokenRepo.On("FindByID", mock.Anything, wantToken.ID).Return(wantToken, nil)
			} else if tc.receivedToken {
				apiTokenRepo.On("FindByID", mock.Anything, wantToken.ID).Maybe().Return(nil, nil)
			}

			if tc.orgExist {
				orgRepo.On("FindByID", mock.Anything, wantOrgID).Return(wantOrg, nil)
			} else if tc.receivedToken {
				orgRepo.On("FindByID", mock.Anything, wantOrgID).Maybe().Return(nil, nil)
			}

			m := WithCurrentAPITokenAndOrgMiddleware(apiTokenUC, orgUC, logger)
			_, err = m(
				func(ctx context.Context, _ interface{}) (interface{}, error) {
					if tc.wantErr {
						return nil, nil
					}

					if !tc.skipped {
						// Check that the wrapped handler contains the user and org
						assert.Equal(t, entities.CurrentOrg(ctx).ID, wantOrg.ID)
						assert.Equal(t, entities.CurrentAPIToken(ctx).ID, wantToken.ID.String())
						assert.Equal(t, CurrentAuthzSubject(ctx), fmt.Sprintf("api-token:%s", wantToken.ID))
					}

					return nil, nil
				})(ctx, nil)

			if tc.wantErr {
				assert.Error(t, err)
				if tc.wantErrContains != "" {
					assert.Contains(t, err.Error(), tc.wantErrContains)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func toTimePtr(t time.Time) *time.Time {
	return &t
}

// The resource scope must reach the service layer from the database row, never from a claim:
// the row is the authorization input, the claim is only ever a cross-check.
func TestWithCurrentAPITokenAndOrgMiddlewareCarriesScope(t *testing.T) {
	logger := log.NewHelper(log.NewStdLogger(io.Discard))
	productID := uuid.New()
	projectA := uuid.New()

	testCases := []struct {
		name string
		// scope as stored on the token row
		rowScope       *authz.ResourceType
		rowScopeID     *uuid.UUID
		rowProjectIDs  []uuid.UUID
		wantProjectIDs []uuid.UUID
		// the instance-admin claim, which is a different notion of "scope" entirely
		instanceScopeClaim string
	}{
		{
			name:           "a product-scoped token",
			rowScope:       toPtr(authz.ResourceTypeProduct),
			rowScopeID:     &productID,
			rowProjectIDs:  []uuid.UUID{projectA},
			wantProjectIDs: []uuid.UUID{projectA},
		},
		{
			name: "an unscoped token carries no scope",
		},
		{
			// The instance-admin claim and the resource scope are independent values.
			name:               "an instance-admin token keeps its instance scope",
			instanceScopeClaim: authz.ScopeInstanceAdmin,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			orgID := uuid.New()
			token := &biz.APIToken{
				ID: uuid.New(), Name: "ci", OrganizationID: orgID,
				Scope: tc.rowScope, ScopeID: tc.rowScopeID, ProjectIDs: tc.rowProjectIDs,
			}

			apiTokenRepo := mocks.NewAPITokenRepo(t)
			apiTokenRepo.On("FindByID", mock.Anything, token.ID).Return(token, nil)
			orgRepo := mocks.NewOrganizationRepo(t)
			orgRepo.On("FindByID", mock.Anything, orgID).Maybe().Return(&biz.Organization{ID: orgID.String()}, nil)

			apiTokenUC, err := biz.NewAPITokenUseCase(apiTokenRepo, &biz.APITokenJWTConfig{SymmetricHmacKey: "test"}, nil, nil, nil, nil)
			require.NoError(t, err)
			orgUC := biz.NewOrganizationUseCase(orgRepo, nil, nil, nil, nil, nil, nil)

			claims := jwt.MapClaims{"aud": apitoken.Audience, "jti": token.ID.String()}
			if tc.instanceScopeClaim != "" {
				claims["scope"] = tc.instanceScopeClaim
			}
			ctx := jwtmiddleware.NewContext(context.Background(), claims)

			var got *entities.APIToken
			_, err = WithCurrentAPITokenAndOrgMiddleware(apiTokenUC, orgUC, logger)(
				func(ctx context.Context, _ interface{}) (interface{}, error) {
					got = entities.CurrentAPIToken(ctx)
					return nil, nil
				})(ctx, nil)
			require.NoError(t, err)

			require.NotNil(t, got)
			assert.Equal(t, tc.rowScope, got.Scope)
			assert.Equal(t, tc.rowScopeID, got.ScopeID)
			assert.Equal(t, tc.instanceScopeClaim, got.InstanceScope)
			assert.Equal(t, tc.wantProjectIDs, got.ProjectIDs)
		})
	}
}

func toPtr[T any](v T) *T {
	return &v
}

// preProductClaimRemoval are the claims a product token was signed with while the control plane
// still minted a product_id claim.
type preProductClaimRemoval struct {
	apitoken.CustomClaims
	ProductID string `json:"product_id,omitempty"`
}

// A JWT minted before the product_id claim was dropped may still carry one. The row decides what
// a token is confined to, so both entry points accept such a JWT and ignore the claim, whatever
// product it names.
func TestAPITokenMiddlewaresIgnoreAProductClaim(t *testing.T) {
	const signingKey = "test"
	logger := log.NewHelper(log.NewStdLogger(io.Discard))
	rowProduct, otherProduct, orgID := uuid.New(), uuid.New(), uuid.New()
	product, organization := authz.ResourceTypeProduct, authz.ResourceTypeOrganization

	testCases := []struct {
		name string
		// rowScope and rowScopeID are the scope stored on the token row
		rowScope   *authz.ResourceType
		rowScopeID *uuid.UUID
		// productClaim is the product_id claim the JWT carries
		productClaim uuid.UUID
	}{
		{name: "the claim names the row's product", rowScope: &product, rowScopeID: &rowProduct, productClaim: rowProduct},
		{name: "the claim names another product", rowScope: &product, rowScopeID: &rowProduct, productClaim: otherProduct},
		{name: "the claim is on an organization-scoped row", rowScope: &organization, rowScopeID: &orgID, productClaim: orgID},
		{name: "the claim is on a row that records no scope", productClaim: otherProduct},
	}

	type entryPoint func(apiTokenUC *biz.APITokenUseCase, orgUC *biz.OrganizationUseCase, signed string, handler middleware.Handler) error
	entryPoints := map[string]entryPoint{
		"API": func(apiTokenUC *biz.APITokenUseCase, orgUC *biz.OrganizationUseCase, signed string, handler middleware.Handler) error {
			claims := jwt.MapClaims{}
			if _, _, err := jwt.NewParser().ParseUnverified(signed, claims); err != nil {
				return err
			}

			_, err := WithCurrentAPITokenAndOrgMiddleware(apiTokenUC, orgUC, logger)(handler)(jwtmiddleware.NewContext(context.Background(), claims), nil)
			return err
		},
		"attestation": func(apiTokenUC *biz.APITokenUseCase, orgUC *biz.OrganizationUseCase, signed string, handler middleware.Handler) error {
			ctx := transport.NewServerContext(context.Background(), &fakeTransport{header: headerCarrier{"Authorization": {"Bearer " + signed}}})
			_, err := middleware.Chain(
				attjwtmiddleware.WithJWTMulti(log.NewStdLogger(io.Discard), attjwtmiddleware.NewAPITokenProvider(signingKey)),
				WithAttestationContextFromAPIToken(apiTokenUC, orgUC, logger),
			)(handler)(ctx, nil)
			return err
		},
	}

	for _, tc := range testCases {
		for entry, run := range entryPoints {
			t.Run(entry+"/"+tc.name, func(t *testing.T) {
				token := &biz.APIToken{ID: uuid.New(), Name: "ci", OrganizationID: orgID, Scope: tc.rowScope, ScopeID: tc.rowScopeID}

				apiTokenRepo := mocks.NewAPITokenRepo(t)
				apiTokenRepo.On("FindByID", mock.Anything, token.ID).Return(token, nil)
				orgRepo := mocks.NewOrganizationRepo(t)
				orgRepo.On("FindByID", mock.Anything, orgID).Return(&biz.Organization{ID: orgID.String()}, nil)

				apiTokenUC, err := biz.NewAPITokenUseCase(apiTokenRepo, &biz.APITokenJWTConfig{SymmetricHmacKey: signingKey}, nil, nil, nil, nil)
				require.NoError(t, err)
				orgUC := biz.NewOrganizationUseCase(orgRepo, nil, nil, nil, nil, nil, nil)

				signed, err := jwt.NewWithClaims(apitoken.SigningMethod, preProductClaimRemoval{
					CustomClaims: apitoken.CustomClaims{
						OrgID: orgID.String(), OrgName: "acme", KeyName: token.Name,
						RegisteredClaims: jwt.RegisteredClaims{ID: token.ID.String(), Issuer: "test", Audience: jwt.ClaimStrings{apitoken.Audience}},
					},
					ProductID: tc.productClaim.String(),
				}).SignedString([]byte(signingKey))
				require.NoError(t, err)

				var got *entities.APIToken
				err = run(apiTokenUC, orgUC, signed, func(ctx context.Context, _ interface{}) (interface{}, error) {
					got = entities.CurrentAPIToken(ctx)
					return nil, nil
				})
				require.NoError(t, err)

				require.NotNil(t, got)
				assert.Equal(t, tc.rowScope, got.Scope)
				assert.Equal(t, tc.rowScopeID, got.ScopeID)
			})
		}
	}
}
