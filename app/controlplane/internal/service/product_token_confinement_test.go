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
	"fmt"
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

// Fixtures shared by the token confinement tests in this package.
const (
	testProjectName = "billing"
	testOrgName     = "acme"
	testJWTKey      = "test"
	testUserEmail   = "user@test.com"
)

func toPtr[T any](v T) *T {
	return &v
}

// productTokenContext is the context the middlewares produce for a product token reaching the
// given projects.
func productTokenContext(projects ...uuid.UUID) context.Context {
	productID := uuid.New()
	token := &entities.APIToken{
		ID:         uuid.NewString(),
		Name:       "ci",
		Scope:      toPtr(authz.ResourceTypeProduct),
		ScopeID:    &productID,
		ProjectIDs: append([]uuid.UUID{}, projects...),
	}
	ctx := entities.WithCurrentAPIToken(context.Background(), token)

	return usercontext.WithAuthzSubject(ctx, (&authz.SubjectAPIToken{ID: token.ID}).String())
}

// RBAC keys on the token's kind: a product token is always under it, whatever its list holds.
func TestRBACEnabledForTokens(t *testing.T) {
	projectID, orgID := uuid.New(), uuid.New()
	orgScope, instanceScope, projectScope := authz.ResourceTypeOrganization, authz.ResourceTypeInstance, authz.ResourceTypeProject

	testCases := []struct {
		name string
		ctx  context.Context
		want bool
	}{
		{name: "a product token", ctx: productTokenContext(uuid.New()), want: true},
		{name: "a product token reaching nothing", ctx: productTokenContext(), want: true},
		{name: "a project token", ctx: entities.WithCurrentAPIToken(context.Background(), &entities.APIToken{ID: uuid.NewString(), ProjectID: &projectID, Scope: &projectScope, ScopeID: &projectID}), want: true},
		{name: "a token recording no scope is under RBAC", ctx: entities.WithCurrentAPIToken(context.Background(), &entities.APIToken{ID: uuid.NewString()}), want: true},
		{name: "an organization token recording its scope", ctx: entities.WithCurrentAPIToken(context.Background(), &entities.APIToken{ID: uuid.NewString(), Scope: &orgScope, ScopeID: &orgID}), want: false},
		{name: "an instance token", ctx: entities.WithCurrentAPIToken(context.Background(), &entities.APIToken{ID: uuid.NewString(), Scope: &instanceScope}), want: false},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, rbacEnabled(tc.ctx))
		})
	}
}

// nil means "RBAC not applied, everything visible", so a confined token never returns it.
func TestVisibleProjectsForTokens(t *testing.T) {
	a, b, projectID, orgID := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	projectScope, orgScope := authz.ResourceTypeProject, authz.ResourceTypeOrganization

	testCases := []struct {
		name string
		ctx  context.Context
		want []uuid.UUID
	}{
		{name: "a product token sees its list", ctx: productTokenContext(a, b), want: []uuid.UUID{a, b}},
		{name: "an empty product sees nothing", ctx: productTokenContext(), want: []uuid.UUID{}},
		{name: "a project token sees its project", ctx: entities.WithCurrentAPIToken(context.Background(), &entities.APIToken{ID: uuid.NewString(), ProjectID: &projectID, Scope: &projectScope, ScopeID: &projectID}), want: []uuid.UUID{projectID}},
		{name: "an organization token is not filtered", ctx: entities.WithCurrentAPIToken(context.Background(), &entities.APIToken{ID: uuid.NewString(), Scope: &orgScope, ScopeID: &orgID}), want: nil},
		{name: "a token recording no scope sees nothing", ctx: entities.WithCurrentAPIToken(context.Background(), &entities.APIToken{ID: uuid.NewString()}), want: []uuid.UUID{}},
	}

	s := newTestService(t)
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			got := s.visibleProjects(tc.ctx)
			if tc.want == nil {
				assert.Nil(t, got)
				return
			}
			require.NotNil(t, got)
			assert.ElementsMatch(t, tc.want, got)
		})
	}
}

