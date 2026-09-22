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

package service

import (
	"context"
	"io"
	"testing"

	pb "github.com/chainloop-dev/chainloop/app/controlplane/api/controlplane/v1"
	"github.com/chainloop-dev/chainloop/app/controlplane/internal/usercontext"
	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/authz"
	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/biz"
	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/biz/mocks"
	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/usercontext/entities"

	kerrors "github.com/go-kratos/kratos/v2/errors"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// Fixtures shared by the scoped-token tests in this package.
const (
	testProjectName  = "billing"
	testProductName  = "checkout-platform"
	testProductScope = string(authz.ResourceTypeProduct)
)

func toPtr[T any](v T) *T {
	return &v
}

// scopedTokenContext builds the context the middlewares produce for a token confined to a
// resource outside this database, carrying the memberships that decide what it reaches.
func scopedTokenContext(token *entities.APIToken, memberships ...*entities.ResourceMembership) context.Context {
	ctx := entities.WithCurrentAPIToken(context.Background(), token)
	ctx = usercontext.WithAuthzSubject(ctx, (&authz.SubjectAPIToken{ID: token.ID}).String())

	return entities.WithMembership(ctx, &entities.Membership{
		MemberID:   uuid.MustParse(token.ID),
		MemberType: authz.MembershipTypeAPIToken,
		Resources:  memberships,
	})
}

// productToken is a token row as the middleware would put it on the context.
func productToken(productID uuid.UUID) *entities.APIToken {
	return &entities.APIToken{
		ID:        uuid.NewString(),
		Name:      "ci",
		Scope:     toPtr(authz.ResourceTypeProduct),
		ScopeID:   &productID,
		ScopeName: toPtr(testProductName),
	}
}

// TestRBACEnabledForTokens pins the single most dangerous decision in this feature: RBAC keys
// on the token's scope column, never on whether memberships happen to exist.
func TestRBACEnabledForTokens(t *testing.T) {
	productID, projectID := uuid.New(), uuid.New()

	testCases := []struct {
		name string
		ctx  context.Context
		want bool
	}{
		{
			name: "a product-scoped token is under RBAC",
			ctx:  scopedTokenContext(productToken(productID), &entities.ResourceMembership{ResourceType: authz.ResourceTypeProject, ResourceID: projectID, Role: authz.RoleProjectAdmin}),
			want: true,
		},
		{
			// Deleting a product removes its memberships. Deriving RBAC from their presence
			// would silently promote the token to organization-wide.
			name: "a product-scoped token whose product was deleted stays under RBAC",
			ctx:  scopedTokenContext(productToken(productID)),
			want: true,
		},
		{
			name: "a project-scoped token is under RBAC, as before",
			ctx:  entities.WithCurrentAPIToken(context.Background(), &entities.APIToken{ID: uuid.NewString(), ProjectID: &projectID}),
			want: true,
		},
		{
			name: "an organization-level token is not, as before",
			ctx:  entities.WithCurrentAPIToken(context.Background(), &entities.APIToken{ID: uuid.NewString()}),
			want: false,
		},
		{
			// The instance-admin claim now lives in its own field. It must not be mistaken
			// for a resource scope and pull the token under RBAC.
			name: "an instance-admin token is not, as before",
			ctx: entities.WithCurrentAPIToken(context.Background(), &entities.APIToken{
				ID: uuid.NewString(), InstanceScope: authz.ScopeInstanceAdmin,
			}),
			want: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, rbacEnabled(tc.ctx))
		})
	}
}

