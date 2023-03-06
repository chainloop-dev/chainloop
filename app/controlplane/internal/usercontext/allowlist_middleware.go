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

	v1 "github.com/chainloop-dev/bedrock/app/controlplane/api/controlplane/v1"
	"github.com/go-kratos/kratos/v2/middleware"
)

// Middleware that checks that the user is defined in the allow list
func CheckUserInAllowList(allowList []string) middleware.Middleware {
	return func(handler middleware.Handler) middleware.Handler {
		return func(ctx context.Context, req interface{}) (interface{}, error) {
			if len(allowList) == 0 {
				return handler(ctx, req)
			}

			// Make sure that this middleware is ran after WithCurrentUser
			user := CurrentUser(ctx)
			if user == nil {
				return nil, v1.ErrorAllowListErrorNotInList("user not found")
			}

			// If there are not items in the allowList we allow all users
			var allow bool
			for _, e := range allowList {
				if e == user.Email {
					allow = true
					break
				}
			}

			if !allow {
				return nil, v1.ErrorAllowListErrorNotInList("user %q not in the allowList", user.Email)
			}

			return handler(ctx, req)
		}
	}
}
