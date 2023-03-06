//
// Copyright 2023 The Chainloop Authors.
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
	"github.com/chainloop-dev/chainloop/app/artifact-cas/internal/conf"
	"github.com/go-kratos/kratos/v2/middleware/recovery"
	"github.com/go-kratos/kratos/v2/transport/http"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// HTTPMetricsServer is a HTTP server that exposes the metrics endpoint
type HTTPMetricsServer struct {
	*http.Server
}

// NewHTTPMetricsServer exposes the metrics endpoint
func NewHTTPMetricsServer(c *conf.Server) (*HTTPMetricsServer, error) {
	var opts = []http.ServerOption{
		http.Middleware(
			recovery.Recovery(),
		),
	}

	if c.HttpMetrics.Network != "" {
		opts = append(opts, http.Network(c.HttpMetrics.Network))
	}
	if c.HttpMetrics.Addr != "" {
		opts = append(opts, http.Address(c.HttpMetrics.Addr))
	}
	if c.HttpMetrics.Timeout != nil {
		opts = append(opts, http.Timeout(c.HttpMetrics.Timeout.AsDuration()))
	}

	srv := http.NewServer(opts...)

	// NOTE: promhttp.Handler() is a singleton that returns the default metrics repository
	srv.Handle("/metrics", promhttp.Handler())

	return &HTTPMetricsServer{srv}, nil
}
