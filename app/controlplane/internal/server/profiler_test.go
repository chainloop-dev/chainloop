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
	nethttp "net/http"
	"net/http/httptest"
	"testing"

	khttp "github.com/go-kratos/kratos/v2/transport/http"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestProfilerServerServesPprof ensures the dedicated, opt-in profiler server
// exposes the pprof endpoints on its own router (not via the default mux).
func TestProfilerServerServesPprof(t *testing.T) {
	srv, err := NewHTTPProfilerServer(&Opts{})
	require.NoError(t, err)

	// Only exercise endpoints that are cheap to serve. /profile and /trace
	// block for seconds by design, so they are intentionally left out.
	for _, path := range []string{"/debug/pprof/", "/debug/pprof/cmdline", "/debug/pprof/heap"} {
		t.Run(path, func(t *testing.T) {
			rec := httptest.NewRecorder()
			req := httptest.NewRequest(nethttp.MethodGet, path, nil)
			srv.ServeHTTP(rec, req)
			assert.Equal(t, nethttp.StatusOK, rec.Code)
		})
	}
}

// TestHardenedServerDoesNotExposeDefaultMux ensures a kratos HTTP server built
// with hardenedRouteOptions does not fall through to http.DefaultServeMux,
// where net/http/pprof registers its handlers at init time. This is what keeps
// /debug/pprof/* off the public API and metrics listeners.
func TestHardenedServerDoesNotExposeDefaultMux(t *testing.T) {
	// Sanity check: importing net/http/pprof (via profiler.go, same package)
	// registers /debug/pprof/* on the process-wide default mux.
	sanity := httptest.NewRecorder()
	nethttp.DefaultServeMux.ServeHTTP(sanity, httptest.NewRequest(nethttp.MethodGet, "/debug/pprof/", nil))
	require.Equal(t, nethttp.StatusOK, sanity.Code, "expected net/http/pprof to be registered on the default mux")

	opts := append([]khttp.ServerOption{khttp.Address("127.0.0.1:0")}, hardenedRouteOptions()...)
	srv := khttp.NewServer(opts...)

	for _, path := range []string{"/debug/pprof/", "/debug/pprof/cmdline", "/debug/vars"} {
		t.Run(path, func(t *testing.T) {
			rec := httptest.NewRecorder()
			srv.ServeHTTP(rec, httptest.NewRequest(nethttp.MethodGet, path, nil))
			assert.Equal(t, nethttp.StatusNotFound, rec.Code, "hardened server must not expose debug endpoints via the default mux")
		})
	}
}
