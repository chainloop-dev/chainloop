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

package usercontext

import (
	"context"

	"github.com/chainloop-dev/chainloop/app/controlplane/internal/usercontext/entities"
	"github.com/chainloop-dev/chainloop/pkg/otelx"
	errorsAPI "github.com/go-kratos/kratos/v2/errors"
	"github.com/go-kratos/kratos/v2/middleware"
)

var suspensionTracer = otelx.Tracer("chainloop-controlplane", "middleware/suspension")

// WithSuspensionMiddleware blocks all requests when the current organization is suspended.
// If there is no org in context (e.g. status endpoints), the request passes through.
func WithSuspensionMiddleware() middleware.Middleware {
	return func(handler middleware.Handler) middleware.Handler {
		return func(ctx context.Context, req interface{}) (interface{}, error) {
			ctx, span := otelx.Start(ctx, suspensionTracer, "WithSuspensionMiddleware")
			defer span.End()

			org := entities.CurrentOrg(ctx)
			if org == nil {
				return handler(ctx, req)
			}

			if org.Suspended {
				return nil, errorsAPI.Forbidden("suspended", "organization is suspended")
			}

			return handler(ctx, req)
		}
	}
}
