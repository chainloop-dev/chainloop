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
	"testing"

	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/usercontext/entities"

	"github.com/getsentry/sentry-go"
	"github.com/go-kratos/kratos/v2/middleware"
	"github.com/go-kratos/kratos/v2/transport"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/grpc/metadata"
)

var _ transport.Transporter = (*testingTransport)(nil)

type testingTransport struct {
	kind      transport.Kind
	endpoint  string
	operation string
}

func (tr *testingTransport) Kind() transport.Kind {
	return tr.kind
}

func (tr *testingTransport) Endpoint() string {
	return tr.endpoint
}

func (tr *testingTransport) Operation() string {
	return tr.operation
}

func (tr *testingTransport) RequestHeader() transport.Header {
	return nil
}

func (tr *testingTransport) ReplyHeader() transport.Header {
	return nil
}

func TestNewSentryContext(t *testing.T) {
	handler := func(_ context.Context, _ interface{}) (interface{}, error) {
		return "response", nil
	}

	middleware := NewSentryContext()
	_, err := middleware(handler)(context.Background(), "request")
	assert.NoError(t, err)
}

// scopeTags returns the tags that the scope of the hub in ctx adds to an event
func scopeTags(ctx context.Context) map[string]string {
	hub := sentry.GetHubFromContext(ctx)
	if hub == nil {
		return nil
	}

	return hub.Scope().ApplyToEvent(sentry.NewEvent(), nil, nil).Tags
}

func TestNewSentryContextOrgTags(t *testing.T) {
	org := &entities.Org{ID: "org-id", Name: "my-org"}
	orgTags := map[string]string{tagOrgID: "org-id", tagOrgName: "my-org"}

	testCases := []struct {
		name        string
		middlewares []middleware.Middleware
		org         *entities.Org
		wantTags    map[string]string
	}{
		{
			name:        "request with an org",
			middlewares: []middleware.Middleware{NewSentryHub(), NewSentryContext()},
			org:         org,
			wantTags:    orgTags,
		},
		{
			name:        "request without an org",
			middlewares: []middleware.Middleware{NewSentryHub(), NewSentryContext()},
			wantTags:    map[string]string{},
		},
		{
			name:        "request without a hub from NewSentryHub",
			middlewares: []middleware.Middleware{NewSentryContext()},
			org:         org,
			wantTags:    orgTags,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			if tc.org != nil {
				ctx = entities.WithCurrentOrg(ctx, tc.org)
			}

			var gotTags map[string]string
			handler := func(ctx context.Context, _ interface{}) (interface{}, error) {
				gotTags = scopeTags(ctx)
				return nil, nil
			}

			_, err := middleware.Chain(tc.middlewares...)(handler)(ctx, "request")
			require.NoError(t, err)
			assert.Equal(t, tc.wantTags, gotTags)
			assert.Empty(t, scopeTags(sentry.SetHubOnContext(context.Background(), sentry.CurrentHub())), "the global scope must not change")
		})
	}
}

// Two requests of different orgs overlap: the first one is still running when the second one
// sets its scope. Each request must keep the tags of its own org.
func TestNewSentryContextConcurrentRequests(t *testing.T) {
	chain := middleware.Chain(NewSentryHub(), NewSentryContext())
	orgA := &entities.Org{ID: "org-a", Name: "org-a-name"}
	orgB := &entities.Org{ID: "org-b", Name: "org-b-name"}

	started, resume := make(chan struct{}), make(chan struct{})
	tagsA := make(chan map[string]string, 1)
	errA := make(chan error, 1)
	go func() {
		_, err := chain(func(ctx context.Context, _ interface{}) (interface{}, error) {
			close(started)
			<-resume
			tagsA <- scopeTags(ctx)
			return nil, nil
		})(entities.WithCurrentOrg(context.Background(), orgA), "request-a")
		errA <- err
	}()

	<-started
	var tagsB map[string]string
	_, err := chain(func(ctx context.Context, _ interface{}) (interface{}, error) {
		tagsB = scopeTags(ctx)
		return nil, nil
	})(entities.WithCurrentOrg(context.Background(), orgB), "request-b")
	require.NoError(t, err)
	close(resume)
	require.NoError(t, <-errA)

	assert.Equal(t, map[string]string{tagOrgID: "org-a", tagOrgName: "org-a-name"}, <-tagsA)
	assert.Equal(t, map[string]string{tagOrgID: "org-b", tagOrgName: "org-b-name"}, tagsB)
}

