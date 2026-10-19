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
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"encoding/gob"
	"encoding/hex"
	"encoding/pem"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"buf.build/go/protovalidate"
	v1 "github.com/chainloop-dev/chainloop/app/artifact-cas/api/cas/v1"
	"github.com/chainloop-dev/chainloop/app/artifact-cas/internal/conf"
	"github.com/chainloop-dev/chainloop/app/artifact-cas/internal/service"
	casJWT "github.com/chainloop-dev/chainloop/internal/robotaccount/cas"
	backend "github.com/chainloop-dev/chainloop/pkg/blobmanager"
	"github.com/chainloop-dev/chainloop/pkg/blobmanager/mocks"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/go-kratos/kratos/v2/transport"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"google.golang.org/genproto/googleapis/bytestream"
	grpcLib "google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	reflection "google.golang.org/grpc/reflection/grpc_reflection_v1"
	"google.golang.org/grpc/status"
)

const (
	casAnyAddr          = "127.0.0.1:0"
	casSharedEndpoint   = "shared"
	casLegacyEndpoint   = "legacy"
	casGRPCRequest      = "grpc"
	casHTTPRequest      = "http"
	casPreflightRequest = "preflight"
	casExpiredTokenCase = "expired token"
)

// diskBackend stores the bytes rather than mocking the service's upload and
// download results. It keeps test artifact contents off the Go heap.
type diskBackend struct{ dir string }

func (b *diskBackend) Upload(_ context.Context, r io.Reader, resource *v1.CASResource) error {
	f, err := os.Create(filepath.Join(b.dir, resource.Digest))
	if err != nil {
		return err
	}
	_, copyErr := io.Copy(f, r)
	closeErr := f.Close()
	if copyErr != nil {
		return copyErr
	}
	return closeErr
}
func (b *diskBackend) Download(_ context.Context, w io.Writer, digest string) error {
	f, err := os.Open(filepath.Join(b.dir, digest))
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = io.Copy(w, f)
	return err
}
func (b *diskBackend) Describe(_ context.Context, digest string) (*v1.CASResource, error) {
	info, err := os.Stat(filepath.Join(b.dir, digest))
	if err != nil {
		return nil, err
	}
	return &v1.CASResource{Digest: digest, FileName: "artifact.bin", Size: info.Size()}, nil
}
func (b *diskBackend) Exists(_ context.Context, digest string) (bool, error) {
	_, err := os.Stat(filepath.Join(b.dir, digest))
	if os.IsNotExist(err) {
		return false, nil
	}
	return err == nil, err
}
func (*diskBackend) CheckWritePermissions(context.Context) error { return nil }

func casTestOpts(t testing.TB) *Opts {
	t.Helper()
	storage := &diskBackend{dir: t.TempDir()}
	provider := mocks.NewProvider(t)
	provider.On("FromCredentials", mock.Anything, "test-secret").Maybe().Return(storage, nil)
	providers := backend.Providers{"test": provider}
	validator, err := protovalidate.New()
	require.NoError(t, err)
	svcOpts := []service.NewOpt{service.WithStagingDir(t.TempDir())}
	return &Opts{
		ServerConfig: &conf.Server{
			Http: &conf.Server_HTTP{Addr: casAnyAddr, Cors: &conf.Server_CORS{AllowOrigins: []string{"https://example.test"}}},
			Grpc: &conf.Server_GRPC{Addr: casAnyAddr},
		},
		AuthConfig:  &conf.Auth{PublicKeyPath: "testdata/test-key.ec.pub"},
		ByteService: service.NewByteStreamService(providers, svcOpts...),
		ResourceSvc: service.NewResourceService(providers, svcOpts...),
		DownloadSvc: service.NewDownloadService(providers, svcOpts...),
		Providers:   providers, Validator: validator, Logger: log.NewStdLogger(io.Discard),
	}
}

