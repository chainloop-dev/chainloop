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
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/chainloop-dev/chainloop/pkg/attestation/crafter/runners/oidc"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
)

func TestNewGitlabClient(t *testing.T) {
	testLogger := zerolog.New(zerolog.Nop()).Level(zerolog.Disabled)
	ctx := context.Background()

	// Save original environment variables
	originalServerURL := os.Getenv(oidc.CI_SERVER_URL_ENV_KEY)
	originalToken := os.Getenv(oidc.GITLAB_OIDC_TOKEN_ENV_KEY)
	defer func() {
		t.Setenv(oidc.CI_SERVER_URL_ENV_KEY, originalServerURL)
		t.Setenv(oidc.GITLAB_OIDC_TOKEN_ENV_KEY, originalToken)
	}()

	tests := []struct {
		name              string
		setupEnv          func(t *testing.T)
		expectErr         bool
		expectErrContains string
	}{
		{
			name: "Missing server URL",
			setupEnv: func(t *testing.T) {
				t.Setenv(oidc.CI_SERVER_URL_ENV_KEY, "")
				t.Setenv(oidc.GITLAB_OIDC_TOKEN_ENV_KEY, "test-token")
			},
			expectErr:         true,
			expectErrContains: "environment variable not set",
		},
		{
			name: "Missing OIDC token",
			setupEnv: func(t *testing.T) {
				t.Setenv(oidc.CI_SERVER_URL_ENV_KEY, "https://gitlab.example.com")
				t.Setenv(oidc.GITLAB_OIDC_TOKEN_ENV_KEY, "")
			},
			expectErr:         true,
			expectErrContains: "environment variable not set",
		},
		// We can't easily test the successful case without mocking the OIDC provider
		// which would require significant refactoring or a mocking library
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.setupEnv(t)
			client, err := oidc.NewGitlabClient(ctx, &testLogger)

			if tt.expectErr {
				assert.Error(t, err)
				if tt.expectErrContains != "" {
					assert.Contains(t, err.Error(), tt.expectErrContains)
				}
				assert.Nil(t, client)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, client)
			}
		})
	}
}

// This test requires mocking the OIDC provider, which is challenging
// without refactoring the code for better testability.
// Here's a sketch of how such a test might look:
func TestParseTokenWithMockProvider(t *testing.T) {
	t.Skip("This test requires mocking the OIDC provider, which is not implemented yet")

	// Setup a mock OIDC provider server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/.well-known/openid-configuration":
			// Respond with a mock OIDC configuration
			json.NewEncoder(w).Encode(map[string]interface{}{
				"issuer":                 "https://mock-gitlab.example.com",
				"jwks_uri":               "https://mock-gitlab.example.com/jwks",
				"token_endpoint":         "https://mock-gitlab.example.com/token",
				"userinfo_endpoint":      "https://mock-gitlab.example.com/userinfo",
				"authorization_endpoint": "https://mock-gitlab.example.com/authorize",
			})
		case "/jwks":
			// Respond with mock JWKs
			// This would need to include the public keys corresponding to the private keys
			// used to sign the test tokens
			json.NewEncoder(w).Encode(map[string]interface{}{
				"keys": []map[string]interface{}{
					// Mock key data would go here
				},
			})
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	// For a real test, we would need to:
	// 1. Create a valid JWT token signed with a private key
	// 2. Configure the mock server to return the corresponding public key
	// 3. Call the parseToken function with the mock server URL and token
	// 4. Verify the token and claims are correctly extracted

	// This would require either:
	// - Making parseToken public or
	// - Refactoring the code to accept a provider interface that can be mocked
}