// TestVisibleProjectsForScopedToken covers the project set a scoped token reaches. nil means
// "RBAC not applied, everything visible", so a scoped token must never return it.
func TestVisibleProjectsForScopedToken(t *testing.T) {
	productID, projectA, projectB := uuid.New(), uuid.New(), uuid.New()

	testCases := []struct {
		name        string
		memberships []*entities.ResourceMembership
		want        []uuid.UUID
	}{
		{
			name: "only the project dimension of its memberships",
			memberships: []*entities.ResourceMembership{
				{ResourceType: authz.ResourceTypeProject, ResourceID: projectA, Role: authz.RoleProjectAdmin},
				{ResourceType: authz.ResourceTypeProduct, ResourceID: productID, Role: authz.RoleProductAdmin},
				{ResourceType: authz.ResourceTypeProject, ResourceID: projectB, Role: authz.RoleProjectAdmin},
			},
			want: []uuid.UUID{projectA, projectB},
		},
		{
			name: "an empty scope reaches nothing",
			want: []uuid.UUID{},
		},
	}

	s := newTestService(t)
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			got := s.visibleProjects(scopedTokenContext(productToken(productID), tc.memberships...))

			require.NotNil(t, got, "must be an empty slice, never nil: nil means RBAC was not applied")
			assert.ElementsMatch(t, tc.want, got)
		})
	}
}

// A scoped token must not be offered project creation: it acts inside a resource it does not
// own, and the create call would refuse it anyway.
func TestCanCreateProjectForTokens(t *testing.T) {
	productID, projectID := uuid.New(), uuid.New()

	testCases := []struct {
		name string
		ctx  context.Context
		want bool
	}{
		{
			name: "a product-scoped token may not",
			ctx:  scopedTokenContext(productToken(productID)),
			want: false,
		},
		{
			name: "a project-scoped token may not, as before",
			ctx:  entities.WithCurrentAPIToken(context.Background(), &entities.APIToken{ID: uuid.NewString(), ProjectID: &projectID}),
			want: false,
		},
		{
			name: "an organization-level token still may",
			ctx:  entities.WithCurrentAPIToken(context.Background(), &entities.APIToken{ID: uuid.NewString()}),
			want: true,
		},
	}

	s := newTestService(t)
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := s.canCreateProject(tc.ctx)
			require.NoError(t, err)
			assert.Equal(t, tc.want, got)
		})
	}
}

// projectsAllowing answers for a whole listing what authorizeResource answers per call, so the
// two must agree: a scoped token reported as unable to act on a project it can act on turns
// into an action the UI never offers.
func TestProjectsAllowingForScopedToken(t *testing.T) {
	productID, reachable, unreachable := uuid.New(), uuid.New(), uuid.New()
	token := productToken(productID)

	projects := []*biz.Project{{ID: reachable, Name: "reachable"}, {ID: unreachable, Name: "unreachable"}}

	testCases := []struct {
		name string
		// role the token holds on the reachable project
		role        authz.Role
		wantAllowed map[uuid.UUID]bool
	}{
		{
			name:        "a project admin membership allows creating a workflow there and nowhere else",
			role:        authz.RoleProjectAdmin,
			wantAllowed: map[uuid.UUID]bool{reachable: true, unreachable: false},
		},
		{
			name:        "a viewer membership allows nothing",
			role:        authz.RoleProjectViewer,
			wantAllowed: map[uuid.UUID]bool{reachable: false, unreachable: false},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// A scoped token carries the same policies a project token carries, so the
			// API-level check that projectsAllowing runs first passes for both cases.
			s := newTestServiceWithTokenPolicies(t, []*authz.Policy{authz.PolicyWorkflowCreate})

			got, err := s.projectsAllowing(
				scopedTokenContext(token, &entities.ResourceMembership{
					ResourceType: authz.ResourceTypeProject, ResourceID: reachable, Role: tc.role,
				}),
				authz.PolicyWorkflowCreate, projects)
			require.NoError(t, err)

			for id, want := range tc.wantAllowed {
				assert.Equal(t, want, got[id], "project %s", id)
			}
		})
	}
}

