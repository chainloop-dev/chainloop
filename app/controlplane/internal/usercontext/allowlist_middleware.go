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
	"strings"

	v1 "github.com/chainloop-dev/chainloop/app/controlplane/api/controlplane/v1"
	conf "github.com/chainloop-dev/chainloop/app/controlplane/internal/conf/controlplane/config/v1"
	"github.com/go-kratos/kratos/v2/middleware"
	"github.com/go-kratos/kratos/v2/transport"
)

// Middleware that checks that the user is defined in the allow list
func CheckUserInAllowList(allowList *conf.Auth_AllowList) middleware.Middleware {
	return func(handler middleware.Handler) middleware.Handler {
		return func(ctx context.Context, req interface{}) (interface{}, error) {
			// Allowlist disabled
			if allowList == nil || len(allowList.GetRules()) == 0 {
				return handler(ctx, req)
			}

			// Make sure that this middleware is ran after WithCurrentUser
			user := CurrentUser(ctx)
			if user == nil {
				return nil, v1.ErrorAllowListErrorNotInList("user not found")
			}

			// Skip if we have explicitly set some routes and the current request is not part of them
			if len(allowList.GetSelectedRoutes()) > 0 && !selectedRoute(ctx, allowList.GetSelectedRoutes()) {
				return handler(ctx, req)
			}

			// If there are not items in the allowList we allow all users
			allow, err := inAllowList(allowList.GetRules(), user.Email)
			if err != nil {
				return nil, v1.ErrorAllowListErrorNotInList("error checking user in allowList: %v", err)
			}

			if !allow {
				msg := fmt.Sprint("user %q not in the allowList", user.Email)
				if allowList.GetCustomMessage() != "" {
					msg = allowList.GetCustomMessage()
				}

				return nil, v1.ErrorAllowListErrorNotInList(msg)
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

func inAllowList(allowList []string, email string) (bool, error) {
	for _, allowListEntry := range allowList {
		// it's a direct email match
		if allowListEntry == email {
			return true, nil
		}

		// Check if the entry is a domain and the email is part of it
		// extract the domain from the allowList entry
		// i.e if the entry is @cyberdyne.io, we get cyberdyne.io
		domainComponent := strings.Split(allowListEntry, "@")
		if len(domainComponent) != 2 {
			return false, fmt.Errorf("invalid domain entry: %q", allowListEntry)
		}

		// it's not a domain since it contains an username, then continue
		if domainComponent[0] != "" {
			continue
		}

		// Compare the domains
		emailComponents := strings.Split(email, "@")
		if len(emailComponents) != 2 {
			return false, fmt.Errorf("invalid email: %q", email)
		}

		// check if against a potential domain entry in the allowList
		if emailComponents[1] == domainComponent[1] {
			return true, nil
		}
	}

	return false, nil
}
