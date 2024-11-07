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
	"testing"

	"github.com/chainloop-dev/chainloop/app/controlplane/internal/usercontext"

	"github.com/go-kratos/kratos/v2/transport"
	"github.com/stretchr/testify/assert"
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

func TestBuildAuthContext(t *testing.T) {
	org := &usercontext.Org{ID: "org1", Name: "OrgName"}
	user := &usercontext.User{ID: "user1"}
	apiToken := &usercontext.APIToken{ID: "token1"}
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
