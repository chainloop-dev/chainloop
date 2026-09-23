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

// Mirrors entities.TestAPITokenScopePredicates for the persisted row: only a product scope
// confines a token to its memberships.
func TestAPITokenScopePredicates(t *testing.T) {
	t.Parallel()

	orgID, projectID, productID := uuid.New(), uuid.New(), uuid.New()

	testCases := []struct {
		name               string
		token              *APIToken
		wantResourceScoped bool
		wantOrgWide        bool
	}{
		{name: "no token", token: nil},
		{name: "an organization token from before the scope columns", token: &APIToken{}, wantOrgWide: true},
		{name: "an organization-scoped token", token: &APIToken{Scope: ToPtr(authz.ResourceTypeOrganization), ScopeID: &orgID}, wantOrgWide: true},
		{name: "a project token from before the scope columns", token: &APIToken{ProjectID: &projectID}},
		{name: "a project-scoped token", token: &APIToken{ProjectID: &projectID, Scope: ToPtr(authz.ResourceTypeProject), ScopeID: &projectID}},
		{name: "an instance-scoped token", token: &APIToken{Scope: ToPtr(authz.ResourceTypeInstance)}, wantOrgWide: true},
		{name: "a product-scoped token", token: &APIToken{Scope: ToPtr(authz.ResourceTypeProduct), ScopeID: &productID}, wantResourceScoped: true},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tc.wantResourceScoped, tc.token.IsResourceScoped())
			assert.Equal(t, tc.wantOrgWide, tc.token.IsOrgWide())
		})
	}
}
