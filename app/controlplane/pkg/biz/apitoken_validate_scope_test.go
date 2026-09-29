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

// The database refuses most incoherent scopes too; this pins the check Create makes itself,
// before anything is written.
func TestValidateTokenScope(t *testing.T) {
	t.Parallel()

	org, otherOrg, project, otherProject, product := uuid.New(), uuid.New(), uuid.New(), uuid.New(), uuid.New()

	testCases := []struct {
		name      string
		scope     authz.ResourceType
		scopeID   *uuid.UUID
		orgID     *uuid.UUID
		projectID *uuid.UUID
		wantErr   bool
	}{
		{name: "organization naming its organization", scope: authz.ResourceTypeOrganization, scopeID: &org, orgID: &org},
		{name: "project naming its project", scope: authz.ResourceTypeProject, scopeID: &project, orgID: &org, projectID: &project},
		{name: "instance with no id", scope: authz.ResourceTypeInstance},

		{name: "organization naming another organization", scope: authz.ResourceTypeOrganization, scopeID: &otherOrg, orgID: &org, wantErr: true},
		{name: "organization with no id", scope: authz.ResourceTypeOrganization, orgID: &org, wantErr: true},
		{name: "organization on a project token", scope: authz.ResourceTypeOrganization, scopeID: &org, orgID: &org, projectID: &project, wantErr: true},
		{name: "organization on an instance-level token", scope: authz.ResourceTypeOrganization, scopeID: &org, wantErr: true},
		{name: "project without its project", scope: authz.ResourceTypeProject, scopeID: &project, orgID: &org, wantErr: true},
		{name: "project naming another project", scope: authz.ResourceTypeProject, scopeID: &otherProject, orgID: &org, projectID: &project, wantErr: true},
		{name: "project with no id", scope: authz.ResourceTypeProject, orgID: &org, projectID: &project, wantErr: true},
		{name: "instance with an id", scope: authz.ResourceTypeInstance, scopeID: &product, wantErr: true},
		{name: "instance on an organization token", scope: authz.ResourceTypeInstance, orgID: &org, wantErr: true},
		{name: "product is not supported yet", scope: authz.ResourceTypeProduct, scopeID: &product, orgID: &org, wantErr: true},
		{name: "a kind tokens are never scoped to", scope: authz.ResourceTypeGroup, scopeID: &product, orgID: &org, wantErr: true},
		{name: "no kind at all", scope: "", orgID: &org, wantErr: true},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			err := validateTokenScope(tc.scope, tc.scopeID, tc.orgID, tc.projectID)
			if !tc.wantErr {
				assert.NoError(t, err)
				return
			}

			assert.True(t, IsErrValidation(err), "want a validation error, got %v", err)
		})
	}
}
