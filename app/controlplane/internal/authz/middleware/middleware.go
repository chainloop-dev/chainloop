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

package middleware

import (
	"context"
	"fmt"

	"github.com/go-kratos/kratos/v2/errors"

	"github.com/chainloop-dev/chainloop/app/controlplane/internal/authz"
	"github.com/chainloop-dev/chainloop/app/controlplane/internal/usercontext"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/go-kratos/kratos/v2/middleware"
	"github.com/go-kratos/kratos/v2/transport"
)

type Enforcer interface {
	Enforce(...interface{}) (bool, error)
}

// Check Authorization for the current API operation against the current user/token
func WithAuthzMiddleware(enforcer Enforcer, logger *log.Helper) middleware.Middleware {
	return func(handler middleware.Handler) middleware.Handler {
		return func(ctx context.Context, req interface{}) (interface{}, error) {
			// Load the API operation from the context
			t, ok := transport.FromServerContext(ctx)
			if !ok {
				return nil, errors.InternalServer("invalid request", "could not get transport from context")
			}

			apiOperation := t.Operation()
			if apiOperation == "" {
				return nil, errors.InternalServer("invalid request", "could not find API request")
			}

			var authzSubject string

			// Currently authz is only implemented for API tokens
			// we skip it if the currentUser is represented by a user
			if user := usercontext.CurrentUser(ctx); user != nil {
				currentOrg := usercontext.CurrentOrg(ctx)
				fmt.Println(currentOrg.MembershipRole)
				// TODO: load the subject from the membership
			} else if token := usercontext.CurrentAPIToken(ctx); token != nil {
				subjectAPIToken := authz.SubjectAPIToken{ID: token.ID}
				authzSubject = subjectAPIToken.String()
			} else {
				return nil, errors.Forbidden("forbidden", "missing auth")
			}

			// For now we can skip the check if the subject is empty
			// TODO: change once we enable authz for users
			if authzSubject != "" {
				if err := checkPolicies(authzSubject, apiOperation, enforcer, logger); err != nil {
					return nil, err
				}
			}

			return handler(ctx, req)
		}
	}
}

func checkPolicies(subject, apiOperation string, enforcer Enforcer, logger *log.Helper) error {
	logger.Infow("msg", "[authZ] checking authorization", "sub", subject, "operation", apiOperation)
	// If there is no entry in the map for this API operation, we deny access
	policies, ok := authz.ServerOperationsMap[apiOperation]
	if !ok {
		return errors.Forbidden("forbidden", "operation not allowed")
	}

	// Ask AuthZ enforcer if the token meets all the policies defined in the map
	for _, p := range policies {
		ok, err := enforcer.Enforce(subject, p.Resource, p.Action)
		if err != nil {
			return errors.InternalServer("internal error", err.Error())
		}

		if !ok {
			logger.Infow("msg", "[authZ] policy not found", "sub", subject, "operation", apiOperation, "resource", p.Resource, "action", p.Action)
			return errors.Forbidden("forbidden", "operation not allowed")
		}
	}

	return nil
}
