//
// Copyright 2026 The Chainloop Authors.
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

package v1

import (
	"context"
	"errors"

	"google.golang.org/grpc"
)

// ErrDiscoverScopeNotReported is returned when a control plane does not report that it limited
// the discovery to the current organization. With a user session, older control planes search
// every organization of the user, so their results must not be trusted.
var ErrDiscoverScopeNotReported = errors.New("the control plane did not limit the discovery to the current organization, upgrade the control plane")

// DiscoverScopeUnaryClientInterceptor fails closed on discover responses that do not report the
// single-organization scope. Add it only to connections that authenticate with a user session:
// older control planes already limit the discovery of API tokens to the organization of the token.
func DiscoverScopeUnaryClientInterceptor() grpc.UnaryClientInterceptor {
	return func(ctx context.Context, method string, req, reply any, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
		if err := invoker(ctx, method, req, reply, cc, opts...); err != nil {
			return err
		}

		if resp, ok := reply.(*ReferrerServiceDiscoverPrivateResponse); ok && !resp.GetCurrentOrgScoped() {
			return ErrDiscoverScopeNotReported
		}

		return nil
	}
}
