//
// Copyright 2024-2025 The Chainloop Authors.
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

	v1 "github.com/chainloop-dev/chainloop/app/controlplane/api/controlplane/v1"
	"github.com/chainloop-dev/chainloop/app/controlplane/internal/usercontext/entities"
	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/biz"
	conf "github.com/chainloop-dev/chainloop/app/controlplane/pkg/conf/controlplane/config/v1"

	"github.com/go-kratos/kratos/v2/middleware"
	"github.com/go-kratos/kratos/v2/transport"
)

// Middleware that checks that the user has access to the current route
// Note that the source of truth is in the end the property set in the DB
// The value in the allowlist conf setting is used as a starting point to populate the property in the DB
func CheckUserHasAccess(allowList *conf.AllowList, userUC biz.UserOrgFinder) middleware.Middleware {
	return func(handler middleware.Handler) middleware.Handler {
		return func(ctx context.Context, req interface{}) (interface{}, error) {
			// API tokens skip the allowlist since they are meant to represent a service
			if token := entities.CurrentAPIToken(ctx); token != nil {
				return handler(ctx, req)
			}

			// Make sure that this middleware is ran after WithCurrentUser
			user := entities.CurrentUser(ctx)
			if user == nil {
				return nil, v1.ErrorAllowListErrorNotInList("user not found")
			}

			// Skip if we have explicitly set some routes and the current request is not part of them
			if allowList != nil && len(allowList.GetSelectedRoutes()) > 0 && !selectedRoute(ctx, allowList.GetSelectedRoutes()) {
				return handler(ctx, req)
			}

			// Load the user from the DB to check the restricted access flag
			userFromDB, err := userUC.FindByID(ctx, user.ID)
			if err != nil {
				return nil, v1.ErrorAllowListErrorNotInList("error loading user: %v", err)
			}

			// If the user doesn't have the setting yet or it's set to true, we need to block the access
			if userFromDB.HasRestrictedAccess == nil || *userFromDB.HasRestrictedAccess {
				msg := fmt.Sprintf("user %q not in the allowList", user.Email)
				if allowList.GetCustomMessage() != "" {
					msg = allowList.GetCustomMessage()
				}

				return nil, v1.ErrorAllowListErrorNotInList("%s", msg)
			}

			return handler(ctx, req)
		}
	}
}

func selectedRoute(ctx context.Context, selectedRoutes []string) bool {
	if info, ok := transport.FromServerContext(ctx); ok {
		for _, route := range selectedRoutes {
			if info.Operation() == route {
				return true
			}
		}
	}

	return false
}
