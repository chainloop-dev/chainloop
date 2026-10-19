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
	"bytes"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"io"
	"math/big"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"buf.build/go/protovalidate"
	conf "github.com/chainloop-dev/chainloop/app/controlplane/internal/conf/controlplane/config/v1"
	"github.com/chainloop-dev/chainloop/app/controlplane/internal/service"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	grpcLib "google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/status"
)

// Shared literals across the server tests.
const (
	testAnyAddr  = "127.0.0.1:0"
	testLoginURL = "http://localhost"
)

// startSinglePortApp starts the real HTTP-owned endpoint and optional legacy
// native gRPC listener. The HTTP-served gRPC handler never starts a listener.
func startSinglePortApp(t *testing.T, legacyListener *bool) (string, string, int) {
	t.Helper()
	return startSinglePortAppWithConfig(t, &conf.Server{
		Grpc:               &conf.Server_GRPC{Addr: testAnyAddr},
		Http:               &conf.Server_HTTP{Addr: testAnyAddr},
		LegacyGrpcListener: legacyListener,
	})
}

func startSinglePortAppWithConfig(t *testing.T, serverConfig *conf.Server) (string, string, int) {
	t.Helper()
	opts := testServerOpts(t, serverConfig)
	gs, err := NewGRPCServers(opts)
	require.NoError(t, err)
	api, err := NewHTTPServer(opts, gs)
	require.NoError(t, err)
	endpoint, err := api.Endpoint()
	require.NoError(t, err)
	_, port, err := net.SplitHostPort(endpoint.Host)
	require.NoError(t, err)

	legacyPort := ""
	serverCount := 1
	if gs.Legacy != nil {
		serverCount++
	}
	errs := make(chan error, serverCount)
	if g := gs.Legacy; g != nil {
		endpoint, err := g.Endpoint()
		require.NoError(t, err)
		_, legacyPort, err = net.SplitHostPort(endpoint.Host)
		require.NoError(t, err)
		go func() { errs <- g.Start(context.Background()) }()
	}
	go func() { errs <- api.Start(context.Background()) }()
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		require.NoError(t, api.Stop(ctx))
		if g := gs.Legacy; g != nil {
			require.NoError(t, g.Stop(ctx))
		}
		for range serverCount {
			select {
			case err := <-errs:
				require.NoError(t, err)
			case <-ctx.Done():
				require.FailNow(t, "API servers did not stop")
			}
		}
	})
	// Wait for the legacy server to enter Serve before a test can stop it.
	// Endpoint binds its socket, but Start runs in another goroutine.
	if legacyPort != "" {
		legacyCreds := insecure.NewCredentials()
		if cfg := serverConfig.GetGrpc().GetTlsConfig(); cfg.GetCertificate() != "" && cfg.GetPrivateKey() != "" {
			legacyCreds = credentials.NewTLS(&tls.Config{InsecureSkipVerify: true}) //nolint:gosec // test certificate
		}
		conn, err := grpcLib.NewClient(net.JoinHostPort("127.0.0.1", legacyPort), grpcLib.WithTransportCredentials(legacyCreds))
		require.NoError(t, err)
		defer conn.Close()
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_, err = grpc_health_v1.NewHealthClient(conn).Check(ctx, &grpc_health_v1.HealthCheckRequest{})
		require.Equal(t, codes.Unauthenticated, status.Code(err))
	}
	return net.JoinHostPort("127.0.0.1", port), legacyPort, serverCount
}

func testServerOpts(t *testing.T, serverConfig *conf.Server) *Opts {
	t.Helper()
	validator, err := protovalidate.New()
	require.NoError(t, err)
	return &Opts{
		ServerConfig: serverConfig,
		AuthConfig:   &conf.Auth{},
		Logger:       log.NewStdLogger(io.Discard),
		Validator:    validator,
		AuthSvc:      &service.AuthService{AuthURLs: &service.AuthURLs{Login: testLoginURL}},
	}
}

