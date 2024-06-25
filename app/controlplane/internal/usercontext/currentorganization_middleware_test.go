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

package usercontext

import (
	"context"
	"io"
	"testing"

	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/authz"
	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/biz"
	bizMocks "github.com/chainloop-dev/chainloop/app/controlplane/pkg/biz/mocks"
	userjwtbuilder "github.com/chainloop-dev/chainloop/app/controlplane/pkg/jwt/user"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
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
				ctx = WithCurrentUser(ctx, &User{ID: wantUser.ID})
			}

			if tc.loggedIn {
				usecase.On("FindByID", ctx, wantUser.ID).Maybe().Return(nil, nil)
			}

			if tc.orgExist {
				usecase.On("CurrentMembership", ctx, wantUser.ID).Return(wantMembership, nil)
			} else if tc.loggedIn {
				usecase.On("CurrentMembership", ctx, wantUser.ID).Maybe().Return(nil, nil)
			}

			m := WithCurrentOrganizationMiddleware(usecase, logger)
			_, err := m(
				func(ctx context.Context, _ interface{}) (interface{}, error) {
					if tc.wantErr {
						return nil, nil
					}

					if !tc.skipped {
						// Check that the wrapped handler contains the org
						assert.Equal(t, CurrentOrg(ctx).ID, wantMembership.Org.ID)
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
