//
// Copyright 2023-2026 The Chainloop Authors.
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
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	h "net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	api "github.com/chainloop-dev/chainloop/app/artifact-cas/api/cas/v1"
	"github.com/chainloop-dev/chainloop/app/artifact-cas/internal/conf"
	"github.com/chainloop-dev/chainloop/app/artifact-cas/internal/service"
	casJWT "github.com/chainloop-dev/chainloop/internal/robotaccount/cas"
	middlewares_http "github.com/chainloop-dev/chainloop/pkg/middlewares/http"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/go-kratos/kratos/v2/middleware/logging"
	"github.com/go-kratos/kratos/v2/middleware/recovery"
	"github.com/go-kratos/kratos/v2/transport/grpc"
	"github.com/go-kratos/kratos/v2/transport/http"
	"github.com/golang-jwt/jwt/v5"
	grpcLib "google.golang.org/grpc"
)

// APIServer owns the shared CAS listener on server.http.addr. The standard
// HTTP server owns every connection and dispatches each request:
//
//	native gRPC (HTTP/2 + application/grpc*) -> httputil.ReverseProxy ->
//	  the private native gRPC server (grpc.Server.Serve, native transport)
//	everything else                         -> the existing Kratos HTTP routes
//
// ByteStream keeps the native transport: the proxy streams the request body
// into the private connection, whose HTTP/2 flow-control window paces the
// proxy's reads, and net/http's own window paces the client. No unbounded
// read-ahead exists anywhere on the path, in either direction. Mixed gRPC and
// ordinary HTTP requests may share one upstream connection in any order, as
// they do on the control plane (spec D-020).
type APIServer struct {
	lis      net.Listener
	httpSrv  *http.Server
	grpcSrv  *grpc.Server
	scheme   string
	workers  sync.WaitGroup
	stopping atomic.Bool
}

// Start serves the shared endpoint until ctx is cancelled or a transport
// fails. The HTTP server serves every connection; the private native gRPC
// server serves the connections the proxy opens to it. A genuine failure of
// either transport — including the private gRPC listener failing — is
// surfaced so the whole endpoint stops, rather than serving HTTP while
// proxied gRPC answers 502.
func (s *APIServer) Start(ctx context.Context) error {
	errCh := make(chan error, 2)
	s.workers.Go(func() { errCh <- s.httpSrv.Start(ctx) })
	s.workers.Go(func() { errCh <- s.grpcSrv.Start(ctx) })
	select {
	case <-ctx.Done():
		return nil
	case err := <-errCh:
		// The ordinary-stop sentinel (ErrClosed / ErrServerStopped) is benign
		// only when it comes from our own Stop. The same error from a
		// spontaneously failed transport must propagate, or the app would keep
		// serving HTTP after the private gRPC server is gone.
		if s.stopping.Load() && isOrdinaryStopError(err) {
			return nil
		}
		return err
	}
}

// Stop drains the HTTP server first — which includes every proxied gRPC
// stream, because those live on HTTP connections — then stops the private
// native gRPC server so its in-flight RPCs finish.
func (s *APIServer) Stop(ctx context.Context) error {
	s.stopping.Store(true)
	httpErr := s.httpSrv.Stop(ctx)
	grpcErr := s.grpcSrv.Stop(ctx)
	s.workers.Wait()
	return errors.Join(httpErr, grpcErr)
}

// Endpoint returns the real bound address of the shared endpoint.
func (s *APIServer) Endpoint() (*url.URL, error) {
	return &url.URL{Scheme: s.scheme, Host: s.lis.Addr().String()}, nil
}