// A product token whose product holds nothing reaches nothing, rather than everything.
func TestProjectsAllowingForScopedTokenWithNoMemberships(t *testing.T) {
	productID, projectID := uuid.New(), uuid.New()
	token := productToken(productID)

	s := newTestServiceWithTokenPolicies(t, []*authz.Policy{authz.PolicyWorkflowCreate})
	got, err := s.projectsAllowing(scopedTokenContext(token), authz.PolicyWorkflowCreate,
		[]*biz.Project{{ID: projectID}})
	require.NoError(t, err)
	assert.False(t, got[projectID])
}

// canCreateContractsInRestrictedMode treats an API token without a project as an org-wide
// service account. A scoped token is not one: it must not be able to create an
// organization-level contract while the organization restricts that to administrators.
func TestCanCreateContractsInRestrictedModeForTokens(t *testing.T) {
	productID, projectID := uuid.New(), uuid.New()

	testCases := []struct {
		name string
		ctx  context.Context
		want bool
	}{
		{
			name: "a product-scoped token may not",
			ctx:  scopedTokenContext(productToken(productID)),
			want: false,
		},
		{
			name: "a project-scoped token may not, as before",
			ctx:  entities.WithCurrentAPIToken(context.Background(), &entities.APIToken{ID: uuid.NewString(), ProjectID: &projectID}),
			want: false,
		},
		{
			name: "an organization-level token still may",
			ctx:  entities.WithCurrentAPIToken(context.Background(), &entities.APIToken{ID: uuid.NewString()}),
			want: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, canCreateContractsInRestrictedMode(tc.ctx))
		})
	}
}

// newTestServiceWithTokenPolicies builds a service whose enforcer can resolve an API-token
// subject to the given ACL, which is what projectsAllowing and authorizeResource consult.
func newTestServiceWithTokenPolicies(t *testing.T, policies []*authz.Policy) *service {
	t.Helper()

	logger := log.NewStdLogger(io.Discard)
	enforcer, err := authz.NewCasbinEnforcer(&authz.Config{RolesMap: authz.RolesMap})
	require.NoError(t, err)

	// Any token id resolves to the given policy set: the callers that care pass their own
	// token's id, and the ones that do not still need the lookup to succeed.
	repo := mocks.NewAPITokenRepo(t)
	repo.On("FindByID", mock.Anything, mock.Anything).Maybe().
		Return(func(_ context.Context, id uuid.UUID) (*biz.APIToken, error) {
			return &biz.APIToken{ID: id, Policies: policies}, nil
		})

	return &service{
		log: log.NewHelper(logger),
		authz: biz.NewAuthzUseCase(&biz.AuthzUseCaseConfig{
			CasbinEnforcer: enforcer,
			APITokenRepo:   repo,
			Logger:         logger,
		}),
	}
}

// authorizeResource's refusal path must be self-diagnosing and must never panic.
// token.ProjectName is nil for every token that is not confined to a project — an
// organization token under withForceRBAC, and every refused scoped token.
func TestAuthorizeResourceRefusalMessages(t *testing.T) {
	productID, projectID, otherProject := uuid.New(), uuid.New(), uuid.New()
	projectName := testProjectName

	testCases := []struct {
		name string
		ctx  context.Context
		// substrings the refusal must carry, so the caller can act on it
		wantContains []string
	}{
		{
			name: "a project-scoped token names its project",
			ctx: entities.WithCurrentAPIToken(context.Background(), &entities.APIToken{
				ID: uuid.NewString(), ProjectID: &otherProject, ProjectName: &projectName,
			}),
			wantContains: []string{testProjectName},
		},
		{
			// forceRBAC brings an organization token here, and it has no project name.
			name:         "an organization-level token says so instead of panicking",
			ctx:          entities.WithCurrentAPIToken(context.Background(), &entities.APIToken{ID: uuid.NewString()}),
			wantContains: []string{"this organization"},
		},
		{
			name:         "a scoped token names the product and the resource kind",
			ctx:          scopedTokenContext(productToken(productID)),
			wantContains: []string{testProductScope, testProductName, string(authz.ResourceTypeProject)},
		},
		{
			// The display name may be absent; the id still identifies the scope.
			name: "a scoped token with no display name falls back to the id",
			ctx: scopedTokenContext(&entities.APIToken{
				ID: uuid.NewString(), Scope: toPtr(authz.ResourceTypeProduct), ScopeID: &productID,
			}),
			wantContains: []string{productID.String()},
		},
		{
			// A scoped token whose memberships were never loaded must be refused, not
			// admitted, and must not panic on a nil membership value.
			name: "a scoped token with no memberships loaded is refused",
			ctx: entities.WithCurrentAPIToken(context.Background(), &entities.APIToken{
				ID: uuid.NewString(), Scope: toPtr(authz.ResourceTypeProduct), ScopeID: &productID,
			}),
			wantContains: []string{"do not have permissions"},
		},
	}

	s := newTestService(t)
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := s.authorizeResource(tc.ctx, authz.PolicyWorkflowCreate, authz.ResourceTypeProject, projectID, withForceRBAC())

			require.Error(t, err)
			assert.True(t, kerrors.IsForbidden(err), "expected forbidden, got %v", err)
			for _, want := range tc.wantContains {
				assert.Contains(t, err.Error(), want)
			}
		})
	}
}

