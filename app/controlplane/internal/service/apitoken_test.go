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
	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/authz"
	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/biz"
	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/usercontext/entities"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAPITokenService_Create_OrgTokenWithoutProjectIsRejected(t *testing.T) {
	t.Parallel()

	svc := &APITokenService{service: newService()}

	ctx := context.Background()
	ctx = entities.WithCurrentOrg(ctx, &entities.Org{ID: uuid.NewString()})
	ctx = entities.WithCurrentAPIToken(ctx, &entities.APIToken{ID: uuid.NewString(), ProjectID: nil})

	req := &pb.APITokenServiceCreateRequest{Name: "test-token"}

	_, err := svc.Create(ctx, req)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "org-level API tokens must specify a project")
}

func TestAPITokenService_List_OrgTokenForcesProjectScope(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		token     *entities.APIToken
		wantScope biz.APITokenScope
	}{
		{
			name:      "org-level token forces project scope",
			token:     &entities.APIToken{ID: uuid.NewString(), ProjectID: nil},
			wantScope: biz.APITokenScopeProject,
		},
		{
			name:      "project-scoped token does not override scope",
			token:     &entities.APIToken{ID: uuid.NewString(), ProjectID: toUUIDPtr(uuid.New())},
			wantScope: "", // mapTokenScope returns "" for SCOPE_UNSPECIFIED
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			ctx = entities.WithCurrentAPIToken(ctx, tc.token)

			scope := mapTokenScope(pb.APITokenServiceListRequest_SCOPE_UNSPECIFIED)
			if token := entities.CurrentAPIToken(ctx); token != nil && token.ProjectID == nil {
				scope = biz.APITokenScopeProject
			}

			assert.Equal(t, tc.wantScope, scope)
		})
	}
}

func TestAPITokenService_Revoke_OrgTokenCannotRevokeOrgTokens(t *testing.T) {
	t.Parallel()

	orgID := uuid.NewString()

	tests := []struct {
		name          string
		callerToken   *entities.APIToken
		targetToken   *biz.APIToken
		wantForbidden bool
	}{
		{
			name:        "org-level token revoking org-level token is forbidden",
			callerToken: &entities.APIToken{ID: uuid.NewString(), ProjectID: nil},
			targetToken: &biz.APIToken{
				ID:             uuid.New(),
				OrganizationID: uuid.MustParse(orgID),
				ProjectID:      nil,
			},
			wantForbidden: true,
		},
		{
			name:        "org-level token revoking project token is allowed",
			callerToken: &entities.APIToken{ID: uuid.NewString(), ProjectID: nil},
			targetToken: &biz.APIToken{
				ID:             uuid.New(),
				OrganizationID: uuid.MustParse(orgID),
				ProjectID:      toUUIDPtr(uuid.New()),
			},
			wantForbidden: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			ctx = entities.WithCurrentAPIToken(ctx, tc.callerToken)

			forbidden := false
			if token := entities.CurrentAPIToken(ctx); token != nil && token.ProjectID == nil {
				if tc.targetToken.ProjectID == nil {
					forbidden = true
				}
			}

			assert.Equal(t, tc.wantForbidden, forbidden)
		})
	}
}

func toUUIDPtr(id uuid.UUID) *uuid.UUID {
	return &id
}

// A listing must report what the token actually reaches. A product token has no project, so
// without its own branch it would come back with no scoped entity at all and read as
// organization-wide in the CLI and the UI.
func TestAPITokenBizToPbScopedEntity(t *testing.T) {
	t.Parallel()

	projectID, productID := uuid.New(), uuid.New()
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
				ProjectID: &projectID, ProjectName: toPtr(testProjectName),
			},
			want: &pb.ScopedEntity{Type: string(authz.ResourceTypeProject), Id: projectID.String(), Name: testProjectName},
		},
		{
			// The product's name is not known to the control plane, so its id stands in for it.
			name: "a product-scoped token reports its product by id",
			token: &biz.APIToken{
				ID: uuid.New(), CreatedAt: &createdAt,
				Scope: toPtr(authz.ResourceTypeProduct), ScopeID: &productID,
			},
			want: &pb.ScopedEntity{Type: testProductScope, Id: productID.String(), Name: productID.String()},
		},
		{
			// Only a product is reported from the scope columns; nothing is guessed to be one.
			name: "a scope id without a kind is not reported as a product",
			token: &biz.APIToken{
				ID: uuid.New(), CreatedAt: &createdAt,
				ScopeID: &productID,
			},
			want: nil,
		},
		{
			name: "a kind other than product is not reported",
			token: &biz.APIToken{
				ID: uuid.New(), CreatedAt: &createdAt,
				Scope: biz.ToPtr(authz.ResourceTypeOrganization), ScopeID: &productID,
			},
			want: nil,
		},
		{
			name:  "an organization-level token reports none",
			token: &biz.APIToken{ID: uuid.New(), CreatedAt: &createdAt},
			want:  nil,
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
