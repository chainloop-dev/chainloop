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

// Only a product scope makes a token product-scoped. ResourceScope names what a token is
// confined to, a project or a product, and nothing for a token acting for its whole organization
// or instance.
func TestAPITokenScopePredicates(t *testing.T) {
	t.Parallel()

	orgID, projectID, productID := uuid.New(), uuid.New(), uuid.New()

	testCases := []struct {
		name              string
		token             *APIToken
		wantProductScoped bool
		wantOrgWide       bool
		// wantKind and wantResource are what ResourceScope reports; wantResource is nil when
		// it reports nothing
		wantKind     authz.ResourceType
		wantResource *uuid.UUID
	}{
		{name: "no token", token: nil},
		{name: "a token recording no scope is confined to nothing", token: &APIToken{}},
		{name: "a project id without a scope names nothing", token: &APIToken{ProjectID: &projectID}},
		{
			name:     "a project scope is read from its scope id",
			token:    &APIToken{Scope: ToPtr(authz.ResourceTypeProject), ScopeID: &projectID},
			wantKind: authz.ResourceTypeProject, wantResource: &projectID,
		},
		{name: "an organization-scoped token", token: &APIToken{Scope: ToPtr(authz.ResourceTypeOrganization), ScopeID: &orgID}, wantOrgWide: true},
		{
			name:     "a project-scoped token",
			token:    &APIToken{ProjectID: &projectID, Scope: ToPtr(authz.ResourceTypeProject), ScopeID: &projectID},
			wantKind: authz.ResourceTypeProject, wantResource: &projectID,
		},
		{name: "an instance-scoped token", token: &APIToken{Scope: ToPtr(authz.ResourceTypeInstance)}, wantOrgWide: true},
		{
			name:              "a product-scoped token",
			token:             &APIToken{Scope: ToPtr(authz.ResourceTypeProduct), ScopeID: &productID},
			wantProductScoped: true,
			wantKind:          authz.ResourceTypeProduct, wantResource: &productID,
		},
		// The repository refuses this row; the accessor still reports nothing rather than a zero id.
		{name: "a product-scoped token without its id", token: &APIToken{Scope: ToPtr(authz.ResourceTypeProduct)}, wantProductScoped: true},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tc.wantProductScoped, tc.token.IsProductScoped())
			assert.Equal(t, tc.wantOrgWide, tc.token.IsOrgWide())

			kind, id, ok := tc.token.ResourceScope()
			if tc.wantResource == nil {
				assert.False(t, ok)
				assert.Empty(t, kind)
				assert.Equal(t, uuid.Nil, id)
				return
			}

			assert.True(t, ok)
			assert.Equal(t, tc.wantKind, kind)
			assert.Equal(t, *tc.wantResource, id)
		})
	}
}

// The organization-level set is what only an organization-wide token may hold.
func TestIsOrgLevelTokenPolicy(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name   string
		policy *authz.Policy
		want   bool
	}{
		{name: "minting tokens", policy: authz.PolicyAPITokenCreate, want: true},
		{name: "listing tokens", policy: authz.PolicyAPITokenList, want: true},
		{name: "revoking tokens", policy: authz.PolicyAPITokenRevoke, want: true},
		{name: "reading registered integrations", policy: authz.PolicyRegisteredIntegrationRead, want: true},
		{name: "a default token policy", policy: authz.PolicyWorkflowRunRead, want: false},
		{name: "no policy", policy: nil, want: false},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tc.want, IsOrgLevelTokenPolicy(tc.policy))
		})
	}
}
