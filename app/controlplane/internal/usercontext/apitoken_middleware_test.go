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
	"bytes"
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

// Claim names, error substrings and entry point names that the tests of both entry points share.
const (
	claimAud          = "aud"
	claimJTI          = "jti"
	claimOrgID        = "org_id"
	claimOrgName      = "org_name"
	claimProjectID    = "project_id"
	claimScopeID      = "scope_id"
	claimScopeType    = "scope_type"
	errScopeMismatch  = "scope mismatch"
	errRecordsNoScope = "records no scope"
	entryAPI          = "API"
	entryAttestation  = "attestation"
)

const (
	// authorizationHeader carries the bearer token the attestation entry point reads.
	authorizationHeader = "Authorization"
	// orgHeader names the organization an instance token acts in.
	orgHeader = "Chainloop-Organization"
	// testSigningKey is the key that signs the test tokens. The testhelpers use the same key.
	testSigningKey = "test"
	// testIssuer is the issuer in the test tokens. Nothing checks it.
	testIssuer = "cp.chainloop"
)

// apiTokenEntryPoints holds one function per entry point. Each function runs a signed API token
// through its entry point, with orgName in the organization header. handler runs only when the
// entry point accepts the token.
var apiTokenEntryPoints = map[string]func(apiTokenUC *biz.APITokenUseCase, orgUC *biz.OrganizationUseCase, signed, orgName string, handler middleware.Handler) error{
	entryAPI: func(apiTokenUC *biz.APITokenUseCase, orgUC *biz.OrganizationUseCase, signed, orgName string, handler middleware.Handler) error {
		claims := jwt.MapClaims{}
		if _, _, err := jwt.NewParser().ParseUnverified(signed, claims); err != nil {
			return err
		}

		ctx := transport.NewServerContext(context.Background(), &fakeTransport{header: headerCarrier{orgHeader: {orgName}}})
		logger := log.NewHelper(log.NewStdLogger(io.Discard))
		_, err := WithCurrentAPITokenAndOrgMiddleware(apiTokenUC, orgUC, logger)(handler)(jwtmiddleware.NewContext(ctx, claims), nil)
		return err
	},
	entryAttestation: func(apiTokenUC *biz.APITokenUseCase, orgUC *biz.OrganizationUseCase, signed, orgName string, handler middleware.Handler) error {
		ctx := transport.NewServerContext(context.Background(), &fakeTransport{header: headerCarrier{
			authorizationHeader: {"Bearer " + signed},
			orgHeader:           {orgName},
		}})
		logger := log.NewHelper(log.NewStdLogger(io.Discard))
		_, err := middleware.Chain(
			attjwtmiddleware.WithJWTMulti(log.NewStdLogger(io.Discard), attjwtmiddleware.NewAPITokenProvider(testSigningKey)),
			WithAttestationContextFromAPIToken(apiTokenUC, orgUC, logger),
		)(handler)(ctx, nil)
		return err
	},
}

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
	// extraClaims adds claims to the JWT.
	extraClaims jwt.MapClaims
	// the middleware logic got skipped
	skipped         bool
	wantErr         bool
	wantErrContains string
}