// A product token passes the resource check only for a project in its list.
func TestAuthorizeResourceForProductTokens(t *testing.T) {
	listed, unlisted := uuid.New(), uuid.New()

	testCases := []struct {
		name         string
		ctx          context.Context
		resourceType authz.ResourceType
		resourceID   uuid.UUID
		wantErr      bool
	}{
		{name: "a listed project", ctx: productTokenContext(listed), resourceType: authz.ResourceTypeProject, resourceID: listed},
		{name: "an unlisted project", ctx: productTokenContext(listed), resourceType: authz.ResourceTypeProject, resourceID: unlisted, wantErr: true},
		{name: "any project, with an empty list", ctx: productTokenContext(), resourceType: authz.ResourceTypeProject, resourceID: listed, wantErr: true},
		{name: "a resource of another kind", ctx: productTokenContext(listed), resourceType: authz.ResourceTypeProduct, resourceID: listed, wantErr: true},
	}

	s := newTestService(t)
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := s.authorizeResource(tc.ctx, authz.PolicyWorkflowRead, tc.resourceType, tc.resourceID)
			if tc.wantErr {
				require.Error(t, err)
				assert.True(t, kerrors.IsForbidden(err), "got %v", err)
				return
			}
			assert.NoError(t, err)
		})
	}
}

// authorizeResource's refusal names what the token is confined to, and never dereferences a
// field the token does not carry: a product token has no ProjectName and names its product by id.
func TestAuthorizeResourceRefusalMessagesForTokens(t *testing.T) {
	projectID, otherProject := uuid.New(), uuid.New()
	projectName := testProjectName
	projectScope, productScope := authz.ResourceTypeProject, authz.ResourceTypeProduct

	product := productTokenContext(otherProject)
	productID := entities.CurrentAPIToken(product).ScopeID

	const (
		wantProjectMessage = `operation not allowed: This auth token is valid only with the project "billing"`
		wantNotConfined    = "operation not allowed: this auth token is not confined to this resource"
	)

	withToken := func(token *entities.APIToken) context.Context {
		token.ID = uuid.NewString()
		return entities.WithCurrentAPIToken(context.Background(), token)
	}

	testCases := []struct {
		name        string
		ctx         context.Context
		wantMessage string
	}{
		{
			name: "a project token recording its scope names its project",
			ctx: withToken(&entities.APIToken{
				ProjectID: &otherProject, ProjectName: &projectName, Scope: &projectScope, ScopeID: &otherProject,
			}),
			wantMessage: wantProjectMessage,
		},
		{
			name:        "a product token names its product by id",
			ctx:         product,
			wantMessage: fmt.Sprintf("operation not allowed: this auth token is valid only with the projects of the product %q", productID.String()),
		},
		{
			name:        "a project id without a scope names nothing",
			ctx:         withToken(&entities.APIToken{ProjectID: &otherProject, ProjectName: &projectName}),
			wantMessage: wantNotConfined,
		},
		{
			name:        "a product scope missing its id names nothing",
			ctx:         withToken(&entities.APIToken{Scope: &productScope, ProjectIDs: []uuid.UUID{otherProject}}),
			wantMessage: wantNotConfined,
		},
	}

	s := newTestService(t)
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := s.authorizeResource(tc.ctx, authz.PolicyWorkflowCreate, authz.ResourceTypeProject, projectID, withForceRBAC())

			require.Error(t, err)
			assert.True(t, kerrors.IsForbidden(err), "got %v", err)
			assert.Equal(t, tc.wantMessage, kerrors.FromError(err).Message)
		})
	}
}

// projectsAllowing agrees with authorizeResource: the listed projects, when the token's own
// policies carry the operation, and nothing when they do not.
func TestProjectsAllowingForProductTokens(t *testing.T) {
	listed, unlisted := uuid.New(), uuid.New()
	projects := []*biz.Project{{ID: listed}, {ID: unlisted}}

	testCases := []struct {
		name     string
		policies []*authz.Policy
		want     map[uuid.UUID]bool
	}{
		{name: "the token carries the policy", policies: []*authz.Policy{authz.PolicyWorkflowCreate}, want: map[uuid.UUID]bool{listed: true, unlisted: false}},
		{name: "the token lacks it", policies: []*authz.Policy{authz.PolicyWorkflowRead}, want: map[uuid.UUID]bool{listed: false, unlisted: false}},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			s := newTestServiceWithTokenPolicies(t, tc.policies)
			got, err := s.projectsAllowing(productTokenContext(listed), authz.PolicyWorkflowCreate, projects)
			require.NoError(t, err)
			for id, want := range tc.want {
				assert.Equal(t, want, got[id], "project %s", id)
			}
		})
	}
}

