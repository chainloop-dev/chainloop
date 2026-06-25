//
// Copyright 2024-2026 The Chainloop Authors.
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
	nethttp "net/http"
	"net/http/pprof"
	"time"

	"github.com/go-kratos/kratos/v2/transport/http"
)

// HTTPProfilerServer is an opt-in HTTP server that exposes the Go pprof
// endpoints on a dedicated port. It is only started when profiling is
// explicitly enabled via configuration (enable_profiler).
type HTTPProfilerServer struct {
	*http.Server
}

// NewHTTPProfilerServer exposes the pprof endpoints on a dedicated port.
//
// The pprof handlers are registered explicitly on this server's own router,
// instead of relying on the process-wide http.DefaultServeMux. Combined with
// hardenedRouteOptions (which prevents every other kratos server from falling
// back to that default mux), this keeps the unauthenticated /debug/pprof/*
// endpoints off the public API and metrics listeners.
func NewHTTPProfilerServer(_ *Opts) (*HTTPProfilerServer, error) {
	serverOpts := append([]http.ServerOption{
		http.Address("0.0.0.0:6060"),
		http.Timeout(10 * time.Second),
	}, hardenedRouteOptions()...)

	httpSrv := http.NewServer(serverOpts...)
	httpSrv.HandlePrefix("/debug/pprof/", pprofMux())

	return &HTTPProfilerServer{httpSrv}, nil
}

// pprofMux builds a dedicated ServeMux with the standard pprof routes so they
// are served only from this server and never registered on the process-wide
// http.DefaultServeMux.
func pprofMux() *nethttp.ServeMux {
	mux := nethttp.NewServeMux()
	mux.HandleFunc("/debug/pprof/", pprof.Index)
	mux.HandleFunc("/debug/pprof/cmdline", pprof.Cmdline)
	mux.HandleFunc("/debug/pprof/profile", pprof.Profile)
	mux.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
	mux.HandleFunc("/debug/pprof/trace", pprof.Trace)
	return mux
}
