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

// The token the service layer sees carries exactly the scope its row records, once the row has
// confirmed the claims the token was signed with.
func TestAPITokenMiddlewareCarriesTheRowScope(t *testing.T) {
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
	project, err := tu.Project.Create(ctx, org.ID, "scoped")
	require.NoError(t, err)

	testCases := []struct {
		name      string
		opts      biz.APITokenCreateOpts
		wantErr   bool
		wantScope *authz.ResourceType
		// wantOrgScoped, wantInstanceScoped and wantReach are what the service layer reads off the token
		wantOrgScoped      bool
		wantInstanceScoped bool
		wantReach          []uuid.UUID
	}{
		{
			name:      "an organization row",
			opts:      biz.APITokenCreateOpts{OrganizationID: &orgID, Scope: biz.ToPtr(authz.ResourceTypeOrganization), ScopeID: &orgID},
			wantScope: biz.ToPtr(authz.ResourceTypeOrganization), wantOrgScoped: true,
		},
		{
			name:      "a project row",
			opts:      biz.APITokenCreateOpts{OrganizationID: &orgID, ProjectID: &project.ID, Scope: biz.ToPtr(authz.ResourceTypeProject), ScopeID: &project.ID},
			wantScope: biz.ToPtr(authz.ResourceTypeProject), wantReach: []uuid.UUID{project.ID},
		},
		{
			// No organization of its own and no org header: the org context stays unset, and the
			// token acts for the whole instance
			name:      "an instance row",
			opts:      biz.APITokenCreateOpts{Scope: biz.ToPtr(authz.ResourceTypeInstance)},
			wantScope: biz.ToPtr(authz.ResourceTypeInstance), wantInstanceScoped: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			opts := tc.opts
			opts.Name, opts.Policies = "row-"+uuid.NewString(), []*authz.Policy{}
			row, err := tu.Repos.APITokenRepo.Create(ctx, &opts)
			require.NoError(t, err)

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
			if tc.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.NotNil(t, got)

			assert.Equal(t, tc.wantScope, got.Scope)
			assert.Equal(t, opts.ScopeID, got.ScopeID)
			assert.Equal(t, opts.ProjectID, got.ProjectID)
			assert.Equal(t, tc.wantOrgScoped, got.IsOrgScoped())
			assert.Equal(t, tc.wantInstanceScoped, got.IsInstanceScoped())
			if tc.wantOrgScoped || tc.wantInstanceScoped {
				assert.Nil(t, got.ReachableProjects())
				return
			}
			assert.Equal(t, tc.wantReach, got.ReachableProjects())
		})
	}
}