// An organization-wide token reaches every project of its organization, and RBAC does not apply
// to it: it passes the resource check, may act on every project and sees them all, forced RBAC
// included. A resource other than a project still refuses it, and names nothing it is confined to.
func TestProjectGatesForOrgWideTokens(t *testing.T) {
	a, b, orgID := uuid.New(), uuid.New(), uuid.New()
	projects := []*biz.Project{{ID: a}, {ID: b}}
	orgScope := authz.ResourceTypeOrganization

	testCases := []struct {
		name  string
		token *entities.APIToken
	}{
		{name: "an organization token recording its scope", token: &entities.APIToken{ID: uuid.NewString(), Scope: &orgScope, ScopeID: &orgID}},
	}

	s := newTestServiceWithTokenPolicies(t, []*authz.Policy{authz.PolicyWorkflowCreate})
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctx := entities.WithCurrentAPIToken(context.Background(), tc.token)
			ctx = usercontext.WithAuthzSubject(ctx, (&authz.SubjectAPIToken{ID: tc.token.ID}).String())

			assert.NoError(t, s.authorizeResource(ctx, authz.PolicyWorkflowCreate, authz.ResourceTypeProject, a))

			assert.NoError(t, s.authorizeResource(ctx, authz.PolicyWorkflowCreate, authz.ResourceTypeProject, a, withForceRBAC()))

			err := s.authorizeResource(ctx, authz.PolicyOrganizationDelete, authz.ResourceTypeOrganization, orgID, withForceRBAC())
			require.Error(t, err)
			assert.True(t, kerrors.IsForbidden(err), "got %v", err)
			assert.Equal(t, "operation not allowed: this auth token is not confined to this resource", kerrors.FromError(err).Message)

			allowed, err := s.projectsAllowing(ctx, authz.PolicyWorkflowCreate, projects)
			require.NoError(t, err)
			assert.Equal(t, map[uuid.UUID]bool{a: true, b: true}, allowed)

			assert.Nil(t, s.visibleProjects(ctx), "nil means the listing is not filtered")
		})
	}
}

// A product token deletes contracts only where it reaches: never an organization contract, and
// only the project contracts of its projects.
func TestCheckContractAccessForProductTokens(t *testing.T) {
	listed, unlisted := uuid.New(), uuid.New()
	s := &WorkflowContractService{service: newTestService(t)}

	testCases := []struct {
		name     string
		contract *biz.WorkflowContract
		checkErr func(t *testing.T, err error)
	}{
		{
			name:     "an organization contract",
			contract: &biz.WorkflowContract{},
			// The gate refuses a global contract before it ever looks at project reach.
			checkErr: func(t *testing.T, err error) { t.Helper(); assert.True(t, kerrors.IsBadRequest(err), "got %v", err) },
		},
		{name: "a contract of a listed project", contract: &biz.WorkflowContract{ScopedEntity: &biz.ScopedEntity{Type: string(biz.ContractScopeProject), ID: listed}}},
		{
			name:     "a contract of an unlisted project",
			contract: &biz.WorkflowContract{ScopedEntity: &biz.ScopedEntity{Type: string(biz.ContractScopeProject), ID: unlisted}},
			// The token is refused the same way a user without a role on the project would be.
			checkErr: func(t *testing.T, err error) { t.Helper(); assert.True(t, kerrors.IsForbidden(err), "got %v", err) },
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := s.checkContractAccess(productTokenContext(listed), tc.contract, authz.PolicyWorkflowContractDelete, false)
			if tc.checkErr != nil {
				require.Error(t, err)
				tc.checkErr(t, err)
				return
			}
			assert.NoError(t, err)
		})
	}
}

