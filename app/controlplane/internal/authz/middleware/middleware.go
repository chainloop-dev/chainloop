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
	"errors"
	"regexp"

	errorsAPI "github.com/go-kratos/kratos/v2/errors"

	"github.com/chainloop-dev/chainloop/app/controlplane/internal/authz"
	"github.com/chainloop-dev/chainloop/app/controlplane/internal/usercontext"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/go-kratos/kratos/v2/middleware"
	"github.com/go-kratos/kratos/v2/transport"
)

type Enforcer interface {
	Enforce(sub string, p *authz.Policy) (bool, error)
}

// Check Authorization for the current API operation against the current user/token
func WithAuthzMiddleware(enforcer Enforcer, logger *log.Helper) middleware.Middleware {
	return func(handler middleware.Handler) middleware.Handler {
		return func(ctx context.Context, req interface{}) (interface{}, error) {
			// Load the authorization subject from the context which might be related to a currentUser or an APItoken
			subject := usercontext.CurrentAuthzSubject(ctx)
			if subject == "" {
				return nil, errorsAPI.Forbidden("forbidden", "missing authentication")
			}

			// Load the API operation from the context
			t, ok := transport.FromServerContext(ctx)
			if !ok {
				return nil, errorsAPI.InternalServer("invalid request", "could not get transport from context")
			}

			apiOperation := t.Operation()
			if apiOperation == "" {
				return nil, errorsAPI.InternalServer("invalid request", "could not find API request")
			}

			// We do not have all the policies related to an admin user defined yet
			// so for now we skip the authorization check for admin users since they are allowed to do anything
			// TODO: fillout the rest of the policies in authz.ServerOperationsMap and remove this check
			if subject == string(authz.RoleAdmin) {
				logger.Infow("msg", "[authZ] skipped", "sub", subject, "operation", apiOperation)
				return handler(ctx, req)
			}

			// Check the policies for the current API operation
			if err := checkPolicies(subject, apiOperation, enforcer, logger); err != nil {
				return nil, err
			}

			return handler(ctx, req)
		}
	}
}

func checkPolicies(subject, apiOperation string, enforcer Enforcer, logger *log.Helper) error {
	logger.Infow("msg", "[authZ] checking authorization", "sub", subject, "operation", apiOperation)
	// If there is no entry in the map for this API operation, we deny access
	policies, err := policiesLookup(apiOperation)
	if err != nil {
		return errorsAPI.Forbidden("forbidden", err.Error())
	}

	// Ask AuthZ enforcer if the token meets all the policies defined in the map
	for _, p := range policies {
		ok, err := enforcer.Enforce(subject, p)
		if err != nil {
			return errorsAPI.InternalServer("internal error", err.Error())
		}

		if !ok {
			logger.Infow("msg", "[authZ] policy not found", "sub", subject, "operation", apiOperation, "resource", p.Resource, "action", p.Action)
			return errorsAPI.Forbidden("forbidden", "operation not allowed")
		}
	}

	return nil
}

// policiesLookup returns the policies required for a given API operation
// it performs a two run lookup
// 1 - It checks if there is an entry in the map
// 2 - if there is not, it runs a regex match in each key in case one of those keys contains a regex
func policiesLookup(apiOperation string) ([]*authz.Policy, error) {
	// Direct match
	policies, found := authz.ServerOperationsMap[apiOperation]
	if found {
		return policies, nil
	}

	// second pass trying to match a regex
	// i.e "/controlplane.v1.OrgMetricsService/.*" -> "/controlplane.v1.OrgMetricsService/Totals"
	for k, policies := range authz.ServerOperationsMap {
		found, _ := regexp.MatchString(k, apiOperation)
		if found {
			return policies, nil
		}
	}

	return nil, errors.New("operation not allowed")
}
