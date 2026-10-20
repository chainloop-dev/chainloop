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
	"bytes"
	"context"
	"crypto/tls"
	"io"
	"net"
	"net/http"
	"net/url"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	v1 "github.com/chainloop-dev/chainloop/app/controlplane/api/controlplane/v1"
	conf "github.com/chainloop-dev/chainloop/app/controlplane/internal/conf/controlplane/config/v1"
	"github.com/go-kratos/kratos/v2/middleware"
	"github.com/go-kratos/kratos/v2/transport"
	kratosgrpc "github.com/go-kratos/kratos/v2/transport/grpc"
	"github.com/stretchr/testify/require"
	grpcLib "google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/peer"
	reflection "google.golang.org/grpc/reflection/grpc_reflection_v1"
	"google.golang.org/grpc/status"
)

// A proxy can send browser traffic after native gRPC on the same upstream
// connection. Verify real HTTP/2 reuse, rather than separate client sockets.
func TestSharedHTTP2ConnectionAfterNativeGRPC(t *testing.T) {
	for _, useTLS := range []bool{false, true} {
		t.Run(map[bool]string{false: "h2c", true: "TLS"}[useTLS], func(t *testing.T) {
			testSharedHTTP2Connection(t, useTLS)
		})
	}
}

func testSharedHTTP2Connection(t *testing.T, useTLS bool) {
	t.Helper()
	const httpKind, grpcKind, preflightKind = "http", "grpc", "preflight"
	off := false
	config := &conf.Server{
		Grpc:               &conf.Server_GRPC{Addr: testAnyAddr},
		Http:               &conf.Server_HTTP{Addr: testAnyAddr},
		LegacyGrpcListener: &off,
	}
	scheme := httpKind
	if useTLS {
		cert, key := writeSelfSignedCert(t)
		config.Http.TlsConfig = &conf.Server_TLS{Certificate: cert, PrivateKey: key}
		scheme = "https"
	}
	addr, _, _ := startSinglePortAppWithConfig(t, config)
	var connections atomic.Int32
	tr := &http.Transport{
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			connections.Add(1)
			return (&net.Dialer{}).DialContext(ctx, network, addr)
		},
		Protocols:       new(http.Protocols),
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true}, //nolint:gosec // test certificate
	}
	tr.Protocols.SetUnencryptedHTTP2(true)
	tr.Protocols.SetHTTP2(true)
	t.Cleanup(tr.CloseIdleConnections)
	client := &http.Client{Transport: tr, Timeout: 5 * time.Second}

	for _, kind := range []string{grpcKind, httpKind, "web", preflightKind, grpcKind, httpKind} {
		path, method := "/grpc.health.v1.Health/Check", http.MethodPost
		switch kind {
		case httpKind:
			path, method = "/statusz", http.MethodGet
		case preflightKind:
			method = http.MethodOptions
		}
		req, err := http.NewRequest(method, scheme+"://"+addr+path, bytes.NewReader([]byte{0, 0, 0, 0, 0}))
		require.NoError(t, err)
		switch kind {
		case grpcKind:
			req.Header.Set("Content-Type", "application/grpc")
			req.Header.Set("Te", "trailers")
		case "web":
			req.Header.Set("Content-Type", "application/grpc-web+proto")
			req.Header.Set("X-Grpc-Web", "1")
		case preflightKind:
			req.Header.Set("Origin", "https://example.test")
			req.Header.Set("Access-Control-Request-Method", "POST")
			req.Header.Set("Access-Control-Request-Headers", "content-type,x-grpc-web")
		}
		resp, err := client.Do(req)
		require.NoError(t, err, kind)
		_, err = io.Copy(io.Discard, resp.Body)
		require.NoError(t, resp.Body.Close())
		require.NoError(t, err)
		expectedStatus := http.StatusOK
		if kind == preflightKind {
			expectedStatus = http.StatusNoContent
		}
		require.Equal(t, expectedStatus, resp.StatusCode, kind)
		require.Equal(t, 2, resp.ProtoMajor)
		if kind == grpcKind {
			grpcStatus := resp.Trailer.Get("Grpc-Status")
			if grpcStatus == "" {
				grpcStatus = resp.Header.Get("Grpc-Status")
			}
			require.Equal(t, "16", grpcStatus)
		}
	}
	require.EqualValues(t, 1, connections.Load(), "all request types must share one HTTP/2 connection")
}

