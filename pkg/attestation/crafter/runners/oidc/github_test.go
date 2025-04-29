//
// Copyright 2025 The Chainloop Authors.
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

package oidc_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/chainloop-dev/chainloop/pkg/attestation/crafter/runners/oidc"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewGitHubClient(t *testing.T) {
	testLogger := zerolog.New(zerolog.Nop()).Level(zerolog.Disabled)
	originalRequestURL := os.Getenv(oidc.RequestURLEnvKey)
	originalRequestToken := os.Getenv(oidc.RequestTokenEnvKey)
	defer func() {
		t.Setenv(oidc.RequestURLEnvKey, originalRequestURL)
		t.Setenv(oidc.RequestTokenEnvKey, originalRequestToken)
	}()

	tests := []struct {
		name              string
		setupEnv          func(t *testing.T)
		expectErr         bool
		expectErrContains string
	}{
		{
			name: "Success",
			setupEnv: func(t *testing.T) {
				t.Setenv(oidc.RequestURLEnvKey, "https://example.com/token")
				t.Setenv(oidc.RequestTokenEnvKey, "test-token")
			},
			expectErr: false,
		},
		{
			name: "Missing request URL",
			setupEnv: func(t *testing.T) {
				t.Setenv(oidc.RequestURLEnvKey, "")
				t.Setenv(oidc.RequestTokenEnvKey, "test-token")
			},
			expectErr:         true,
			expectErrContains: "environment variable not set",
		},
		{
			name: "Invalid request URL",
			setupEnv: func(t *testing.T) {
				t.Setenv(oidc.RequestURLEnvKey, "invalid-url")
				t.Setenv(oidc.RequestTokenEnvKey, "test-token")
			},
			expectErr:         true,
			expectErrContains: "invalid request URL",
		},
		{
			name: "Missing bearer token",
			setupEnv: func(t *testing.T) {
				t.Setenv(oidc.RequestURLEnvKey, "https://example.com/token")
				t.Setenv(oidc.RequestTokenEnvKey, "")
			},
			expectErr:         true,
			expectErrContains: "environment variable not set",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.setupEnv(t)
			client, err := oidc.NewGitHubClient(&testLogger)

			if tt.expectErr {
				assert.Error(t, err)
				if tt.expectErrContains != "" {
					assert.Contains(t, err.Error(), tt.expectErrContains)
				}
				return
			}

			assert.NoError(t, err)
			assert.NotNil(t, client)
		})
	}
}

func TestWithAudience(t *testing.T) {
	t.Setenv(oidc.RequestURLEnvKey, "https://example.com/token")
	t.Setenv(oidc.RequestTokenEnvKey, "test-token")

	testLogger := zerolog.New(zerolog.Nop()).Level(zerolog.Disabled)
	testAudience := []string{"test-audience"}
	_, err := oidc.NewGitHubClient(
		&testLogger,
		oidc.WithAudience(testAudience),
	)
	require.NoError(t, err)

	t.Log("Audience option test completed successfully")
}

func TestTokenRequest(t *testing.T) {
	testLogger := zerolog.New(zerolog.Nop()).Level(zerolog.Disabled)
	originalRequestURL := os.Getenv(oidc.RequestURLEnvKey)
	originalRequestToken := os.Getenv(oidc.RequestTokenEnvKey)
	defer func() {
		t.Setenv(oidc.RequestURLEnvKey, originalRequestURL)
		t.Setenv(oidc.RequestTokenEnvKey, originalRequestToken)
	}()

	tests := []struct {
		name              string
		serverHandler     func(w http.ResponseWriter, r *http.Request)
		expectErr         bool
		expectErrContains string
	}{
		{
			name: "Non-200 response",
			serverHandler: func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, "bearer test-token", r.Header.Get("Authorization"))
				w.WriteHeader(http.StatusInternalServerError)
				_, _ = w.Write([]byte("Internal Server Error"))
			},
			expectErr:         true,
			expectErrContains: "response: 500",
		},
		{
			name: "Invalid JSON response",
			serverHandler: func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(`{"value": "token", invalid`))
			},
			expectErr:         true,
			expectErrContains: "parsing JSON",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(tt.serverHandler))
			defer server.Close()

			t.Setenv(oidc.RequestURLEnvKey, server.URL)
			t.Setenv(oidc.RequestTokenEnvKey, "test-token")

			client, err := oidc.NewGitHubClient(&testLogger)
			require.NoError(t, err)

			_, err = client.Token(context.Background())
			assert.Error(t, err)
			if tt.expectErrContains != "" {
				assert.Contains(t, err.Error(), tt.expectErrContains)
			}
		})
	}
}
