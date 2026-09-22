//
// Copyright 2024-2026 The Chainloop Authors.
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

	v1 "github.com/chainloop-dev/chainloop/app/controlplane/api/controlplane/v1"
	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/authz"
	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/biz"
	bizMocks "github.com/chainloop-dev/chainloop/app/controlplane/pkg/biz/mocks"
	userjwtbuilder "github.com/chainloop-dev/chainloop/app/controlplane/pkg/jwt/user"
	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/usercontext/entities"
	"github.com/chainloop-dev/chainloop/pkg/cache"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestWithCurrentOrganizationMiddleware(t *testing.T) {
	logger := log.NewHelper(log.NewStdLogger(io.Discard))
	testCases := []struct {
		name     string
		loggedIn bool
		audience string
		orgExist bool
		// the middleware logic got skipped
		skipped bool
		wantErr bool
	}{
		{
			name:     "logged in, user and org exists",
			loggedIn: true,
			audience: userjwtbuilder.Audience,
			orgExist: true,
			wantErr:  false,
		},
		{
			name:     "logged in, org does not exist",
			loggedIn: true,
			audience: userjwtbuilder.Audience,
			wantErr:  true,
		},
	}

	wantUser := &biz.User{ID: uuid.NewString()}
	wantMembership := &biz.Membership{
		Org:  &biz.Organization{ID: uuid.NewString()},
		Role: authz.RoleViewer,
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			usecase := bizMocks.NewUserOrgFinder(t)
			ctx := context.Background()
			if tc.loggedIn {
				ctx = entities.WithCurrentUser(ctx, &entities.User{ID: wantUser.ID})
				usecase.On("FindByID", mock.Anything, wantUser.ID).Maybe().Return(nil, nil)
			}

			if tc.orgExist {
				usecase.On("CurrentMembership", mock.Anything, wantUser.ID).Return(wantMembership, nil)
			} else if tc.loggedIn {
				usecase.On("CurrentMembership", mock.Anything, wantUser.ID).Maybe().Return(nil, nil)
			}

			m := WithCurrentOrganizationMiddleware(usecase, nil, logger)
			_, err := m(
				func(ctx context.Context, _ interface{}) (interface{}, error) {
					if tc.wantErr {
						return nil, nil
					}

					if !tc.skipped {
						// Check that the wrapped handler contains the org
						assert.Equal(t, entities.CurrentOrg(ctx).ID, wantMembership.Org.ID)
						assert.Equal(t, CurrentAuthzSubject(ctx), string(authz.RoleViewer))
					}

					return nil, nil
				})(ctx, nil)

			if tc.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestGetFromResourceWorkflowContract(t *testing.T) {
	tests := []struct {
		name          string
		rawContract   []byte
		expectedOrg   string
		expectedError bool
	}{
		{
			name: "valid contract with organization in metadata",
			rawContract: []byte(`apiVersion: chainloop.dev/v1
kind: Contract
metadata:
  name: test-contract
  organization: test-org
spec:
  materials:
    - name: my-image
      type: CONTAINER_IMAGE`),
			expectedOrg:   "test-org",
			expectedError: false,
		},
		{
			name: "valid contract without organization",
			rawContract: []byte(`apiVersion: chainloop.dev/v1
kind: Contract
metadata:
  name: test-contract
spec:
  materials:
    - name: my-image
      type: CONTAINER_IMAGE`),
			expectedOrg:   "",
			expectedError: false,
		},
		{
			name: "JSON format contract with organization",
			rawContract: []byte(`{
  "apiVersion": "chainloop.dev/v1",
  "kind": "Contract",
  "metadata": {
    "name": "test-contract",
    "organization": "json-org"
  },
  "spec": {
    "materials": [
      {
        "name": "my-image",
        "type": "CONTAINER_IMAGE"
      }
    ]
  }
}`),
			expectedOrg:   "json-org",
			expectedError: false,
		},
		{
			name:          "empty raw contract",
			rawContract:   []byte{},
			expectedOrg:   "",
			expectedError: false,
		},
		{
			name:          "nil raw contract",
			rawContract:   nil,
			expectedOrg:   "",
			expectedError: false,
		},
		{
			name: "old schema format (CraftingSchema) - no organization field",
			rawContract: []byte(`schemaVersion: v1
materials:
  - name: my-image
    type: CONTAINER_IMAGE
runner:
  type: GITHUB_ACTION`),
			expectedOrg:   "",
			expectedError: false,
		},
		{
			name: "old schema format JSON - no organization field",
			rawContract: []byte(`{
  "schemaVersion": "v1",
  "materials": [
    {
      "name": "my-image",
      "type": "CONTAINER_IMAGE"
    }
  ],
  "runner": {
    "type": "GITHUB_ACTION"
  }
}`),
			expectedOrg:   "",
			expectedError: false,
		},
		{
			name: "invalid format",
			rawContract: []byte(`invalid yaml content
			this is not parseable`),
			expectedOrg:   "",
			expectedError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test with CreateRequest
			createReq := &v1.WorkflowContractServiceCreateRequest{
				RawContract: tt.rawContract,
			}

			org, err := getFromResource(createReq)
			if tt.expectedError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expectedOrg, org)
			}

			// Test with UpdateRequest
			updateReq := &v1.WorkflowContractServiceUpdateRequest{
				RawContract: tt.rawContract,
			}

			org, err = getFromResource(updateReq)
			if tt.expectedError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expectedOrg, org)
			}
		})
	}
}

