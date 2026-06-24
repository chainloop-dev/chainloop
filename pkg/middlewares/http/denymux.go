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

package http

import (
	nhttp "net/http"

	"github.com/go-kratos/kratos/v2/transport/http"
)

// DenyDefaultMuxFallthrough returns Kratos server options that stop the HTTP
// server from routing unmatched requests to http.DefaultServeMux.
//
// By default Kratos sets the router's NotFoundHandler and MethodNotAllowedHandler
// to http.DefaultServeMux (CVE-2026-6993, CWE-441). Packages such as
// net/http/pprof, expvar and golang.org/x/net/trace auto-register handlers on
// that global mux from their init() functions, so any unmatched route on a
// public server leaks /debug/pprof/*, /debug/vars and /debug/requests.
//
// Returning a plain 404/405 instead severs that fallthrough on every network
// path. Registered routes are matched by the router and never reach these
// handlers, so legitimate endpoints are unaffected.
func DenyDefaultMuxFallthrough() []http.ServerOption {
	return []http.ServerOption{
		http.NotFoundHandler(nhttp.HandlerFunc(func(w nhttp.ResponseWriter, _ *nhttp.Request) {
			w.WriteHeader(nhttp.StatusNotFound)
		})),
		http.MethodNotAllowedHandler(nhttp.HandlerFunc(func(w nhttp.ResponseWriter, _ *nhttp.Request) {
			w.WriteHeader(nhttp.StatusMethodNotAllowed)
		})),
	}
}
