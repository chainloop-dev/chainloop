//
// Copyright 2026 The Chainloop Authors.
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
	"io"
	"os"
	"testing"
	"time"

	"github.com/chainloop-dev/chainloop/app/controlplane/internal/usercontext/attjwtmiddleware"
	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/authz"
	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/biz"
	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/biz/testhelpers"
	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/data/ent"
	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/jwt/apitoken"
	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/usercontext/entities"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/go-kratos/kratos/v2/middleware"
	jwtmiddleware "github.com/go-kratos/kratos/v2/middleware/auth/jwt"
	"github.com/go-kratos/kratos/v2/transport"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The token the service layer sees carries exactly the scope its row records, once the row has
// confirmed the claims the token was signed with.
func TestAPITokenMiddlewareCarriesTheRowScope(t *testing.T) {
	if !testhelpers.IntegrationTestsEnabled() {
		t.Skip()
	}

	tu := testhelpers.NewTestingUseCases(t)
	defer tu.DB.Close(t)

	ctx := context.Background()
	logger := log.NewHelper(log.NewStdLogger(io.Discard))

	org, err := tu.Organization.CreateWithRandomName(ctx)
	require.NoError(t, err)
	orgID := uuid.MustParse(org.ID)
	project, err := tu.Project.Create(ctx, org.ID, "scoped")
	require.NoError(t, err)

	testCases := []struct {
		name      string
		opts      biz.APITokenCreateOpts
		wantErr   bool
		wantScope *authz.ResourceType
		// wantOrgScoped, wantInstanceScoped and wantReach are what the service layer reads off the token
		wantOrgScoped      bool
		wantInstanceScoped bool
		wantReach          []uuid.UUID
	}{
		{
			name:      "an organization row",
			opts:      biz.APITokenCreateOpts{OrganizationID: &orgID, Scope: biz.ToPtr(authz.ResourceTypeOrganization), ScopeID: &orgID},
			wantScope: biz.ToPtr(authz.ResourceTypeOrganization), wantOrgScoped: true,
		},
		{
			name:      "a project row",
			opts:      biz.APITokenCreateOpts{OrganizationID: &orgID, ProjectID: &project.ID, Scope: biz.ToPtr(authz.ResourceTypeProject), ScopeID: &project.ID},
			wantScope: biz.ToPtr(authz.ResourceTypeProject), wantReach: []uuid.UUID{project.ID},
		},
		{
			// No organization of its own and no org header: the org context stays unset, and the
			// token acts for the whole instance
			name:      "an instance row",
			opts:      biz.APITokenCreateOpts{Scope: biz.ToPtr(authz.ResourceTypeInstance)},
			wantScope: biz.ToPtr(authz.ResourceTypeInstance), wantInstanceScoped: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			opts := tc.opts
			opts.Name, opts.Policies = "row-"+uuid.NewString(), []*authz.Policy{}
			row, err := tu.Repos.APITokenRepo.Create(ctx, &opts)
			require.NoError(t, err)

			signed, err := tu.APIToken.RegenerateJWT(ctx, row.ID, time.Hour)
			require.NoError(t, err)
			claims := jwt.MapClaims{}
			_, _, err = jwt.NewParser().ParseUnverified(signed.JWT, claims)
			require.NoError(t, err)

			var got *entities.APIToken
			_, err = WithCurrentAPITokenAndOrgMiddleware(tu.APIToken, tu.Organization, logger)(
				func(ctx context.Context, _ interface{}) (interface{}, error) {
					got = entities.CurrentAPIToken(ctx)
					return nil, nil
				})(jwtmiddleware.NewContext(ctx, claims), nil)
			if tc.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.NotNil(t, got)

			assert.Equal(t, tc.wantScope, got.Scope)
			assert.Equal(t, opts.ScopeID, got.ScopeID)
			assert.Equal(t, opts.ProjectID, got.ProjectID)
			assert.Equal(t, tc.wantOrgScoped, got.IsOrgScoped())
			assert.Equal(t, tc.wantInstanceScoped, got.IsInstanceScoped())
			if tc.wantOrgScoped || tc.wantInstanceScoped {
				assert.Nil(t, got.ReachableProjects())
				return
			}
			assert.Equal(t, tc.wantReach, got.ReachableProjects())
		})
	}
}