func TestSinglePortServesAllProtocols(t *testing.T) {
	addr, _, _ := startSinglePortApp(t, nil)

	t.Run("native gRPC", func(t *testing.T) {
		conn, err := grpcLib.NewClient(addr, grpcLib.WithTransportCredentials(insecure.NewCredentials()))
		require.NoError(t, err)
		defer conn.Close()

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_, err = grpc_health_v1.NewHealthClient(conn).Check(ctx, &grpc_health_v1.HealthCheckRequest{})
		// The RPC must reach the HTTP-served gRPC handler, whose middleware chain
		// rejects unauthenticated calls exactly as it does today on the
		// dedicated gRPC port (R-002: same behavior as today).
		require.Error(t, err)
		assert.Equal(t, codes.Unauthenticated, status.Code(err))
	})

	t.Run("plain HTTP/1.1", func(t *testing.T) {
		resp, err := http.Get("http://" + addr + "/statusz")
		require.NoError(t, err)
		defer resp.Body.Close()
		assert.Equal(t, http.StatusOK, resp.StatusCode)
	})

	t.Run("plain HTTP/2 without TLS", func(t *testing.T) {
		tr := &http.Transport{}
		tr.Protocols = new(http.Protocols)
		tr.Protocols.SetUnencryptedHTTP2(true)
		client := &http.Client{Transport: tr}
		resp, err := client.Get("http://" + addr + "/statusz")
		require.NoError(t, err)
		defer resp.Body.Close()
		assert.Equal(t, http.StatusOK, resp.StatusCode)
	})

	t.Run("gRPC-Web", func(t *testing.T) {
		// An empty HealthCheckRequest serialized as a grpc-web frame:
		// compression flag 0 + 4 zero length bytes.
		req, err := http.NewRequest(http.MethodPost, "http://"+addr+"/grpc.health.v1.Health/Check", bytes.NewReader([]byte{0, 0, 0, 0, 0}))
		require.NoError(t, err)
		req.Header.Set("content-type", "application/grpc-web+proto")
		req.Header.Set("x-grpc-web", "1")

		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()
		assert.Equal(t, http.StatusOK, resp.StatusCode)
	})

	t.Run("native gRPC on an HTTP-owned HTTP/2 connection", func(t *testing.T) {
		// One HTTP/2 client connection that first speaks plain HTTP and then
		// a native gRPC call: the spec promises this direction works, via the
		// HTTP server handing the request to the gRPC server.
		tr := &http.Transport{}
		tr.Protocols = new(http.Protocols)
		tr.Protocols.SetUnencryptedHTTP2(true)
		client := &http.Client{Transport: tr}

		resp, err := client.Get("http://" + addr + "/statusz")
		require.NoError(t, err)
		_, _ = io.Copy(io.Discard, resp.Body)
		resp.Body.Close()
		require.Equal(t, http.StatusOK, resp.StatusCode)

		// A native gRPC call (content-type application/grpc) with an empty
		// HealthCheckRequest frame on the same connection.
		req, err := http.NewRequest(http.MethodPost, "http://"+addr+"/grpc.health.v1.Health/Check", bytes.NewReader([]byte{0, 0, 0, 0, 0}))
		require.NoError(t, err)
		req.Header.Set("content-type", "application/grpc")
		req.Header.Set("te", "trailers")

		resp, err = client.Do(req)
		require.NoError(t, err)
		_, _ = io.Copy(io.Discard, resp.Body)
		resp.Body.Close()
		assert.Equal(t, http.StatusOK, resp.StatusCode)
		// The gRPC server answered through the same middleware chain as on
		// the dedicated gRPC port: the unauthenticated call is rejected.
		assert.Equal(t, "16", resp.Trailer.Get("Grpc-Status"), "expected an Unauthenticated gRPC status")
	})
}

