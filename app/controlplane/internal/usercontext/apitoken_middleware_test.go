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
	"fmt"
	"io"
	"testing"
	"time"

	conf "github.com/chainloop-dev/chainloop/app/controlplane/internal/conf/controlplane/config/v1"
	"github.com/chainloop-dev/chainloop/app/controlplane/internal/jwt/apitoken"
	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/biz"
	bizMocks "github.com/chainloop-dev/chainloop/app/controlplane/pkg/biz/mocks"
	"github.com/go-kratos/kratos/v2/log"
	jwtmiddleware "github.com/go-kratos/kratos/v2/middleware/auth/jwt"
	"github.com/golang-jwt/jwt/v4"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWithCurrentAPITokenAndOrgMiddleware(t *testing.T) {
	logger := log.NewHelper(log.NewStdLogger(io.Discard))
	testCases := []struct {
		name          string
		receivedToken bool
		audience      string
		tokenExists   bool
		tokenRevoked  bool
		orgExist      bool
		// the middleware logic got skipped
		skipped bool
		wantErr bool
	}{
		{
			name:          "invalid audience", // in this case it gets ignored
			receivedToken: true,
			audience:      "another-aud",
			skipped:       true,
		},
		{
			name:          "token and org exists",
			receivedToken: true,
			audience:      apitoken.Audience,
			tokenExists:   true,
			orgExist:      true,
			wantErr:       false,
		},
		{
			name:          "token revoked",
			receivedToken: true,
			audience:      apitoken.Audience,
			tokenExists:   true,
			tokenRevoked:  true,
			wantErr:       true,
		},
		{
			name:          "token does not exist",
			receivedToken: true,
			audience:      apitoken.Audience,
			tokenExists:   false,
			wantErr:       true,
		},
		{
			name:          "org does not exist",
			receivedToken: true,
			audience:      apitoken.Audience,
			tokenExists:   true,
			wantErr:       true,
		},
		{
			name:          "no token received",
			receivedToken: false,
			audience:      apitoken.Audience,
			wantErr:       true,
		},
	}

	for _, tc := range testCases {
		wantOrgID := uuid.New()
		wantOrg := &biz.Organization{ID: wantOrgID.String()}
		wantToken := &biz.APIToken{ID: uuid.New(), OrganizationID: wantOrgID}

		t.Run(tc.name, func(t *testing.T) {
			apiTokenRepo := bizMocks.NewAPITokenRepo(t)
			orgRepo := bizMocks.NewOrganizationRepo(t)
			apiTokenUC, err := biz.NewAPITokenUseCase(apiTokenRepo, &conf.Auth{GeneratedJwsHmacSecret: "test"}, nil, nil)
			require.NoError(t, err)
			orgUC := biz.NewOrganizationUseCase(orgRepo, nil, nil, nil, nil)
			require.NoError(t, err)

			ctx := context.Background()
			if tc.receivedToken {
				c := jwt.MapClaims{
					"aud": tc.audience,
					"jti": wantToken.ID.String(),
				}

				ctx = jwtmiddleware.NewContext(ctx, c)
			}

			if tc.tokenExists {
				if tc.tokenRevoked {
					wantToken.RevokedAt = toTimePtr(time.Now())
				}

				apiTokenRepo.On("FindByID", ctx, wantToken.ID).Return(wantToken, nil)
			} else if tc.receivedToken {
				apiTokenRepo.On("FindByID", ctx, wantToken.ID).Maybe().Return(nil, nil)
			}

			if tc.orgExist {
				orgRepo.On("FindByID", ctx, wantOrgID).Return(wantOrg, nil)
			} else if tc.receivedToken {
				orgRepo.On("FindByID", ctx, wantOrgID).Maybe().Return(nil, nil)
			}

			m := WithCurrentAPITokenAndOrgMiddleware(apiTokenUC, orgUC, logger)
			_, err = m(
				func(ctx context.Context, _ interface{}) (interface{}, error) {
					if tc.wantErr {
						return nil, nil
					}

					if !tc.skipped {
						// Check that the wrapped handler contains the user and org
						assert.Equal(t, CurrentOrg(ctx).ID, wantOrg.ID)
						assert.Equal(t, CurrentAPIToken(ctx).ID, wantToken.ID.String())
						assert.Equal(t, CurrentAuthzSubject(ctx), fmt.Sprintf("api-token:%s", wantToken.ID))
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