const scopeBackfillMigration = "../../pkg/data/ent/migrate/migrations/20260930160057.sql"

// authenticated is what an entry point put in the context, or why it refused the token.
type authenticated struct {
	token *entities.APIToken
	org   *entities.Org
	err   error
}

// authenticateAtBothEntryPoints sends signed through the API and the attestation entry points,
// naming orgName in the organization header.
func authenticateAtBothEntryPoints(t *testing.T, tu *testhelpers.TestingUseCases, signed, orgName string) map[string]authenticated {
	t.Helper()
	const orgHeader = "Chainloop-Organization"
	logger := log.NewHelper(log.NewStdLogger(io.Discard))

	capture := func(out *authenticated) middleware.Handler {
		return func(ctx context.Context, _ interface{}) (interface{}, error) {
			out.token, out.org = entities.CurrentAPIToken(ctx), entities.CurrentOrg(ctx)
			return nil, nil
		}
	}

	var api, attestation authenticated

	claims := jwt.MapClaims{}
	_, _, err := jwt.NewParser().ParseUnverified(signed, claims)
	require.NoError(t, err)
	ctx := transport.NewServerContext(context.Background(), &fakeTransport{header: headerCarrier{orgHeader: {orgName}}})
	_, api.err = WithCurrentAPITokenAndOrgMiddleware(tu.APIToken, tu.Organization, logger)(capture(&api))(jwtmiddleware.NewContext(ctx, claims), nil)

	ctx = transport.NewServerContext(context.Background(), &fakeTransport{header: headerCarrier{
		authorizationHeader: {"Bearer " + signed},
		orgHeader:           {orgName},
	}})
	_, attestation.err = middleware.Chain(
		attjwtmiddleware.WithJWTMulti(log.NewStdLogger(io.Discard), attjwtmiddleware.NewAPITokenProvider("test")),
		WithAttestationContextFromAPIToken(tu.APIToken, tu.Organization, logger),
	)(capture(&attestation))(ctx, nil)

	return map[string]authenticated{entryAPI: api, entryAttestation: attestation}
}

// signLegacy signs claims the way a control plane from before the scope claims did, with the
// testhelpers' signing key.
func signLegacy(t *testing.T, tokenID uuid.UUID, claims jwt.MapClaims) string {
	t.Helper()
	all := jwt.MapClaims{"token_name": "legacy", claimJTI: tokenID.String(), "iss": "cp.chainloop", claimAud: []string{apitoken.Audience}}
	for k, v := range claims {
		all[k] = v
	}

	signed, err := jwt.NewWithClaims(apitoken.SigningMethod, all).SignedString([]byte("test"))
	require.NoError(t, err)

	return signed
}

