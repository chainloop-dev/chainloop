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

package usercontext

import (
	"context"
	"io"
	"testing"
	"time"

	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/authz"
	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/biz"
	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/biz/testhelpers"
	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/usercontext/entities"
	"github.com/go-kratos/kratos/v2/log"
	jwtmiddleware "github.com/go-kratos/kratos/v2/middleware/auth/jwt"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A token row from before the scope columns existed reaches the service layer naming its scope,
// derived from its organization and project when the row is read.
func TestAPITokenMiddlewareCarriesTheScopeOfLegacyRows(t *testing.T) {
	if !testhelpers.IntegrationTestsEnabled() {
		t.Skip()
	}

	tu := testhelpers.NewTestingUseCases(t)
	defer tu.DB.Close(t)

	ctx := context.Background()
	logger := log.NewHelper(log.NewStdLogger(io.Discard))

	org, err := tu.Organization.CreateWithRandomName(ctx)
	require.NoError(t, err)
	orgID := uuid.MustParse(org.ID)
	project, err := tu.Project.Create(ctx, org.ID, "legacy")
	require.NoError(t, err)

	testCases := []struct {
		name        string
		org         *uuid.UUID
		project     *uuid.UUID
		wantScope   authz.ResourceType
		wantScopeID *uuid.UUID
	}{
		{name: "an organization token", org: &orgID, wantScope: authz.ResourceTypeOrganization, wantScopeID: &orgID},
		{name: "a project token", org: &orgID, project: &project.ID, wantScope: authz.ResourceTypeProject, wantScopeID: &project.ID},
		{name: "an instance token", wantScope: authz.ResourceTypeInstance},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			row, err := tu.Repos.APITokenRepo.Create(ctx, &biz.APITokenCreateOpts{
				Name: "legacy-" + uuid.NewString(), OrganizationID: tc.org, ProjectID: tc.project, Policies: []*authz.Policy{},
			})
			require.NoError(t, err)

			stored, err := tu.Data.DB.APIToken.Get(ctx, row.ID)
			require.NoError(t, err)
			require.Nil(t, stored.Scope, "the row must be one from before the scope columns")

			// The JWT the control plane signs for this row, project and instance-admin claims included
			signed, err := tu.APIToken.RegenerateJWT(ctx, row.ID, time.Hour)
			require.NoError(t, err)
			claims := jwt.MapClaims{}
			_, _, err = jwt.NewParser().ParseUnverified(signed.JWT, claims)
			require.NoError(t, err)

			var got *entities.APIToken
			_, err = WithCurrentAPITokenAndOrgMiddleware(tu.APIToken, tu.Organization, logger)(
				func(ctx context.Context, _ interface{}) (interface{}, error) {
					got = entities.CurrentAPIToken(ctx)
					return nil, nil
				})(jwtmiddleware.NewContext(ctx, claims), nil)
			require.NoError(t, err)

			require.NotNil(t, got)
			require.NotNil(t, got.Scope)
			assert.Equal(t, tc.wantScope, *got.Scope)
			assert.Equal(t, tc.wantScopeID, got.ScopeID)
			assert.Equal(t, tc.project, got.ProjectID)
		})
	}
}