func startCASTestServers(t testing.TB, opts *Opts) (sharedURL, legacyAddr string) {
	t.Helper()
	servers, err := NewGRPCServers(opts)
	require.NoError(t, err)
	api, err := NewHTTPServer(opts, servers)
	require.NoError(t, err)
	endpoint, err := api.Endpoint()
	require.NoError(t, err)
	startCASTestTransport(t, api)
	if servers.Legacy != nil {
		legacy, err := servers.Legacy.Endpoint()
		require.NoError(t, err)
		legacyAddr = legacy.Host
		startCASTestTransport(t, servers.Legacy)
	}
	return endpoint.String(), legacyAddr
}

func startCASTestTransport(t testing.TB, srv transport.Server) {
	t.Helper()
	done := make(chan error, 1)
	go func() { done <- srv.Start(context.Background()) }()
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		require.NoError(t, srv.Stop(ctx))
		select {
		case err := <-done:
			require.NoError(t, err)
		case <-ctx.Done():
			require.FailNow(t, "CAS server did not stop")
		}
	})
}

func casTestConn(t testing.TB, addr string, useTLS bool) *grpcLib.ClientConn {
	t.Helper()
	creds := insecure.NewCredentials()
	if useTLS {
		creds = credentials.NewTLS(&tls.Config{InsecureSkipVerify: true}) //nolint:gosec // ephemeral test certificate
	}
	conn, err := grpcLib.NewClient(addr, grpcLib.WithTransportCredentials(creds))
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.Close() })
	return conn
}

func casTestToken(t testing.TB, role casJWT.Role, maxBytes int64, expires time.Duration) string {
	t.Helper()
	builder, err := casJWT.NewBuilder(casJWT.WithPrivateKey("testdata/test-key.ec.pem"), casJWT.WithIssuer("cas-transport-test"), casJWT.WithExpiration(expires))
	require.NoError(t, err)
	token, err := builder.GenerateJWT("test", "test-secret", casJWT.JWTAudience, role, maxBytes, "test-org")
	require.NoError(t, err)
	return token
}

func casTestContext(token string) (context.Context, context.CancelFunc) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	if token != "" {
		ctx = metadata.AppendToOutgoingContext(ctx, "authorization", "Bearer "+token)
	}
	return ctx, cancel
}

func casResourceName(t testing.TB, resource *v1.CASResource) string {
	t.Helper()
	var buf bytes.Buffer
	require.NoError(t, gob.NewEncoder(&buf).Encode(resource))
	return base64.StdEncoding.EncodeToString(buf.Bytes())
}

func casTestUpload(t testing.TB, client bytestream.ByteStreamClient, token, digest string, payload io.Reader, size int64) error {
	t.Helper()
	ctx, cancel := casTestContext(token)
	defer cancel()
	stream, err := client.Write(ctx)
	if err != nil {
		return err
	}
	resource := casResourceName(t, &v1.CASResource{Digest: digest, FileName: "artifact.bin"})
	chunk := make([]byte, 64<<10)
	for {
		n, readErr := payload.Read(chunk)
		if readErr != nil && !errors.Is(readErr, io.EOF) {
			return readErr
		}
		req := &bytestream.WriteRequest{ResourceName: resource, Data: chunk[:n], FinishWrite: errors.Is(readErr, io.EOF)}
		if err = stream.Send(req); err != nil {
			break
		}
		if errors.Is(readErr, io.EOF) {
			break
		}
	}
	resp, err := stream.CloseAndRecv()
	if err == nil {
		require.Equal(t, size, resp.GetCommittedSize())
	}
	return err
}

