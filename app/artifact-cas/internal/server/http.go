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
	"net/url"
	"sync"
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

	"github.com/soheilhy/cmux"
)

// APIServer owns the shared CAS listener on server.http.addr. cmux reads the
// first request of each new connection and hands the whole connection to one
// of the two existing transports:
//
//	native gRPC (HTTP/2 + application/grpc*) -> grpc.Server.Serve (native transport)
//	everything else                          -> the existing Kratos HTTP server
//
// ByteStream therefore keeps the grpc-go native transport, so large uploads
// stream with bounded memory exactly as they do on the legacy listener. This
// follows the separate-connection contract in D-017: native gRPC and HTTP
// use separate connections, with HTTP/1.1 on the HTTP side.
type APIServer struct {
	lis     net.Listener
	mux     cmux.CMux
	grpcL   net.Listener
	httpL   net.Listener
	grpcSrv *grpc.Server
	httpSrv *http.Server
	scheme  string
	workers sync.WaitGroup
}

// Start serves the shared endpoint until ctx is cancelled or a transport
// fails. The HTTP server serves the connections cmux matched to it, the
// native gRPC server serves its own matched connections, and cmux keeps
// accepting new ones. Each goroutine reports through errCh; an orderly
// Stop closes the listener and all three return ordinary shutdown errors.
func (s *APIServer) Start(ctx context.Context) error {
	errCh := make(chan error, 3)
	s.workers.Go(func() { errCh <- s.httpSrv.Start(ctx) })
	s.workers.Go(func() { errCh <- s.grpcSrv.Serve(s.grpcL) })
	s.workers.Go(func() { errCh <- s.mux.Serve() })
	select {
	case <-ctx.Done():
		return nil
	case err := <-errCh:
		if isOrdinaryStopError(err) {
			return nil
		}
		return err
	}
}

// Stop closes pending classification sockets, then drains both transports.
func (s *APIServer) Stop(ctx context.Context) error {
	// Stop accepting new connections on the shared endpoint. cmux.Close only
	// marks the match listeners closed: active connections keep draining.
	closeErr := s.lis.Close()
	s.mux.Close()
	httpErr := s.httpSrv.Stop(ctx)
	// http.Server.Shutdown closes its listener, which is the cmux mux
	// listener wrapping the root listener this Stop already closed. That
	// double close surfaces net.ErrClosed; it is not a serving failure.
	if httpErr != nil && errors.Is(httpErr, net.ErrClosed) {
		httpErr = nil
	}
	grpcErr := s.grpcSrv.Stop(ctx)
	s.workers.Wait()
	if closeErr != nil && !errors.Is(closeErr, net.ErrClosed) {
		return errors.Join(closeErr, httpErr, grpcErr)
	}
	return errors.Join(httpErr, grpcErr)
}

// Endpoint returns the real bound address of the shared endpoint.
func (s *APIServer) Endpoint() (*url.URL, error) {
	return &url.URL{Scheme: s.scheme, Host: s.lis.Addr().String()}, nil
}

// NewHTTPServer binds the shared CAS endpoint and classifies each new
// connection by its first request through cmux.
func NewHTTPServer(settings *Opts, grpcServers *GRPCServers) (*APIServer, error) {
	c, authConf := settings.ServerConfig, settings.AuthConfig
	downloadSvc, providers, logger := settings.DownloadSvc, settings.Providers, settings.Logger
	if c.GetHttp().GetAddr() == "" {
		return nil, errors.New("server.http.addr must be set for the shared CAS endpoint")
	}
	tlsConfig, err := loadServerTLSConfig(c.GetHttp().GetTlsConfig())
	if err != nil {
		return nil, err
	}

	// Validate before binding, so initialization errors do not leak a listener.
	publicKey, err := parsePublicKey(authConf, logger)
	if err != nil {
		return nil, err
	}

	network := c.Http.GetNetwork()
	if network == "" {
		network = "tcp"
	}
	lis, err := net.Listen(network, c.Http.GetAddr())
	if err != nil {
		return nil, fmt.Errorf("binding the shared CAS endpoint %q: %w", c.Http.GetAddr(), err)
	}
	lis = &classificationListener{Listener: lis, pending: make(map[*classificationConn]*time.Timer), timeout: classificationTimeout}
	scheme := "http"
	if tlsConfig != nil {
		// Terminate TLS before cmux so one certificate covers both
		// transports, with ALPN offering HTTP/2 and HTTP/1.1.
		lis = tls.NewListener(lis, tlsConfig)
		scheme = "https"
	}

	mux := cmux.New(lis)
	mux.SetReadTimeout(classificationTimeout)
	// Native gRPC: match HTTP/2 connections whose first request declares a
	// native gRPC content type (application/grpc, application/grpc+proto,
	// ...). The SETTINGS-writing writer unblocks gRPC clients, which send
	// their preface and SETTINGS and then wait for the server's SETTINGS
	// before their first HEADERS frame (Argo CD ships this matcher for the
	// same reason). Once it writes, the connection belongs to the native
	// gRPC server: cmux documents that it cannot be handed to the HTTP
	// server afterwards. CAS has no gRPC-Web endpoint (spec D-011), so no
	// application/grpc-web* exclusion is needed.
	grpcL := mux.MatchWithWriters(cmux.HTTP2MatchHeaderFieldPrefixSendSettings("content-type", "application/grpc"))
	// Other connections go to the HTTP server, which supports HTTP/1.1 only.
	// Plain HTTP/2 and mixed connections are outside the D-017 contract.
	httpL := mux.Match(cmux.Any())

	var opts = []http.ServerOption{
		// The HTTP server never binds its own socket: it serves the
		// connections cmux matched to it on the shared listener.
		http.Listener(httpL),
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

	srv.Protocols = new(h.Protocols)
	srv.Protocols.SetHTTP1(true)
	// Bound HTTP headers after handoff; streaming responses have no deadline.
	srv.ReadHeaderTimeout = 60 * time.Second

	log.NewHelper(logger).Infof("[shared] CAS endpoint (%s) listening on: %s", scheme, lis.Addr().String())

	return &APIServer{
		lis:     lis,
		mux:     mux,
		grpcL:   grpcL,
		httpL:   httpL,
		grpcSrv: grpcServers.Handler,
		httpSrv: srv,
		scheme:  scheme,
	}, nil
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
		errors.Is(err, cmux.ErrServerClosed) ||
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