func TestLegacyGRPCListenerFlag(t *testing.T) {
	t.Run("on by default", func(t *testing.T) {
		_, legacyPort, gsCount := startSinglePortApp(t, nil)
		require.NotEmpty(t, legacyPort)
		assert.Equal(t, 2, gsCount)

		// The legacy listener serves native gRPC: the call reaches the server
		// and is rejected by the same middleware chain as on the single port.
		conn, err := grpcLib.NewClient("127.0.0.1:"+legacyPort, grpcLib.WithTransportCredentials(insecure.NewCredentials()))
		require.NoError(t, err)
		defer conn.Close()
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_, err = grpc_health_v1.NewHealthClient(conn).Check(ctx, &grpc_health_v1.HealthCheckRequest{})
		require.Error(t, err)
		assert.Equal(t, codes.Unauthenticated, status.Code(err))

		// It does NOT serve the HTTP API: grpc-go rejects non-HTTP/2 requests
		// outright.
		_, herr := http.Get("http://127.0.0.1:" + legacyPort + "/statusz")
		require.Error(t, herr, "the legacy gRPC listener must not serve the HTTP API")
	})

	t.Run("explicitly on", func(t *testing.T) {
		on := true
		_, legacyPort, gsCount := startSinglePortApp(t, &on)
		assert.Equal(t, 2, gsCount)
		assert.NotEmpty(t, legacyPort)
	})

	t.Run("off listens on the single port only", func(t *testing.T) {
		off := false
		_, legacyPort, gsCount := startSinglePortApp(t, &off)
		assert.Equal(t, 1, gsCount)
		assert.Empty(t, legacyPort)
	})
}

// TestSinglePortTLS proves R-004: one TLS configuration on the single port
// serves gRPC (HTTP/2), gRPC-Web and plain HTTP, negotiated through ALPN.
// TestPartialTLSConfigRejected proves that a partial tls_config is a hard
// startup error instead of silently disabling TLS on the shared endpoint.
func TestPartialTLSConfigRejected(t *testing.T) {
	certPath, keyPath := writeSelfSignedCert(t)

	serverConfig := &conf.Server{
		Grpc: &conf.Server_GRPC{
			Addr: testAnyAddr,
		},
		Http: &conf.Server_HTTP{
			Addr:      testAnyAddr,
			TlsConfig: &conf.Server_TLS{Certificate: certPath}, // private key missing
		},
	}

	_, err := newHTTPServer(&Opts{ServerConfig: serverConfig}, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "incomplete TLS configuration")

	serverConfig.Http.TlsConfig = &conf.Server_TLS{PrivateKey: keyPath} // certificate missing
	_, err = newHTTPServer(&Opts{ServerConfig: serverConfig}, nil)
	require.Error(t, err)
}

func TestSinglePortTLS(t *testing.T) {
	certPath, keyPath := writeSelfSignedCert(t)

	serverConfig := &conf.Server{
		Grpc: &conf.Server_GRPC{
			Addr: testAnyAddr,
		},
		Http: &conf.Server_HTTP{
			Addr: testAnyAddr,
			TlsConfig: &conf.Server_TLS{
				Certificate: certPath,
				PrivateKey:  keyPath,
			},
		},
	}

	addr, _, _ := startSinglePortAppWithConfig(t, serverConfig)

	// TLS client without ALPN restriction: HTTP/1.1 over TLS must work.
	client := &http.Client{Transport: &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true}}} //nolint:gosec // test server with a self-signed certificate
	resp, err := client.Get("https://" + addr + "/statusz")
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	// Native gRPC over TLS uses the gRPC HTTP adapter.
	gconn, err := grpcLib.NewClient(addr, grpcLib.WithTransportCredentials(credentials.NewTLS(&tls.Config{InsecureSkipVerify: true}))) //nolint:gosec // test server with a self-signed certificate
	require.NoError(t, err)
	defer gconn.Close()
	gctx, gcancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer gcancel()
	gresp, gerr := grpc_health_v1.NewHealthClient(gconn).Check(gctx, &grpc_health_v1.HealthCheckRequest{})
	require.Error(t, gerr)
	assert.Nil(t, gresp)
	assert.Equal(t, codes.Unauthenticated, status.Code(gerr))

	// gRPC-Web over TLS: served by the HTTP server's wrapper.
	wreq, werr := http.NewRequest(http.MethodPost, "https://"+addr+"/grpc.health.v1.Health/Check", strings.NewReader("\x00\x00\x00\x00\x00"))
	require.NoError(t, werr)
	wreq.Header.Set("content-type", "application/grpc-web+proto")
	wreq.Header.Set("x-grpc-web", "1")
	wresp, werr := client.Do(wreq)
	require.NoError(t, werr)
	defer wresp.Body.Close()
	assert.Equal(t, http.StatusOK, wresp.StatusCode)

	// TLS client forcing HTTP/2: ALPN must negotiate h2 against the same
	// certificate and port.
	h2Client := &http.Client{Transport: &http.Transport{
		TLSClientConfig:   &tls.Config{InsecureSkipVerify: true}, //nolint:gosec // test server with a self-signed certificate
		ForceAttemptHTTP2: true,
	}}
	resp, err = h2Client.Get("https://" + addr + "/statusz")
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, "HTTP/2.0", resp.Proto)
}