// A scoped token holding the resource but not a role that grants the operation is refused for
// the permission, not for the scope: the two refusals mean different things to the caller.
func TestAuthorizeResourceScopedTokenWithInsufficientRole(t *testing.T) {
	productID, projectID := uuid.New(), uuid.New()

	s := newTestService(t)
	err := s.authorizeResource(
		scopedTokenContext(productToken(productID), &entities.ResourceMembership{
			ResourceType: authz.ResourceTypeProject, ResourceID: projectID, Role: authz.RoleProjectViewer,
		}),
		authz.PolicyWorkflowCreate, authz.ResourceTypeProject, projectID)

	require.Error(t, err)
	assert.True(t, kerrors.IsForbidden(err), "expected forbidden, got %v", err)
	assert.Contains(t, err.Error(), string(authz.RoleProjectViewer))
	assert.NotContains(t, err.Error(), testProductName, "this is a missing permission, not an out-of-scope resource")
}

// The token's own ACL is the ceiling on every path. The attestation endpoints are skipped by
// the authz middleware (grpc.go registers WithCurrentMembershipsMiddleware but no
// WithAuthzMiddleware on that chain), so authorizeResource is the only gate there, and its
// membership branch enforces the membership ROLE. RoleProjectAdmin carries
// PolicyAPITokenCreate and PolicyAPITokenRevoke, exactly the organization-level policies a
// scoped token is denied, so a platform-written role must not hand them back.
func TestAuthorizeResourceKeepsOrgLevelPoliciesOutOfReach(t *testing.T) {
	productID, projectID := uuid.New(), uuid.New()

	testCases := []struct {
		name string
		// tokenPolicies is the token's own ACL, i.e. what its row carries
		tokenPolicies []*authz.Policy
		// role is the membership the platform wrote for it on the project
		role       authz.Role
		op         *authz.Policy
		wantAllows bool
	}{
		{
			name:          "the role grants it and the token carries it",
			tokenPolicies: []*authz.Policy{authz.PolicyWorkflowCreate},
			role:          authz.RoleProjectAdmin,
			op:            authz.PolicyWorkflowCreate,
			wantAllows:    true,
		},
		{
			// The ACL and RolesMap are separate vocabularies. A role legitimately grants
			// things no token ACL lists, so the role alone decides outside the org-level set.
			name:          "the role grants it and the token's ACL does not list it",
			tokenPolicies: []*authz.Policy{authz.PolicyWorkflowRunRead},
			role:          authz.RoleProjectAdmin,
			op:            authz.PolicyWorkflowCreate,
			wantAllows:    true,
		},
		{
			// The concrete case: attestation is unmapped in ServerOperationsMap, so no token
			// ACL carries workflow_run:create. Denying it here refuses every attestation to
			// the very credential this scope exists to issue.
			name:          "a project admin membership can attest",
			tokenPolicies: defaultPoliciesForTest(),
			role:          authz.RoleProjectAdmin,
			op:            authz.PolicyWorkflowRunCreate,
			wantAllows:    true,
		},
		{
			name:          "a project admin membership can update a run it is attesting",
			tokenPolicies: defaultPoliciesForTest(),
			role:          authz.RoleProjectAdmin,
			op:            authz.PolicyWorkflowRunUpdate,
			wantAllows:    true,
		},
		{
			// Every organization-level policy stays out of reach, not just the token-minting
			// pair: a scoped token is created with none of them.
			name:          "project admin cannot hand back the org-level integration policies",
			tokenPolicies: defaultPoliciesForTest(),
			role:          authz.RoleProjectAdmin,
			op:            authz.PolicyRegisteredIntegrationRead,
			wantAllows:    false,
		},
		{
			// The permission ceiling this feature is built on.
			name:          "project admin cannot hand back the org-level token policies",
			tokenPolicies: defaultPoliciesForTest(),
			role:          authz.RoleProjectAdmin,
			op:            authz.PolicyAPITokenCreate,
			wantAllows:    false,
		},
		{
			name:          "the token carries it but the role does not grant it",
			tokenPolicies: []*authz.Policy{authz.PolicyWorkflowCreate},
			role:          authz.RoleProjectViewer,
			op:            authz.PolicyWorkflowCreate,
			wantAllows:    false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			token := productToken(productID)
			s := newTestServiceWithTokenPolicies(t, tc.tokenPolicies)

			err := s.authorizeResource(
				scopedTokenContext(token, &entities.ResourceMembership{
					ResourceType: authz.ResourceTypeProject, ResourceID: projectID, Role: tc.role,
				}),
				tc.op, authz.ResourceTypeProject, projectID)

			if tc.wantAllows {
				assert.NoError(t, err)
				return
			}

			require.Error(t, err)
			assert.True(t, kerrors.IsForbidden(err), "expected forbidden, got %v", err)
		})
	}
}

