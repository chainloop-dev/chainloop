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
	"crypto/tls"
	"fmt"
	"net"

	conf "github.com/chainloop-dev/chainloop/app/controlplane/internal/conf/controlplane/config/v1"
	"github.com/chainloop-dev/chainloop/pkg/httpgrpcmux"
	"github.com/go-kratos/kratos/v2/log"
)

// NewAPIMux creates the connection multiplexer that serves gRPC, gRPC-Web and
// HTTP from the single HTTP port, as specified in
// docs/specs/issue-3628-single-endpoint.md. The mux owns the listener on the
// current HTTP address, so the HTTP-facing clients and hostnames keep working
// unchanged. When the operator configures TLS, the listener terminates it
// before the classifier and offers HTTP/2 and HTTP/1.1 through ALPN, so one
// certificate covers all protocols.
//
// The returned cleanup closes the mux and its listener; wire runs it when a
// later provider fails or on shutdown.
func NewAPIMux(c *conf.Server, logger log.Logger) (*httpgrpcmux.Mux, func(), error) {
	httpCfg := c.GetHttp()
	if httpCfg.GetAddr() == "" {
		// An empty address would silently bind an ephemeral port, and the
		// shared API would not be where operators expect it.
		return nil, nil, fmt.Errorf("server.http.addr must be set: the single port cannot bind an empty address")
	}
	network := httpCfg.GetNetwork()
	if network == "" {
		network = "tcp"
	}
	l, err := net.Listen(network, httpCfg.GetAddr())
	if err != nil {
		return nil, nil, fmt.Errorf("listening on the single port %q: %w", httpCfg.GetAddr(), err)
	}

	var raw = l
	tlsEnabled := false
	if tlsConf, err := loadServerTLSConfig(httpCfg.GetTlsConfig()); err != nil {
		_ = l.Close()
		return nil, nil, err
	} else if tlsConf != nil {
		raw = tls.NewListener(l, tlsConf)
		tlsEnabled = true
	}

	mux := httpgrpcmux.New(raw, &httpgrpcmux.Options{Logger: logger})
	helper := log.NewHelper(logger)
	helper.Infow("msg", "serving gRPC, gRPC-Web and HTTP from the single port", "addr", httpCfg.GetAddr(), "tls", tlsEnabled)
	if legacyGRPCListenerEnabled(c) {
		helper.Warnw("msg", fmt.Sprintf("the gRPC-only listener on %q is deprecated; it will be disabled by default in a future release and removed afterwards. Set server.legacy_grpc_listener to false to listen on the single port only", c.GetGrpc().GetAddr()))
	}

	return mux, mux.Close, nil
}

// loadServerTLSConfig builds a tls.Config from the certificate and private key
// paths of the server configuration. It returns nil when no TLS is configured.
// For the single port the config negotiates HTTP/2 and HTTP/1.1 through ALPN.
func loadServerTLSConfig(tlsConf *conf.Server_TLS) (*tls.Config, error) {
	if tlsConf == nil {
		return nil, nil
	}
	cert := tlsConf.GetCertificate()
	privKey := tlsConf.GetPrivateKey()
	switch {
	case cert == "" && privKey == "":
		return nil, nil
	case cert == "" || privKey == "":
		// A partial configuration is a typo: refuse it instead of silently
		// exposing the endpoint over plaintext.
		return nil, fmt.Errorf("incomplete TLS configuration: certificate and private key must be set together")
	}
	certPair, err := tls.LoadX509KeyPair(cert, privKey)
	if err != nil {
		return nil, fmt.Errorf("loading server TLS certificate: %w", err)
	}
	return &tls.Config{
		Certificates: []tls.Certificate{certPair},
		MinVersion:   tls.VersionTLS12,
		// Serve gRPC (HTTP/2) and HTTP/1.1 from the same certificate.
		NextProtos: []string{"h2", "http/1.1"},
	}, nil
}

// legacyGRPCListenerEnabled reports whether the deprecated gRPC-only listener
// must stay on. It defaults to true when unset, so that existing
// installations and the Helm chart keep working during the deprecation period.
func legacyGRPCListenerEnabled(c *conf.Server) bool {
	if c.LegacyGrpcListener != nil {
		return *c.LegacyGrpcListener
	}
	return true
}
