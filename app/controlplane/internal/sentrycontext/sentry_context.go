//
// Copyright 2024-2026 The Chainloop Authors.
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
	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/usercontext/entities"

	"github.com/getsentry/sentry-go"
	"github.com/go-kratos/kratos/v2/middleware"
	"github.com/go-kratos/kratos/v2/middleware/logging"
	"github.com/go-kratos/kratos/v2/transport"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/grpc/metadata"
)

// defaultTracingHeaders is a list of headers that are used to extract tracing information.
var defaultTracingHeaders = []string{"X-Request-ID", "X-Correlation-ID", "X-Trace-ID"}

// Sentry tags that identify the organization of the current request
const (
	tagOrgID   = "org.id"
	tagOrgName = "org.name"
)

// NewSentryHub returns a middleware that gives each request its own Sentry hub on its context,
// so concurrent requests do not share a scope. Put it before any middleware that reports to Sentry.
func NewSentryHub() middleware.Middleware {
	return func(handler middleware.Handler) middleware.Handler {
		return func(ctx context.Context, req interface{}) (interface{}, error) {
			return handler(sentry.SetHubOnContext(ctx, sentry.CurrentHub().Clone()), req)
		}
	}
}

// NewSentryContext returns a middleware that adds context to Sentry for the current request
// that will be sent along with the error if one occurs.
// It sets the scope of the request hub that NewSentryHub added, and adds that hub if it is missing.
func NewSentryContext() middleware.Middleware {
	return func(handler middleware.Handler) middleware.Handler {
		return func(ctx context.Context, req interface{}) (interface{}, error) {
			hub := sentry.GetHubFromContext(ctx)
			if hub == nil {
				hub = sentry.CurrentHub().Clone()
				ctx = sentry.SetHubOnContext(ctx, hub)
			}

			configureScope(ctx, hub.Scope(), req)
			return handler(ctx, req)
		}
	}
}

// configureScope sets the account and request information of the current request on the scope
func configureScope(ctx context.Context, scope *sentry.Scope, req interface{}) {
	org := entities.CurrentOrg(ctx)
	user := entities.CurrentUser(ctx)
	apiToken := entities.CurrentAPIToken(ctx)
	role := usercontext.CurrentAuthzSubject(ctx)

	scope.SetContext("Account", buildAuthContext(user, apiToken, org, role))
	scope.SetContext("Request", buildRequestContext(ctx, req))

	// Tags make the organization searchable in Sentry. They do not take part in issue grouping.
	if org != nil {
		scope.SetTag(tagOrgID, org.ID)
		scope.SetTag(tagOrgName, org.Name)
	}
}

// buildAuthContext creates a map of the user and membership information
func buildAuthContext(user *entities.User, apiToken *entities.APIToken, org *entities.Org, role string) map[string]interface{} {
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

	requestContext := map[string]interface{}{
		"protocol":   protocol,
		"operation":  operation,
		"args":       extractArgs(req),
		"request-id": extractTracingIDFromMetadata(ctx),
	}

	// Surface the OpenTelemetry trace/span IDs so a Sentry event can be cross-referenced
	// with its distributed trace in the OTLP backend (Tempo/Jaeger). The gRPC server is
	// instrumented via otelgrpc, so a valid span context is present on ctx when tracing is
	// enabled. These are informational fields and do not alter Sentry's canonical trace_id.
	if sc := trace.SpanContextFromContext(ctx); sc.IsValid() {
		requestContext["otel-trace-id"] = sc.TraceID().String()
		requestContext["otel-span-id"] = sc.SpanID().String()
	}

	return requestContext
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