// This is a focused parity check. The full token and authorization matrix in
// the spec remains a separate qualification requirement.
func TestSharedAndLegacyGRPCBehavior(t *testing.T) {
	addr, legacyPort, _ := startSinglePortApp(t, nil)
	for _, endpoint := range []string{addr, "127.0.0.1:" + legacyPort} {
		t.Run(endpoint, func(t *testing.T) {
			conn, err := grpcLib.NewClient(endpoint, grpcLib.WithTransportCredentials(insecure.NewCredentials()))
			require.NoError(t, err)
			defer conn.Close()
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			_, err = v1.NewStatusServiceClient(conn).Statusz(ctx, &v1.StatuszRequest{})
			require.NoError(t, err)
			for _, token := range []string{"", "Bearer invalid.jwt.token"} {
				callCtx := metadata.AppendToOutgoingContext(ctx, "authorization", token)
				_, err = grpc_health_v1.NewHealthClient(conn).Check(callCtx, &grpc_health_v1.HealthCheckRequest{})
				require.Equal(t, codes.Unauthenticated, status.Code(err))
			}
			// Reflection exercises the Kratos bidirectional-stream interceptor,
			// including its endpoint metadata on a handler with no native listener.
			stream, err := reflection.NewServerReflectionClient(conn).ServerReflectionInfo(ctx)
			require.NoError(t, err)
			for range 2 {
				require.NoError(t, stream.Send(&reflection.ServerReflectionRequest{
					MessageRequest: &reflection.ServerReflectionRequest_ListServices{ListServices: ""},
				}))
				resp, err := stream.Recv()
				require.NoError(t, err)
				require.NotEmpty(t, resp.GetListServicesResponse().GetService())
			}
			require.NoError(t, stream.CloseSend())
			_, err = stream.Recv()
			require.ErrorIs(t, err, io.EOF)
		})
	}
}

func TestAPIServerDrainsRPCAndPreservesTLSMetadata(t *testing.T) {
	off := false
	cert, key := writeSelfSignedCert(t)
	opts := testServerOpts(t, &conf.Server{
		Grpc:               &conf.Server_GRPC{Addr: testAnyAddr},
		Http:               &conf.Server_HTTP{Addr: testAnyAddr, TlsConfig: &conf.Server_TLS{Certificate: cert, PrivateKey: key}},
		LegacyGrpcListener: &off,
	})
	release := make(chan struct{})
	unblock := sync.OnceFunc(func() { close(release) })
	defer unblock()
	entered := make(chan context.Context, 1)
	g := newGRPCServer(opts, kratosgrpc.Endpoint(&url.URL{Scheme: "grpcs", Host: testAnyAddr}),
		kratosgrpc.Middleware(func(next middleware.Handler) middleware.Handler {
			return func(ctx context.Context, req any) (any, error) {
				entered <- ctx
				select {
				case <-release:
					return next(ctx, req)
				case <-ctx.Done():
					return nil, ctx.Err()
				}
			}
		}))
	api, err := NewHTTPServer(opts, &GRPCServers{Handler: g})
	require.NoError(t, err)
	endpoint, err := api.Endpoint()
	require.NoError(t, err)
	serveErr := make(chan error, 1)
	go func() { serveErr <- api.Start(context.Background()) }()
	t.Cleanup(func() {
		unblock()
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		require.NoError(t, api.Stop(ctx))
	})
	conn, err := grpcLib.NewClient(endpoint.Host, grpcLib.WithTransportCredentials(credentials.NewTLS(
		&tls.Config{InsecureSkipVerify: true}))) //nolint:gosec // test certificate
	require.NoError(t, err)
	defer conn.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	rpcErr := make(chan error, 1)
	go func() {
		callCtx := metadata.AppendToOutgoingContext(ctx, "x-dispatch-test", "preserved")
		_, err := v1.NewStatusServiceClient(conn).Statusz(callCtx, &v1.StatuszRequest{})
		rpcErr <- err
	}()
	select {
	case callCtx := <-entered:
		p, ok := peer.FromContext(callCtx)
		require.True(t, ok)
		tlsInfo, ok := p.AuthInfo.(credentials.TLSInfo)
		require.True(t, ok, "the HTTP adapter must retain TLS peer information")
		require.Equal(t, "h2", tlsInfo.State.NegotiatedProtocol)
		tr, ok := transport.FromServerContext(callCtx)
		require.True(t, ok)
		require.Equal(t, "/controlplane.v1.StatusService/Statusz", tr.Operation())
		require.Equal(t, "preserved", tr.RequestHeader().Get("x-dispatch-test"))
	case <-ctx.Done():
		require.FailNow(t, "RPC did not enter the middleware")
	}
	shutdownStarted := make(chan struct{})
	api.RegisterOnShutdown(sync.OnceFunc(func() { close(shutdownStarted) }))
	stopErr := make(chan error, 1)
	go func() { stopErr <- api.Stop(ctx) }()
	select {
	case <-shutdownStarted:
	case <-ctx.Done():
		require.FailNow(t, "HTTP shutdown did not start")
	}
	unblock()
	require.NoError(t, <-rpcErr, "shutdown must allow the in-flight RPC to finish")
	require.NoError(t, <-stopErr)
	require.NoError(t, <-serveErr)
}
