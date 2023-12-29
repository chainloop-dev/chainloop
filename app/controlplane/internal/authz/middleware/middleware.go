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

package middleware

import (
	"context"

	"github.com/go-kratos/kratos/v2/errors"

	"github.com/chainloop-dev/chainloop/app/controlplane/internal/authz"
	"github.com/chainloop-dev/chainloop/app/controlplane/internal/usercontext"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/go-kratos/kratos/v2/middleware"
	"github.com/go-kratos/kratos/v2/transport"
)

// Check API token authorization
// If the token is not available we do nothing
func WithCurrentAPITokenAuthzMiddleware(enforcer *authz.Enforcer, logger *log.Helper) middleware.Middleware {
	return func(handler middleware.Handler) middleware.Handler {
		return func(ctx context.Context, req interface{}) (interface{}, error) {
			token := usercontext.CurrentAPIToken(ctx)
			// We do nothing if the token is not available
			if token == nil {
				return handler(ctx, req)
			}

			// 1 - Check that the current operation is in the server operations ACL map
			t, ok := transport.FromServerContext(ctx)
			if !ok {
				return nil, errors.InternalServer("invalid request", "could not get transport from context")
			}

			apiOperation := t.Operation()
			if apiOperation == "" {
				return nil, errors.InternalServer("invalid request", "could not find API request")
			}

			subject := authz.SubjectAPIToken{ID: token.ID}

			logger.Infow("msg", "[authZ] checking authorization", "sub", subject.String(), "operation", apiOperation)

			// 2 - check that the API operation is mapped, return forbidden if not
			policies, ok := serverOperations[apiOperation]
			if !ok {
				return nil, errors.Forbidden("forbidden", "operation not allowed")
			}

			// 3 - Ask enforcer if the token meets all the policies defined in the map
			for _, p := range policies {
				ok, err := enforcer.Enforce(subject.String(), p.Resource, p.Action)
				if err != nil {
					return nil, errors.InternalServer("internal error", err.Error())
				}

				if !ok {
					logger.Infow("msg", "[authZ] policy not found", "sub", subject.String(), "operation", apiOperation, "resource", p.Resource, "action", p.Action)
					return nil, errors.Forbidden("forbidden", "operation not allowed")
				}
			}

			return handler(ctx, req)
		}
	}
}

// Contains a map of server operations to the ResourceAction tuples that are
// required to perform the operation
// If it contains more than one, a single match will suffice
type ServerOperationMap map[string][]*authz.Policy

// serverOperations is a map of server operations to the resources and actions
// that are required to perform the operation
var serverOperations = ServerOperationMap{
	// Workflow Contracts
	"/controlplane.v1.WorkflowContractService/List":     {authz.PolicyWorkflowContractList},
	"/controlplane.v1.WorkflowContractService/Describe": {authz.PolicyWorkflowContractRead},
	"/controlplane.v1.WorkflowContractService/Update":   {authz.PolicyWorkflowContractUpdate},
	"/controlplane.v1.CASCredentialsService/Get":        {authz.PolicyArtifactDownload},
	"/controlplane.v1.ReferrerService/DiscoverPrivate":  {authz.PolicyReferrerRead},
}
