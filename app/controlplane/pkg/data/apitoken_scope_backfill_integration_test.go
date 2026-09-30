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

package data_test

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/authz"
	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/biz"
	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/biz/testhelpers"
	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/data/ent"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// scopeBackfillMigration records the scope of every token from before the scope columns existed.
const scopeBackfillMigration = "ent/migrate/migrations/20260930160057.sql"

// Every row the backfill touches comes out with the scope a new token of its shape records, and
// passes the rules a new token is held to. A row that already records a scope is left alone.
func TestAPITokenScopeBackfillMigration(t *testing.T) {
	if !testhelpers.IntegrationTestsEnabled() {
		t.Skip()
	}

	tu := testhelpers.NewTestingUseCases(t)
	defer tu.DB.Close(t)

	ctx := context.Background()

	org, err := tu.Organization.CreateWithRandomName(ctx)
	require.NoError(t, err)
	orgID := uuid.MustParse(org.ID)
	project, err := tu.Project.Create(ctx, org.ID, "legacy")
	require.NoError(t, err)
	workflow, err := tu.Workflow.Create(ctx, &biz.WorkflowCreateOpts{OrgID: org.ID, Name: "legacy", Project: project.Name})
	require.NoError(t, err)
	productID := uuid.New()

	testCases := []struct {
		name string
		// row writes the columns a control plane from before the scope columns wrote
		row         func(*ent.APITokenCreate) *ent.APITokenCreate
		wantScope   authz.ResourceType
		wantScopeID *uuid.UUID
	}{
		{
			name:      "an organization token",
			row:       func(c *ent.APITokenCreate) *ent.APITokenCreate { return c.SetOrganizationID(orgID) },
			wantScope: authz.ResourceTypeOrganization, wantScopeID: &orgID,
		},
		{
			name: "a project token",
			row: func(c *ent.APITokenCreate) *ent.APITokenCreate {
				return c.SetOrganizationID(orgID).SetProjectID(project.ID)
			},
			wantScope: authz.ResourceTypeProject, wantScopeID: &project.ID,
		},
		{
			name: "a workflow-pinned token takes its project",
			row: func(c *ent.APITokenCreate) *ent.APITokenCreate {
				return c.SetOrganizationID(orgID).SetProjectID(project.ID).SetWorkflowID(workflow.ID)
			},
			wantScope: authz.ResourceTypeProject, wantScopeID: &project.ID,
		},
		{
			name:      "a token with no organization is the instance's",
			row:       func(c *ent.APITokenCreate) *ent.APITokenCreate { return c },
			wantScope: authz.ResourceTypeInstance,
		},
		{
			name: "a revoked token is backfilled too",
			row: func(c *ent.APITokenCreate) *ent.APITokenCreate {
				return c.SetOrganizationID(orgID).SetRevokedAt(time.Now())
			},
			wantScope: authz.ResourceTypeOrganization, wantScopeID: &orgID,
		},
		{
			name: "a product token keeps the scope it records",
			row: func(c *ent.APITokenCreate) *ent.APITokenCreate {
				return c.SetOrganizationID(orgID).SetScope(authz.ResourceTypeProduct).SetScopeID(productID).
					SetProjectIds([]uuid.UUID{project.ID})
			},
			wantScope: authz.ResourceTypeProduct, wantScopeID: &productID,
		},
	}

	ids := make([]uuid.UUID, len(testCases))
	for i, tc := range testCases {
		row, err := tc.row(tu.Data.DB.APIToken.Create().SetName("legacy-" + uuid.NewString())).Save(ctx)
		require.NoError(t, err)
		ids[i] = row.ID
	}

	backfill, err := os.ReadFile(scopeBackfillMigration)
	require.NoError(t, err)
	// Twice: rerunning the statement is how a row written without a scope after the migration
	// ran gets repaired, so it must leave already-backfilled rows as they are.
	for range 2 {
		_, err = tu.Data.SQLDB.ExecContext(ctx, string(backfill))
		require.NoError(t, err)
	}

	optional := func(id uuid.UUID) *uuid.UUID {
		if id == uuid.Nil {
			return nil
		}
		return &id
	}

	for i, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			stored, err := tu.Data.DB.APIToken.Get(ctx, ids[i])
			require.NoError(t, err)

			require.NotNil(t, stored.Scope)
			assert.Equal(t, tc.wantScope, *stored.Scope)
			assert.Equal(t, tc.wantScopeID, stored.ScopeID)
			assert.NoError(t, biz.ValidateTokenShape(stored.Scope, stored.ScopeID, optional(stored.OrganizationID), optional(stored.ProjectID), stored.ProjectIds))
		})
	}
}
