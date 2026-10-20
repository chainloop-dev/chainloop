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
	"strings"
	"sync"
	"sync/atomic"
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
	casTestFileName     = "artifact.bin"
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
	return &v1.CASResource{Digest: digest, FileName: casTestFileName, Size: info.Size()}, nil
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
	resource := casResourceName(t, &v1.CASResource{Digest: digest, FileName: casTestFileName})
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

// TestCASProxyPreservesAuthority pins the proxy Rewrite contract: the native
// server must observe the client's original :authority, the same value a
// direct connection to the legacy listener would carry, so authority-based
// interceptors and telemetry do not see the loopback dial target.
func TestCASProxyPreservesAuthority(t *testing.T) {
	opts := casTestOpts(t)
	sharedURL, _ := startCASTestServers(t, opts)
	host, port, err := net.SplitHostPort(sharedURL[len("http://"):])
	require.NoError(t, err)

	// Use an authority that differs from the dial address so a cleared Out.Host
	// would be visible in the :authority the native server receives.
	const authority = "cas.authority.test:443"
	conn, err := grpcLib.NewClient(net.JoinHostPort(host, port), grpcLib.WithAuthority(authority), grpcLib.WithTransportCredentials(insecure.NewCredentials()))
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.Close() })
	ctx, cancel := casTestContext("")
	defer cancel()
	_, err = v1.NewStatusServiceClient(conn).Statusz(ctx, &v1.StatuszRequest{})
	require.NoError(t, err)
}

// TestCASProxyErrorTerminatesHTTP2Stream pins the ErrorHandler contract: on an
// upstream failure the proxy writes only the status header, and net/http
// finalizes the HTTP/2 response with END_STREAM when the handler returns, so
// the client gets a clean 502 instead of a stream left open.
func TestCASProxyErrorTerminatesHTTP2Stream(t *testing.T) {
	proxy := grpcProxy("127.0.0.1:1", log.NewStdLogger(io.Discard)) // nothing listening
	srv := httptest.NewUnstartedServer(proxy)
	srv.EnableHTTP2 = true
	srv.StartTLS()
	defer srv.Close()

	tr := &http.Transport{Protocols: new(http.Protocols), TLSClientConfig: &tls.Config{InsecureSkipVerify: true}} //nolint:gosec // local test certificate
	tr.Protocols.SetHTTP2(true)
	defer tr.CloseIdleConnections()
	client := &http.Client{Transport: tr, Timeout: 5 * time.Second}

	req, err := http.NewRequest(http.MethodPost, srv.URL+"/cas.v1.StatusService/Statusz", bytes.NewReader([]byte{0, 0, 0, 0, 0}))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/grpc")
	req.Header.Set("Te", "trailers")
	resp, err := client.Do(req)
	require.NoError(t, err)
	// Reading the body to completion must terminate, not wait for more frames.
	_, err = io.Copy(io.Discard, resp.Body)
	require.NoError(t, resp.Body.Close())
	require.NoError(t, err)
	require.Equal(t, http.StatusBadGateway, resp.StatusCode)
	require.Equal(t, 2, resp.ProtoMajor)
}

// pacedUploads gates a ByteStream service so each direction can pause its
// consumer at a fixed transferred byte count. The hook stays inert until arm
// is closed (so a setup upload through the same service is not paused), and
// the gate wait is bounded: the hook runs on the server's stream goroutine,
// where an unbounded wait would deadlock cancellation, because the hook never
// sees the stream context. Production wiring never sets the hook.
func pacedUploads(t *testing.T, pauseAt int64) (arm func(), release chan struct{}, hook func(int64)) {
	armed, release := make(chan struct{}), make(chan struct{})
	t.Cleanup(func() { close(release) })
	return func() { close(armed) }, release, func(transferred int64) {
		select {
		case <-armed:
		default:
			return
		}
		if transferred >= pauseAt {
			select {
			case <-release:
			case <-time.After(3 * time.Second):
			}
		}
	}
}