// defaultPoliciesForTest mirrors what a scoped token actually carries after Task 3: the
// project token's set, never the organization-level one.
func defaultPoliciesForTest() []*authz.Policy {
	return []*authz.Policy{
		authz.PolicyWorkflowRunList, authz.PolicyWorkflowRunRead,
		authz.PolicyWorkflowRead, authz.PolicyWorkflowList, authz.PolicyWorkflowCreate,
		authz.PolicyWorkflowContractList, authz.PolicyWorkflowContractRead,
		authz.PolicyWorkflowContractUpdate, authz.PolicyWorkflowContractCreate,
		authz.PolicyArtifactDownload, authz.PolicyReferrerRead, authz.PolicyOrganizationRead,
		authz.PolicyAvailableIntegrationRead, authz.PolicyAvailableIntegrationList,
		authz.PolicyAttachedIntegrationList, authz.PolicyAttachedIntegrationAttach,
		authz.PolicyArtifactUpload,
	}
}

// The refusal and the listing both fall back to the scope id when there is no display name.
// The option stored a pointer to the empty string, so the fallback never fired and the message
// named neither the product nor its id — the exact self-diagnosis it exists to provide.
func TestOutOfScopeMessageFallsBackToTheIDWhenUnnamed(t *testing.T) {
	productID, projectID := uuid.New(), uuid.New()

	testCases := []struct {
		name      string
		scopeName *string
		want      string
	}{
		{name: "a named scope prints the name", scopeName: toPtr(testProductName), want: testProductName},
		{name: "no name falls back to the id", scopeName: nil, want: productID.String()},
		{name: "an empty name falls back to the id", scopeName: toPtr(""), want: productID.String()},
	}

	s := newTestServiceWithTokenPolicies(t, defaultPoliciesForTest())
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctx := scopedTokenContext(&entities.APIToken{
				ID: uuid.NewString(), Scope: toPtr(authz.ResourceTypeProduct),
				ScopeID: &productID, ScopeName: tc.scopeName,
			})

			err := s.authorizeResource(ctx, authz.PolicyWorkflowCreate, authz.ResourceTypeProject, projectID)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tc.want)
			assert.NotContains(t, err.Error(), `""`, "the refusal must never name an empty scope")
		})
	}
}

