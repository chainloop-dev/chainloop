//
// Copyright 2024 The Chainloop Authors.
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
	"time"

	_ "net/http/pprof"

	"github.com/go-kratos/kratos/v2/transport/http"
)

// HTTPMetricsServer is a HTTP server that exposes the metrics endpoint
type HTTPProfilerServer struct {
	*http.Server
}

// NewHTTPProfilerServer exposes the metrics endpoint in another port
func NewHTTPProfilerServer(opts *Opts) (*HTTPProfilerServer, error) {
	httpSrv := http.NewServer(http.Address("0.0.0.0:6060"), http.Timeout(10*time.Second))

	return &HTTPProfilerServer{httpSrv}, nil
}