func TestSharedCASStatus(t *testing.T) {
	sharedURL, legacy := startCASTestServers(t, casTestOpts(t))
	resp, err := http.Get(sharedURL + "/statusz")
	require.NoError(t, err)
	_, err = io.Copy(io.Discard, resp.Body)
	require.NoError(t, resp.Body.Close())
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	for _, addr := range []string{sharedURL[len("http://"):], legacy} {
		conn := casTestConn(t, addr, false)
		ctx, cancel := casTestContext("")
		_, err = v1.NewStatusServiceClient(conn).Statusz(ctx, &v1.StatuszRequest{})
		require.NoError(t, err)
		stream, err := reflection.NewServerReflectionClient(conn).ServerReflectionInfo(ctx)
		require.NoError(t, err)
		require.NoError(t, stream.Send(&reflection.ServerReflectionRequest{MessageRequest: &reflection.ServerReflectionRequest_ListServices{}}))
		result, err := stream.Recv()
		require.NoError(t, err)
		require.NotEmpty(t, result.GetListServicesResponse().GetService())
		require.NoError(t, stream.CloseSend())
		cancel()
	}
}

func TestCASByteStreamTransportParity(t *testing.T) {
	for _, useTLS := range []bool{false, true} {
		t.Run(map[bool]string{false: "h2c", true: "TLS"}[useTLS], func(t *testing.T) {
			for _, endpoint := range []string{casSharedEndpoint, casLegacyEndpoint} {
				t.Run(endpoint, func(t *testing.T) {
					opts := casTestOpts(t)
					if useTLS {
						opts.ServerConfig.Http.TlsConfig = casTestTLS(t)
						opts.ServerConfig.Grpc.TlsConfig = casTestTLS(t)
					}
					sharedURL, legacy := startCASTestServers(t, opts)
					addr := legacy
					if endpoint == casSharedEndpoint {
						addr = sharedURL[len("http://"):]
						if useTLS {
							addr = sharedURL[len("https://"):]
						}
					}
					conn := casTestConn(t, addr, useTLS)
					client := bytestream.NewByteStreamClient(conn)
					content := bytes.Repeat([]byte("streamed artifact\n"), 64<<10)
					sum := sha256.Sum256(content)
					digest := hex.EncodeToString(sum[:])
					up := casTestToken(t, casJWT.Uploader, 0, time.Hour)
					down := casTestToken(t, casJWT.Downloader, 0, time.Hour)
					require.NoError(t, casTestUpload(t, client, up, digest, bytes.NewReader(content), int64(len(content))))
					ctx, cancel := casTestContext(down)
					defer cancel()
					stream, err := client.Read(ctx, &bytestream.ReadRequest{ResourceName: digest})
					require.NoError(t, err)
					hash := sha256.New()
					var size int64
					for {
						msg, err := stream.Recv()
						if errors.Is(err, io.EOF) {
							break
						}
						require.NoError(t, err)
						n, err := hash.Write(msg.GetData())
						require.NoError(t, err)
						size += int64(n)
					}
					require.EqualValues(t, len(content), size)
					require.Equal(t, digest, hex.EncodeToString(hash.Sum(nil)))
					_, err = v1.NewResourceServiceClient(conn).Describe(ctx, &v1.ResourceServiceDescribeRequest{Digest: digest})
					require.NoError(t, err)
					_, err = v1.NewResourceServiceClient(conn).Describe(ctx, &v1.ResourceServiceDescribeRequest{})
					require.Equal(t, codes.InvalidArgument, status.Code(err))

					// The HTTP download remains usable regardless of which RPC listener uploaded the blob.
					// HTTP/1.1 only (D-017/R-009): the HTTP side of the shared
					// endpoint does not take plain HTTP/2 connections.
					tr := &http.Transport{Protocols: new(http.Protocols), TLSClientConfig: &tls.Config{InsecureSkipVerify: true}} //nolint:gosec // test certificate
					tr.Protocols.SetHTTP1(true)
					defer tr.CloseIdleConnections()
					req, err := http.NewRequestWithContext(ctx, http.MethodGet, sharedURL+"/download/sha256:"+digest, nil)
					require.NoError(t, err)
					req.Header.Set("Authorization", "Bearer "+down)
					response, err := (&http.Client{Transport: tr, Timeout: 5 * time.Second}).Do(req)
					require.NoError(t, err)
					require.Equal(t, http.StatusOK, response.StatusCode)
					hash.Reset()
					size, err = io.Copy(hash, response.Body)
					require.NoError(t, response.Body.Close())
					require.NoError(t, err)
					require.EqualValues(t, len(content), size)
					require.Equal(t, digest, hex.EncodeToString(hash.Sum(nil)))
				})
			}
		})
	}
}