func TestWithCurrentAPITokenAndOrgMiddleware(t *testing.T) {
	logger := log.NewHelper(log.NewStdLogger(io.Discard))
	matchingWorkflowID := uuid.New()
	otherWorkflowID := uuid.New()
	projectID := uuid.New()
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
			name:            "token revoked",
			receivedToken:   true,
			audience:        apitoken.Audience,
			tokenExists:     true,
			tokenRevoked:    true,
			wantErr:         true,
			wantErrContains: "revoked",
		},
		{
			name:          "token does not exist",
			receivedToken: true,
			audience:      apitoken.Audience,
			tokenExists:   false,
			wantErr:       true,
		},
		{
			name:            "org does not exist",
			receivedToken:   true,
			audience:        apitoken.Audience,
			tokenExists:     true,
			wantErr:         true,
			wantErrContains: "organization not found",
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
		{
			name:            "scope claims disagree with the DB row",
			receivedToken:   true,
			audience:        apitoken.Audience,
			tokenExists:     true,
			extraClaims:     jwt.MapClaims{claimScopeType: "product", claimScopeID: uuid.NewString()},
			wantErr:         true,
			wantErrContains: errScopeMismatch,
		},
		{
			name:            "a claim of the wrong type is refused",
			receivedToken:   true,
			audience:        apitoken.Audience,
			extraClaims:     jwt.MapClaims{claimProjectID: 42},
			wantErr:         true,
			wantErrContains: "mapping the API-token claims",
		},
	}

	for _, tc := range testCases {
		wantOrgID := uuid.New()
		wantOrg := &biz.Organization{ID: wantOrgID.String()}
		wantToken := &biz.APIToken{ID: uuid.New(), OrganizationID: wantOrgID, Scope: biz.ToPtr(authz.ResourceTypeOrganization), ScopeID: &wantOrgID}

		t.Run(tc.name, func(t *testing.T) {
			apiTokenRepo := mocks.NewAPITokenRepo(t)
			orgRepo := mocks.NewOrganizationRepo(t)
			apiTokenUC, err := biz.NewAPITokenUseCase(apiTokenRepo, &biz.APITokenJWTConfig{SymmetricHmacKey: testSigningKey}, nil, nil, nil, nil)
			require.NoError(t, err)
			orgUC := biz.NewOrganizationUseCase(orgRepo, nil, nil, nil, nil, nil, nil)
			require.NoError(t, err)

			ctx := context.Background()
			if tc.receivedToken {
				c := jwt.MapClaims{
					claimAud:   tc.audience,
					claimJTI:   wantToken.ID.String(),
					claimOrgID: wantOrgID.String(),
				}
				if tc.workflowIDClaim != "" {
					// A workflow claim always comes with its project
					c["workflow_id"] = tc.workflowIDClaim
					c[claimProjectID] = projectID.String()
					wantToken.ProjectID = &projectID
					wantToken.Scope = biz.ToPtr(authz.ResourceTypeProject)
					wantToken.ScopeID = &projectID
				}
				for k, v := range tc.extraClaims {
					c[k] = v
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

// After the row matches the signed claims, the service layer gets the scope from the database row.
func TestWithCurrentAPITokenAndOrgMiddlewareCarriesScope(t *testing.T) {
	logger := log.NewHelper(log.NewStdLogger(io.Discard))
	productID := uuid.New()
	projectA := uuid.New()
	orgID := uuid.New()

	testCases := []struct {
		name string
		// scope as stored on the token row
		rowOrg        uuid.UUID
		rowScope      *authz.ResourceType
		rowScopeID    *uuid.UUID
		rowProjectIDs []uuid.UUID
		// claims are the signed claims other than aud and jti.
		claims jwt.MapClaims
	}{
		{
			name:          "a product-scoped token",
			rowOrg:        orgID,
			rowScope:      biz.ToPtr(authz.ResourceTypeProduct),
			rowScopeID:    &productID,
			rowProjectIDs: []uuid.UUID{projectA},
			claims:        jwt.MapClaims{claimOrgID: orgID.String(), claimScopeType: "product", claimScopeID: productID.String()},
		},
		{
			name:       "an organization token with legacy claims",
			rowOrg:     orgID,
			rowScope:   biz.ToPtr(authz.ResourceTypeOrganization),
			rowScopeID: &orgID,
			claims:     jwt.MapClaims{claimOrgID: orgID.String()},
		},
		{
			name:     "an instance token",
			rowScope: biz.ToPtr(authz.ResourceTypeInstance),
			claims:   jwt.MapClaims{claimOrgID: "", "scope": authz.ScopeInstanceAdmin, claimScopeType: "instance"},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			token := &biz.APIToken{
				ID: uuid.New(), Name: "ci", OrganizationID: tc.rowOrg,
				Scope: tc.rowScope, ScopeID: tc.rowScopeID, ProjectIDs: tc.rowProjectIDs,
			}

			apiTokenRepo := mocks.NewAPITokenRepo(t)
			apiTokenRepo.On("FindByID", mock.Anything, token.ID).Return(token, nil)
			orgRepo := mocks.NewOrganizationRepo(t)
			orgRepo.On("FindByID", mock.Anything, orgID).Maybe().Return(&biz.Organization{ID: orgID.String()}, nil)

			apiTokenUC, err := biz.NewAPITokenUseCase(apiTokenRepo, &biz.APITokenJWTConfig{SymmetricHmacKey: testSigningKey}, nil, nil, nil, nil)
			require.NoError(t, err)
			orgUC := biz.NewOrganizationUseCase(orgRepo, nil, nil, nil, nil, nil, nil)

			claims := jwt.MapClaims{claimAud: apitoken.Audience, claimJTI: token.ID.String()}
			for k, v := range tc.claims {
				claims[k] = v
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
			assert.Equal(t, tc.rowProjectIDs, got.ProjectIDs)
		})
	}
}

// The middleware logs a row that disagrees with its signed claims as a security event. The log line
// includes the token ID and never the raw token.
func TestWithCurrentAPITokenAndOrgMiddlewareLogsAClaimsMismatch(t *testing.T) {
	const signedToken = "raw.signed.token"
	orgID := uuid.New()
	token := &biz.APIToken{ID: uuid.New(), OrganizationID: orgID, Scope: biz.ToPtr(authz.ResourceTypeOrganization), ScopeID: &orgID}

	apiTokenRepo := mocks.NewAPITokenRepo(t)
	apiTokenRepo.On("FindByID", mock.Anything, token.ID).Return(token, nil)
	orgRepo := mocks.NewOrganizationRepo(t)
	apiTokenUC, err := biz.NewAPITokenUseCase(apiTokenRepo, &biz.APITokenJWTConfig{SymmetricHmacKey: testSigningKey}, nil, nil, nil, nil)
	require.NoError(t, err)
	orgUC := biz.NewOrganizationUseCase(orgRepo, nil, nil, nil, nil, nil, nil)

	var buf bytes.Buffer
	logger := log.NewHelper(log.NewStdLogger(&buf))

	claims := jwt.MapClaims{
		claimAud: apitoken.Audience, claimJTI: token.ID.String(), claimOrgID: orgID.String(),
		claimScopeType: string(authz.ResourceTypeProduct), claimScopeID: uuid.NewString(),
		"raw": signedToken,
	}

	_, err = WithCurrentAPITokenAndOrgMiddleware(apiTokenUC, orgUC, logger)(
		func(context.Context, interface{}) (interface{}, error) { return nil, nil })(jwtmiddleware.NewContext(context.Background(), claims), nil)

	require.Error(t, err)
	assert.Contains(t, buf.String(), "disagrees with its signed claims")
	assert.Contains(t, buf.String(), token.ID.String())
	assert.NotContains(t, buf.String(), signedToken)
}

// preProductClaimRemoval are the claims a product token was signed with while the control plane
// still minted a product_id claim.
type preProductClaimRemoval struct {
	apitoken.CustomClaims
	ProductID string `json:"product_id,omitempty"`
}

// A JWT minted before the product_id claim was removed may still carry one. Both entry points
// ignore that claim, whatever product it names. The signed scope claims decide what the token
// reaches, and the row must match them.
func TestAPITokenMiddlewaresIgnoreAProductClaim(t *testing.T) {
	rowProduct, otherProduct, orgID := uuid.New(), uuid.New(), uuid.New()
	product, organization := authz.ResourceTypeProduct, authz.ResourceTypeOrganization

	testCases := []struct {
		name string
		// rowScope and rowScopeID are the scope stored on the token row
		rowScope   *authz.ResourceType
		rowScopeID *uuid.UUID
		// signScope signs the row's scope into the scope claims
		signScope bool
		// productClaim is the product_id claim the JWT carries
		productClaim uuid.UUID
		wantErr      string
	}{
		{name: "the claim names the row's product", rowScope: &product, rowScopeID: &rowProduct, signScope: true, productClaim: rowProduct},
		{name: "the claim names another product", rowScope: &product, rowScopeID: &rowProduct, signScope: true, productClaim: otherProduct},
		{name: "the claim is on an organization-scoped row", rowScope: &organization, rowScopeID: &orgID, productClaim: orgID},
		{name: "a product token minted before the scope claims", rowScope: &product, rowScopeID: &rowProduct, productClaim: rowProduct, wantErr: "create a new one"},
		{name: "the claim is on a row that records no scope", productClaim: otherProduct, wantErr: errRecordsNoScope},
	}

	for _, tc := range testCases {
		for entry, run := range apiTokenEntryPoints {
			t.Run(entry+"/"+tc.name, func(t *testing.T) {
				token := &biz.APIToken{ID: uuid.New(), Name: "ci", OrganizationID: orgID, Scope: tc.rowScope, ScopeID: tc.rowScopeID}

				apiTokenRepo := mocks.NewAPITokenRepo(t)
				apiTokenRepo.On("FindByID", mock.Anything, token.ID).Return(token, nil)
				orgRepo := mocks.NewOrganizationRepo(t)
				orgRepo.On("FindByID", mock.Anything, orgID).Maybe().Return(&biz.Organization{ID: orgID.String()}, nil)

				apiTokenUC, err := biz.NewAPITokenUseCase(apiTokenRepo, &biz.APITokenJWTConfig{SymmetricHmacKey: testSigningKey}, nil, nil, nil, nil)
				require.NoError(t, err)
				orgUC := biz.NewOrganizationUseCase(orgRepo, nil, nil, nil, nil, nil, nil)

				jwtClaims := apitoken.CustomClaims{
					OrgID: orgID.String(), OrgName: "acme", KeyName: token.Name,
					RegisteredClaims: jwt.RegisteredClaims{ID: token.ID.String(), Issuer: testIssuer, Audience: jwt.ClaimStrings{apitoken.Audience}},
				}
				if tc.signScope {
					jwtClaims.ScopeType, jwtClaims.ScopeID = string(*tc.rowScope), tc.rowScopeID.String()
				}

				signed, err := jwt.NewWithClaims(apitoken.SigningMethod, preProductClaimRemoval{
					CustomClaims: jwtClaims,
					ProductID:    tc.productClaim.String(),
				}).SignedString([]byte(testSigningKey))
				require.NoError(t, err)

				var got *entities.APIToken
				err = run(apiTokenUC, orgUC, signed, "", func(ctx context.Context, _ interface{}) (interface{}, error) {
					got = entities.CurrentAPIToken(ctx)
					return nil, nil
				})
				if tc.wantErr != "" {
					require.ErrorContains(t, err, tc.wantErr)
					return
				}
				require.NoError(t, err)

				require.NotNil(t, got)
				assert.Equal(t, tc.rowScope, got.Scope)
				assert.Equal(t, tc.rowScopeID, got.ScopeID)
			})
		}
	}
}

// The token's row decides whether the token takes the instance-admin path. That path reads the
// organization from the request header, not from the token row. The JWT's claims must name the same
// scope. Both entry points behave the same.
func TestAPITokenMiddlewaresResolveInstanceAdminTokens(t *testing.T) {
	rowOrg := &biz.Organization{ID: uuid.NewString(), Name: "row-org"}
	headerOrg := &biz.Organization{ID: uuid.NewString(), Name: "header-org"}

	testCases := []struct {
		name string
		// scopeClaim is the JWT's "scope" claim
		scopeClaim string
		// rowOrg is the organization on the token row, nil for an instance token
		rowOrg *biz.Organization
		// header is the organization named in the request header
		header  string
		wantOrg *biz.Organization
		wantErr string
	}{
		{name: "an instance-admin token takes the organization in the header", scopeClaim: authz.ScopeInstanceAdmin, header: headerOrg.Name, wantOrg: headerOrg},
		{name: "an instance-admin token without the header has no organization", scopeClaim: authz.ScopeInstanceAdmin},
		{name: "an organization token takes its row's organization", rowOrg: rowOrg, header: headerOrg.Name, wantOrg: rowOrg},
		// The signed claims and the row must agree, and the claims must agree with themselves.
		{name: "an instance row whose JWT names no scope is refused", header: headerOrg.Name, wantErr: "scope claims"},
		{name: "an organization row whose JWT carries the instance-admin claim is refused", scopeClaim: authz.ScopeInstanceAdmin, rowOrg: rowOrg, header: headerOrg.Name, wantErr: "scope claims"},
	}

	for _, tc := range testCases {
		for entry, run := range apiTokenEntryPoints {
			t.Run(entry+"/"+tc.name, func(t *testing.T) {
				token := &biz.APIToken{ID: uuid.New(), Name: "ci"}
				jwtClaims := apitoken.CustomClaims{
					KeyName:          token.Name,
					Scope:            tc.scopeClaim,
					RegisteredClaims: jwt.RegisteredClaims{ID: token.ID.String(), Issuer: testIssuer, Audience: jwt.ClaimStrings{apitoken.Audience}},
				}

				apiTokenRepo := mocks.NewAPITokenRepo(t)
				orgRepo := mocks.NewOrganizationRepo(t)
				var rowOrgID *uuid.UUID
				if tc.rowOrg != nil {
					token.OrganizationID = uuid.MustParse(tc.rowOrg.ID)
					rowOrgID = &token.OrganizationID
					jwtClaims.OrgID, jwtClaims.OrgName = tc.rowOrg.ID, tc.rowOrg.Name
					orgRepo.On("FindByID", mock.Anything, token.OrganizationID).Maybe().Return(tc.rowOrg, nil)
				}
				// The row records its scope: its organization's, or the instance's when it has none
				if rowOrgID != nil {
					token.Scope, token.ScopeID = biz.ToPtr(authz.ResourceTypeOrganization), rowOrgID
				} else {
					token.Scope = biz.ToPtr(authz.ResourceTypeInstance)
				}
				if tc.wantOrg == headerOrg {
					orgRepo.On("FindByName", mock.Anything, headerOrg.Name).Return(headerOrg, nil)
				}
				apiTokenRepo.On("FindByID", mock.Anything, token.ID).Return(token, nil)

				apiTokenUC, err := biz.NewAPITokenUseCase(apiTokenRepo, &biz.APITokenJWTConfig{SymmetricHmacKey: testSigningKey}, nil, nil, nil, nil)
				require.NoError(t, err)
				orgUC := biz.NewOrganizationUseCase(orgRepo, nil, nil, nil, nil, nil, nil)

				signed, err := jwt.NewWithClaims(apitoken.SigningMethod, jwtClaims).SignedString([]byte(testSigningKey))
				require.NoError(t, err)

				var gotOrg *entities.Org
				var gotToken *entities.APIToken
				var gotSubject string
				err = run(apiTokenUC, orgUC, signed, tc.header, func(ctx context.Context, _ interface{}) (interface{}, error) {
					gotOrg, gotToken, gotSubject = entities.CurrentOrg(ctx), entities.CurrentAPIToken(ctx), CurrentAuthzSubject(ctx)
					return nil, nil
				})
				if tc.wantErr != "" {
					require.ErrorContains(t, err, tc.wantErr)
					return
				}
				require.NoError(t, err)

				require.NotNil(t, gotToken)
				assert.Equal(t, token.ID.String(), gotToken.ID)
				assert.Equal(t, (&authz.SubjectAPIToken{ID: token.ID.String()}).String(), gotSubject)
				if tc.wantOrg == nil {
					assert.Nil(t, gotOrg)
					return
				}
				require.NotNil(t, gotOrg)
				assert.Equal(t, tc.wantOrg.ID, gotOrg.ID)
				assert.Equal(t, tc.wantOrg.Name, gotOrg.Name)
			})
		}
	}
}