func TestLegacyTLSIsIndependentFromSharedEndpoint(t *testing.T) {
	cert, key := writeSelfSignedCert(t)
	for _, sharedTLS := range []bool{false, true} {
		t.Run(map[bool]string{false: "plaintext shared port", true: "TLS shared port"}[sharedTLS], func(t *testing.T) {
			cfg := &conf.Server{
				Grpc: &conf.Server_GRPC{Addr: testAnyAddr, TlsConfig: &conf.Server_TLS{Certificate: cert, PrivateKey: key}},
				Http: &conf.Server_HTTP{Addr: testAnyAddr},
			}
			sharedCreds := insecure.NewCredentials()
			if sharedTLS {
				cfg.Http.TlsConfig = cfg.Grpc.TlsConfig
				sharedCreds = credentials.NewTLS(&tls.Config{InsecureSkipVerify: true}) //nolint:gosec // test certificate
			}
			addr, _, _ := startSinglePortAppWithConfig(t, cfg)
			// The helper verifies that the legacy listener requires TLS. This
			// call verifies that its credentials do not affect the HTTP handler.
			conn, err := grpcLib.NewClient(addr, grpcLib.WithTransportCredentials(sharedCreds))
			require.NoError(t, err)
			defer conn.Close()
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			_, err = grpc_health_v1.NewHealthClient(conn).Check(ctx, &grpc_health_v1.HealthCheckRequest{})
			require.Equal(t, codes.Unauthenticated, status.Code(err))
		})
	}
}

func writeSelfSignedCert(t *testing.T) (string, string) {
	t.Helper()

	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	tmpl := x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "localhost"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(time.Hour),
		IPAddresses:  []net.IP{net.ParseIP("127.0.0.1")},
		DNSNames:     []string{"localhost"},
	}
	der, err := x509.CreateCertificate(rand.Reader, &tmpl, &tmpl, &key.PublicKey, key)
	require.NoError(t, err)

	certPath := filepath.Join(t.TempDir(), "tls.crt")
	keyPath := filepath.Join(t.TempDir(), "tls.key")
	require.NoError(t, os.WriteFile(certPath, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}), 0o600))
	require.NoError(t, os.WriteFile(keyPath, pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)}), 0o600))
	return certPath, keyPath
}

// TestEmptyHTTPAddrRejected proves the single port never binds an ephemeral
// address: a missing server.http.addr is a startup error, not a silent :0.
func TestEmptyHTTPAddrRejected(t *testing.T) {
	serverConfig := &conf.Server{
		Grpc: &conf.Server_GRPC{Addr: testAnyAddr},
		Http: &conf.Server_HTTP{}, // addr omitted
	}
	_, err := NewHTTPServer(&Opts{ServerConfig: serverConfig}, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "server.http.addr must be set")
}
