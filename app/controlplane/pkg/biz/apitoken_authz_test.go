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
// project-scoped tokens. Every token receives defaultAuthzPolicies verbatim whatever its scope, and a
// policy held by no role in authz.RolesMap is reachable for users only via the IsAdmin() bypass in
// the authz middleware. Shipping such a policy in the default set therefore hands every project admin
// an org-admin capability, which is how PolicyRegisteredIntegrationAdd became an escalation (alongside
// a robot-account create policy, since removed from authz entirely).
func TestDefaultTokenPoliciesAreHeldByAHumanRole(t *testing.T) {
	heldByRole := make(map[authz.Policy]struct{})
	for _, policies := range authz.RolesMap {
		for _, p := range policies {
			heldByRole[*p] = struct{}{}
		}
	}

	for _, p := range defaultAuthzPolicies {
		assert.Containsf(t, heldByRole, *p,
			"policy %s:%s is in defaultAuthzPolicies but held by no role in authz.RolesMap, so every "+
				"project-scoped token receives an org-admin-only capability. Move it to orgLevelTokenPolicies.",
			p.Resource, p.Action)
	}
}

// TestOrgLevelPoliciesStayOutOfDefaults backstops the test above, which stops guarding CP-N4 the day
// a project-level role is granted one of these policies in RolesMap. The two sets must stay disjoint.
func TestOrgLevelPoliciesStayOutOfDefaults(t *testing.T) {
	for _, p := range orgLevelTokenPolicies {
		assert.NotContainsf(t, defaultAuthzPolicies, p,
			"policy %s:%s is org-level-only but also in defaultAuthzPolicies, so project-scoped tokens receive it",
			p.Resource, p.Action)
	}
}
