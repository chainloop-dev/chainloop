//
// Copyright 2023 The Chainloop Authors.
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

	"github.com/chainloop-dev/chainloop/app/controlplane/internal/biz"
	bizMocks "github.com/chainloop-dev/chainloop/app/controlplane/internal/biz/mocks"
	"github.com/chainloop-dev/chainloop/app/controlplane/internal/jwt/user"
	userjwtbuilder "github.com/chainloop-dev/chainloop/app/controlplane/internal/jwt/user"
	"github.com/go-kratos/kratos/v2/log"
	jwtmiddleware "github.com/go-kratos/kratos/v2/middleware/auth/jwt"
	"github.com/golang-jwt/jwt/v4"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

var emptyHandler = func(_ context.Context, _ interface{}) (interface{}, error) { return nil, nil }

func TestWithCurrentUserAndOrgMiddleware(t *testing.T) {
	logger := log.NewHelper(log.NewStdLogger(io.Discard))
	testCases := []struct {
		name      string
		loggedIn  bool
		audience  string
		userExist bool
		orgExist  bool
		wantErr   bool
	}{
		{
			name:     "invalid audience",
			loggedIn: true,
			audience: "another-aud",
			wantErr:  true,
		},
		{
			name:      "logged in, user and org exists",
			loggedIn:  true,
			audience:  user.Audience,
			userExist: true,
			orgExist:  true,
			wantErr:   false,
		},
		{
			name:      "logged in, user does not exist",
			loggedIn:  true,
			audience:  user.Audience,
			userExist: false,
			wantErr:   true,
		},
		{
			name:      "logged in, org does not exist",
			loggedIn:  true,
			audience:  user.Audience,
			userExist: true,
			wantErr:   true,
		},
		{
			name:     "not logged in",
			loggedIn: false,
			audience: user.Audience,
			wantErr:  true,
		},
	}

	wantUser := &biz.User{ID: uuid.NewString()}
	wantOrg := &biz.Organization{ID: uuid.NewString()}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			usecase := bizMocks.NewUserOrgFinder(t)
			ctx := context.Background()
			if tc.loggedIn {
				ctx = jwtmiddleware.NewContext(ctx, &userjwtbuilder.CustomClaims{
					UserID: wantUser.ID,
					RegisteredClaims: jwt.RegisteredClaims{
						Audience: jwt.ClaimStrings{tc.audience},
					},
				})
			}

			if tc.userExist {
				usecase.On("FindByID", ctx, wantUser.ID).Return(wantUser, nil)
			} else if tc.loggedIn {
				usecase.On("FindByID", ctx, wantUser.ID).Maybe().Return(nil, nil)
			}

			if tc.orgExist {
				usecase.On("CurrentOrg", ctx, wantUser.ID).Return(wantOrg, nil)
			} else if tc.loggedIn {
				usecase.On("CurrentOrg", ctx, wantUser.ID).Maybe().Return(nil, nil)
			}

			m := WithCurrentUserAndOrgMiddleware(usecase, logger)
			_, err := m(
				func(ctx context.Context, _ interface{}) (interface{}, error) {
					if tc.wantErr {
						return nil, nil
					}

					// Check that the wrapped handler contains the user and org
					assert.Equal(t, CurrentOrg(ctx).ID, wantOrg.ID)
					assert.Equal(t, CurrentUser(ctx).ID, wantUser.ID)

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
