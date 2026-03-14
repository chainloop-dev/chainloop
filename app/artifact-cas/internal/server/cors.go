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

import "net/http"

// CORSMiddleware returns an http.Handler that applies CORS headers based on allowedOrigins.
// If allowedOrigins is empty, the middleware is a passthrough (CORS disabled).
// If "*" is in the list, any origin is allowed.
// Otherwise, only origins in the list are echoed back.
// OPTIONS preflight requests are short-circuited with 204 No Content before reaching the next handler.
func CORSMiddleware(allowedOrigins []string, next http.Handler) http.Handler {
	if len(allowedOrigins) == 0 {
		return next
	}

	wildcard := false
	allowed := make(map[string]struct{}, len(allowedOrigins))
	for _, o := range allowedOrigins {
		if o == "*" {
			wildcard = true
		}
		allowed[o] = struct{}{}
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if origin == "" {
			next.ServeHTTP(w, r)
			return
		}

		var matchedOrigin string
		switch {
		case wildcard:
			matchedOrigin = "*"
		default:
			if _, ok := allowed[origin]; ok {
				matchedOrigin = origin
			}
		}

		// Always set Vary: Origin when the response depends on the Origin header,
		// so HTTP caches don't serve a cached response for the wrong origin.
		// Use Add to avoid clobbering any existing Vary values.
		w.Header().Add("Vary", "Origin")

		if matchedOrigin == "" {
			next.ServeHTTP(w, r)
			return
		}

		w.Header().Set("Access-Control-Allow-Origin", matchedOrigin)

		if r.Method == http.MethodOptions {
			w.Header().Set("Access-Control-Allow-Methods", "GET, HEAD, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type")
			w.Header().Set("Access-Control-Max-Age", "86400")
			w.WriteHeader(http.StatusNoContent)
			return
		}

		next.ServeHTTP(w, r)
	})
}