func casTestTLS(t testing.TB) *conf.Server_TLS {
	t.Helper()
	srv := httptest.NewTLSServer(http.NotFoundHandler())
	defer srv.Close()
	pair := srv.TLS.Certificates[0]
	key, err := x509.MarshalPKCS8PrivateKey(pair.PrivateKey)
	require.NoError(t, err)
	dir := t.TempDir()
	certPath, keyPath := filepath.Join(dir, "cert.pem"), filepath.Join(dir, "key.pem")
	require.NoError(t, os.WriteFile(certPath, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: pair.Certificate[0]}), 0600))
	require.NoError(t, os.WriteFile(keyPath, pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: key}), 0600))
	return &conf.Server_TLS{Certificate: certPath, PrivateKey: keyPath}
}

// Each protocol uses its own connections through the shared port (D-017).
func TestCASSharedProtocols(t *testing.T) {
	for _, useTLS := range []bool{false, true} {
		t.Run(map[bool]string{false: "h2c", true: "TLS"}[useTLS], func(t *testing.T) {
			opts := casTestOpts(t)
			if useTLS {
				opts.ServerConfig.Http.TlsConfig = casTestTLS(t)
			}
			sharedURL, _ := startCASTestServers(t, opts)

			perform := func(kind string) {
				tr := &http.Transport{Protocols: new(http.Protocols), TLSClientConfig: &tls.Config{InsecureSkipVerify: true}} //nolint:gosec // test certificate
				if kind == casGRPCRequest {
					// Native gRPC arrives over HTTP/2, as the gRPC upstream
					// pool of a configured ingress dials it.
					tr.Protocols.SetHTTP2(true)
					tr.Protocols.SetUnencryptedHTTP2(true)
				} else {
					// D-017/R-009: the HTTP side of the shared endpoint
					// supports HTTP/1.1 only. A plain HTTP/2 client
					// acknowledges the matcher's injected SETTINGS frame,
					// and net/http rejects that acknowledgement (GOAWAY,
					// PROTOCOL_ERROR). The HTTP upstream pool therefore dials
					// HTTP/1.1 (D-018).
					tr.Protocols.SetHTTP1(true)
				}
				defer tr.CloseIdleConnections()
				client := &http.Client{Transport: tr, Timeout: 5 * time.Second}

				path, method := "/cas.v1.StatusService/Statusz", http.MethodPost
				if kind == casHTTPRequest {
					path, method = "/statusz", http.MethodGet
				}
				if kind == casPreflightRequest {
					path, method = "/download/sha256:abc", http.MethodOptions
				}
				req, err := http.NewRequest(method, sharedURL+path, bytes.NewReader([]byte{0, 0, 0, 0, 0}))
				require.NoError(t, err)
				if kind == casGRPCRequest {
					req.Header.Set("Content-Type", "application/grpc")
					req.Header.Set("Te", "trailers")
				}
				if kind == casPreflightRequest {
					req.Header.Set("Origin", "https://example.test")
					req.Header.Set("Access-Control-Request-Method", "GET")
				}
				resp, err := client.Do(req)
				require.NoError(t, err)
				_, err = io.Copy(io.Discard, resp.Body)
				require.NoError(t, resp.Body.Close())
				require.NoError(t, err)
				expected := http.StatusOK
				if kind == casPreflightRequest {
					expected = http.StatusNoContent
				}
				require.Equal(t, expected, resp.StatusCode, kind)
				if kind == casGRPCRequest {
					require.Equal(t, 2, resp.ProtoMajor)
					require.Equal(t, "0", resp.Trailer.Get("Grpc-Status"))
				} else {
					// D-017/R-009: the HTTP side of the shared endpoint is HTTP/1.1.
					require.Equal(t, 1, resp.ProtoMajor)
				}
			}

			// Each protocol works through the shared port on its own
			// connections, as a proxy with separate upstream pools sends them.
			// A plain HTTP/2 connection cannot fall through to the HTTP side:
			// D-017 records that the matcher's SETTINGS injection makes
			// net/http reject the client's acknowledgement (GOAWAY,
			// PROTOCOL_ERROR), so mixed native-gRPC and plain-HTTP reuse of
			// one upstream HTTP/2 connection is unsupported by contract.
			perform(casGRPCRequest)
			perform(casHTTPRequest)
			perform(casPreflightRequest)
			perform(casHTTPRequest)
			perform(casGRPCRequest)
		})
	}
}

