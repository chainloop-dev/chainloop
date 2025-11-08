//
// Copyright 2024-2025 The Chainloop Authors.
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

package authz

import (
	"io"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)


// simulate 2 enforcers on the same database (by acting on the same file enforcer)
func TestSyncMultipleEnforcers(t *testing.T) {
	testCases := []struct {
		name              string
		newEnforcerConfig *Config
		expectErr         bool
		numPolicies       int
		numSubjects       int
		numAdminActions   int
	}{
		{
			name:              "empty config",
			newEnforcerConfig: &Config{},
			expectErr:         false,
			numPolicies:       3,
			numSubjects:       2,
			numAdminActions:   2,
		},
		{
			name: "new actions on different resources for same roles",
			newEnforcerConfig: &Config{
				ManagedResources: []string{ResourceGroup},
				RolesMap: map[Role][]*Policy{
					RoleAdmin: {{
						Resource: ResourceGroup,
						Action:   ActionCreate,
					}},
				},
			},
			expectErr:       false,
			numPolicies:     4,
			numSubjects:     2,
			numAdminActions: 3,
		},
		{
			name: "new actions on different resources for new roles",
			newEnforcerConfig: &Config{
				ManagedResources: []string{ResourceGroup},
				RolesMap: map[Role][]*Policy{
					RoleProjectAdmin: {{
						Resource: ResourceGroup,
						Action:   ActionCreate,
					}},
				},
			},
			expectErr:       false,
			numSubjects:     3,
			numPolicies:     4,
			numAdminActions: 2,
		},
		{
			name: "reset admin actions on same resource, collision",
			newEnforcerConfig: &Config{
				ManagedResources: []string{ResourceWorkflow},
				RolesMap: map[Role][]*Policy{
					RoleAdmin: {}, // this should remove all admin actions from enforcer
				},
			},
			expectErr:       false,
			numSubjects:     1,
			numPolicies:     1,
			numAdminActions: 0,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			e, c := testEnforcer(t)
			defer c.Close()

			// initial import
			err := syncRBACRoles(e, &Config{
				ManagedResources: []string{ResourceWorkflow, ResourceWorkflowRun},
				RolesMap: map[Role][]*Policy{
					RoleAdmin: {{
						Resource: ResourceWorkflow,
						Action:   ActionCreate,
					}, {
						Resource: ResourceWorkflow,
						Action:   ActionDelete,
					}},
					RoleOrgMember: {{
						Resource: ResourceWorkflowRun,
						Action:   ActionList,
					}},
				},
			})
			require.NoError(t, err)

			// sync with test case config
			err = syncRBACRoles(e, tc.newEnforcerConfig)
			if tc.expectErr {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)

			policies, err := e.GetPolicy()
			assert.NoError(t, err)
			assert.Len(t, policies, tc.numPolicies)

			adminCount := 0
			for _, r := range policies {
				if r[0] == string(RoleAdmin) {
					adminCount++
				}
			}
			assert.Equal(t, tc.numAdminActions, adminCount)

			subs, err := e.GetAllSubjects()
			assert.NoError(t, err)
			assert.Len(t, subs, tc.numSubjects) // We need to count the Viewer role
		})
	}
}


func TestSyncRBACRoles(t *testing.T) {
	e, closer := testEnforcer(t)
	defer closer.Close()

	// load all the roles
	err := syncRBACRoles(e, &Config{RolesMap: RolesMap})
	assert.NoError(t, err)

	// Check the inherited roles owner -> admin -> viewer
	u, err := e.GetUsersForRole(string(RoleViewer))
	assert.NoError(t, err)
	// admin is a viewer
	assert.Equal(t, string(RoleAdmin), u[0])
	// owner is an admin
	u, err = e.GetUsersForRole(string(RoleAdmin))
	assert.NoError(t, err)
	assert.Equal(t, string(RoleOwner), u[0])

	// Make sure we are adding all the policies for the listed roles
	for r, policies := range RolesMap {
		got, err := e.GetFilteredPolicy(0, string(r))
		assert.NoError(t, err)
		assert.Len(t, got, len(policies))
	}

	// Check that an admin can inherit the policies from the viewer
	ok, err := e.Enforce(string(RoleAdmin), PolicyWorkflowContractList)
	assert.NoError(t, err)
	assert.True(t, ok)
	// and owner from viewer
	ok, err = e.Enforce(string(RoleOwner), PolicyWorkflowContractList)
	assert.NoError(t, err)
	assert.True(t, ok)
}

func TestDoSync(t *testing.T) {
	e, closer := testEnforcer(t)
	defer closer.Close()

	// Clear any existing policy
	e.ClearPolicy()

	policiesM := map[Role][]*Policy{
		"foo": {
			PolicyWorkflowContractList,
			PolicyWorkflowContractRead,
		}, "bar": {
			PolicyArtifactDownload,
		},
	}

	// load custom policies
	err := doSync(e, &Config{RolesMap: policiesM})
	assert.NoError(t, err)
	got, err := e.GetPolicy()
	assert.NoError(t, err)
	assert.Len(t, got, 3)

	// update stored map removing one item of one role
	policiesM = map[Role][]*Policy{
		"foo": {
			PolicyWorkflowContractList,
		},
		"bar": {
			PolicyArtifactDownload,
		},
	}

	err = doSync(e, &Config{RolesMap: policiesM, ManagedResources: []string{ResourceWorkflowContract, ResourceCASArtifact}})
	assert.NoError(t, err)
	got, err = e.GetPolicy()
	assert.NoError(t, err)
	assert.Len(t, got, 2)

	// or deleting a whole section
	policiesM = map[Role][]*Policy{
		"bar": {
			PolicyArtifactDownload,
		},
	}

	err = doSync(e, &Config{RolesMap: policiesM, ManagedResources: []string{ResourceWorkflowContract, ResourceCASArtifact}})
	assert.NoError(t, err)
	got, err = e.GetPolicy()
	assert.NoError(t, err)
	assert.Len(t, got, 1)

	// add additional policy, only deletes policies for "known" resources
	// or deleting a whole section
	policiesM = map[Role][]*Policy{
		"bar": {
			PolicyAttachedIntegrationDetach,
		},
	}
	err = doSync(e, &Config{RolesMap: policiesM})
	assert.NoError(t, err)
	got, err = e.GetPolicy()
	assert.NoError(t, err)
	assert.Len(t, got, 2)
}


func testEnforcer(t *testing.T) (*Enforcer, io.Closer) {
	f, err := os.CreateTemp(t.TempDir(), "policy*.csv")
	if err != nil {
		require.FailNow(t, err.Error())
	}

	enforcer, err := NewFiletypeEnforcer(f.Name(), &Config{})
	require.NoError(t, err)
	return enforcer, f
}
