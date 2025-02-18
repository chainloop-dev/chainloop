//
// Copyright 2025 The Chainloop Authors.
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
	"errors"
	"fmt"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/golang-jwt/jwt/v4"

	"github.com/chainloop-dev/chainloop/app/controlplane/internal/usercontext/attjwtmiddleware"
	"github.com/chainloop-dev/chainloop/app/controlplane/internal/usercontext/entities"
	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/biz"
	"github.com/go-kratos/kratos/v2/middleware"
)

func WithAttestationContextFromFederatedInfo(orgUC *biz.OrganizationUseCase, logger *log.Helper) middleware.Middleware {
	return func(handler middleware.Handler) middleware.Handler {
		return func(ctx context.Context, req interface{}) (interface{}, error) {
			authInfo, ok := attjwtmiddleware.FromJWTAuthContext(ctx)
			// If not found means that there is no currentUser set in the context
			if !ok {
				logger.Warn("couldn't extract org/user, JWT parser middleware not running before this one?")
				return nil, errors.New("can't extract JWT info from the context")
			}

			// If the token is not an API token, we don't need to do anything
			if authInfo.ProviderKey != attjwtmiddleware.FederatedProviderKey {
				return handler(ctx, req)
			}

			claims, ok := authInfo.Claims.(*jwt.MapClaims)
			if !ok {
				return nil, errors.New("error mapping the claims")
			}

			orgID := (*claims)["orgId"].(string)

			ctx = withRobotAccount(ctx, &RobotAccount{OrgID: orgID, ProviderKey: attjwtmiddleware.FederatedProviderKey})
			// Find the associated organization
			org, err := orgUC.FindByID(ctx, orgID)
			if err != nil {
				return nil, fmt.Errorf("error retrieving the organization: %w", err)
			} else if org == nil {
				return nil, errors.New("organization not found")
			}

			// Set the current organization and API-Token in the context
			ctx = entities.WithCurrentOrg(ctx, &entities.Org{Name: org.Name, ID: org.ID, CreatedAt: org.CreatedAt})
			logger.Infow("msg", "[authN] processed credentials", "type", "Federated delegation")

			return handler(ctx, req)
		}
	}
}