// These focused cases compare the existing middleware on both transports;
// they do not replace the complete original R-002 qualification matrix.
func TestCASAuthenticationAndUploadErrors(t *testing.T) {
	for _, endpoint := range []string{casSharedEndpoint, casLegacyEndpoint} {
		t.Run(endpoint, func(t *testing.T) {
			opts := casTestOpts(t)
			sharedURL, legacy := startCASTestServers(t, opts)
			addr := legacy
			if endpoint == casSharedEndpoint {
				addr = sharedURL[len("http://"):]
			}
			conn := casTestConn(t, addr, false)
			client := bytestream.NewByteStreamClient(conn)
			content := []byte("an authenticated CAS artifact")
			sum := sha256.Sum256(content)
			digest := hex.EncodeToString(sum[:])
			tests := []struct {
				name, token, digest string
				code                codes.Code
			}{
				{name: "missing token", digest: digest, code: codes.Unauthenticated},
				{name: "malformed token", token: "invalid.jwt.token", digest: digest, code: codes.Unauthenticated},
				{name: casExpiredTokenCase, token: casTestToken(t, casJWT.Uploader, 0, -time.Hour), digest: digest, code: codes.Unauthenticated},
				{name: "downloader cannot upload", token: casTestToken(t, casJWT.Downloader, 0, time.Hour), digest: digest, code: codes.Unauthenticated},
				{name: "upload size cap", token: casTestToken(t, casJWT.Uploader, 1, time.Hour), digest: digest, code: codes.ResourceExhausted},
				{name: "digest mismatch", token: casTestToken(t, casJWT.Uploader, 0, time.Hour), digest: hex.EncodeToString(make([]byte, sha256.Size)), code: codes.InvalidArgument},
			}
			for _, tc := range tests {
				t.Run(tc.name, func(t *testing.T) {
					err := casTestUpload(t, client, tc.token, tc.digest, bytes.NewReader(content), int64(len(content)))
					require.Equal(t, tc.code, status.Code(err), "%v", err)
				})
			}
			for _, tc := range []struct {
				name, token string
				code        codes.Code
			}{
				{name: "missing token", code: codes.Unauthenticated},
				{name: casExpiredTokenCase, token: casTestToken(t, casJWT.Downloader, 0, -time.Hour), code: codes.Unauthenticated},
				{name: "uploader cannot download", token: casTestToken(t, casJWT.Uploader, 0, time.Hour), code: codes.Unauthenticated},
			} {
				t.Run("read/"+tc.name, func(t *testing.T) {
					ctx, cancel := casTestContext(tc.token)
					defer cancel()
					stream, err := client.Read(ctx, &bytestream.ReadRequest{ResourceName: digest})
					require.NoError(t, err)
					_, err = stream.Recv()
					require.Equal(t, tc.code, status.Code(err), "%v", err)
				})
			}
		})
	}
}

