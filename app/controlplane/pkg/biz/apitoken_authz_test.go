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
	"github.com/stretchr/testify/assert"
)

// TestDefaultTokenPoliciesAreHeldByAHumanRole guards against privilege escalation through
// project-scoped tokens: a policy held by no role in RolesMap is reachable for users only via the
// authz middleware's IsAdmin() bypass, so shipping it to every token makes it an org-admin capability
// that any project-scoped token would also get.
func TestDefaultTokenPoliciesAreHeldByAHumanRole(t *testing.T) {
	heldByRole := make(map[authz.Policy]struct{})
	for _, policies := range authz.RolesMap {
		for _, p := range policies {
			heldByRole[*p] = struct{}{}
		}
	}

	for _, p := range defaultAuthzPolicies {
		assert.Containsf(t, heldByRole, *p,
			"policy %s:%s is in defaultAuthzPolicies but held by no role in RolesMap; move it to orgLevelTokenPolicies",
			p.Resource, p.Action)
	}
}

// Backstops the test above, which stops guarding anything if a project-level role is ever granted one
// of these policies in RolesMap. The two sets must stay disjoint.
func TestOrgLevelPoliciesStayOutOfDefaults(t *testing.T) {
	for _, p := range orgLevelTokenPolicies {
		assert.NotContainsf(t, defaultAuthzPolicies, p,
			"policy %s:%s is org-level-only but also in defaultAuthzPolicies, so project-scoped tokens receive it",
			p.Resource, p.Action)
	}
}

// The permission to issue a sandbox identity must never reach a token or a user by default (D-004).
func TestSandboxIdentityPolicyIsNeverGrantedByDefault(t *testing.T) {
	assert.NotContains(t, defaultAuthzPolicies, authz.PolicySandboxIdentityIssue)
	assert.NotContains(t, orgLevelTokenPolicies, authz.PolicySandboxIdentityIssue)

	for role, policies := range authz.RolesMap {
		assert.NotContainsf(t, policies, authz.PolicySandboxIdentityIssue, "role %s holds the sandbox identity policy", role)
	}
}

func TestCheckSandboxIdentityPolicy(t *testing.T) {
	testCases := []struct {
		name     string
		policies []*authz.Policy
		isSystem bool
		wantErr  bool
	}{
		{name: "a system token", policies: []*authz.Policy{authz.PolicyArtifactUpload, authz.PolicySandboxIdentityIssue}, isSystem: true},
		{name: "a token that is not a system token", policies: []*authz.Policy{{Resource: authz.ResourceSandboxIdentity, Action: authz.ActionCreate}}, wantErr: true},
		{name: "a token without the policy", policies: []*authz.Policy{authz.PolicyArtifactUpload}},
		{name: "a token with a nil policy", policies: []*authz.Policy{nil}},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := checkSandboxIdentityPolicy(tc.policies, tc.isSystem)
			if tc.wantErr {
				assert.True(t, IsErrValidation(err))
				return
			}

			assert.NoError(t, err)
		})
	}
}
