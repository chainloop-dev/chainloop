//
// Copyright 2023 The Chainloop Authors.
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

package oidc

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math/big"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	tokenRequestEndpoint  = "/token-request"
	oidcDiscoveryEndpoint = "/.well-known/openid-configuration"
	jwksEndpoint          = "/jwks"

	expectedGitHubIssuer = "https://token.actions.githubusercontent.com"
	testBearerTokenValue = "test-bearer-token"
	testKeyID            = "test-key-id"
)

func TestGitHubOIDCClient_Token(t *testing.T) {
	mockServer, privKey, serverURL := setupOIDCMocks(t)
	tokenRequestURL := fmt.Sprintf("%s%s", serverURL, tokenRequestEndpoint)

	claims := createStandardClaims()
	signedToken, err := createSignedToken(claims, privKey)
	require.NoError(t, err, "Failed to create signed token")

	baseHandler := defaultMockHandler(t, signedToken, testBearerTokenValue, privKey)
	mockServer.Config.Handler = baseHandler

	tests := []struct {
		name              string
		mockHandler       http.HandlerFunc
		expectToken       *OIDCToken
		expectErr         bool
		expectErrContains string
		setupEnv          func(t *testing.T)
		clientAudience    []string
	}{
		{
			name: "Non-200 response",
			mockHandler: func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == tokenRequestEndpoint {
					w.WriteHeader(http.StatusInternalServerError)
					_, _ = w.Write([]byte("Internal Server Error"))
				} else {
					setupOIDCMocksHandler(w, r, privKey)
				}
			},
			expectErr:         true,
			expectErrContains: "response: 500",
		},
		{
			name: "Payload decoding error",
			mockHandler: func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == tokenRequestEndpoint {
					w.Header().Set("Content-Type", "application/json")
					_, _ = w.Write([]byte(`{"value": "not-a-token", "invalid-json`))
				} else {
					setupOIDCMocksHandler(w, r, privKey)
				}
			},
			expectErr:         true,
			expectErrContains: "parsing JSON",
		},
		{
			name: "Token verification error - bad signature",
			mockHandler: func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == tokenRequestEndpoint {
					wrongPrivKey, _ := rsa.GenerateKey(rand.Reader, 2048)
					wrongToken := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
					wrongToken.Header["kid"] = "test-key-id"
					wrongSignedToken, _ := wrongToken.SignedString(wrongPrivKey)

					resp := map[string]string{"value": wrongSignedToken}
					w.Header().Set("Content-Type", "application/json")
					_ = json.NewEncoder(w).Encode(resp)
				} else {
					setupOIDCMocksHandler(w, r, privKey)
				}
			},
			expectErr:         true,
			expectErrContains: "failed to verify signature",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			originalURL := os.Getenv("ACTIONS_ID_TOKEN_REQUEST_URL")
			originalToken := os.Getenv("ACTIONS_ID_TOKEN_REQUEST_TOKEN")
			t.Cleanup(func() {
				t.Setenv("ACTIONS_ID_TOKEN_REQUEST_URL", originalURL)
				t.Setenv("ACTIONS_ID_TOKEN_REQUEST_TOKEN", originalToken)
			})
			t.Setenv("ACTIONS_ID_TOKEN_REQUEST_URL", tokenRequestURL)
			t.Setenv("ACTIONS_ID_TOKEN_REQUEST_TOKEN", testBearerTokenValue)

			if tt.setupEnv != nil {
				tt.setupEnv(t)
			}

			if tt.mockHandler != nil {
				mockServer.Config.Handler = tt.mockHandler
			}
			t.Cleanup(func() { mockServer.Config.Handler = baseHandler })

			ctx := oidc.ClientContext(context.Background(), mockServer.Client())
			client, err := NewOIDCGitHubClient(ctx)
			require.NoError(t, err)

			var actualToken *OIDCToken
			func() {
				defer func() {
					if r := recover(); r != nil {
						err = fmt.Errorf("panic occurred: %v", r)
					}
				}()
				actualToken, err = client.Token(context.Background())
			}()

			if tt.expectErr {
				assert.Error(t, err)
				assert.Nil(t, actualToken)
			} else {
				if err != nil {
					assert.NoError(t, err, "Expected success but got error")
					return
				}
				if actualToken == nil {
					assert.NotNil(t, actualToken, "Token is nil but no error was returned")
					return
				}
				assert.Equal(t, tt.expectToken.Issuer, actualToken.Issuer)
				assert.Equal(t, tt.expectToken.JobWorkflowRef, actualToken.JobWorkflowRef)
			}
		})
	}
}

