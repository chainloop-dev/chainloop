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
	h "net/http"
	"strings"
	"time"

	api "github.com/chainloop-dev/chainloop/app/artifact-cas/api/cas/v1"
	"github.com/chainloop-dev/chainloop/app/artifact-cas/internal/conf"
	"github.com/chainloop-dev/chainloop/app/artifact-cas/internal/service"
	casJWT "github.com/chainloop-dev/chainloop/internal/robotaccount/cas"
	middlewares_http "github.com/chainloop-dev/chainloop/pkg/middlewares/http"
	"github.com/go-kratos/kratos/v2/middleware/logging"
	"github.com/go-kratos/kratos/v2/middleware/recovery"
	"github.com/go-kratos/kratos/v2/transport/grpc"
	"github.com/go-kratos/kratos/v2/transport/http"
	"github.com/golang-jwt/jwt/v5"
)

// APIServer drains HTTP streams before stopping the gRPC handler.
type APIServer struct {
	*http.Server
	grpcHandler *grpc.Server
}

func (s *APIServer) Stop(ctx context.Context) error {
	httpErr := s.Server.Stop(ctx)
	return errors.Join(httpErr, s.grpcHandler.Stop(ctx))
}

// NewHTTPServer dispatches every request on the existing CAS HTTP endpoint.
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
	var opts = []http.ServerOption{
		http.Middleware(
			recovery.Recovery(),
			logging.Server(logger),
		),
	}
	if c.Http.Network != "" {
		opts = append(opts, http.Network(c.Http.Network))
	}
	if c.Http.Addr != "" {
		opts = append(opts, http.Address(c.Http.Addr))
	}
	if c.Http.Timeout != nil {
		opts = append(opts, http.Timeout(c.Http.Timeout.AsDuration()))
	}

	if tlsConfig != nil {
		opts = append(opts, http.TLSConfig(tlsConfig))
	}

	// Parse the public key once on initialization instead of on every request
	// TODO: implement jwks endpoint
	publicKey, err := parsePublicKey(authConf, logger)
	if err != nil {
		return nil, err
	}

	srv := http.NewServer(opts...)

	downloadHandler := middlewares_http.AuthFromHeaderOrQueryParam(loadPublicKey(publicKey), claimsFunc(), casJWT.SigningMethod, downloadSvc)
	srv.Handle(service.DownloadPath, CORSMiddleware(c.GetHttp().GetCors().GetAllowOrigins(), downloadHandler))
	api.RegisterStatusServiceHTTPServer(srv, service.NewStatusService(Version, providers))
	shared := http.NewServer(opts...)
	shared.Protocols = new(h.Protocols)
	shared.Protocols.SetHTTP1(true)
	shared.Protocols.SetHTTP2(true)
	shared.Protocols.SetUnencryptedHTTP2(true)
	shared.ReadHeaderTimeout = 60 * time.Second
	shared.Handler = h.HandlerFunc(func(w h.ResponseWriter, r *h.Request) {
		contentType := strings.ToLower(r.Header.Get("Content-Type"))
		if r.ProtoMajor == 2 && strings.HasPrefix(contentType, "application/grpc") && !strings.HasPrefix(contentType, "application/grpc-web") {
			grpcServers.Handler.ServeHTTP(w, r)
			return
		}
		srv.ServeHTTP(w, r)
	})
	return &APIServer{Server: shared, grpcHandler: grpcServers.Handler}, nil
}

// claimsFunc returns the claims function for the JWT middleware that casts the claims to the correct type
func claimsFunc() middlewares_http.ClaimsFunc {
	return func() jwt.Claims {
		return &casJWT.Claims{}
	}
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