// An organization-wide token may only revoke tokens confined to a project. The guard has to
// make that decision itself: authorizeResource returns on its first line for any caller whose
// RBAC is disabled, which every organization-wide token is, so the resource check below it
// cannot refuse a target the guard lets through.
func TestRevokeConfinesWhatAnOrgWideTokenCanDestroy(t *testing.T) {
	orgID, projectID, productID := uuid.New(), uuid.New(), uuid.New()
	productScope := authz.ResourceTypeProduct

	testCases := []struct {
		name        string
		target      *biz.APIToken
		wantAllowed bool
	}{
		{
			name:        "an organization-wide target is refused",
			target:      &biz.APIToken{ID: uuid.New(), Name: "t", OrganizationID: orgID},
			wantAllowed: false,
		},
		{
			name:        "a project-scoped target is allowed",
			target:      &biz.APIToken{ID: uuid.New(), Name: "t", OrganizationID: orgID, ProjectID: &projectID},
			wantAllowed: true,
		},
		{
			// The regression: keying the guard on IsOrgWide let this target through, and a
			// CI credential could revoke every platform-issued product token in the
			// organization -- tokens it cannot even see, since List forces the project scope.
			name: "a resource-scoped target is refused",
			target: &biz.APIToken{
				ID: uuid.New(), Name: "t", OrganizationID: orgID,
				Scope: &productScope, ScopeID: &productID,
			},
			wantAllowed: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			logger := log.NewStdLogger(io.Discard)
			enforcer, err := authz.NewCasbinEnforcer(&authz.Config{RolesMap: authz.RolesMap})
			require.NoError(t, err)

			caller := &entities.APIToken{ID: uuid.NewString(), Name: "ci"}

			repo := mocks.NewAPITokenRepo(t)
			repo.On("FindByIDInOrg", mock.Anything, mock.Anything, mock.Anything).Maybe().Return(tc.target, nil)
			repo.On("FindByID", mock.Anything, mock.Anything).Maybe().
				Return(func(_ context.Context, id uuid.UUID) (*biz.APIToken, error) {
					return &biz.APIToken{ID: id, Policies: defaultPoliciesForTest()}, nil
				})
			repo.On("Revoke", mock.Anything, mock.Anything, mock.Anything).Maybe().Return(nil)

			authzUC := biz.NewAuthzUseCase(&biz.AuthzUseCaseConfig{
				CasbinEnforcer: enforcer, APITokenRepo: repo, Logger: logger,
			})
			// A nil publisher makes the auditor a no-op, so Revoke can run to completion and the
			// test observes the authorization decision rather than a missing dependency.
			uc, err := biz.NewAPITokenUseCase(repo, &biz.APITokenJWTConfig{SymmetricHmacKey: "test"}, authzUC, nil,
				biz.NewAuditorUseCase(nil, logger), logger)
			require.NoError(t, err)

			ctx := entities.WithCurrentAPIToken(context.Background(), caller)
			ctx = usercontext.WithAuthzSubject(ctx, (&authz.SubjectAPIToken{ID: caller.ID}).String())
			ctx = entities.WithCurrentOrg(ctx, &entities.Org{ID: orgID.String(), Name: "acme"})

			_, err = NewAPITokenService(uc, WithLogger(logger), WithEnforcer(authzUC)).
				Revoke(ctx, &pb.APITokenServiceRevokeRequest{Id: tc.target.ID.String()})

			if tc.wantAllowed {
				assert.NoError(t, err)
				return
			}

			require.Error(t, err)
			assert.True(t, kerrors.IsForbidden(err), "expected forbidden, got %v", err)
		})
	}
}