// TestCASSharedPausedConsumerBackpressure is the durable regression for the
// read-ahead property: with the consumer paused mid-stream, the producer must
// stop advancing after bounded buffering, and cancellation must release both
// sides. A whole-stream buffer anywhere on the path (the measured failure of
// grpc.Server.ServeHTTP) would let the upload keep advancing to its full size.
func TestCASSharedPausedConsumerBackpressure(t *testing.T) {
	const pauseAt = 8 << 20 // pause once 8 MiB have transferred
	t.Run("upload", func(t *testing.T) {
		opts := casTestOpts(t)
		arm, release, hook := pacedUploads(t, pauseAt)
		opts.ByteService = service.NewByteStreamService(opts.Providers, append(serviceOptsForTest(t), service.WithPaceHook(hook))...)
		sharedURL, _ := startCASTestServers(t, opts)
		client := bytestream.NewByteStreamClient(casTestConn(t, sharedURL[len("http://"):], false))
		up := casTestToken(t, casJWT.Uploader, 0, time.Hour)

		const size = 64 << 20
		hash := sha256.New()
		_, err := io.CopyN(hash, zeroReader{}, size)
		require.NoError(t, err)
		digest := hex.EncodeToString(hash.Sum(nil))

		arm()
		ctx, cancel := casTestContext(up)
		defer cancel()
		stream, err := client.Write(ctx)
		require.NoError(t, err)
		resource := casResourceName(t, &v1.CASResource{Digest: digest, FileName: casTestFileName})
		chunk := make([]byte, 1<<20)
		var sent atomic.Int64
		sendErr := make(chan error, 1)
		go func() {
			for sent.Load() < size {
				n := min(int64(len(chunk)), size-sent.Load())
				err := stream.Send(&bytestream.WriteRequest{ResourceName: resource, Data: chunk[:n]})
				if err != nil {
					sendErr <- err
					return
				}
				sent.Add(n)
			}
			sendErr <- stream.CloseSend()
		}()

		// Wait until the consumer has absorbed pauseAt bytes, then give the
		// pipeline a moment: if any hop buffered whole streams, the client
		// would keep advancing far past pauseAt.
		require.Eventually(t, func() bool { return sent.Load() >= pauseAt }, 30*time.Second, 5*time.Millisecond)
		time.Sleep(500 * time.Millisecond)
		sentWhenPaused := sent.Load()

		// Cancellation must release the paused stream on both sides.
		cancel()
		select {
		case err := <-sendErr:
			require.Error(t, err, "the canceled upload must surface an error client-side")
		case <-time.After(10 * time.Second):
			require.FailNow(t, "cancellation did not release the paused upload")
		}

		// Assert the stall only after the stream is torn down, so a failed
		// assertion cannot leak the sender goroutine into cleanup.
		require.Less(t, sentWhenPaused, int64(size)/4,
			"an unbounded buffer let the producer advance past the paused consumer; sent %d bytes", sentWhenPaused)
		_ = release // closed by t.Cleanup
	})

	t.Run("download", func(t *testing.T) {
		opts := casTestOpts(t)
		arm, release, hook := pacedUploads(t, pauseAt)
		opts.ByteService = service.NewByteStreamService(opts.Providers, append(serviceOptsForTest(t), service.WithPaceHook(hook))...)
		sharedURL, _ := startCASTestServers(t, opts)
		client := bytestream.NewByteStreamClient(casTestConn(t, sharedURL[len("http://"):], false))
		up, down := casTestToken(t, casJWT.Uploader, 0, time.Hour), casTestToken(t, casJWT.Downloader, 0, time.Hour)

		const size = 64 << 20
		hash := sha256.New()
		_, err := io.CopyN(hash, zeroReader{}, size)
		require.NoError(t, err)
		digest := hex.EncodeToString(hash.Sum(nil))
		// Upload the fixture with the gate still inert.
		require.NoError(t, casTestUpload(t, client, up, digest, io.LimitReader(zeroReader{}, size), size))

		arm()
		ctx, cancel := casTestContext(down)
		defer cancel()
		stream, err := client.Read(ctx, &bytestream.ReadRequest{ResourceName: digest})
		require.NoError(t, err)
		var received atomic.Int64
		recvErr := make(chan error, 1)
		go func() {
			for {
				resp, err := stream.Recv()
				if err != nil {
					recvErr <- err
					return
				}
				received.Add(int64(len(resp.GetData())))
			}
		}()

		require.Eventually(t, func() bool { return received.Load() >= pauseAt }, 30*time.Second, 5*time.Millisecond)
		time.Sleep(500 * time.Millisecond)
		receivedWhenPaused := received.Load()

		cancel()
		select {
		case <-recvErr:
		case <-time.After(10 * time.Second):
			require.FailNow(t, "cancellation did not release the paused download")
		}

		require.Less(t, receivedWhenPaused, int64(size)/4,
			"an unbounded buffer let the server stream past the paused consumer; received %d bytes", receivedWhenPaused)
		_ = release // closed by t.Cleanup
	})
}