func TestCASLegacyListenerAndTLSConfiguration(t *testing.T) {
	for _, tc := range []struct {
		name       string
		enabled    *bool
		wantLegacy bool
	}{
		{name: "default", wantLegacy: true},
		{name: "explicit on", enabled: new(true), wantLegacy: true},
		{name: "explicit off", enabled: new(false)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			opts := casTestOpts(t)
			opts.ServerConfig.LegacyGrpcListener = tc.enabled
			if !tc.wantLegacy {
				opts.ServerConfig.Grpc.TlsConfig = &conf.Server_TLS{Certificate: "unused-invalid-cert", PrivateKey: "unused-invalid-key"}
			}
			sharedURL, legacy := startCASTestServers(t, opts)
			require.Equal(t, tc.wantLegacy, legacy != "")
			conn := casTestConn(t, sharedURL[len("http://"):], false)
			ctx, cancel := casTestContext("")
			defer cancel()
			_, err := v1.NewStatusServiceClient(conn).Statusz(ctx, &v1.StatuszRequest{})
			require.NoError(t, err)
		})
	}
	for _, tc := range []struct {
		name    string
		http    *conf.Server_HTTP
		wantErr string
	}{
		{name: "missing shared address", http: &conf.Server_HTTP{}, wantErr: "server.http.addr must be set"},
		{name: "partial shared TLS", http: &conf.Server_HTTP{Addr: casAnyAddr, TlsConfig: &conf.Server_TLS{Certificate: "only-cert"}}, wantErr: "incomplete TLS configuration"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			opts := casTestOpts(t)
			opts.ServerConfig.Http = tc.http
			opts.ServerConfig.LegacyGrpcListener = new(false)
			servers, err := NewGRPCServers(opts)
			require.NoError(t, err)
			defer servers.Handler.Server.Stop()
			_, err = NewHTTPServer(opts, servers)
			require.ErrorContains(t, err, tc.wantErr)
		})
	}
}

func TestCASCancelledUpload(t *testing.T) {
	for _, endpoint := range []string{casSharedEndpoint, casLegacyEndpoint} {
		t.Run(endpoint, func(t *testing.T) {
			opts := casTestOpts(t)
			sharedURL, legacy := startCASTestServers(t, opts)
			addr := legacy
			if endpoint == casSharedEndpoint {
				addr = sharedURL[len("http://"):]
			}
			client := bytestream.NewByteStreamClient(casTestConn(t, addr, false))
			ctx, cancel := casTestContext(casTestToken(t, casJWT.Uploader, 0, time.Hour))
			defer cancel()
			stream, err := client.Write(ctx)
			require.NoError(t, err)
			digest := hex.EncodeToString(make([]byte, sha256.Size))
			require.NoError(t, stream.Send(&bytestream.WriteRequest{ResourceName: casResourceName(t, &v1.CASResource{Digest: digest}), Data: make([]byte, 64<<10)}))
			cancel()
			_, err = stream.CloseAndRecv()
			require.Equal(t, codes.Canceled, status.Code(err))
			// No partial blob reaches the backend after cancellation.
			p, err := opts.Providers["test"].FromCredentials(context.Background(), "test-secret")
			require.NoError(t, err)
			require.Eventually(t, func() bool {
				exists, err := p.Exists(context.Background(), digest)
				return err == nil && !exists
			}, time.Second, 10*time.Millisecond)
		})
	}
}