// A scoped token must arrive at the service layer with its memberships loaded, through the
// same context value a user's memberships use — that is what makes the authorization path a
// token takes the one people already take.
func TestWithCurrentMembershipsMiddlewareForAPITokens(t *testing.T) {
	productID, projectID := uuid.New(), uuid.New()

	testCases := []struct {
		name string
		// scopeID set means the token row carries a resource scope
		scopeID *uuid.UUID
		// tokenMemberships is what the repository returns for the token
		tokenMemberships []*biz.Membership
		wantLoaded       bool
		wantResources    []uuid.UUID
	}{
		{
			name:    "a scoped token gets its memberships",
			scopeID: &productID,
			tokenMemberships: []*biz.Membership{{
				ResourceType: authz.ResourceTypeProject, ResourceID: projectID, Role: authz.RoleProjectAdmin,
			}},
			wantLoaded:    true,
			wantResources: []uuid.UUID{projectID},
		},
		{
			// The product was deleted, so its memberships are gone. The membership value must
			// still be present and empty: absent would read as "RBAC not applied".
			name:             "a scoped token with no memberships gets an empty set, not nothing",
			scopeID:          &productID,
			tokenMemberships: []*biz.Membership{},
			wantLoaded:       true,
			wantResources:    []uuid.UUID{},
		},
		{
			// Today's behaviour for project, organization and instance tokens: no memberships
			// and no database call at all.
			name:       "an unscoped token is left alone",
			wantLoaded: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			tokenID := uuid.New()
			membershipUC := bizMocks.NewMembershipsRBAC(t)
			if tc.scopeID != nil {
				membershipUC.On("ListAllMembershipsForAPIToken", mock.Anything, tokenID).
					Once().Return(tc.tokenMemberships, nil)
			}

			ctx := entities.WithCurrentAPIToken(context.Background(), &entities.APIToken{
				ID: tokenID.String(), Scope: toPtr(authz.ResourceTypeProduct), ScopeID: tc.scopeID,
			})

			var seen *entities.Membership
			_, err := WithCurrentMembershipsMiddleware(membershipUC, newTestMembershipsCache(t))(
				func(ctx context.Context, _ interface{}) (interface{}, error) {
					seen = entities.CurrentMembership(ctx)
					return nil, nil
				})(ctx, nil)
			require.NoError(t, err)

			if !tc.wantLoaded {
				assert.Nil(t, seen)
				return
			}

			require.NotNil(t, seen)
			assert.Equal(t, tokenID, seen.MemberID)
			assert.Equal(t, authz.MembershipTypeAPIToken, seen.MemberType)

			got := make([]uuid.UUID, 0, len(seen.Resources))
			for _, rm := range seen.Resources {
				got = append(got, rm.ResourceID)
			}
			assert.ElementsMatch(t, tc.wantResources, got)
		})
	}
}

// User ids and token ids are both UUIDs drawn from different namespaces, so an entry cached
// for one principal kind must never be served to the other.
func TestMembershipsCacheKeyIsNamespacedByMemberType(t *testing.T) {
	sharedID := uuid.New()
	cache := newTestMembershipsCache(t)

	userProject, tokenProject := uuid.New(), uuid.New()

	membershipUC := bizMocks.NewMembershipsRBAC(t)
	membershipUC.On("ListAllMembershipsForUser", mock.Anything, sharedID).Once().
		Return([]*biz.Membership{{ResourceType: authz.ResourceTypeProject, ResourceID: userProject, Role: authz.RoleProjectAdmin}}, nil)
	membershipUC.On("ListAllMembershipsForAPIToken", mock.Anything, sharedID).Once().
		Return([]*biz.Membership{{ResourceType: authz.ResourceTypeProject, ResourceID: tokenProject, Role: authz.RoleProjectAdmin}}, nil)

	resourcesFor := func(ctx context.Context) []uuid.UUID {
		var got []uuid.UUID
		_, err := WithCurrentMembershipsMiddleware(membershipUC, cache)(
			func(ctx context.Context, _ interface{}) (interface{}, error) {
				for _, rm := range entities.CurrentMembership(ctx).Resources {
					got = append(got, rm.ResourceID)
				}
				return nil, nil
			})(ctx, nil)
		require.NoError(t, err)

		return got
	}

	userCtx := entities.WithCurrentUser(context.Background(), &entities.User{ID: sharedID.String()})
	assert.Equal(t, []uuid.UUID{userProject}, resourcesFor(userCtx))

	tokenCtx := entities.WithCurrentAPIToken(context.Background(), &entities.APIToken{
		ID: sharedID.String(), Scope: toPtr(authz.ResourceTypeProduct), ScopeID: &sharedID,
	})
	assert.Equal(t, []uuid.UUID{tokenProject}, resourcesFor(tokenCtx),
		"the token must not be served the user's cached memberships")

	// And each principal's own entry is still cached: the mocks are Once().
	assert.Equal(t, []uuid.UUID{userProject}, resourcesFor(userCtx))
	assert.Equal(t, []uuid.UUID{tokenProject}, resourcesFor(tokenCtx))
}

func newTestMembershipsCache(t *testing.T) cache.Cache[*entities.Membership] {
	t.Helper()

	c, err := cache.New[*entities.Membership](cache.WithTTL(time.Minute))
	require.NoError(t, err)

	return c
}