// Organization, project, workflow-pinned and instance tokens minted before the scope claims keep
// working, with the scope they always had: their rows went through the scope backfill and their
// claims imply the same scope. A product token minted before the scope claims, and a row written
// with no scope after the backfill ran, are refused.
func TestAPITokenMiddlewareAcceptsTokensMintedBeforeTheScopeClaims(t *testing.T) {
	if !testhelpers.IntegrationTestsEnabled() {
		t.Skip()
	}

	tu := testhelpers.NewTestingUseCases(t)
	defer tu.DB.Close(t)
	ctx := context.Background()

	org, err := tu.Organization.CreateWithRandomName(ctx)
	require.NoError(t, err)
	orgID := uuid.MustParse(org.ID)
	headerOrg, err := tu.Organization.CreateWithRandomName(ctx)
	require.NoError(t, err)
	headerOrgID := uuid.MustParse(headerOrg.ID)
	project, err := tu.Project.Create(ctx, org.ID, "legacy")
	require.NoError(t, err)
	other, err := tu.Project.Create(ctx, org.ID, "other")
	require.NoError(t, err)
	workflow, err := tu.Workflow.Create(ctx, &biz.WorkflowCreateOpts{OrgID: org.ID, Name: "legacy", Project: project.Name})
	require.NoError(t, err)
	product := uuid.New()

	orgClaims := jwt.MapClaims{claimOrgID: org.ID, claimOrgName: org.Name}
	projectClaims := jwt.MapClaims{claimOrgID: org.ID, claimOrgName: org.Name, claimProjectID: project.ID.String(), "project_name": project.Name}
	instanceClaims := jwt.MapClaims{claimOrgID: "", claimOrgName: "", "scope": authz.ScopeInstanceAdmin}

	testCases := []struct {
		name string
		// row writes the columns the control plane that minted the token wrote
		row    func(*ent.APITokenCreate) *ent.APITokenCreate
		claims jwt.MapClaims
		header string
		// afterBackfill writes the row once the backfill has run, as a control plane from before
		// the scope columns does during a rolling upgrade or after a rollback
		afterBackfill bool

		wantErr      string
		wantScope    authz.ResourceType
		wantScopeID  *uuid.UUID
		wantOrg      *uuid.UUID
		wantReachAll bool
		wantReach    []uuid.UUID
	}{
		{
			name:      "oldest organization token, no org_name claim",
			row:       func(c *ent.APITokenCreate) *ent.APITokenCreate { return c.SetOrganizationID(orgID) },
			claims:    jwt.MapClaims{claimOrgID: org.ID},
			wantScope: authz.ResourceTypeOrganization, wantScopeID: &orgID, wantOrg: &orgID, wantReachAll: true,
		},
		{
			name:   "organization token ignores the organization header",
			row:    func(c *ent.APITokenCreate) *ent.APITokenCreate { return c.SetOrganizationID(orgID) },
			claims: orgClaims, header: headerOrg.Name,
			wantScope: authz.ResourceTypeOrganization, wantScopeID: &orgID, wantOrg: &orgID, wantReachAll: true,
		},
		{
			name: "project token",
			row: func(c *ent.APITokenCreate) *ent.APITokenCreate {
				return c.SetOrganizationID(orgID).SetProjectID(project.ID)
			},
			claims:    projectClaims,
			wantScope: authz.ResourceTypeProject, wantScopeID: &project.ID, wantOrg: &orgID, wantReach: []uuid.UUID{project.ID},
		},
		{
			name: "workflow-pinned token",
			row: func(c *ent.APITokenCreate) *ent.APITokenCreate {
				return c.SetOrganizationID(orgID).SetProjectID(project.ID).SetWorkflowID(workflow.ID)
			},
			claims: jwt.MapClaims{claimOrgID: org.ID, claimOrgName: org.Name, claimProjectID: project.ID.String(), "project_name": project.Name,
				"workflow_id": workflow.ID.String(), "workflow_name": workflow.Name},
			wantScope: authz.ResourceTypeProject, wantScopeID: &project.ID, wantOrg: &orgID, wantReach: []uuid.UUID{project.ID},
		},
		{
			name:   "instance token takes the organization header",
			row:    func(c *ent.APITokenCreate) *ent.APITokenCreate { return c },
			claims: instanceClaims, header: headerOrg.Name,
			wantScope: authz.ResourceTypeInstance, wantOrg: &headerOrgID, wantReachAll: true,
		},
		{
			name:      "instance token without the header has no organization",
			row:       func(c *ent.APITokenCreate) *ent.APITokenCreate { return c },
			claims:    instanceClaims,
			wantScope: authz.ResourceTypeInstance, wantReachAll: true,
		},
		{
			name: "product token minted before the scope claims",
			row: func(c *ent.APITokenCreate) *ent.APITokenCreate {
				return c.SetOrganizationID(orgID).SetScope(authz.ResourceTypeProduct).SetScopeID(product).SetProjectIds([]uuid.UUID{project.ID})
			},
			claims:  orgClaims,
			wantErr: "create a new one",
		},
		{
			name: "revoked organization token",
			row: func(c *ent.APITokenCreate) *ent.APITokenCreate {
				return c.SetOrganizationID(orgID).SetRevokedAt(time.Now())
			},
			claims:  orgClaims,
			wantErr: "revoked",
		},
		{
			name:          "organization row written with no scope after the backfill",
			row:           func(c *ent.APITokenCreate) *ent.APITokenCreate { return c.SetOrganizationID(orgID) },
			claims:        orgClaims,
			afterBackfill: true,
			wantErr:       errRecordsNoScope,
		},
		{
			name:          "instance row written with no scope after the backfill",
			row:           func(c *ent.APITokenCreate) *ent.APITokenCreate { return c },
			claims:        instanceClaims,
			afterBackfill: true,
			wantErr:       errRecordsNoScope,
		},
	}

	create := func(i int) uuid.UUID {
		row, err := testCases[i].row(tu.Data.DB.APIToken.Create().SetName("legacy-" + uuid.NewString())).Save(ctx)
		require.NoError(t, err)
		return row.ID
	}

	ids := make([]uuid.UUID, len(testCases))
	for i, tc := range testCases {
		if !tc.afterBackfill {
			ids[i] = create(i)
		}
	}

	backfill, err := os.ReadFile(scopeBackfillMigration)
	require.NoError(t, err)
	_, err = tu.Data.SQLDB.ExecContext(ctx, string(backfill))
	require.NoError(t, err)

	for i, tc := range testCases {
		if tc.afterBackfill {
			ids[i] = create(i)
		}
	}

	for i, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			for entry, got := range authenticateAtBothEntryPoints(t, tu, signLegacy(t, ids[i], tc.claims), tc.header) {
				if tc.wantErr != "" {
					assert.ErrorContains(t, got.err, tc.wantErr, entry)
					assert.NotErrorIs(t, got.err, biz.ErrAPITokenClaimsMismatch, entry)
					continue
				}

				require.NoError(t, got.err, entry)
				require.NotNil(t, got.token, entry)
				require.NotNil(t, got.token.Scope, entry)
				assert.Equal(t, tc.wantScope, *got.token.Scope, entry)
				assert.Equal(t, tc.wantScopeID, got.token.ScopeID, entry)
				if tc.wantOrg == nil {
					assert.Nil(t, got.org, entry)
				} else {
					require.NotNil(t, got.org, entry)
					assert.Equal(t, tc.wantOrg.String(), got.org.ID, entry)
				}
				if tc.wantReachAll {
					assert.Nil(t, got.token.ReachableProjects(), entry)
					assert.True(t, got.token.ReachesProject(other.ID), entry)
				} else {
					assert.Equal(t, tc.wantReach, got.token.ReachableProjects(), entry)
					assert.False(t, got.token.ReachesProject(other.ID), entry)
				}
			}
		})
	}
}

