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
	"github.com/go-kratos/kratos/v2/transport/http"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// HTTPMetricsServer is a HTTP server that exposes the metrics endpoint
type HTTPMetricsServer struct {
	*http.Server
}

// NewHTTPMetricsServer exposes the metrics endpoint in another port
func NewHTTPMetricsServer(opts *Opts) (*HTTPMetricsServer, error) {
	var serverOpts = []http.ServerOption{}

	if v := opts.ServerConfig.HttpMetrics.Network; v != "" {
		serverOpts = append(serverOpts, http.Network(v))
	}
	if v := opts.ServerConfig.HttpMetrics.Addr; v != "" {
		serverOpts = append(serverOpts, http.Address(v))
	}
	if v := opts.ServerConfig.HttpMetrics.Timeout; v != nil {
		serverOpts = append(serverOpts, http.Timeout(v.AsDuration()))
	}

	httpSrv := http.NewServer(serverOpts...)
	// NOTE: promhttp.Handler() is a singleton that returns the default metrics repository
	httpSrv.Handle("/metrics", promhttp.Handler())

	return &HTTPMetricsServer{httpSrv}, nil
}
