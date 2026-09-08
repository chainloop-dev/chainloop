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

	"github.com/chainloop-dev/chainloop/app/controlplane/internal/usercontext"
	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/authz"
	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/biz"
	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/usercontext/entities"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newTestService builds a service with a real casbin enforcer, so the
// permission answers come from the same rules the enforcement path uses.
func newTestService(t *testing.T) *service {
	t.Helper()

	logger := log.NewStdLogger(io.Discard)
	enforcer, err := authz.NewCasbinEnforcer(&authz.Config{RolesMap: authz.RolesMap})
	require.NoError(t, err)

	return &service{
		log: log.NewHelper(logger),
		authz: biz.NewAuthzUseCase(&biz.AuthzUseCaseConfig{
			CasbinEnforcer: enforcer,
			Logger:         logger,
		}),
	}
}

// projectMembership is one of the caller's roles on a project.
func projectMembership(id uuid.UUID, role authz.Role) *entities.ResourceMembership {
	return &entities.ResourceMembership{
		ResourceType: authz.ResourceTypeProject,
		ResourceID:   id,
		Role:         role,
	}
}

// rbacContext puts the caller's memberships on the context under a role that
// turns RBAC on, which is what makes the per-project check apply.
func rbacContext(memberships ...*entities.ResourceMembership) context.Context {
	ctx := usercontext.WithAuthzSubject(context.Background(), string(authz.RoleOrgMember))

	return entities.WithMembership(ctx, &entities.Membership{Resources: memberships})
}

// TestProjectsAllowing covers which projects a listing may offer as a workflow
// target. Getting this wrong either hides projects the caller can use or, worse,
// offers ones that fail at creation time.
func TestProjectsAllowing(t *testing.T) {
	admin, viewer, both := uuid.New(), uuid.New(), uuid.New()

	projects := []*biz.Project{
		{ID: admin, Name: "admin-project"},
		{ID: viewer, Name: "viewer-project"},
		{ID: both, Name: "both-project"},
	}

	testCases := []struct {
		name        string
		ctx         context.Context
		wantAllowed map[uuid.UUID]bool
	}{
		{
			name: "a project admin may create a workflow, a viewer may not",
			ctx: rbacContext(
				projectMembership(admin, authz.RoleProjectAdmin),
				projectMembership(viewer, authz.RoleProjectViewer),
			),
			wantAllowed: map[uuid.UUID]bool{admin: true, viewer: false, both: false},
		},
		{
			// A caller can hold several roles on one project, directly and
			// through a product. Any one of them granting the permission is
			// enough, which is how the enforcement path behaves.
			name: "holding both roles on a project allows it, whatever the order",
			ctx: rbacContext(
				projectMembership(both, authz.RoleProjectViewer),
				projectMembership(both, authz.RoleProjectAdmin),
			),
			wantAllowed: map[uuid.UUID]bool{admin: false, viewer: false, both: true},
		},
		{
			name: "the weaker role listed last does not revoke the stronger one",
			ctx: rbacContext(
				projectMembership(both, authz.RoleProjectAdmin),
				projectMembership(both, authz.RoleProjectViewer),
			),
			wantAllowed: map[uuid.UUID]bool{admin: false, viewer: false, both: true},
		},
		{
			name:        "no project memberships allows nothing",
			ctx:         rbacContext(),
			wantAllowed: map[uuid.UUID]bool{admin: false, viewer: false, both: false},
		},
	}

	s := newTestService(t)

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := s.projectsAllowing(tc.ctx, authz.PolicyWorkflowCreate, projects)
			require.NoError(t, err)

			for id, want := range tc.wantAllowed {
				assert.Equal(t, want, got[id], "project %s", id)
			}
		})
	}
}

// TestProjectsAllowingWithoutRBAC covers the organization roles RBAC does not
// narrow. Whether they may create a workflow is decided by the role alone, so
// it is the same answer for every project: all of them, or none.
func TestProjectsAllowingWithoutRBAC(t *testing.T) {
	testCases := []struct {
		name string
		role authz.Role
		want bool
	}{
		{
			name: "an organization admin may create a workflow anywhere",
			role: authz.RoleAdmin,
			want: true,
		},
		{
			// An organization viewer may list projects, so it reaches here, but
			// its role does not carry the permission. Reporting these as
			// writable would offer a workflow the create call then refuses.
			name: "an organization viewer may create a workflow nowhere",
			role: authz.RoleViewer,
			want: false,
		},
	}

	s := newTestService(t)
	one, two := uuid.New(), uuid.New()
	projects := []*biz.Project{{ID: one}, {ID: two}}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			require.False(t, tc.role.RBACEnabled(), "precondition: this role bypasses RBAC")
			ctx := usercontext.WithAuthzSubject(context.Background(), string(tc.role))

			got, err := s.projectsAllowing(ctx, authz.PolicyWorkflowCreate, projects)
			require.NoError(t, err)
			assert.Equal(t, tc.want, got[one])
			assert.Equal(t, tc.want, got[two])
		})
	}
}
