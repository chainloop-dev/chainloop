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
	"testing"

	"github.com/chainloop-dev/chainloop/app/artifact-cas/internal/conf"
	"github.com/stretchr/testify/require"
)

func TestCASMetricsTLSConfiguration(t *testing.T) {
	for _, tc := range []struct {
		name string
		tls  *conf.Server_TLS
	}{
		{name: "plaintext"},
		{name: "empty TLS config", tls: &conf.Server_TLS{}},
		{name: "TLS config", tls: &conf.Server_TLS{Certificate: "cert.pem", PrivateKey: "key.pem"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := NewHTTPMetricsServer(&conf.Server{HttpMetrics: &conf.Server_HTTP{Addr: casAnyAddr, TlsConfig: tc.tls}})
			if tc.tls != nil {
				require.ErrorContains(t, err, "server.http_metrics.tls_config is unsupported")
			} else {
				require.NoError(t, err)
			}
		})
	}
}