// A product token must not be offered project creation: it acts inside a product it does not
// own, and the create call would refuse it anyway.
func TestCanCreateProjectForTokens(t *testing.T) {
	projectID, orgID := uuid.New(), uuid.New()
	projectScope := authz.ResourceTypeProject

	testCases := []struct {
		name string
		ctx  context.Context
		want bool
	}{
		{
			name: "a product token may not",
			ctx:  productTokenContext(uuid.New()),
			want: false,
		},
		{
			name: "a product token reaching nothing may not",
			ctx:  productTokenContext(),
			want: false,
		},
		{
			name: "a project token may not",
			ctx:  entities.WithCurrentAPIToken(context.Background(), &entities.APIToken{ID: uuid.NewString(), ProjectID: &projectID, Scope: &projectScope, ScopeID: &projectID}),
			want: false,
		},
		{
			name: "an organization token may",
			ctx: entities.WithCurrentAPIToken(context.Background(), &entities.APIToken{
				ID: uuid.NewString(), Scope: toPtr(authz.ResourceTypeOrganization), ScopeID: &orgID,
			}),
			want: true,
		},
		{
			name: "a token recording no scope may not",
			ctx:  entities.WithCurrentAPIToken(context.Background(), &entities.APIToken{ID: uuid.NewString()}),
			want: false,
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

// canCreateContractsInRestrictedMode treats an organization-wide token as a service account
// standing in for an administrator. A product token is not one: it must not be able to create an
// organization-level contract while the organization restricts that to administrators.
func TestCanCreateContractsInRestrictedModeForTokens(t *testing.T) {
	projectID, orgID := uuid.New(), uuid.New()

	testCases := []struct {
		name string
		ctx  context.Context
		want bool
	}{
		{
			name: "a product token may not",
			ctx:  productTokenContext(uuid.New()),
			want: false,
		},
		{
			name: "a product token reaching nothing may not",
			ctx:  productTokenContext(),
			want: false,
		},
		{
			name: "a project token may not",
			ctx: entities.WithCurrentAPIToken(context.Background(), &entities.APIToken{
				ID: uuid.NewString(), ProjectID: &projectID, Scope: toPtr(authz.ResourceTypeProject), ScopeID: &projectID,
			}),
			want: false,
		},
		{
			name: "a token recording no scope may not",
			ctx:  entities.WithCurrentAPIToken(context.Background(), &entities.APIToken{ID: uuid.NewString()}),
			want: false,
		},
		{
			name: "an organization token recording its scope still may",
			ctx: entities.WithCurrentAPIToken(context.Background(), &entities.APIToken{
				ID: uuid.NewString(), Scope: toPtr(authz.ResourceTypeOrganization), ScopeID: &orgID,
			}),
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
// subject to the given ACL, which is what projectsAllowing consults.
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

// defaultPoliciesForTest mirrors what a confined token carries: the project token's set, never
// the organization-level one.
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

// An organization-wide token may only revoke tokens confined to a project. The guard has to
// make that decision itself: authorizeResource returns on its first line for any caller whose
// RBAC is disabled, which every organization-wide token is, so the resource check below it
// cannot refuse a target the guard lets through.
func TestRevokeConfinesWhatAnOrgWideTokenCanDestroy(t *testing.T) {
	orgID, projectID, productID := uuid.New(), uuid.New(), uuid.New()
	productScope, orgScope, projectScope := authz.ResourceTypeProduct, authz.ResourceTypeOrganization, authz.ResourceTypeProject

	// Every token records its scope, the older ones through the scope backfill migration.
	callers := []struct {
		name  string
		token *entities.APIToken
	}{
		{name: "organization-scoped caller", token: &entities.APIToken{ID: uuid.NewString(), Name: "ci", Scope: &orgScope, ScopeID: &orgID}},
	}

	testCases := []struct {
		name        string
		target      *biz.APIToken
		wantAllowed bool
	}{
		{
			name:        "a target recording no scope is refused",
			target:      &biz.APIToken{ID: uuid.New(), Name: "t", OrganizationID: orgID},
			wantAllowed: false,
		},
		{
			name:        "an organization-scoped target is refused",
			target:      &biz.APIToken{ID: uuid.New(), Name: "t", OrganizationID: orgID, Scope: &orgScope, ScopeID: &orgID},
			wantAllowed: false,
		},
		{
			name: "a project-scoped target recording its scope is allowed",
			target: &biz.APIToken{
				ID: uuid.New(), Name: "t", OrganizationID: orgID, ProjectID: &projectID,
				Scope: &projectScope, ScopeID: &projectID,
			},
			wantAllowed: true,
		},
		{
			// Keying the guard on IsOrgWide would let this target through, and a CI credential
			// could revoke every platform-issued product token in the organization -- tokens it
			// cannot even see, since List forces the project scope.
			name: "a product-scoped target is refused",
			target: &biz.APIToken{
				ID: uuid.New(), Name: "t", OrganizationID: orgID,
				Scope: &productScope, ScopeID: &productID, ProjectIDs: []uuid.UUID{projectID},
			},
			wantAllowed: false,
		},
	}

	for _, c := range callers {
		for _, tc := range testCases {
			t.Run(c.name+"/"+tc.name, func(t *testing.T) {
				logger := log.NewStdLogger(io.Discard)
				enforcer, err := authz.NewCasbinEnforcer(&authz.Config{RolesMap: authz.RolesMap})
				require.NoError(t, err)

				caller := c.token

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
				uc, err := biz.NewAPITokenUseCase(repo, &biz.APITokenJWTConfig{SymmetricHmacKey: testJWTKey}, authzUC, nil,
					biz.NewAuditorUseCase(nil, logger), logger)
				require.NoError(t, err)

				ctx := entities.WithCurrentAPIToken(context.Background(), caller)
				ctx = usercontext.WithAuthzSubject(ctx, (&authz.SubjectAPIToken{ID: caller.ID}).String())
				ctx = entities.WithCurrentOrg(ctx, &entities.Org{ID: orgID.String(), Name: testOrgName})

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
}

// Revoking a product-scoped token authorizes against the product the token is confined to.
// This repository's RolesMap grants no policies to RoleProductAdmin, so through the control
// plane only an organization admin can revoke such a token; a member is refused whatever roles
// it holds. Without the resource check a member could revoke every product token in its org.
func TestRevokeOfAResourceScopedTokenByAUser(t *testing.T) {
	orgID, productID, projectID := uuid.New(), uuid.New(), uuid.New()
	productScope := authz.ResourceTypeProduct
	target := &biz.APIToken{
		ID: uuid.New(), Name: "t", OrganizationID: orgID,
		Scope: &productScope, ScopeID: &productID, ProjectIDs: []uuid.UUID{projectID},
	}

	testCases := []struct {
		name        string
		role        authz.Role
		memberships []*entities.ResourceMembership
		wantAllowed bool
	}{
		{name: "an organization admin", role: authz.RoleAdmin, wantAllowed: true},
		{
			name:        "a member who administers the token's product",
			role:        authz.RoleOrgMember,
			memberships: []*entities.ResourceMembership{{ResourceType: authz.ResourceTypeProduct, ResourceID: productID, Role: authz.RoleProductAdmin}},
		},
		{
			// The token reaches this project, which gives its administrator no say over it.
			name:        "a member who administers a project the token reaches",
			role:        authz.RoleOrgMember,
			memberships: []*entities.ResourceMembership{{ResourceType: authz.ResourceTypeProject, ResourceID: projectID, Role: authz.RoleProjectAdmin}},
		},
		{name: "a member with no role", role: authz.RoleOrgMember},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			logger := log.NewStdLogger(io.Discard)
			enforcer, err := authz.NewCasbinEnforcer(&authz.Config{RolesMap: authz.RolesMap})
			require.NoError(t, err)

			repo := mocks.NewAPITokenRepo(t)
			repo.On("FindByIDInOrg", mock.Anything, mock.Anything, mock.Anything).Return(target, nil)
			repo.On("FindByID", mock.Anything, mock.Anything).Maybe().Return(target, nil)
			repo.On("Revoke", mock.Anything, mock.Anything, mock.Anything).Maybe().Return(nil)

			authzUC := biz.NewAuthzUseCase(&biz.AuthzUseCaseConfig{CasbinEnforcer: enforcer, APITokenRepo: repo, Logger: logger})
			uc, err := biz.NewAPITokenUseCase(repo, &biz.APITokenJWTConfig{SymmetricHmacKey: testJWTKey}, authzUC, nil,
				biz.NewAuditorUseCase(nil, logger), logger)
			require.NoError(t, err)

			userID := uuid.New()
			ctx := entities.WithCurrentUser(context.Background(), &entities.User{ID: userID.String(), Email: testUserEmail})
			ctx = usercontext.WithAuthzSubject(ctx, string(tc.role))
			ctx = entities.WithCurrentOrg(ctx, &entities.Org{ID: orgID.String(), Name: testOrgName})
			ctx = entities.WithMembership(ctx, &entities.Membership{UserID: userID, Resources: tc.memberships})

			_, err = NewAPITokenService(uc, WithLogger(logger), WithEnforcer(authzUC)).
				Revoke(ctx, &pb.APITokenServiceRevokeRequest{Id: target.ID.String()})

			if tc.wantAllowed {
				assert.NoError(t, err)
				return
			}

			require.Error(t, err)
			assert.True(t, kerrors.IsForbidden(err), "expected forbidden, got %v", err)
		})
	}
}

// Revoke authorizes against what the target is confined to, for every kind of token: an
// organization-wide one only an admin manages, a project one whoever may revoke tokens in that
// project, a product one whoever administers the product, and a product scope missing its id no
// one. Neither may anyone revoke a row recording no scope: rerunning the scope backfill gives it one.
func TestRevokeAuthorizesWhereTheTargetIsConfined(t *testing.T) {
	orgID, projectID, productID := uuid.New(), uuid.New(), uuid.New()
	orgScope, projectScope, productScope := authz.ResourceTypeOrganization, authz.ResourceTypeProject, authz.ResourceTypeProduct
	projectName := testProjectName

	target := func(tok biz.APIToken) *biz.APIToken {
		tok.ID, tok.Name, tok.OrganizationID = uuid.New(), "t", orgID
		return &tok
	}
	noScope := target(biz.APIToken{ProjectID: &projectID, ProjectName: &projectName})
	scopedOrg := target(biz.APIToken{Scope: &orgScope, ScopeID: &orgID})
	scopedProject := target(biz.APIToken{ProjectID: &projectID, ProjectName: &projectName, Scope: &projectScope, ScopeID: &projectID})
	product := target(biz.APIToken{Scope: &productScope, ScopeID: &productID, ProjectIDs: []uuid.UUID{projectID}})
	productWithoutID := target(biz.APIToken{Scope: &productScope, ProjectIDs: []uuid.UUID{projectID}})

	admin := []*entities.ResourceMembership(nil)
	projectAdmin := []*entities.ResourceMembership{{ResourceType: authz.ResourceTypeProject, ResourceID: projectID, Role: authz.RoleProjectAdmin}}

	testCases := []struct {
		name        string
		target      *biz.APIToken
		role        authz.Role
		memberships []*entities.ResourceMembership
		// wantErr classifies the refusal; nil when the revoke goes through
		wantErr func(error) bool
	}{
		{name: "an admin revokes an organization token recording its scope", target: scopedOrg, role: authz.RoleAdmin, memberships: admin},
		{name: "a member cannot manage an organization token recording its scope", target: scopedOrg, role: authz.RoleOrgMember, memberships: projectAdmin, wantErr: kerrors.IsBadRequest},
		{name: "a project admin revokes a project token recording its scope", target: scopedProject, role: authz.RoleOrgMember, memberships: projectAdmin},
		{name: "a member with no role cannot revoke a project token recording its scope", target: scopedProject, role: authz.RoleOrgMember, wantErr: kerrors.IsForbidden},
		{name: "an admin revokes a product token", target: product, role: authz.RoleAdmin, memberships: admin},
		{name: "a project admin cannot revoke a product token reaching the project", target: product, role: authz.RoleOrgMember, memberships: projectAdmin, wantErr: kerrors.IsForbidden},
		{name: "an admin cannot revoke a product scope missing its id", target: productWithoutID, role: authz.RoleAdmin, memberships: admin, wantErr: kerrors.IsBadRequest},
		{name: "a member cannot revoke a product scope missing its id", target: productWithoutID, role: authz.RoleOrgMember, memberships: projectAdmin, wantErr: kerrors.IsBadRequest},
		{name: "an admin cannot revoke a row recording no scope", target: noScope, role: authz.RoleAdmin, memberships: admin, wantErr: kerrors.IsBadRequest},
		{name: "a project admin cannot revoke a row recording no scope", target: noScope, role: authz.RoleOrgMember, memberships: projectAdmin, wantErr: kerrors.IsBadRequest},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			logger := log.NewStdLogger(io.Discard)
			enforcer, err := authz.NewCasbinEnforcer(&authz.Config{RolesMap: authz.RolesMap})
			require.NoError(t, err)

			repo := mocks.NewAPITokenRepo(t)
			repo.On("FindByIDInOrg", mock.Anything, mock.Anything, mock.Anything).Return(tc.target, nil)
			repo.On("FindByID", mock.Anything, mock.Anything).Maybe().Return(tc.target, nil)
			repo.On("Revoke", mock.Anything, mock.Anything, mock.Anything).Maybe().Return(nil)

			authzUC := biz.NewAuthzUseCase(&biz.AuthzUseCaseConfig{CasbinEnforcer: enforcer, APITokenRepo: repo, Logger: logger})
			uc, err := biz.NewAPITokenUseCase(repo, &biz.APITokenJWTConfig{SymmetricHmacKey: testJWTKey}, authzUC, nil,
				biz.NewAuditorUseCase(nil, logger), logger)
			require.NoError(t, err)

			userID := uuid.New()
			ctx := entities.WithCurrentUser(context.Background(), &entities.User{ID: userID.String(), Email: testUserEmail})
			ctx = usercontext.WithAuthzSubject(ctx, string(tc.role))
			ctx = entities.WithCurrentOrg(ctx, &entities.Org{ID: orgID.String(), Name: testOrgName})
			ctx = entities.WithMembership(ctx, &entities.Membership{UserID: userID, Resources: tc.memberships})

			_, err = NewAPITokenService(uc, WithLogger(logger), WithEnforcer(authzUC)).
				Revoke(ctx, &pb.APITokenServiceRevokeRequest{Id: tc.target.ID.String()})

			if tc.wantErr == nil {
				assert.NoError(t, err)
				return
			}

			require.Error(t, err)
			assert.True(t, tc.wantErr(err), "unexpected refusal: %v", err)
		})
	}
}

// A token recording no scope, such as a row a control plane from before the scope columns wrote
// after the backfill ran, is confined to nothing: it passes no project check, forced RBAC
// included, and may create neither projects nor organization-level contracts.
func TestTokensRecordingNoScopeReachNothing(t *testing.T) {
	token := &entities.APIToken{ID: uuid.NewString()}
	ctx := entities.WithCurrentAPIToken(context.Background(), token)
	ctx = usercontext.WithAuthzSubject(ctx, (&authz.SubjectAPIToken{ID: token.ID}).String())

	s := newTestServiceWithTokenPolicies(t, []*authz.Policy{authz.PolicyWorkflowCreate})

	assert.Error(t, s.authorizeResource(ctx, authz.PolicyWorkflowCreate, authz.ResourceTypeProject, uuid.New()))
	assert.Error(t, s.authorizeResource(ctx, authz.PolicyWorkflowCreate, authz.ResourceTypeProject, uuid.New(), withForceRBAC()))

	canCreate, err := s.canCreateProject(ctx)
	require.NoError(t, err)
	assert.False(t, canCreate)
	assert.False(t, canCreateContractsInRestrictedMode(ctx))

	visible := s.visibleProjects(ctx)
	require.NotNil(t, visible)
	assert.Empty(t, visible)
}
