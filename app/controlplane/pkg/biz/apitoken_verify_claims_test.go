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
	"errors"
	"testing"

	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/authz"
	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/jwt/apitoken"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

// errScopeMismatch is the message of a row whose scope differs from the signed one.
const errScopeMismatch = "scope mismatch"

// The claims bind what a token was granted; its row may only confirm them. Whatever wrote a row
// that disagrees, the token is refused rather than widened or moved.
func TestAPITokenVerifyClaims(t *testing.T) {
	org, otherOrg := uuid.New(), uuid.New()
	project, otherProject := uuid.New(), uuid.New()
	product, otherProduct := uuid.New(), uuid.New()
	workflow := uuid.New()

	orgKind, projectKind := authz.ResourceTypeOrganization, authz.ResourceTypeProject
	productKind, instanceKind := authz.ResourceTypeProduct, authz.ResourceTypeInstance

	orgRow := &APIToken{OrganizationID: org, Scope: &orgKind, ScopeID: &org}
	projectRow := &APIToken{OrganizationID: org, ProjectID: &project, Scope: &projectKind, ScopeID: &project}
	workflowRow := &APIToken{OrganizationID: org, ProjectID: &project, WorkflowID: &workflow, Scope: &projectKind, ScopeID: &project}
	productRow := &APIToken{OrganizationID: org, Scope: &productKind, ScopeID: &product, ProjectIDs: []uuid.UUID{project}}
	instanceRow := &APIToken{Scope: &instanceKind}

	signedOrg := apitoken.CustomClaims{OrgID: org.String(), ScopeType: string(authz.ResourceTypeOrganization), ScopeID: org.String()}
	signedProject := apitoken.CustomClaims{OrgID: org.String(), ProjectID: project.String(), ScopeType: string(authz.ResourceTypeProject), ScopeID: project.String()}
	signedWorkflow := apitoken.CustomClaims{OrgID: org.String(), ProjectID: project.String(), WorkflowID: workflow.String(), ScopeType: string(authz.ResourceTypeProject), ScopeID: project.String()}
	signedProduct := apitoken.CustomClaims{OrgID: org.String(), ScopeType: string(authz.ResourceTypeProduct), ScopeID: product.String()}
	legacyOrg := apitoken.CustomClaims{OrgID: org.String()}
	legacyProject := apitoken.CustomClaims{OrgID: org.String(), ProjectID: project.String()}
	legacyWorkflow := apitoken.CustomClaims{OrgID: org.String(), ProjectID: project.String(), WorkflowID: workflow.String()}
	// widenedProjectRow is a project token's row rewritten to its whole organization
	widenedProjectRow := &APIToken{OrganizationID: org, ProjectID: &project, Scope: &orgKind, ScopeID: &org}

	testCases := []struct {
		name    string
		row     *APIToken
		claims  apitoken.CustomClaims
		wantErr string
		// mismatch marks a refusal that means the row was written wrongly: it must wrap
		// ErrAPITokenClaimsMismatch so the middleware logs it as a security event
		mismatch bool
	}{
		{name: "signed organization token", row: orgRow, claims: signedOrg},
		{name: "signed project token", row: projectRow, claims: signedProject},
		{name: "signed workflow-pinned token", row: workflowRow, claims: signedWorkflow},
		{name: "signed product token", row: productRow, claims: signedProduct},
		{name: "signed instance token", row: instanceRow, claims: apitoken.CustomClaims{Scope: authz.ScopeInstanceAdmin, ScopeType: string(authz.ResourceTypeInstance)}},
		{name: "legacy organization token", row: orgRow, claims: legacyOrg},
		{name: "legacy project token", row: projectRow, claims: legacyProject},
		{name: "legacy workflow-pinned token", row: workflowRow, claims: legacyWorkflow},
		{name: "legacy instance token", row: instanceRow, claims: apitoken.CustomClaims{Scope: authz.ScopeInstanceAdmin}},

		{name: "a row recording no scope", row: &APIToken{OrganizationID: org}, claims: legacyOrg, wantErr: "records no scope"},
		{name: "a signed token whose row's scope was cleared", row: &APIToken{OrganizationID: org}, claims: signedOrg, wantErr: "records no scope", mismatch: true},
		{name: "a product token minted before the scope claims", row: productRow, claims: legacyOrg, wantErr: "create a new one"},
		{name: "malformed scope claims", row: orgRow, claims: apitoken.CustomClaims{OrgID: org.String(), ScopeType: string(authz.ResourceTypeOrganization)}, wantErr: "scope claims", mismatch: true},
		{name: "an instance-admin claim on an organization row", row: orgRow, claims: apitoken.CustomClaims{OrgID: org.String(), Scope: authz.ScopeInstanceAdmin}, wantErr: "scope claims", mismatch: true},
		{name: "a project row widened to its organization, signed claims", row: widenedProjectRow, claims: signedProject, wantErr: errScopeMismatch, mismatch: true},
		{name: "a project row widened to its organization, legacy claims", row: widenedProjectRow, claims: legacyProject, wantErr: errScopeMismatch, mismatch: true},
		{name: "an organization row recording no scope id", row: &APIToken{OrganizationID: org, Scope: &orgKind}, claims: signedOrg, wantErr: errScopeMismatch, mismatch: true},
		{name: "an organization row made an instance row, signed claims", row: &APIToken{OrganizationID: org, Scope: &instanceKind}, claims: signedOrg, wantErr: errScopeMismatch, mismatch: true},
		{name: "an organization row made an instance row, legacy claims", row: &APIToken{OrganizationID: org, Scope: &instanceKind}, claims: legacyOrg, wantErr: errScopeMismatch, mismatch: true},
		{name: "a project row moved to another project", row: &APIToken{OrganizationID: org, ProjectID: &project, Scope: &projectKind, ScopeID: &otherProject}, claims: signedProject, wantErr: errScopeMismatch, mismatch: true},
		{name: "a product row moved to another product", row: &APIToken{OrganizationID: org, Scope: &productKind, ScopeID: &otherProduct}, claims: signedProduct, wantErr: errScopeMismatch, mismatch: true},
		{name: "an organization row moved to another organization", row: &APIToken{OrganizationID: otherOrg, Scope: &orgKind, ScopeID: &otherOrg}, claims: legacyOrg, wantErr: errScopeMismatch, mismatch: true},
		{name: "a product row moved to another organization", row: &APIToken{OrganizationID: otherOrg, Scope: &productKind, ScopeID: &product}, claims: signedProduct, wantErr: "organization mismatch", mismatch: true},
		{name: "the project claim disagrees with the row's project column", row: &APIToken{OrganizationID: org, ProjectID: &otherProject, Scope: &projectKind, ScopeID: &project}, claims: signedProject, wantErr: "project mismatch", mismatch: true},
		{name: "a workflow claim on a row with no workflow", row: projectRow, claims: signedWorkflow, wantErr: "workflow mismatch", mismatch: true},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.row.VerifyClaims(&tc.claims)
			if tc.wantErr == "" {
				assert.NoError(t, err)
				return
			}

			assert.ErrorContains(t, err, tc.wantErr)
			assert.Equal(t, tc.mismatch, errors.Is(err, ErrAPITokenClaimsMismatch))
		})
	}

	t.Run("no token", func(t *testing.T) {
		var missing *APIToken
		assert.ErrorContains(t, missing.VerifyClaims(&legacyOrg), "not found")
	})

	t.Run("no claims", func(t *testing.T) {
		assert.ErrorContains(t, orgRow.VerifyClaims(nil), "no claims")
	})
}
