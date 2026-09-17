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
	"io"
	"net/http"
	"testing"
	"time"

	"buf.build/go/protovalidate"
	conf "github.com/chainloop-dev/chainloop/app/controlplane/internal/conf/controlplane/config/v1"
	"github.com/chainloop-dev/chainloop/app/controlplane/internal/service"
	"github.com/go-kratos/kratos/v2/log"
	kratoshttp "github.com/go-kratos/kratos/v2/transport/http"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	grpcLib "google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/health/grpc_health_v1"
)

// withInMemoryTracing installs an in-memory OTel tracer provider for the
// duration of the test and returns the exporter.
func withInMemoryTracing(t *testing.T) *tracetest.InMemoryExporter {
	t.Helper()
	exporter := tracetest.NewInMemoryExporter()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSyncer(exporter))
	prev := otel.GetTracerProvider()
	otel.SetTracerProvider(tp)
	t.Cleanup(func() { otel.SetTracerProvider(prev) })
	return exporter
}

func spanNames(exporter *tracetest.InMemoryExporter) []string {
	spans := exporter.GetSpans()
	names := make([]string, 0, len(spans))
	for _, s := range spans {
		names = append(names, s.Name)
	}
	return names
}

// The gRPC server must produce server spans even when max_recv_msg_size is
// configured. kratos' grpc.Options assigns s.grpcOpts instead of appending,
// so a second grpc.Options call in NewGRPCServer would silently drop the
// OTel stats handler. This exercises the real NewGRPCServer constructor.
func TestGRPCServerSpanWithMaxRecvMsgSize(t *testing.T) {
	exporter := withInMemoryTracing(t)

	validator, err := protovalidate.New()
	require.NoError(t, err)

	srv, err := NewGRPCServer(&Opts{
		ServerConfig: &conf.Server{
			Grpc: &conf.Server_GRPC{
				Addr:           "127.0.0.1:0",
				MaxRecvMsgSize: 8 * 1024 * 1024,
			},
		},
		AuthConfig: &conf.Auth{},
		Logger:     log.NewStdLogger(io.Discard),
		Validator:  validator,
		AuthSvc:    &service.AuthService{AuthURLs: &service.AuthURLs{Login: "http://localhost"}},
	})
	require.NoError(t, err)

	// Endpoint() binds the listener so the address is known before Start.
	endpoint, err := srv.Endpoint()
	require.NoError(t, err)
	go func() { _ = srv.Start(context.Background()) }()
	defer func() { _ = srv.Stop(context.Background()) }()

	conn, err := grpcLib.NewClient(endpoint.Host, grpcLib.WithTransportCredentials(insecure.NewCredentials()))
	require.NoError(t, err)
	defer conn.Close()

	// The request itself is rejected by the auth middleware, but the
	// transport-level stats handler records the server span regardless.
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		_, _ = grpc_health_v1.NewHealthClient(conn).Check(ctx, &grpc_health_v1.HealthCheckRequest{})
		cancel()
		if names := spanNames(exporter); len(names) > 0 {
			assert.Contains(t, names, "grpc.health.v1.Health/Check")
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	require.Fail(t, "expected gRPC server span from otelgrpc stats handler")
}

// The HTTP server must produce a transport-level server span so transcoded
// REST requests don't root their traces at the first middleware span. This
// exercises the production httpMiddlewares chain used by NewHTTPServer.
func TestHTTPServerSpanFromTracingMiddleware(t *testing.T) {
	exporter := withInMemoryTracing(t)

	validator, err := protovalidate.New()
	require.NoError(t, err)

	srv := kratoshttp.NewServer(
		kratoshttp.Address("127.0.0.1:0"),
		kratoshttp.Middleware(httpMiddlewares(&Opts{
			AuthConfig: &conf.Auth{},
			Logger:     log.NewStdLogger(io.Discard),
			Validator:  validator,
		})...),
	)
	r := srv.Route("/")
	// Mirror generated handlers: kratos runs the middleware chain only when
	// the handler invokes ctx.Middleware (transport/http/context.go).
	r.GET("/ping", func(ctx kratoshttp.Context) error {
		h := ctx.Middleware(func(context.Context, any) (any, error) {
			return "ok", nil
		})
		out, err := h(ctx, &grpc_health_v1.HealthCheckRequest{})
		if err != nil {
			return err
		}
		return ctx.String(200, out.(string))
	})

	endpoint, err := srv.Endpoint()
	require.NoError(t, err)
	go func() { _ = srv.Start(context.Background()) }()
	defer func() { _ = srv.Stop(context.Background()) }()

	// The request is rejected by the auth middleware (401), but the tracing
	// middleware is outermost and records the server span regardless.
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		resp, err := http.Get("http://" + endpoint.Host + "/ping")
		if err == nil {
			_ = resp.Body.Close()
		}
		if names := spanNames(exporter); len(names) > 0 {
			assert.Contains(t, names, "/ping")
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	require.Fail(t, "expected HTTP server span from tracing middleware")
}
