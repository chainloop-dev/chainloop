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
	"testing"
	"time"

	pb "github.com/chainloop-dev/chainloop/app/controlplane/api/controlplane/v1"
	"github.com/chainloop-dev/chainloop/app/controlplane/internal/usercontext"
	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/authz"
	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/biz"
	bizMocks "github.com/chainloop-dev/chainloop/app/controlplane/pkg/biz/mocks"
	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/usercontext/entities"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestAPITokenService_Create_OrgTokenWithoutProjectIsRejected(t *testing.T) {
	t.Parallel()

	svc := &APITokenService{service: newService()}

	orgID := uuid.New()
	ctx := context.Background()
	ctx = entities.WithCurrentOrg(ctx, &entities.Org{ID: orgID.String()})
	ctx = entities.WithCurrentAPIToken(ctx, &entities.APIToken{ID: uuid.NewString(), Scope: toPtr(authz.ResourceTypeOrganization), ScopeID: &orgID})

	req := &pb.APITokenServiceCreateRequest{Name: "test-token"}

	_, err := svc.Create(ctx, req)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "org-level API tokens must specify a project")
}

// An organization-wide token may only list project tokens, whatever scope it asks for; every
// other caller gets the scope it asked for. Driven through List itself, down to the filters the
// repository receives, so the override cannot drift away from what is asserted here.
func TestAPITokenServiceListForcesProjectScopeForOrgTokens(t *testing.T) {
	t.Parallel()

	orgID, projectID, productID := uuid.New(), uuid.New(), uuid.New()

	testCases := []struct {
		name string
		// caller is the API token making the request; nil means a user
		caller       *entities.APIToken
		requested    pb.APITokenServiceListRequest_Scope
		wantScope    authz.ResourceType
		wantProjects []uuid.UUID
	}{
		{
			name:      "an organization token is forced to project tokens",
			caller:    &entities.APIToken{ID: uuid.NewString(), Scope: toPtr(authz.ResourceTypeOrganization), ScopeID: &orgID},
			wantScope: authz.ResourceTypeProject,
		},
		{
			name:      "an organization token asking for global tokens is still forced",
			caller:    &entities.APIToken{ID: uuid.NewString(), Scope: toPtr(authz.ResourceTypeOrganization), ScopeID: &orgID},
			requested: pb.APITokenServiceListRequest_SCOPE_GLOBAL,
			wantScope: authz.ResourceTypeProject,
		},
		{
			// Not organization-wide either, and confined to nothing: narrowed to no project.
			name:         "a token recording no scope is narrowed to no project",
			caller:       &entities.APIToken{ID: uuid.NewString()},
			requested:    pb.APITokenServiceListRequest_SCOPE_GLOBAL,
			wantScope:    authz.ResourceTypeOrganization,
			wantProjects: []uuid.UUID{},
		},
		{
			// Not organization-wide, so not forced: it is narrowed to its projects instead.
			name:         "a product token keeps the scope it asks for, narrowed to its projects",
			caller:       &entities.APIToken{ID: uuid.NewString(), Scope: toPtr(authz.ResourceTypeProduct), ScopeID: &productID, ProjectIDs: []uuid.UUID{projectID}},
			requested:    pb.APITokenServiceListRequest_SCOPE_GLOBAL,
			wantScope:    authz.ResourceTypeOrganization,
			wantProjects: []uuid.UUID{projectID},
		},
		{
			// Empty, not nil: nil would mean RBAC does not narrow this caller at all.
			name:         "a product token reaching nothing is narrowed to no project",
			caller:       &entities.APIToken{ID: uuid.NewString(), Scope: toPtr(authz.ResourceTypeProduct), ScopeID: &productID, ProjectIDs: []uuid.UUID{}},
			wantProjects: []uuid.UUID{},
		},
		{
			name:         "a project token keeps the scope it asks for",
			caller:       &entities.APIToken{ID: uuid.NewString(), ProjectID: &projectID, Scope: toPtr(authz.ResourceTypeProject), ScopeID: &projectID},
			requested:    pb.APITokenServiceListRequest_SCOPE_GLOBAL,
			wantScope:    authz.ResourceTypeOrganization,
			wantProjects: []uuid.UUID{projectID},
		},
		{
			name:      "a user keeps the scope it asks for",
			requested: pb.APITokenServiceListRequest_SCOPE_GLOBAL,
			wantScope: authz.ResourceTypeOrganization,
		},
		{
			name:      "a user asking for no scope gets none",
			wantScope: "",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			var got *biz.APITokenListFilters
			repo := bizMocks.NewAPITokenRepo(t)
			repo.On("List", mock.Anything, mock.Anything, mock.Anything).Once().
				Run(func(args mock.Arguments) { got = args.Get(2).(*biz.APITokenListFilters) }).
				Return([]*biz.APIToken{}, nil)
			uc, err := biz.NewAPITokenUseCase(repo, &biz.APITokenJWTConfig{SymmetricHmacKey: "test"}, nil, nil, nil, nil)
			require.NoError(t, err)

			ctx := entities.WithCurrentOrg(context.Background(), &entities.Org{ID: orgID.String(), Name: "acme"})
			if tc.caller != nil {
				ctx = entities.WithCurrentAPIToken(ctx, tc.caller)
			} else {
				ctx = usercontext.WithAuthzSubject(ctx, string(authz.RoleAdmin))
			}

			_, err = NewAPITokenService(uc).List(ctx, &pb.APITokenServiceListRequest{Scope: tc.requested})
			require.NoError(t, err)
			require.NotNil(t, got)
			assert.Equal(t, tc.wantScope, got.FilterByScope)
			assert.Equal(t, tc.wantProjects, got.FilterByProjects)
		})
	}
}

