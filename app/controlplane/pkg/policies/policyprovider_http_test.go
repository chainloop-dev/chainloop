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

package policies

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestResolveHTTPStatusHandling(t *testing.T) {
	testCases := []struct {
		name       string
		statusCode int
		wantErr    error
	}{
		{
			name:       "401 returns ErrUnauthorized",
			statusCode: http.StatusUnauthorized,
			wantErr:    ErrUnauthorized,
		},
		{
			name:       "403 returns ErrUnauthorized",
			statusCode: http.StatusForbidden,
			wantErr:    ErrUnauthorized,
		},
		{
			name:       "404 returns ErrNotFound",
			statusCode: http.StatusNotFound,
			wantErr:    ErrNotFound,
		},
		{
			name:       "500 returns generic error",
			statusCode: http.StatusInternalServerError,
			wantErr:    nil, // generic error, not a sentinel
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(tc.statusCode)
			}))
			defer server.Close()

			provider := &PolicyProvider{
				name: "test",
				url:  server.URL,
			}

			_, _, err := provider.Resolve("test-policy", "", ProviderAuthOpts{Token: "test-token"})
			require.Error(t, err)

			if tc.wantErr != nil {
				assert.ErrorIs(t, err, tc.wantErr)
			} else {
				assert.NotErrorIs(t, err, ErrUnauthorized)
				assert.NotErrorIs(t, err, ErrNotFound)
				assert.Contains(t, err.Error(), fmt.Sprintf("got %d", tc.statusCode))
			}
		})
	}
}

func TestResolveGroupHTTPStatusHandling(t *testing.T) {
	testCases := []struct {
		name       string
		statusCode int
		wantErr    error
	}{
		{
			name:       "401 returns ErrUnauthorized",
			statusCode: http.StatusUnauthorized,
			wantErr:    ErrUnauthorized,
		},
		{
			name:       "403 returns ErrUnauthorized",
			statusCode: http.StatusForbidden,
			wantErr:    ErrUnauthorized,
		},
		{
			name:       "404 returns ErrNotFound",
			statusCode: http.StatusNotFound,
			wantErr:    ErrNotFound,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(tc.statusCode)
			}))
			defer server.Close()

			provider := &PolicyProvider{
				name: "test",
				url:  server.URL,
			}

			_, _, err := provider.ResolveGroup("test-group", "", ProviderAuthOpts{Token: "test-token"})
			require.Error(t, err)
			assert.ErrorIs(t, err, tc.wantErr)
		})
	}
}

func TestValidateAttachmentHTTPStatusHandling(t *testing.T) {
	testCases := []struct {
		name       string
		statusCode int
		wantErr    error
		errNil     bool
	}{
		{
			name:       "401 returns ErrUnauthorized",
			statusCode: http.StatusUnauthorized,
			wantErr:    ErrUnauthorized,
		},
		{
			name:       "403 returns ErrUnauthorized",
			statusCode: http.StatusForbidden,
			wantErr:    ErrUnauthorized,
		},
		{
			name:       "404 is ignored",
			statusCode: http.StatusNotFound,
			errNil:     true,
		},
		{
			name:       "405 is ignored",
			statusCode: http.StatusMethodNotAllowed,
			errNil:     true,
		},
		{
			name:       "500 returns generic error",
			statusCode: http.StatusInternalServerError,
			wantErr:    nil,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(tc.statusCode)
			}))
			defer server.Close()

			provider := &PolicyProvider{
				name: "test",
				url:  server.URL,
			}

			err := provider.ValidateAttachment(nil, "test-token")
			if tc.errNil {
				assert.NoError(t, err)
				return
			}

			require.Error(t, err)
			if tc.wantErr != nil {
				assert.ErrorIs(t, err, tc.wantErr)
			} else {
				assert.NotErrorIs(t, err, ErrUnauthorized)
				assert.Contains(t, err.Error(), fmt.Sprintf("got %d", tc.statusCode))
			}
		})
	}
}
