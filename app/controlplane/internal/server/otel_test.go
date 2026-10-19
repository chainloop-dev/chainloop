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

	addr, legacyPort, _ := startSinglePortAppWithConfig(t, &conf.Server{
		Grpc: &conf.Server_GRPC{
			Addr:           testAnyAddr,
			MaxRecvMsgSize: 8 * 1024 * 1024,
		},
		Http: &conf.Server_HTTP{Addr: testAnyAddr},
	})
	for _, endpoint := range []string{addr, "127.0.0.1:" + legacyPort} {
		exporter.Reset()
		conn, err := grpcLib.NewClient(endpoint, grpcLib.WithTransportCredentials(insecure.NewCredentials()))
		require.NoError(t, err)
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		_, _ = grpc_health_v1.NewHealthClient(conn).Check(ctx, &grpc_health_v1.HealthCheckRequest{})
		cancel()
		require.NoError(t, conn.Close())
		// The transport records a span even when authentication rejects the RPC.
		require.Eventually(t, func() bool {
			return len(spanNames(exporter)) > 0
		}, time.Second, time.Millisecond)
		assert.Contains(t, spanNames(exporter), "grpc.health.v1.Health/Check", endpoint)
	}
}

// The HTTP server must produce a transport-level server span so transcoded
// REST requests don't root their traces at the first middleware span. This
// exercises the production httpMiddlewares chain used by NewHTTPServer.
func TestHTTPServerSpanFromTracingMiddleware(t *testing.T) {
	exporter := withInMemoryTracing(t)

	validator, err := protovalidate.New()
	require.NoError(t, err)

	srv := kratoshttp.NewServer(
		kratoshttp.Address(testAnyAddr),
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
