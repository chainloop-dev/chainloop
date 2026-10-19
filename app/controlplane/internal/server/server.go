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
	nethttp "net/http"

	"github.com/go-kratos/kratos/v2/transport/http"
	"github.com/google/wire"
)

// ProviderSet is server providers.
var ProviderSet = wire.NewSet(
	NewGRPCServer,
	NewHTTPServer,
	NewHTTPMetricsServer,
	NewHTTPProfilerServer,
	NewTracerProvider,
	wire.Struct(new(Opts), "*"),
)

var Version = "dev"

// hardenedRouteOptions returns kratos HTTP server options that stop the server
// from delegating unmatched routes to http.DefaultServeMux.
//
// By default go-kratos sets every server's NotFoundHandler and
// MethodNotAllowedHandler to http.DefaultServeMux. Anything registered on that
// process-wide mux -- most notably net/http/pprof's /debug/pprof/* handlers,
// which register themselves at init time -- then becomes reachable, without
// authentication, on every kratos HTTP listener (including the public API and
// metrics ports). These options replace that fallthrough with plain 404/405
// responses so debug endpoints are only ever served where we register them
// explicitly.
func hardenedRouteOptions() []http.ServerOption {
	notFound := nethttp.HandlerFunc(func(w nethttp.ResponseWriter, r *nethttp.Request) {
		nethttp.NotFound(w, r)
	})
	methodNotAllowed := nethttp.HandlerFunc(func(w nethttp.ResponseWriter, _ *nethttp.Request) {
		nethttp.Error(w, nethttp.StatusText(nethttp.StatusMethodNotAllowed), nethttp.StatusMethodNotAllowed)
	})

	return []http.ServerOption{
		http.NotFoundHandler(notFound),
		http.MethodNotAllowedHandler(methodNotAllowed),
	}
}
