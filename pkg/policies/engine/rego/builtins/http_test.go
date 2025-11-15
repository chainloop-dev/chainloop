// Copyright 2024-2025 The Chainloop Authors.
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

package builtins

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"testing"

	"github.com/open-policy-agent/opa/v1/rego"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockHTTPClient is a mock HTTP client for testing
type mockHTTPClient struct {
	response *http.Response
	err      error
}

func (m *mockHTTPClient) Do(_ *http.Request) (*http.Response, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.response, nil
}

func TestHTTPWithAuth(t *testing.T) {
	// Reset to default after tests
	defer ResetHTTPClient()

	tests := []struct {
		name           string
		policy         string
		mockResponse   *http.Response
		mockErr        error
		expectedStatus int
		expectError    bool
	}{
		{
			name: "successful JSON response",
			policy: `package test
import rego.v1

result := chainloop.http_with_auth("https://api.example.com/data", {
    "Authorization": "Bearer token123",
    "X-Custom-Header": "value"
})`,
			mockResponse: &http.Response{
				StatusCode: 200,
				Status:     "200 OK",
				Body:       io.NopCloser(bytes.NewBufferString(`{"key": "value", "count": 42}`)),
				Header: http.Header{
					"Content-Type": []string{"application/json"},
				},
			},
			expectedStatus: 200,
			expectError:    false,
		},
		{
			name: "non-JSON response",
			policy: `package test
import rego.v1

result := chainloop.http_with_auth("https://api.example.com/text", {"Authorization": "Bearer token"})`,
			mockResponse: &http.Response{
				StatusCode: 200,
				Status:     "200 OK",
				Body:       io.NopCloser(bytes.NewBufferString(`plain text response`)),
				Header: http.Header{
					"Content-Type": []string{"text/plain"},
				},
			},
			expectedStatus: 200,
			expectError:    false,
		},
		{
			name: "error response",
			policy: `package test
import rego.v1

result := chainloop.http_with_auth("https://api.example.com/error", {"Authorization": "Bearer token"})`,
			mockResponse: &http.Response{
				StatusCode: 404,
				Status:     "404 Not Found",
				Body:       io.NopCloser(bytes.NewBufferString(`{"error": "not found"}`)),
				Header:     http.Header{},
			},
			expectedStatus: 404,
			expectError:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set up mock HTTP client
			SetHTTPClient(func() HTTPClient {
				return &mockHTTPClient{
					response: tt.mockResponse,
					err:      tt.mockErr,
				}
			})

			// Create registry with HTTP built-ins
			registry := NewRegistry()
			require.NoError(t, RegisterHTTPBuiltins())

			// Get the built-in and add it to a new registry
			httpBuiltin, ok := Get(httpWithAuthBuiltinName)
			require.True(t, ok)
			require.NoError(t, registry.Register(httpBuiltin))

			// Register globally (permissive mode)
			require.NoError(t, registry.RegisterGlobal(true))

			// Prepare rego evaluation
			ctx := context.Background()
			r := rego.New(
				rego.Query("data.test.result"),
				rego.Module("test.rego", tt.policy),
			)
			rs, err := r.Eval(ctx)

			if tt.expectError {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)
			require.Len(t, rs, 1)
			require.Len(t, rs[0].Expressions, 1)

			result, ok := rs[0].Expressions[0].Value.(map[string]interface{})
			require.True(t, ok)

			// The status is returned as a number, convert it appropriately
			statusVal := result["status"]
			var status int
			switch v := statusVal.(type) {
			case json.Number:
				i, err := v.Int64()
				require.NoError(t, err)
				status = int(i)
			case float64:
				status = int(v)
			case int:
				status = v
			case int64:
				status = int(v)
			default:
				require.Fail(t, "unexpected status type", "got type: %T", statusVal)
			}

			assert.Equal(t, tt.expectedStatus, status)
		})
	}
}

func TestHTTPBuiltinRegistration(t *testing.T) {
	t.Run("HTTP built-in is registered", func(t *testing.T) {
		def, ok := Get(httpWithAuthBuiltinName)
		assert.True(t, ok)
		assert.NotNil(t, def)
		assert.Equal(t, httpWithAuthBuiltinName, def.Name)
		assert.Equal(t, SecurityLevelPermissive, def.SecurityLevel)
	})

	t.Run("HTTP built-in has correct signature", func(t *testing.T) {
		def, ok := Get(httpWithAuthBuiltinName)
		require.True(t, ok)
		require.NotNil(t, def.Decl)
		assert.Equal(t, httpWithAuthBuiltinName, def.Decl.Name)
	})
}
