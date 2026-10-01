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

package entities

import (
	"testing"

	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/authz"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

// What a token reaches, and the resource it names, follow its kind, never whether scope_id or a
// list is present. A token acting for its whole organization or instance names nothing and
// reaches every project; a token recording no scope, or a scope missing its id, names and reaches
// nothing.
func TestAPITokenScope(t *testing.T) {
	t.Parallel()

	orgID, projectID, productID, a, b := uuid.New(), uuid.New(), uuid.New(), uuid.New(), uuid.New()
	product, project, organization, instance := authz.ResourceTypeProduct, authz.ResourceTypeProject, authz.ResourceTypeOrganization, authz.ResourceTypeInstance

	testCases := []struct {
		name               string
		token              *APIToken
		wantOrgScoped      bool
		wantInstanceScoped bool
		wantProductScoped  bool
		wantReach          []uuid.UUID
		// wantKind and wantID are what ResourceScope names, when wantOK
		wantKind authz.ResourceType
		wantID   uuid.UUID
		wantOK   bool
	}{
		{name: "no token", token: nil, wantReach: []uuid.UUID{}},
		{name: "a token recording no scope", token: &APIToken{}, wantReach: []uuid.UUID{}},
		{name: "a project id without a scope", token: &APIToken{ProjectID: &projectID}, wantReach: []uuid.UUID{}},
		{name: "an organization token", token: &APIToken{Scope: &organization, ScopeID: &orgID}, wantOrgScoped: true},
		{name: "an instance token", token: &APIToken{Scope: &instance}, wantInstanceScoped: true},
		{
			name: "a project token", token: &APIToken{ProjectID: &projectID, Scope: &project, ScopeID: &projectID},
			wantReach: []uuid.UUID{projectID}, wantKind: project, wantID: projectID, wantOK: true,
		},
		{
			name: "a project scope is read from its scope id", token: &APIToken{Scope: &project, ScopeID: &projectID},
			wantReach: []uuid.UUID{projectID}, wantKind: project, wantID: projectID, wantOK: true,
		},
		{name: "a project scope missing its id", token: &APIToken{Scope: &project, ProjectID: &projectID}, wantReach: []uuid.UUID{}},
		{
			name: "a product token", token: &APIToken{Scope: &product, ScopeID: &productID, ProjectIDs: []uuid.UUID{a, b}},
			wantProductScoped: true, wantReach: []uuid.UUID{a, b}, wantKind: product, wantID: productID, wantOK: true,
		},
		{
			name: "a product token reaching nothing", token: &APIToken{Scope: &product, ScopeID: &productID, ProjectIDs: []uuid.UUID{}},
			wantProductScoped: true, wantReach: []uuid.UUID{}, wantKind: product, wantID: productID, wantOK: true,
		},
		{
			name: "a product token whose list is missing", token: &APIToken{Scope: &product, ScopeID: &productID},
			wantProductScoped: true, wantReach: []uuid.UUID{}, wantKind: product, wantID: productID, wantOK: true,
		},
		{name: "a product scope missing its id", token: &APIToken{Scope: &product}, wantProductScoped: true, wantReach: []uuid.UUID{}},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tc.wantOrgScoped, tc.token.IsOrgScoped())
			assert.Equal(t, tc.wantInstanceScoped, tc.token.IsInstanceScoped())
			assert.Equal(t, tc.wantProductScoped, tc.token.IsProductScoped())

			kind, id, ok := tc.token.ResourceScope()
			assert.Equal(t, tc.wantOK, ok)
			assert.Equal(t, tc.wantKind, kind)
			assert.Equal(t, tc.wantID, id)

			got := tc.token.ReachableProjects()
			if tc.wantOrgScoped || tc.wantInstanceScoped {
				assert.Nil(t, got, "nil means an organization or instance token is not restricted to a list")
				assert.True(t, tc.token.ReachesProject(uuid.New()), "an organization or instance token reaches every project of its organization")
				return
			}
			assert.NotNil(t, got, "a confined token's reach is never nil")
			assert.Equal(t, tc.wantReach, got)
			for _, id := range tc.wantReach {
				assert.True(t, tc.token.ReachesProject(id))
			}
			assert.False(t, tc.token.ReachesProject(uuid.New()))
		})
	}
}
