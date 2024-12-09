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

	"github.com/chainloop-dev/chainloop/app/controlplane/internal/usercontext/entities"
	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/biz"
	bizMocks "github.com/chainloop-dev/chainloop/app/controlplane/pkg/biz/mocks"
	userjwtbuilder "github.com/chainloop-dev/chainloop/app/controlplane/pkg/jwt/user"
	"github.com/go-kratos/kratos/v2/log"
	jwtmiddleware "github.com/go-kratos/kratos/v2/middleware/auth/jwt"
	"github.com/golang-jwt/jwt/v4"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

var emptyHandler = func(_ context.Context, _ interface{}) (interface{}, error) { return nil, nil }

func TestWithCurrentUserMiddleware(t *testing.T) {
	logger := log.NewHelper(log.NewStdLogger(io.Discard))
	testCases := []struct {
		name      string
		loggedIn  bool
		audience  string
		userExist bool
		// the middleware logic got skipped
		skipped bool
		wantErr bool
	}{
		{
			name:     "invalid audience", // in this case it gets ignored
			loggedIn: true,
			audience: "another-aud",
			skipped:  true,
		},
		{
			name:      "logged in, user exists",
			loggedIn:  true,
			audience:  userjwtbuilder.Audience,
			userExist: true,
			wantErr:   false,
		},
		{
			name:      "logged in, user does not exist",
			loggedIn:  true,
			audience:  userjwtbuilder.Audience,
			userExist: false,
			wantErr:   true,
		},
		{
			name:     "not logged in",
			loggedIn: false,
			audience: userjwtbuilder.Audience,
			wantErr:  true,
		},
	}

	wantUser := &biz.User{ID: uuid.NewString()}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			usecase := bizMocks.NewUserOrgFinder(t)
			ctx := context.Background()
			if tc.loggedIn {
				c := jwt.MapClaims{
					"aud":     tc.audience,
					"user_id": wantUser.ID,
				}

				ctx = jwtmiddleware.NewContext(ctx, c)
			}

			if tc.userExist {
				usecase.On("FindByID", ctx, wantUser.ID).Return(wantUser, nil)
			} else if tc.loggedIn {
				usecase.On("FindByID", ctx, wantUser.ID).Maybe().Return(nil, nil)
			}

			m := WithCurrentUserMiddleware(usecase, logger)
			_, err := m(
				func(ctx context.Context, _ interface{}) (interface{}, error) {
					if tc.wantErr {
						return nil, nil
					}

					if !tc.skipped {
						// Check that the wrapped handler contains the user
						assert.Equal(t, entities.CurrentUser(ctx).ID, wantUser.ID)
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