// NewHTTPServer binds the shared CAS endpoint and dispatches every request.
// When it returns an error, it has closed the private gRPC socket it would
// have consumed, so a failed construction does not leak it.
func NewHTTPServer(settings *Opts, grpcServers *GRPCServers) (*APIServer, error) {
	c, authConf := settings.ServerConfig, settings.AuthConfig
	downloadSvc, providers, logger := settings.DownloadSvc, settings.Providers, settings.Logger
	if c.GetHttp().GetAddr() == "" {
		_ = grpcServers.Close()
		return nil, errors.New("server.http.addr must be set for the shared CAS endpoint")
	}
	tlsConfig, err := loadServerTLSConfig(c.GetHttp().GetTlsConfig())
	if err != nil {
		_ = grpcServers.Close()
		return nil, err
	}

	// Validate before binding, so initialization errors do not leak a listener.
	publicKey, err := parsePublicKey(authConf, logger)
	if err != nil {
		_ = grpcServers.Close()
		return nil, err
	}

	network := c.Http.GetNetwork()
	if network == "" {
		network = "tcp"
	}
	lis, err := net.Listen(network, c.Http.GetAddr())
	if err != nil {
		_ = grpcServers.Close()
		return nil, fmt.Errorf("binding the shared CAS endpoint %q: %w", c.Http.GetAddr(), err)
	}
	scheme := "http"
	if tlsConfig != nil {
		// One certificate covers both transports, with ALPN offering
		// HTTP/2 and HTTP/1.1.
		lis = tls.NewListener(lis, tlsConfig)
		scheme = "https"
	}

	var opts = []http.ServerOption{
		// The HTTP server never binds its own socket: it serves the shared
		// listener this function bound.
		http.Listener(lis),
		http.Middleware(
			recovery.Recovery(),
			logging.Server(logger),
		),
	}
	if c.Http.Timeout != nil {
		opts = append(opts, http.Timeout(c.Http.Timeout.AsDuration()))
	}

	srv := http.NewServer(opts...)

	downloadHandler := middlewares_http.AuthFromHeaderOrQueryParam(loadPublicKey(publicKey), claimsFunc(), casJWT.SigningMethod, downloadSvc)
	srv.Handle(service.DownloadPath, CORSMiddleware(c.GetHttp().GetCors().GetAllowOrigins(), downloadHandler))
	api.RegisterStatusServiceHTTPServer(srv, service.NewStatusService(Version, providers))

	routes := srv.Handler
	srv.Protocols = new(h.Protocols)
	srv.Protocols.SetHTTP1(true)
	srv.Protocols.SetHTTP2(true)
	srv.Protocols.SetUnencryptedHTTP2(true)
	// Bound how long a client may take to deliver its request headers
	// (slowloris protection). Response streaming is not affected.
	srv.ReadHeaderTimeout = 60 * time.Second
	// One proxy and one transport for the process: its connection pool is
	// reused across requests. Dispatch each request: native gRPC goes through
	// the standard reverse proxy to the private native server; everything else
	// uses the existing HTTP routes. CAS has no gRPC-Web endpoint (spec
	// D-011); the grpc-web exclusion keeps the content-type check consistent
	// with the control plane.
	proxy := grpcProxy(grpcServers.PrivateAddr, logger)
	srv.Handler = h.HandlerFunc(func(w h.ResponseWriter, r *h.Request) {
		if r.ProtoMajor == 2 && isNativeGRPCContentType(r.Header.Get("Content-Type")) {
			proxy.ServeHTTP(w, r)
			return
		}
		routes.ServeHTTP(w, r)
	})

	log.NewHelper(logger).Infof("[shared] CAS endpoint (%s) listening on: %s", scheme, lis.Addr().String())
	if legacyGRPCListenerEnabled(c) {
		log.NewHelper(logger).Warnw("msg", fmt.Sprintf("the gRPC-only listener on %q is deprecated; use the shared HTTP endpoint and set server.legacy_grpc_listener to false when all clients have moved", c.GetGrpc().GetAddr()))
	}

	return &APIServer{
		lis:     lis,
		httpSrv: srv,
		grpcSrv: grpcServers.Private,
		scheme:  scheme,
	}, nil
}

