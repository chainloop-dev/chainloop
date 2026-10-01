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

package biz

import (
	"testing"

	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/authz"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

// A token's scope must agree with the organization and project it is on, and only a product token
// carries a project list, which it always does. Create checks this against the scope it settles
// on, and the repository before every write.
func TestValidateTokenShape(t *testing.T) {
	t.Parallel()

	org, otherOrg, project, otherProject, product := uuid.New(), uuid.New(), uuid.New(), uuid.New(), uuid.New()
	kind := func(k authz.ResourceType) *authz.ResourceType { return &k }
	noProjects := []uuid.UUID{}

	testCases := []struct {
		name       string
		scope      *authz.ResourceType
		scopeID    *uuid.UUID
		orgID      *uuid.UUID
		projectID  *uuid.UUID
		projectIDs []uuid.UUID
		wantErr    bool
	}{
		{name: "organization naming its organization", scope: kind(authz.ResourceTypeOrganization), scopeID: &org, orgID: &org},
		{name: "project naming its project", scope: kind(authz.ResourceTypeProject), scopeID: &project, orgID: &org, projectID: &project},
		{name: "instance with no id", scope: kind(authz.ResourceTypeInstance)},
		{name: "product naming its product", scope: kind(authz.ResourceTypeProduct), scopeID: &product, orgID: &org, projectIDs: noProjects},
		{name: "a row recording no scope", orgID: &org},

		{name: "organization naming another organization", scope: kind(authz.ResourceTypeOrganization), scopeID: &otherOrg, orgID: &org, wantErr: true},
		{name: "organization with no id", scope: kind(authz.ResourceTypeOrganization), orgID: &org, wantErr: true},
		{name: "organization on a project token", scope: kind(authz.ResourceTypeOrganization), scopeID: &org, orgID: &org, projectID: &project, wantErr: true},
		{name: "organization on an instance-level token", scope: kind(authz.ResourceTypeOrganization), scopeID: &org, wantErr: true},
		{name: "project without its project", scope: kind(authz.ResourceTypeProject), scopeID: &project, orgID: &org, wantErr: true},
		{name: "project naming another project", scope: kind(authz.ResourceTypeProject), scopeID: &otherProject, orgID: &org, projectID: &project, wantErr: true},
		{name: "project with no id", scope: kind(authz.ResourceTypeProject), orgID: &org, projectID: &project, wantErr: true},
		{name: "instance with an id", scope: kind(authz.ResourceTypeInstance), scopeID: &product, wantErr: true},
		{name: "instance on an organization token", scope: kind(authz.ResourceTypeInstance), orgID: &org, wantErr: true},
		{name: "product alongside a project", scope: kind(authz.ResourceTypeProduct), scopeID: &product, orgID: &org, projectID: &project, projectIDs: noProjects, wantErr: true},
		{name: "product on an instance-level token", scope: kind(authz.ResourceTypeProduct), scopeID: &product, projectIDs: noProjects, wantErr: true},
		{name: "product with no id", scope: kind(authz.ResourceTypeProduct), orgID: &org, projectIDs: noProjects, wantErr: true},
		{name: "product without its project list", scope: kind(authz.ResourceTypeProduct), scopeID: &product, orgID: &org, wantErr: true},
		{name: "a project list on an organization token", scope: kind(authz.ResourceTypeOrganization), scopeID: &org, orgID: &org, projectIDs: noProjects, wantErr: true},
		{name: "a scope id without a kind", scopeID: &org, orgID: &org, wantErr: true},
		{name: "a kind tokens are never scoped to", scope: kind(authz.ResourceTypeGroup), scopeID: &product, orgID: &org, wantErr: true},
		{name: "an empty kind", scope: kind(""), orgID: &org, wantErr: true},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			err := ValidateTokenShape(tc.scope, tc.scopeID, tc.orgID, tc.projectID, tc.projectIDs)
			if !tc.wantErr {
				assert.NoError(t, err)
				return
			}

			assert.True(t, IsErrValidation(err), "want a validation error, got %v", err)
		})
	}
}
