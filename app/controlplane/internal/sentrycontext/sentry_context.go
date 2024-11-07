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

package sentrycontext

import (
	"context"
	"fmt"

	"github.com/chainloop-dev/chainloop/app/controlplane/internal/usercontext"

	"github.com/getsentry/sentry-go"
	"github.com/go-kratos/kratos/v2/middleware"
	"github.com/go-kratos/kratos/v2/middleware/logging"
	"github.com/go-kratos/kratos/v2/transport"
	"google.golang.org/grpc/metadata"
)

// defaultTracingHeaders is a list of headers that are used to extract tracing information.
var defaultTracingHeaders = []string{"X-Request-ID", "X-Correlation-ID", "X-Trace-ID"}

// NewSentryContext returns a middleware that adds context to Sentry for the current request
// that will be sent along with the error if one occurs
func NewSentryContext() middleware.Middleware {
	return func(handler middleware.Handler) middleware.Handler {
		return func(ctx context.Context, req interface{}) (interface{}, error) {
			addSentryContext(ctx, req)
			return handler(ctx, req)
		}
	}
}

// addSentryContext adds context to Sentry for the current request
func addSentryContext(ctx context.Context, req interface{}) {
	org := usercontext.CurrentOrg(ctx)
	user := usercontext.CurrentUser(ctx)
	apiToken := usercontext.CurrentAPIToken(ctx)
	role := usercontext.CurrentAuthzSubject(ctx)

	// ConfigureScope allows to set context that will be sent along with the error if one occurs
	sentry.ConfigureScope(func(scope *sentry.Scope) {
		scope.SetContext("Account", buildAuthContext(user, apiToken, org, role))
		scope.SetContext("Request", buildRequestContext(ctx, req))
	})
}

// buildAuthContext creates a map of the user and membership information
func buildAuthContext(user *usercontext.User, apiToken *usercontext.APIToken, org *usercontext.Org, role string) map[string]interface{} {
	if org == nil {
		return nil
	}

	val := map[string]interface{}{
		"orgID":   org.ID,
		"orgName": org.Name,
		"role":    role,
	}

	if user != nil {
		val["id"] = user.ID
		val["serviceAccount"] = false
	} else if apiToken != nil {
		val["id"] = apiToken.ID
		val["serviceAccount"] = true
	}

	return val
}

// buildRequestContext creates a map of the request information
func buildRequestContext(ctx context.Context, req interface{}) map[string]interface{} {
	var protocol, operation string

	if info, ok := transport.FromServerContext(ctx); ok {
		protocol = info.Kind().String()
		operation = info.Operation()
	}

	return map[string]interface{}{
		"protocol":   protocol,
		"operation":  operation,
		"args":       extractArgs(req),
		"request-id": extractTracingIDFromMetadata(ctx),
	}
}

// extractTracingIDFromMetadata extracts the tracing ID from the metadata.
func extractTracingIDFromMetadata(ctx context.Context) string {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return ""
	}

	for _, header := range defaultTracingHeaders {
		if val := md.Get(header); len(val) != 0 {
			return val[0]
		}
	}

	return ""
}

// extractArgs returns the string of the req
// Logic extracted from: https://github.com/go-kratos/kratos/blob/f8b97f675b32dfad02edae12d83053c720720b5b/middleware/logging/logging.go#L103
func extractArgs(req interface{}) string {
	if redacter, ok := req.(logging.Redacter); ok {
		return redacter.Redact()
	}
	if stringer, ok := req.(fmt.Stringer); ok {
		return stringer.String()
	}
	return fmt.Sprintf("%+v", req)
}