func TestCASSharedShutdownDrainsUpload(t *testing.T) {
	opts := casTestOpts(t)
	opts.ServerConfig.LegacyGrpcListener = new(false)
	provider := opts.Providers["test"].(*mocks.Provider)
	entered, release := make(chan struct{}), make(chan struct{})
	unblock := sync.OnceFunc(func() { close(release) })
	defer unblock()
	provider.ExpectedCalls[0].Run(func(mock.Arguments) { close(entered); <-release })
	servers, err := NewGRPCServers(opts)
	require.NoError(t, err)
	api, err := NewHTTPServer(opts, servers)
	require.NoError(t, err)
	endpoint, err := api.Endpoint()
	require.NoError(t, err)
	startCASTestTransport(t, api)
	client := bytestream.NewByteStreamClient(casTestConn(t, endpoint.Host, false))
	ctx, cancel := casTestContext(casTestToken(t, casJWT.Uploader, 0, time.Hour))
	defer cancel()
	content := make([]byte, 64<<10)
	sum := sha256.Sum256(content)
	stream, err := client.Write(ctx)
	require.NoError(t, err)
	require.NoError(t, stream.Send(&bytestream.WriteRequest{ResourceName: casResourceName(t, &v1.CASResource{Digest: hex.EncodeToString(sum[:])}), Data: content}))
	select {
	case <-entered:
	case <-ctx.Done():
		require.FailNow(t, "upload did not reach the backend lookup")
	}
	stopped := make(chan error, 1)
	go func() {
		stopCtx, stopCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer stopCancel()
		stopped <- api.Stop(stopCtx)
	}()
	select {
	case err := <-stopped:
		require.FailNow(t, "shutdown returned before the upload completed", "%v", err)
	case <-time.After(50 * time.Millisecond):
	}
	unblock()
	require.NoError(t, stream.Send(&bytestream.WriteRequest{ResourceName: casResourceName(t, &v1.CASResource{Digest: hex.EncodeToString(sum[:])}), FinishWrite: true}))
	response, err := stream.CloseAndRecv()
	require.NoError(t, err)
	require.EqualValues(t, len(content), response.GetCommittedSize())
	select {
	case err := <-stopped:
		require.NoError(t, err)
	case <-ctx.Done():
		require.FailNow(t, "shutdown did not complete after the upload")
	}
}

// Slow clients must not survive shutdown while cmux is still reading their first request.
func TestCASSharedShutdownClosesUnclassifiedConnections(t *testing.T) {
	for _, useTLS := range []bool{false, true} {
		t.Run(map[bool]string{false: "plaintext", true: "TLS handshake"}[useTLS], func(t *testing.T) {
			opts := casTestOpts(t)
			opts.ServerConfig.LegacyGrpcListener = new(false)
			if useTLS {
				opts.ServerConfig.Http.TlsConfig = casTestTLS(t)
			}
			servers, err := NewGRPCServers(opts)
			require.NoError(t, err)
			api, err := NewHTTPServer(opts, servers)
			require.NoError(t, err)
			endpoint, err := api.Endpoint()
			require.NoError(t, err)
			startCASTestTransport(t, api)
			conn, err := net.Dial("tcp", endpoint.Host)
			require.NoError(t, err)
			defer conn.Close()
			if !useTLS {
				_, err = conn.Write([]byte("PRI "))
				require.NoError(t, err)
			}
			// Allow the connection to reach the classifier, including a TLS handshake.
			time.Sleep(50 * time.Millisecond)
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()
			require.NoError(t, api.Stop(ctx))
			require.NoError(t, conn.SetReadDeadline(time.Now().Add(200*time.Millisecond)))
			var data [1]byte
			_, err = conn.Read(data[:])
			require.ErrorIs(t, err, io.EOF, "shutdown must close a socket still waiting for classification")
		})
	}
}

func TestCASSharedInitializationDoesNotBindOnInvalidKey(t *testing.T) {
	opts := casTestOpts(t)
	opts.ServerConfig.LegacyGrpcListener = new(false)
	servers, err := NewGRPCServers(opts)
	require.NoError(t, err)
	defer func() { require.NoError(t, servers.Handler.Stop(context.Background())) }()
	reserved, err := net.Listen("tcp", casAnyAddr)
	require.NoError(t, err)
	opts.ServerConfig.Http.Addr = reserved.Addr().String()
	require.NoError(t, reserved.Close())
	opts.AuthConfig.PublicKeyPath = "testdata/missing-public-key"
	_, err = NewHTTPServer(opts, servers)
	require.Error(t, err)
	retry, err := net.Listen("tcp", opts.ServerConfig.Http.Addr)
	require.NoError(t, err, "failed initialization must leave the listener address available")
	require.NoError(t, retry.Close())
}