// serviceOptsForTest mirrors the test wiring in casTestOpts so pace-hook
// variants get the same staging option.
func serviceOptsForTest(tb testing.TB) []service.NewOpt {
	return []service.NewOpt{service.WithStagingDir(tb.TempDir())}
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
					// The download uses HTTP/1.1: the shared endpoint serves
					// plain HTTP over HTTP/1.1 and HTTP/2 (D-020), and the
					// legacy behavior over HTTP/1.1 must keep working.
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

// Native gRPC and ordinary HTTP must work through the shared port (D-020).
func TestCASSharedProtocols(t *testing.T) {
	for _, useTLS := range []bool{false, true} {
		t.Run(map[bool]string{false: "h2c", true: "TLS"}[useTLS], func(t *testing.T) {
			opts := casTestOpts(t)
			if useTLS {
				opts.ServerConfig.Http.TlsConfig = casTestTLS(t)
			}
			sharedURL, _ := startCASTestServers(t, opts)

			var connections atomic.Int32
			// One catch-all upstream connection that carries both protocols, as
			// a single-location ingress with an h2c (or TLS h2) upstream sends
			// them. HTTP/1 must stay off in the client: net/http only uses h2c
			// prior knowledge when HTTP/1 is disabled.
			tr := &http.Transport{Protocols: new(http.Protocols), TLSClientConfig: &tls.Config{InsecureSkipVerify: true}} //nolint:gosec // test certificate
			tr.Protocols.SetHTTP2(true)
			tr.Protocols.SetUnencryptedHTTP2(true)
			tr.Protocols.SetHTTP1(false)
			tr.DialContext = func(ctx context.Context, network, addr string) (net.Conn, error) {
				connections.Add(1)
				return (&net.Dialer{}).DialContext(ctx, network, addr)
			}
			defer tr.CloseIdleConnections()
			client := &http.Client{Transport: tr, Timeout: 5 * time.Second}

			perform := func(kind string) {
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
					// Native gRPC requires HTTP/2 and its trailers round-trip
					// through the reverse proxy to the private native server.
					require.Equal(t, 2, resp.ProtoMajor)
					require.Equal(t, "0", resp.Trailer.Get("Grpc-Status"))
				}
			}

			// D-020: the shared endpoint dispatches each request, so native
			// gRPC and ordinary HTTP interleave in any order — including on
			// one pooled upstream HTTP/2 connection, as a single-location
			// ingress sends them.
			perform(casGRPCRequest)
			perform(casHTTPRequest)
			perform(casGRPCRequest)
			perform(casPreflightRequest)
			perform(casHTTPRequest)
			perform(casGRPCRequest)

			// All six requests reused one HTTP/2 connection.
			require.EqualValues(t, 1, connections.Load())
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

// TestCASLegacyListenerDeprecationWarning covers the R-006 startup warning:
// the shared endpoint logs a deprecation warning when the legacy gRPC
// listener is enabled, matching the control plane.
func TestCASLegacyListenerDeprecationWarning(t *testing.T) {
	for _, tc := range []struct {
		name    string
		enabled *bool
		wantLog bool
	}{
		{name: "default on", wantLog: true},
		{name: "explicit on", enabled: new(true), wantLog: true},
		{name: "explicit off", enabled: new(false), wantLog: false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			opts := casTestOpts(t)
			opts.ServerConfig.LegacyGrpcListener = tc.enabled
			var buf bytes.Buffer
			opts.Logger = log.NewStdLogger(&buf)
			servers, err := NewGRPCServers(opts)
			require.NoError(t, err)
			api, err := NewHTTPServer(opts, servers)
			require.NoError(t, err)
			require.Equal(t, tc.wantLog, strings.Contains(buf.String(), "the gRPC-only listener"), "logs: %s", buf.String())
			require.NoError(t, api.Stop(context.Background()))
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
			// A failed NewHTTPServer must close the private socket itself.
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

// TestCASSharedConcurrentUploadsMemoryBounded pushes several large streams
// through the shared endpoint at once, multiplexed on one connection as a
// single-location ingress sends them. The payload is generated on the fly, so
// no test-owned buffer is resident. Each hop must window-pace the data: if
// any hop buffered whole streams instead, the sampled peak heap would reach
// the total transferred bytes. The single-stream benchmark showed the pacing
// directly (about 32 MiB peak for a 1 GiB upload, against about 1.25 GiB on
// the rejected grpc.Server.ServeHTTP adapter under the same conditions); this
// test adds the concurrent-stream dimension.
func TestCASSharedConcurrentUploadsMemoryBounded(t *testing.T) {
	const (
		streams = 4
		size    = 128 << 20 // 128 MiB per stream, 512 MiB total
	)
	opts := casTestOpts(t)
	sharedURL, _ := startCASTestServers(t, opts)
	client := bytestream.NewByteStreamClient(casTestConn(t, sharedURL[len("http://"):], false))
	up := casTestToken(t, casJWT.Uploader, 0, time.Hour)

	hash := sha256.New()
	_, err := io.CopyN(hash, zeroReader{}, size)
	require.NoError(t, err)
	digest := hex.EncodeToString(hash.Sum(nil))

	stopSample := sampleCASHeap()
	var wg sync.WaitGroup
	errs := make(chan error, streams)
	for range streams {
		wg.Add(1)
		go func() {
			defer wg.Done()
			errs <- casTestUpload(t, client, up, digest, io.LimitReader(zeroReader{}, size), size)
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		require.NoError(t, err)
	}
	peak := stopSample()
	t.Logf("peak heap: %d MiB for %d MiB across %d streams", peak>>20, (size*streams)>>20, streams)

	// Whole-stream buffering on any hop would push the peak toward the
	// transferred bytes; window pacing keeps it at per-stream buffers.
	require.Less(t, peak, uint64(300<<20),
		"four concurrent 128 MiB uploads must not buffer whole streams; peak heap %d bytes", peak)
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

// TestCASSharedGRPCFailureStopsEndpoint: if the private gRPC transport fails,
// the shared endpoint must not keep serving HTTP while proxied gRPC answers
// 502. Start must surface the failure so the app stops.
func TestCASSharedGRPCFailureStopsEndpoint(t *testing.T) {
	opts := casTestOpts(t)
	opts.ServerConfig.LegacyGrpcListener = new(false)
	servers, err := NewGRPCServers(opts)
	require.NoError(t, err)
	api, err := NewHTTPServer(opts, servers)
	require.NoError(t, err)
	// Simulate the private gRPC transport failing: close its listener before
	// the servers start. grpc-go's Serve on the closed listener returns an
	// error that Start must not swallow.
	require.NoError(t, servers.Close())
	startErr := make(chan error, 1)
	go func() { startErr <- api.Start(context.Background()) }()
	select {
	case err := <-startErr:
		require.Error(t, err, "a failed private gRPC transport must fail the shared endpoint")
	case <-time.After(10 * time.Second):
		require.FailNow(t, "Start kept serving after the private gRPC transport failed")
	}
	// Best-effort teardown of the HTTP server that started on the shared port.
	_ = api.Stop(context.Background())
}

func TestCASSharedInitializationDoesNotBindOnInvalidKey(t *testing.T) {
	opts := casTestOpts(t)
	opts.ServerConfig.LegacyGrpcListener = new(false)
	servers, err := NewGRPCServers(opts)
	require.NoError(t, err)
	defer func() { require.NoError(t, servers.Private.Stop(context.Background())) }()
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
	// The failed constructor closed the private socket: connections are refused.
	require.NoError(t, servers.Close(), "Close after a failed construction is safe")
	_, err = net.DialTimeout("tcp", servers.PrivateAddr, time.Second)
	require.Error(t, err, "the private socket must refuse connections after the failed construction")
}

// TestCASPrivateSocketClosedWhenNeverStarted covers the stop-before-serve
// path: the socket must be closable and stop accepting after Close, even
// though Private.Stop alone does not close a listener registered through
// grpc.Listener before Serve runs.
func TestCASPrivateSocketClosedWhenNeverStarted(t *testing.T) {
	opts := casTestOpts(t)
	opts.ServerConfig.LegacyGrpcListener = new(false)
	servers, err := NewGRPCServers(opts)
	require.NoError(t, err)
	require.NoError(t, servers.Close())
	_, err = net.DialTimeout("tcp", servers.PrivateAddr, time.Second)
	require.Error(t, err, "the private socket must refuse connections after Close")
	require.NoError(t, servers.Close(), "Close is idempotent")
}
