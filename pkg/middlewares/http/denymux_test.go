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
	"net/http/httptest"
	"testing"

	"github.com/go-kratos/kratos/v2/transport/http"
	"github.com/stretchr/testify/assert"
)

// register a sentinel handler on the global mux to emulate what net/http/pprof,
// expvar and x/net/trace do from their init() functions.
func init() {
	nhttp.DefaultServeMux.HandleFunc("/debug/sentinel", func(w nhttp.ResponseWriter, _ *nhttp.Request) {
		w.WriteHeader(nhttp.StatusOK)
		_, _ = w.Write([]byte("LEAKED"))
	})
}

func TestDenyDefaultMuxFallthrough(t *testing.T) {
	testCases := []struct {
		name       string
		opts       []http.ServerOption
		path       string
		wantStatus int
	}{
		{
			name:       "without the option the request falls through to DefaultServeMux",
			opts:       nil,
			path:       "/debug/sentinel",
			wantStatus: nhttp.StatusOK,
		},
		{
			name:       "with the option an unmatched route returns 404",
			opts:       DenyDefaultMuxFallthrough(),
			path:       "/debug/sentinel",
			wantStatus: nhttp.StatusNotFound,
		},
		{
			name:       "with the option a registered route still works",
			opts:       DenyDefaultMuxFallthrough(),
			path:       "/healthz",
			wantStatus: nhttp.StatusOK,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			srv := http.NewServer(tc.opts...)
			srv.HandleFunc("/healthz", func(w nhttp.ResponseWriter, _ *nhttp.Request) {
				w.WriteHeader(nhttp.StatusOK)
			})

			rec := httptest.NewRecorder()
			req := httptest.NewRequest(nhttp.MethodGet, tc.path, nil)
			srv.ServeHTTP(rec, req)

			assert.Equal(t, tc.wantStatus, rec.Code)
			if tc.path == "/debug/sentinel" && tc.wantStatus == nhttp.StatusNotFound {
				assert.NotContains(t, rec.Body.String(), "LEAKED")
			}
		})
	}
}
