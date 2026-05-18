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

package server

import (
	"context"
	nhttp "net/http"

	"github.com/go-kratos/kratos/v2/middleware"
	jwtMiddleware "github.com/go-kratos/kratos/v2/middleware/auth/jwt"

	casJWT "github.com/chainloop-dev/chainloop/internal/robotaccount/cas"
	backend "github.com/chainloop-dev/chainloop/pkg/blobmanager"
)

// withRequestingOrgFromClaims reads the CAS JWT claims (already
// verified and stashed in ctx by the JWT middleware) and stamps the
// org UUID on the context via backend.WithRequestingOrg. Managed CAS
// providers consume this to scope per-tenant STS sessions; other
// providers ignore it.
//
// A missing or empty OrgID is treated as "no managed binding" — ctx
// passes through unchanged so legacy tokens minted before the org-id
// claim was added continue to work for non-managed providers.
func withRequestingOrgFromClaims(ctx context.Context) context.Context {
	raw, ok := jwtMiddleware.FromContext(ctx)
	if !ok {
		return ctx
	}
	claims, ok := raw.(*casJWT.Claims)
	if !ok || claims.OrgID == "" {
		return ctx
	}
	return backend.WithRequestingOrg(ctx, claims.OrgID)
}

// requestingOrgMiddleware is a kratos middleware that runs after the
// JWT middleware on unary gRPC requests and enriches ctx with the
// requesting org from the verified claims.
func requestingOrgMiddleware() middleware.Middleware {
	return func(handler middleware.Handler) middleware.Handler {
		return func(ctx context.Context, req any) (any, error) {
			return handler(withRequestingOrgFromClaims(ctx), req)
		}
	}
}

// requestingOrgHTTPMiddleware wraps an HTTP handler so the request
// context carries the requesting org. Apply it BETWEEN the JWT
// middleware (which populates the claims in ctx) and the actual
// handler.
func requestingOrgHTTPMiddleware(next nhttp.Handler) nhttp.Handler {
	return nhttp.HandlerFunc(func(w nhttp.ResponseWriter, r *nhttp.Request) {
		next.ServeHTTP(w, r.WithContext(withRequestingOrgFromClaims(r.Context())))
	})
}