func setupOIDCMocks(t *testing.T) (*httptest.Server, *rsa.PrivateKey, string) {
	r := require.New(t)

	privKey, err := rsa.GenerateKey(rand.Reader, 2048)
	r.NoError(err, "Failed to generate RSA key pair")

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		setupOIDCMocksHandler(w, r, privKey)
	})

	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)

	return server, privKey, server.URL
}

func setupOIDCMocksHandler(w http.ResponseWriter, r *http.Request, privKey *rsa.PrivateKey) {
	switch r.URL.Path {
	case oidcDiscoveryEndpoint:
		discovery := map[string]string{
			"issuer":                                expectedGitHubIssuer,
			"jwks_uri":                              fmt.Sprintf("http://%s%s", r.Host, jwksEndpoint),
			"response_types_supported":              "id_token",
			"subject_types_supported":               "public",
			"id_token_signing_alg_values_supported": "RS256",
		}
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(discovery); err != nil {
			http.Error(w, "Failed to encode discovery document", http.StatusInternalServerError)
			return
		}
	case jwksEndpoint:
		pubKey := &privKey.PublicKey
		eBytes := big.NewInt(int64(pubKey.E)).Bytes()
		if len(eBytes) < 3 {
			padded := make([]byte, 3)
			copy(padded[3-len(eBytes):], eBytes)
			eBytes = padded
		}
		jwk := map[string]string{
			"kty": "RSA",
			"kid": testKeyID,
			"e":   base64.RawURLEncoding.EncodeToString(eBytes),
			"n":   base64.RawURLEncoding.EncodeToString(pubKey.N.Bytes()),
		}
		jwks := map[string][]map[string]string{"keys": {jwk}}
		jwksJSON, err := json.Marshal(jwks)
		if err != nil {
			http.Error(w, "Failed to marshal JWKS", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		if _, err := w.Write(jwksJSON); err != nil {
			return
		}
	default:
		http.NotFound(w, r)
	}
}

func createStandardClaims() jwt.MapClaims {
	return jwt.MapClaims{
		"iss":        expectedGitHubIssuer,
		"sub":        "repo:octo-org/octo-repo:ref:refs/heads/main",
		"aud":        "test-audience",
		"exp":        time.Now().Add(time.Hour).Unix(),
		"nbf":        time.Now().Add(-time.Minute).Unix(),
		"iat":        time.Now().Unix(),
		"jti":        "test-jti",
		"ref":        "refs/heads/main",
		"repository": "octo-org/octo-repo",
		"run_id":     "1234567890",
		"sha":        "test-sha",
	}
}

func createSignedToken(claims jwt.MapClaims, privKey *rsa.PrivateKey) (string, error) {
	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	token.Header["kid"] = testKeyID
	return token.SignedString(privKey)
}

func defaultMockHandler(t *testing.T, signedToken, bearerToken string, privKey *rsa.PrivateKey) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case tokenRequestEndpoint:
			assert.Equal(t, "bearer "+bearerToken, r.Header.Get("Authorization"))

			resp := map[string]string{"value": signedToken}
			w.Header().Set("Content-Type", "application/json")
			if err := json.NewEncoder(w).Encode(resp); err != nil {
				http.Error(w, "Failed to encode response", http.StatusInternalServerError)
				return
			}
		default:
			setupOIDCMocksHandler(w, r, privKey)
		}
	}
}