func TestBuildAuthContext(t *testing.T) {
	org := &entities.Org{ID: "org1", Name: "OrgName"}
	user := &entities.User{ID: "user1"}
	apiToken := &entities.APIToken{ID: "token1"}
	role := "admin"

	t.Run("with user and org", func(t *testing.T) {
		authContext := buildAuthContext(user, nil, org, role)
		assert.NotNil(t, authContext)
		assert.Equal(t, "user1", authContext["id"])
		assert.Equal(t, false, authContext["serviceAccount"])
		assert.Equal(t, "org1", authContext["orgID"])
		assert.Equal(t, "OrgName", authContext["orgName"])
		assert.Equal(t, "admin", authContext["role"])
	})

	t.Run("with apiToken and org", func(t *testing.T) {
		authContext := buildAuthContext(nil, apiToken, org, role)
		assert.NotNil(t, authContext)
		assert.Equal(t, "token1", authContext["id"])
		assert.Equal(t, true, authContext["serviceAccount"])
		assert.Equal(t, "org1", authContext["orgID"])
		assert.Equal(t, "OrgName", authContext["orgName"])
		assert.Equal(t, "admin", authContext["role"])
	})

	t.Run("with nil org", func(t *testing.T) {
		authContext := buildAuthContext(user, nil, nil, role)
		assert.Nil(t, authContext)
	})

	t.Run("with nil user and apiToken", func(t *testing.T) {
		authContext := buildAuthContext(nil, nil, org, role)
		assert.NotNil(t, authContext)
	})
}

func TestBuildRequestContext(t *testing.T) {
	ctx := context.Background()
	req := "request"

	t.Run("with transport info", func(t *testing.T) {
		ctx := transport.NewServerContext(ctx, &testingTransport{
			kind:      transport.KindGRPC,
			operation: "TestOperation",
		})
		requestContext := buildRequestContext(ctx, req)
		assert.NotNil(t, requestContext)
		assert.Equal(t, "grpc", requestContext["protocol"])
		assert.Equal(t, "TestOperation", requestContext["operation"])
	})

	t.Run("without transport info", func(t *testing.T) {
		requestContext := buildRequestContext(ctx, req)
		assert.NotNil(t, requestContext)
		assert.Equal(t, "", requestContext["protocol"])
		assert.Equal(t, "", requestContext["operation"])
	})

	t.Run("with a valid otel span context", func(t *testing.T) {
		traceID, _ := trace.TraceIDFromHex("36b9df80c3e0c37e920caef35fa6eca0")
		spanID, _ := trace.SpanIDFromHex("a1b2c3d4e5f60718")
		sc := trace.NewSpanContext(trace.SpanContextConfig{
			TraceID: traceID,
			SpanID:  spanID,
		})
		ctx := trace.ContextWithSpanContext(context.Background(), sc)

		requestContext := buildRequestContext(ctx, req)
		assert.Equal(t, "36b9df80c3e0c37e920caef35fa6eca0", requestContext["otel-trace-id"])
		assert.Equal(t, "a1b2c3d4e5f60718", requestContext["otel-span-id"])
	})

	t.Run("without a valid otel span context", func(t *testing.T) {
		requestContext := buildRequestContext(context.Background(), req)
		assert.NotContains(t, requestContext, "otel-trace-id")
		assert.NotContains(t, requestContext, "otel-span-id")
	})
}

func TestExtractTracingIDFromMetadata(t *testing.T) {
	t.Run("with tracing headers", func(t *testing.T) {
		md := metadata.New(map[string]string{"X-Request-ID": "request-id"})
		ctx := metadata.NewIncomingContext(context.Background(), md)
		tracingID := extractTracingIDFromMetadata(ctx)
		assert.Equal(t, "request-id", tracingID)
	})

	t.Run("without tracing headers", func(t *testing.T) {
		ctx := context.Background()
		tracingID := extractTracingIDFromMetadata(ctx)
		assert.Equal(t, "", tracingID)
	})
}

func TestExtractArgs(t *testing.T) {
	req := "request"
	args := extractArgs(req)
	assert.Equal(t, "request", args)
}
