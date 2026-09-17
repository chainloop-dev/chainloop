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
	"net"
	"net/http/httptest"
	"testing"

	v1 "github.com/chainloop-dev/chainloop/app/controlplane/api/controlplane/v1"
	conf "github.com/chainloop-dev/chainloop/app/controlplane/internal/conf/controlplane/config/v1"
	"github.com/go-kratos/kratos/v2/transport/grpc"
	"github.com/go-kratos/kratos/v2/transport/http"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	"go.opentelemetry.io/otel/trace"
	grpcLib "google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
)

// newRecordingTracerProvider returns a TracerProvider that keeps every finished
// span in memory so tests can assert on them.
func newRecordingTracerProvider() (trace.TracerProvider, *tracetest.SpanRecorder) {
	recorder := tracetest.NewSpanRecorder()
	return sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(recorder)), recorder
}

// statusServiceStub is a minimal StatusService implementation, enough to drive a
// request through the transport under test.
type statusServiceStub struct {
	v1.UnimplementedStatusServiceServer
}

func (s *statusServiceStub) Statusz(_ context.Context, _ *v1.StatuszRequest) (*v1.StatuszResponse, error) {
	return &v1.StatuszResponse{}, nil
}

func (s *statusServiceStub) Infoz(_ context.Context, _ *v1.InfozRequest) (*v1.InfozResponse, error) {
	return &v1.InfozResponse{}, nil
}

// TestRawGRPCServerOptions covers the regression where setting
// server.grpc.max_recv_msg_size silently dropped the OTel stats handler: Kratos'
// grpc.Options assigns instead of appending, so both raw options have to travel
// in a single call.
func TestRawGRPCServerOptions(t *testing.T) {
	const statuszOperation = "controlplane.v1.StatusService/Statusz"

	testCases := []struct {
		name           string
		maxRecvMsgSize int32
		// wantResourceExhausted expects the request to be rejected for exceeding
		// the configured max_recv_msg_size.
		wantResourceExhausted bool
	}{
		{
			name:           "no max_recv_msg_size configured",
			maxRecvMsgSize: 0,
		},
		{
			name:           "max_recv_msg_size configured",
			maxRecvMsgSize: 1024 * 1024,
		},
		{
			// StatuszRequest{Readiness: true} serializes to 2 bytes, so a 1 byte
			// limit proves the option is still applied alongside the stats handler.
			name:                  "max_recv_msg_size enforced",
			maxRecvMsgSize:        1,
			wantResourceExhausted: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			tp, recorder := newRecordingTracerProvider()

			lis, err := net.Listen("tcp", "127.0.0.1:0")
			require.NoError(t, err)

			srv := grpc.NewServer(
				grpc.Listener(lis),
				grpc.Options(rawGRPCServerOptions(&conf.Server_GRPC{MaxRecvMsgSize: tc.maxRecvMsgSize}, tp)...),
			)
			v1.RegisterStatusServiceServer(srv, &statusServiceStub{})

			go func() { _ = srv.Start(context.Background()) }()
			t.Cleanup(func() { _ = srv.Stop(context.Background()) })

			conn, err := grpcLib.NewClient(lis.Addr().String(), grpcLib.WithTransportCredentials(insecure.NewCredentials()))
			require.NoError(t, err)
			t.Cleanup(func() { _ = conn.Close() })

			_, err = v1.NewStatusServiceClient(conn).Statusz(context.Background(), &v1.StatuszRequest{Readiness: true})
			if tc.wantResourceExhausted {
				require.Error(t, err)
				assert.Equal(t, codes.ResourceExhausted, status.Code(err))
			} else {
				require.NoError(t, err)
			}

			// Regardless of the outcome the transport must have opened a server span.
			spans := recorder.Ended()
			require.Len(t, spans, 1)
			assert.Equal(t, statuszOperation, spans[0].Name())
			assert.Equal(t, trace.SpanKindServer, spans[0].SpanKind())
		})
	}
}

// TestHTTPServerTracingMiddleware asserts the HTTP transport opens a server span
// rooted at the incoming trace context, so transcoded requests no longer root at
// whichever application middleware happens to run first.
func TestHTTPServerTracingMiddleware(t *testing.T) {
	tp, recorder := newRecordingTracerProvider()

	srv := http.NewServer(http.Middleware(serverTracingMiddleware(tp)))
	v1.RegisterStatusServiceHTTPServer(srv, &statusServiceStub{})

	ts := httptest.NewServer(srv)
	t.Cleanup(ts.Close)

	req := httptest.NewRequest("GET", "/statusz", nil)
	req.RequestURI = ""
	req.URL, _ = req.URL.Parse(ts.URL + "/statusz")
	// A remote parent, to prove the propagator extracts incoming trace context.
	req.Header.Set("traceparent", "00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01")

	resp, err := ts.Client().Do(req)
	require.NoError(t, err)
	t.Cleanup(func() { _ = resp.Body.Close() })
	require.Equal(t, 200, resp.StatusCode)

	spans := recorder.Ended()
	require.Len(t, spans, 1)
	assert.Equal(t, v1.OperationStatusServiceStatusz, spans[0].Name())
	assert.Equal(t, trace.SpanKindServer, spans[0].SpanKind())
	assert.Equal(t, "4bf92f3577b34da6a3ce929d0e0e4736", spans[0].SpanContext().TraceID().String())
	assert.Equal(t, "00f067aa0ba902b7", spans[0].Parent().SpanID().String())
}

// TestHTTPServerMiddlewaresOpenWithTracing pins the tracing middleware to the
// head of the HTTP chain: anything running before it would become the trace root.
func TestHTTPServerMiddlewaresOpenWithTracing(t *testing.T) {
	tp, recorder := newRecordingTracerProvider()

	opts := &Opts{
		TracerProvider: tp,
		AuthConfig:     &conf.Auth{},
		ServerConfig:   &conf.Server{},
	}

	middlewares := httpServerMiddlewares(opts)
	require.NotEmpty(t, middlewares)

	srv := http.NewServer(http.Middleware(middlewares[0]))
	v1.RegisterStatusServiceHTTPServer(srv, &statusServiceStub{})

	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, httptest.NewRequest("GET", "/statusz", nil))
	require.Equal(t, 200, rec.Code)

	spans := recorder.Ended()
	require.Len(t, spans, 1)
	assert.Equal(t, v1.OperationStatusServiceStatusz, spans[0].Name())
	assert.Equal(t, trace.SpanKindServer, spans[0].SpanKind())
}
