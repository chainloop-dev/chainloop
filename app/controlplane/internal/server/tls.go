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

	conf "github.com/chainloop-dev/chainloop/app/controlplane/internal/conf/controlplane/config/v1"
)

// loadServerTLSConfig builds a tls.Config from the certificate and private key
// paths of the server configuration. It returns nil when no TLS is configured.
// For the single port the config negotiates HTTP/2 and HTTP/1.1 through ALPN.
func loadServerTLSConfig(tlsConf *conf.Server_TLS) (*tls.Config, error) {
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