// A listing reports what a token is confined to: its project by name, or its product by id. A
// token acting for its whole organization or instance reports nothing, whether or not its row
// records a scope.
func TestAPITokenBizToPbScopedEntity(t *testing.T) {
	t.Parallel()

	projectID, productID, orgID := uuid.New(), uuid.New(), uuid.New()
	createdAt := time.Now()

	testCases := []struct {
		name  string
		token *biz.APIToken
		want  *pb.ScopedEntity
	}{
		{
			name: "a project-scoped token reports its project",
			token: &biz.APIToken{
				ID: uuid.New(), CreatedAt: &createdAt,
				ProjectID: &projectID, ProjectName: biz.ToPtr("billing"),
				Scope: biz.ToPtr(authz.ResourceTypeProject), ScopeID: &projectID,
			},
			want: &pb.ScopedEntity{Type: string(authz.ResourceTypeProject), Id: projectID.String(), Name: "billing"},
		},
		{
			// The product's name is not known to the control plane, so its id stands in for it.
			// Without its own branch a product token would read as organization-wide.
			name: "a product-scoped token reports its product by id",
			token: &biz.APIToken{
				ID: uuid.New(), CreatedAt: &createdAt,
				Scope: toPtr(authz.ResourceTypeProduct), ScopeID: &productID, ProjectIDs: []uuid.UUID{projectID},
			},
			want: &pb.ScopedEntity{Type: string(authz.ResourceTypeProduct), Id: productID.String(), Name: productID.String()},
		},
		{
			name: "a scope id without a kind is not reported",
			token: &biz.APIToken{
				ID: uuid.New(), CreatedAt: &createdAt,
				ScopeID: &productID,
			},
			want: nil,
		},
		{
			// An organization token is confined to no resource it could report.
			name: "an organization-scoped token reports none",
			token: &biz.APIToken{
				ID: uuid.New(), CreatedAt: &createdAt,
				Scope: biz.ToPtr(authz.ResourceTypeOrganization), ScopeID: &orgID,
			},
			want: nil,
		},
		{
			name: "a project id without a scope is not reported",
			token: &biz.APIToken{
				ID: uuid.New(), CreatedAt: &createdAt,
				ProjectID: &projectID, ProjectName: biz.ToPtr("billing"),
			},
			want: nil,
		},
		{
			name:  "a token recording no scope reports none",
			token: &biz.APIToken{ID: uuid.New(), CreatedAt: &createdAt},
			want:  nil,
		},
		{
			name: "an instance-scoped token reports none",
			token: &biz.APIToken{
				ID: uuid.New(), CreatedAt: &createdAt,
				Scope: biz.ToPtr(authz.ResourceTypeInstance),
			},
			want: nil,
		},
		{
			name: "a product scope missing its id is not reported",
			token: &biz.APIToken{
				ID: uuid.New(), CreatedAt: &createdAt,
				Scope: toPtr(authz.ResourceTypeProduct), ProjectIDs: []uuid.UUID{projectID},
			},
			want: nil,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got := apiTokenBizToPb(tc.token).GetScopedEntity()
			if tc.want == nil {
				assert.Nil(t, got)
				return
			}

			require.NotNil(t, got)
			assert.Equal(t, tc.want.GetType(), got.GetType())
			assert.Equal(t, tc.want.GetId(), got.GetId())
			assert.Equal(t, tc.want.GetName(), got.GetName())
		})
	}
}

// The public listing scopes map onto resource kinds; "global" is the organization's own tokens.
func TestMapTokenScope(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		in   pb.APITokenServiceListRequest_Scope
		want authz.ResourceType
	}{
		{in: pb.APITokenServiceListRequest_SCOPE_UNSPECIFIED, want: ""},
		{in: pb.APITokenServiceListRequest_SCOPE_PROJECT, want: authz.ResourceTypeProject},
		{in: pb.APITokenServiceListRequest_SCOPE_GLOBAL, want: authz.ResourceTypeOrganization},
	}

	for _, tc := range testCases {
		t.Run(tc.in.String(), func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tc.want, mapTokenScope(tc.in))
		})
	}
}
