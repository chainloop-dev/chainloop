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

	err = doSync(e, &Config{RolesMap: policiesM})
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

	err = doSync(e, &Config{RolesMap: policiesM})
	assert.NoError(t, err)
	got, err = e.GetPolicy()
	assert.NoError(t, err)
	assert.Len(t, got, 1)

	// replace policy for a role - old policies are removed and new ones added
	policiesM = map[Role][]*Policy{
		"bar": {
			PolicyAttachedIntegrationDetach,
		},
	}
	err = doSync(e, &Config{RolesMap: policiesM})
	assert.NoError(t, err)
	got, err = e.GetPolicy()
	assert.NoError(t, err)
	assert.Len(t, got, 1)

	// verify the new policy is present
	assert.Equal(t, "bar", got[0][0])
	assert.Equal(t, "integration_attached", got[0][1])
	assert.Equal(t, "delete", got[0][2])
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