// grpcProxy builds the reverse proxy that carries native gRPC requests to the
// private native gRPC server over h2c. httputil.ReverseProxy streams request
// and response bodies, forwards hop-by-hop-excluded headers (including gRPC
// metadata such as the authorization token), and copies response trailers,
// so the existing gRPC frames pass through without decoding or rebuilding.
// gRPC responses carry no Content-Length, so ReverseProxy flushes each write
// immediately through its streaming special case, which gRPC streaming
// requires. The transport speaks only unencrypted HTTP/2 (h2c prior
// knowledge): the private server is gRPC-only and loopback-only. The
// Rewrite restores the incoming authority after SetURL clears it, so the
// native server receives the original :authority for interceptor parity.
func grpcProxy(target string, logger log.Logger) *httputil.ReverseProxy {
	// Fixed for the process lifetime: built once rather than per request.
	targetURL := &url.URL{Scheme: "http", Host: target}
	transport := &h.Transport{
		Protocols:             new(h.Protocols),
		DialContext:           (&net.Dialer{Timeout: 10 * time.Second}).DialContext,
		MaxIdleConns:          100,
		MaxIdleConnsPerHost:   100,
		IdleConnTimeout:       90 * time.Second,
		ExpectContinueTimeout: time.Second,
		// Byte-exact pass-through: gRPC carries its own per-message content
		// coding, and the transport must neither add transport-level gzip
		// headers nor transparently decode the proxied body.
		DisableCompression: true,
	}
	transport.Protocols.SetUnencryptedHTTP2(true)
	transport.Protocols.SetHTTP1(false)
	return &httputil.ReverseProxy{
		Rewrite: func(pr *httputil.ProxyRequest) {
			// SetURL only reads the target, so one shared URL is safe across
			// concurrent requests.
			pr.SetURL(targetURL)
			// SetURL clears Out.Host for security (RFC 7230 host injection),
			// but gRPC carries the routing target in :authority. Restore the
			// incoming authority so the native server's interceptors observe
			// the same value they would on a direct connection.
			pr.Out.Host = pr.In.Host
		},
		Transport: transport,
		ErrorHandler: func(w h.ResponseWriter, _ *h.Request, err error) {
			log.NewHelper(logger).Errorf("shared endpoint gRPC proxy error: %v", err)
			w.WriteHeader(h.StatusBadGateway)
		},
	}
}

// isNativeGRPCContentType reports whether a content type selects the native
// gRPC protocol family, excluding gRPC-Web. CAS has no gRPC-Web endpoint, but
// the exclusion keeps the dispatch consistent with the control plane.
func isNativeGRPCContentType(contentType string) bool {
	contentType = strings.ToLower(contentType)
	return strings.HasPrefix(contentType, "application/grpc") &&
		!strings.HasPrefix(contentType, "application/grpc-web")
}

// claimsFunc returns the claims function for the JWT middleware that casts the claims to the correct type
func claimsFunc() middlewares_http.ClaimsFunc {
	return func() jwt.Claims {
		return &casJWT.Claims{}
	}
}

// isOrdinaryStopError reports whether a transport error is the expected
// result of an orderly Stop rather than a serving failure.
func isOrdinaryStopError(err error) bool {
	if err == nil {
		return true
	}
	return errors.Is(err, net.ErrClosed) ||
		errors.Is(err, h.ErrServerClosed) ||
		errors.Is(err, grpcLib.ErrServerStopped)
}

// loadServerTLSConfig applies TLS independently of the legacy gRPC listener.
func loadServerTLSConfig(c *conf.Server_TLS) (*tls.Config, error) {
	cert, key := c.GetCertificate(), c.GetPrivateKey()
	if cert == "" && key == "" {
		return nil, nil
	}
	if cert == "" || key == "" {
		return nil, errors.New("incomplete TLS configuration: certificate and private key must be set together")
	}
	pair, err := tls.LoadX509KeyPair(cert, key)
	if err != nil {
		return nil, fmt.Errorf("loading shared CAS TLS certificate: %w", err)
	}
	return &tls.Config{
		Certificates: []tls.Certificate{pair}, MinVersion: tls.VersionTLS12,
		NextProtos: []string{"h2", "http/1.1"},
	}, nil
}
