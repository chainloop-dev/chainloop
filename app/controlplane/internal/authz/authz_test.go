//
// Copyright 2024 The Chainloop Authors.
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
	"fmt"
	"io"
	"os"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAddPolicies(t *testing.T) {
	testcases := []struct {
		name               string
		subject            *SubjectAPIToken
		policies           []*Policy
		wantErr            bool
		wantNumberPolicies int
	}{
		{
			name:    "empty policies and subject",
			wantErr: true,
		},
		{
			name: "no subject",
			policies: []*Policy{
				PolicyWorkflowContractList,
			},
			wantErr: true,
		},
		{
			name:    "no policies",
			subject: &SubjectAPIToken{ID: uuid.NewString()},
			wantErr: true,
		},
		{
			name:    "adds two policies",
			subject: &SubjectAPIToken{ID: uuid.NewString()},
			policies: []*Policy{
				PolicyWorkflowContractList,
				PolicyReferrerRead,
			},
			wantNumberPolicies: 2,
		},
		{
			name: "handles duplicated policies",
			subject: &SubjectAPIToken{
				ID: uuid.NewString(),
			},
			policies: []*Policy{
				PolicyWorkflowContractList,
				PolicyWorkflowContractRead,
				PolicyWorkflowContractUpdate,
				PolicyWorkflowContractList,
				PolicyArtifactDownload,
				PolicyWorkflowContractUpdate,
				PolicyArtifactDownload,
			},
			wantNumberPolicies: 4,
		},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			enforcer, closer := testEnforcer(t)
			closer.Close()

			err := enforcer.AddPolicies(tc.subject, tc.policies...)
			if tc.wantErr {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)

			for _, p := range tc.policies {
				ok := enforcer.HasPolicy(tc.subject.String(), p.Resource, p.Action)
				assert.True(t, ok, fmt.Sprintf("policy %s:%s not found", p.Resource, p.Action))
			}

			gotLength := enforcer.GetFilteredPolicy(0, tc.subject.String())
			assert.Len(t, gotLength, tc.wantNumberPolicies)
		})
	}
}

func TestAddPoliciesDuplication(t *testing.T) {
	want := []*Policy{
		PolicyWorkflowContractList,
		PolicyWorkflowContractRead,
	}

	enforcer, closer := testEnforcer(t)
	defer closer.Close()
	sub := &SubjectAPIToken{ID: uuid.NewString()}

	err := enforcer.AddPolicies(sub, want...)
	require.NoError(t, err)
	got := enforcer.GetFilteredPolicy(0, sub.String())
	assert.Len(t, got, 2)

	// Update the list of policies we want to add by appending an extra one
	want = append(want, PolicyWorkflowContractUpdate)
	// AddPolicies only add the policies that are not already present preventing duplication
	err = enforcer.AddPolicies(sub, want...)
	assert.NoError(t, err)
	got = enforcer.GetFilteredPolicy(0, sub.String())
	assert.Len(t, got, 3)
}

func TestSyncRBACRoles(t *testing.T) {
	e, closer := testEnforcer(t)
	defer closer.Close()

	// load all the roles
	err := syncRBACRoles(e)
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
	for r, policies := range rolesMap {
		got := e.GetFilteredPolicy(0, string(r))
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
	err := doSync(e, policiesM)
	assert.NoError(t, err)
	assert.Len(t, e.GetPolicy(), 3)

	// update stored map removing one item of one role
	policiesM = map[Role][]*Policy{
		"foo": {
			PolicyWorkflowContractList,
		},
		"bar": {
			PolicyArtifactDownload,
		},
	}

	err = doSync(e, policiesM)
	assert.NoError(t, err)
	assert.Len(t, e.GetPolicy(), 2)

	// or deleting a whole section
	policiesM = map[Role][]*Policy{
		"bar": {
			PolicyArtifactDownload,
		},
	}

	err = doSync(e, policiesM)
	assert.NoError(t, err)
	assert.Len(t, e.GetPolicy(), 1)
}

func TestClearPolicies(t *testing.T) {
	want := []*Policy{
		PolicyWorkflowContractList,
		PolicyWorkflowContractRead,
	}

	enforcer, closer := testEnforcer(t)
	defer closer.Close()
	sub := &SubjectAPIToken{ID: uuid.NewString()}
	sub2 := &SubjectAPIToken{ID: uuid.NewString()}

	// Create policies for two different subjects
	err := enforcer.AddPolicies(sub, want...)
	require.NoError(t, err)
	err = enforcer.AddPolicies(sub2, want...)
	require.NoError(t, err)
	// Each have 2 items
	got := enforcer.GetFilteredPolicy(0, sub.String())
	assert.Len(t, got, 2)

	// Clear all the policies for the subject
	err = enforcer.ClearPolicies(sub)
	assert.NoError(t, err)
	// there should be no policies left for this user
	got = enforcer.GetFilteredPolicy(0, sub.String())
	assert.Len(t, got, 0)
	// but the other user should still have 2
	got = enforcer.GetFilteredPolicy(0, sub2.String())
	assert.Len(t, got, 2)
}

func testEnforcer(t *testing.T) (*Enforcer, io.Closer) {
	f, err := os.CreateTemp(t.TempDir(), "policy*.csv")
	if err != nil {
		require.FailNow(t, err.Error())
	}

	enforcer, err := NewFiletypeEnforcer(f.Name())
	require.NoError(t, err)
	return enforcer, f
}
