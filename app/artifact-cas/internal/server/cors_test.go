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
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCORSMiddleware(t *testing.T) {
	handlerCalled := false
	inner := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		handlerCalled = true
		w.WriteHeader(http.StatusOK)
	})

	tests := []struct {
		name           string
		allowedOrigins []string
		method         string
		origin         string
		wantStatus     int
		wantACHeader   string // expected Access-Control-Allow-Origin value, empty if absent
		wantVary       bool
		wantHandler    bool // whether inner handler should be called
	}{
		{
			name:           "empty origins - passthrough",
			allowedOrigins: nil,
			method:         http.MethodGet,
			origin:         "http://example.com",
			wantStatus:     http.StatusOK,
			wantHandler:    true,
		},
		{
			name:           "no Origin header - passthrough",
			allowedOrigins: []string{"http://example.com"},
			method:         http.MethodGet,
			origin:         "",
			wantStatus:     http.StatusOK,
			wantHandler:    true,
		},
		{
			name:           "matching origin GET",
			allowedOrigins: []string{"http://example.com"},
			method:         http.MethodGet,
			origin:         "http://example.com",
			wantStatus:     http.StatusOK,
			wantACHeader:   "http://example.com",
			wantVary:       true,
			wantHandler:    true,
		},
		{
			name:           "matching origin OPTIONS preflight",
			allowedOrigins: []string{"http://example.com"},
			method:         http.MethodOptions,
			origin:         "http://example.com",
			wantStatus:     http.StatusNoContent,
			wantACHeader:   "http://example.com",
			wantVary:       true,
			wantHandler:    false,
		},
		{
			name:           "non-matching origin",
			allowedOrigins: []string{"http://example.com"},
			method:         http.MethodGet,
			origin:         "http://evil.com",
			wantStatus:     http.StatusOK,
			wantVary:       true,
			wantHandler:    true,
		},
		{
			name:           "wildcard allows any origin",
			allowedOrigins: []string{"*"},
			method:         http.MethodGet,
			origin:         "http://anything.com",
			wantStatus:     http.StatusOK,
			wantACHeader:   "*",
			wantHandler:    true,
		},
		{
			name:           "wildcard OPTIONS preflight",
			allowedOrigins: []string{"*"},
			method:         http.MethodOptions,
			origin:         "http://anything.com",
			wantStatus:     http.StatusNoContent,
			wantACHeader:   "*",
			wantHandler:    false,
		},
		{
			name:           "multiple origins - match second",
			allowedOrigins: []string{"http://a.example.com", "http://b.example.com"},
			method:         http.MethodGet,
			origin:         "http://b.example.com",
			wantStatus:     http.StatusOK,
			wantACHeader:   "http://b.example.com",
			wantVary:       true,
			wantHandler:    true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			handlerCalled = false
			handler := CORSMiddleware(tc.allowedOrigins, inner)

			req := httptest.NewRequest(tc.method, "/download/sha256:abc", nil)
			if tc.origin != "" {
				req.Header.Set("Origin", tc.origin)
			}

			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)

			assert.Equal(t, tc.wantStatus, rec.Code)
			assert.Equal(t, tc.wantHandler, handlerCalled)

			if tc.wantACHeader != "" {
				assert.Equal(t, tc.wantACHeader, rec.Header().Get("Access-Control-Allow-Origin"))
			} else {
				assert.Empty(t, rec.Header().Get("Access-Control-Allow-Origin"))
			}

			if tc.wantVary {
				assert.Equal(t, "Origin", rec.Header().Get("Vary"))
			}
		})
	}
}