// A row that disagrees with the claims its token was signed with refuses the token, at both entry
// points, as a security event. Whatever wrote the row, it cannot widen the token, move it to
// another resource or organization, or make it an instance token.
func TestAPITokenMiddlewareRefusesARowThatDisagreesWithItsClaims(t *testing.T) {
	if !testhelpers.IntegrationTestsEnabled() {
		t.Skip()
	}

	tu := testhelpers.NewTestingUseCases(t)
	defer tu.DB.Close(t)
	ctx := context.Background()

	org, err := tu.Organization.CreateWithRandomName(ctx)
	require.NoError(t, err)
	otherOrg, err := tu.Organization.CreateWithRandomName(ctx)
	require.NoError(t, err)
	orgID, otherOrgID := uuid.MustParse(org.ID), uuid.MustParse(otherOrg.ID)
	project, err := tu.Project.Create(ctx, org.ID, "signed")
	require.NoError(t, err)
	other, err := tu.Project.Create(ctx, org.ID, "other")
	require.NoError(t, err)
	product, otherProduct := uuid.New(), uuid.New()

	orgToken := []biz.APITokenCreateOpt{}
	projectToken := []biz.APITokenCreateOpt{biz.APITokenWithProject(project)}
	productToken := []biz.APITokenCreateOpt{biz.APITokenWithScope(authz.ResourceTypeProduct, &product), biz.APITokenWithProjectIDs([]uuid.UUID{project.ID})}

	testCases := []struct {
		name string
		opts []biz.APITokenCreateOpt
		// alter rewrites the row after the token was minted, nil leaves it as minted
		alter   func(*ent.APITokenUpdateOne) *ent.APITokenUpdateOne
		header  string
		wantErr string
	}{
		{name: "an organization token as minted", opts: orgToken},
		{name: "a project token as minted", opts: projectToken},
		{name: "a product token as minted", opts: productToken},
		{name: "a project token widened to its organization", opts: projectToken, wantErr: errScopeMismatch,
			alter: func(u *ent.APITokenUpdateOne) *ent.APITokenUpdateOne {
				return u.SetScope(authz.ResourceTypeOrganization).SetScopeID(orgID)
			}},
		{name: "an organization token made an instance token", opts: orgToken, header: otherOrg.Name, wantErr: errScopeMismatch,
			alter: func(u *ent.APITokenUpdateOne) *ent.APITokenUpdateOne {
				return u.SetScope(authz.ResourceTypeInstance).ClearScopeID()
			}},
		{name: "a project token moved to another project", opts: projectToken, wantErr: errScopeMismatch,
			alter: func(u *ent.APITokenUpdateOne) *ent.APITokenUpdateOne { return u.SetScopeID(other.ID) }},
		{name: "an organization token moved to another organization", opts: orgToken, wantErr: errScopeMismatch,
			alter: func(u *ent.APITokenUpdateOne) *ent.APITokenUpdateOne {
				return u.SetOrganizationID(otherOrgID).SetScopeID(otherOrgID)
			}},
		{name: "a product token moved to another product", opts: productToken, wantErr: errScopeMismatch,
			alter: func(u *ent.APITokenUpdateOne) *ent.APITokenUpdateOne { return u.SetScopeID(otherProduct) }},
		{name: "a product token moved to another organization", opts: productToken, wantErr: "organization mismatch",
			alter: func(u *ent.APITokenUpdateOne) *ent.APITokenUpdateOne { return u.SetOrganizationID(otherOrgID) }},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			token, err := tu.APIToken.Create(ctx, "signed-"+uuid.NewString()[:8], nil, nil, &org.ID, tc.opts...)
			require.NoError(t, err)
			if tc.alter != nil {
				require.NoError(t, tc.alter(tu.Data.DB.APIToken.UpdateOneID(token.ID)).Exec(ctx))
			}

			for entry, got := range authenticateAtBothEntryPoints(t, tu, token.JWT, tc.header) {
				if tc.wantErr != "" {
					assert.ErrorContains(t, got.err, tc.wantErr, entry)
					assert.ErrorIs(t, got.err, biz.ErrAPITokenClaimsMismatch, entry)
					assert.Nil(t, got.token, entry)
					continue
				}

				require.NoError(t, got.err, entry)
				require.NotNil(t, got.token, entry)
				assert.Equal(t, token.Scope, got.token.Scope, entry)
				assert.Equal(t, token.ScopeID, got.token.ScopeID, entry)
			}
		})
	}
}
