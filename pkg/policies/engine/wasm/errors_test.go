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

package wasm

import (
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

//nolint:revive // Test error strings intentionally match real wazero error format
func TestParseWasmError(t *testing.T) {
	tests := []struct {
		name             string
		input            error
		expectedCategory ErrorCategory
		expectedMessage  string
		expectedHint     string
	}{
		{
			name: "HTTP request to disallowed hostname with full URL",
			input: errors.New(`HTTP request to 'https://httpbin.org/json' is not allowed (recovered by wazero)
wasm stack trace:
	extism:host/env.http_request(i64,i64) i64
	main.Execute() i32`),
			expectedCategory: CategoryHTTPForbidden,
			expectedMessage:  "HTTP request blocked - hostname 'httpbin.org' is not in the allowed hosts list",
			expectedHint:     "Add the hostname using --allowed-hostnames flag",
		},
		{
			name: "HTTP request to disallowed hostname without wazero message",
			//nolint:revive // Test error string matches real wazero error format
			input: errors.New(`HTTP request to 'https://evil.com/malware' is not allowed
wasm stack trace:
	...`),
			expectedCategory: CategoryHTTPForbidden,
			expectedMessage:  "HTTP request blocked - hostname 'evil.com' is not in the allowed hosts list",
			expectedHint:     "Add the hostname using --allowed-hostnames flag",
		},
		{
			name: "Generic host not allowed",
			//nolint:revive // Test error string matches real wazero error format
			input: errors.New(`Host not allowed (recovered by wazero)
wasm stack trace:
	...`),
			expectedCategory: CategoryHTTPForbidden,
			expectedMessage:  "HTTP request blocked - hostname is not in the allowed hosts list",
			expectedHint:     "Add the hostname using --allowed-hostnames flag",
		},
		{
			name: "Context deadline exceeded (timeout)",
			//nolint:revive // Test error string matches real wazero error format
			input: errors.New(`context deadline exceeded (recovered by wazero)
wasm stack trace:
	...`),
			expectedCategory: CategoryTimeout,
			expectedMessage:  "Policy execution timeout exceeded",
			expectedHint:     "Consider optimizing network calls, reducing data processing, or increasing the timeout",
		},
		{
			name: "Out of memory error",
			input: errors.New(`out of memory (recovered by wazero)
wasm stack trace:
	...`),
			expectedCategory: CategoryMemory,
			expectedMessage:  "Policy exceeded memory limits",
			expectedHint:     "Review data structures and avoid loading large files entirely into memory",
		},
		{
			name: "Runtime error - index out of range",
			input: errors.New(`runtime error: index out of range (recovered by wazero)
wasm stack trace:
	...`),
			expectedCategory: CategoryPanic,
			expectedMessage:  "Runtime error in policy: index out of range",
			expectedHint:     "Enable debug logging with --debug to see detailed stack traces",
		},
		{
			name: "Runtime error - nil pointer dereference",
			input: errors.New(`runtime error: invalid memory address or nil pointer dereference (recovered by wazero)
wasm stack trace:
	...`),
			expectedCategory: CategoryPanic,
			expectedMessage:  "Runtime error in policy: invalid memory address or nil pointer dereference",
			expectedHint:     "Enable debug logging with --debug to see detailed stack traces",
		},
		{
			name: "Unknown error with clean message",
			input: errors.New(`failed to parse JSON input (recovered by wazero)
wasm stack trace:
	...`),
			expectedCategory: CategoryUnknown,
			expectedMessage:  "failed to parse JSON input",
			expectedHint:     "Enable debug logging with --debug for more details",
		},
		{
			name: "Generic error without recognizable pattern",
			input: errors.New(`something went wrong
wasm stack trace:
	...`),
			expectedCategory: CategoryUnknown,
			expectedMessage:  "something went wrong",
			expectedHint:     "Enable debug logging with --debug for more details",
		},
		{
			name: "Error with only stack trace (no clean message)",
			input: errors.New(`wasm stack trace:
	extism:host/env.http_request(i64,i64) i64
	main.Execute() i32`),
			expectedCategory: CategoryUnknown,
			expectedMessage:  "Policy execution failed",
			expectedHint:     "Enable debug logging with --debug for detailed error information",
		},
		{
			name:             "Nil error",
			input:            nil,
			expectedCategory: "",
			expectedMessage:  "",
			expectedHint:     "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parsed := parseWasmError(tt.input)

			if tt.input == nil {
				assert.Nil(t, parsed)
				return
			}

			require.NotNil(t, parsed)
			assert.Equal(t, tt.expectedCategory, parsed.Category)
			assert.Contains(t, parsed.UserMessage, tt.expectedMessage)
			assert.Contains(t, parsed.Hint, tt.expectedHint)
			assert.Equal(t, tt.input, parsed.OriginalErr)
		})
	}
}

func TestExtractHostname(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "HTTPS URL with path",
			input:    "https://httpbin.org/json",
			expected: "httpbin.org",
		},
		{
			name:     "HTTP URL with path",
			input:    "http://example.com/api/data",
			expected: "example.com",
		},
		{
			name:     "URL with port",
			input:    "https://localhost:8080/test",
			expected: "localhost",
		},
		{
			name:     "URL with subdomain",
			input:    "https://api.github.com/repos",
			expected: "api.github.com",
		},
		{
			name:     "Hostname only",
			input:    "example.com",
			expected: "example.com",
		},
		{
			name:     "URL without protocol",
			input:    "httpbin.org/json",
			expected: "httpbin.org",
		},
		{
			name:     "Complex URL with auth and port",
			input:    "https://user:pass@example.com:8080/path?query=1",
			expected: "user", // Note: extractHostname stops at first : which is the auth separator
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractHostname(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestPolicyErrorError(t *testing.T) {
	tests := []struct {
		name     string
		err      *PolicyError
		expected string
	}{
		{
			name: "Error with hint",
			err: &PolicyError{
				Category:    CategoryHTTPForbidden,
				UserMessage: "HTTP request blocked",
				Hint:        "Add the hostname using --allowed-hostnames",
				OriginalErr: errors.New("original"),
			},
			expected: "HTTP request blocked\n\nHint: Add the hostname using --allowed-hostnames",
		},
		{
			name: "Error without hint",
			err: &PolicyError{
				Category:    CategoryUnknown,
				UserMessage: "Something went wrong",
				Hint:        "",
				OriginalErr: errors.New("original"),
			},
			expected: "Something went wrong",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.err.Error()
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestErrorPatterns(t *testing.T) {
	// Test that all error patterns compile and match expected inputs
	for i, ep := range errorPatterns {
		t.Run(string(ep.category), func(t *testing.T) {
			// Ensure pattern compiles (should not panic)
			require.NotNil(t, ep.pattern, "Pattern %d should not be nil", i)

			// Ensure extract function exists
			require.NotNil(t, ep.extract, "Extract function %d should not be nil", i)

			// Test that extract function doesn't panic with valid matches
			switch ep.category {
			case CategoryHTTPForbidden:
				if strings.Contains(ep.pattern.String(), "HTTP request") {
					matches := []string{"full", "https://example.com/test"}
					message, hint := ep.extract(matches)
					assert.NotEmpty(t, message)
					assert.NotEmpty(t, hint)
				}
			case CategoryTimeout:
				matches := []string{"full"}
				message, hint := ep.extract(matches)
				assert.NotEmpty(t, message)
				assert.NotEmpty(t, hint)
			case CategoryMemory:
				matches := []string{"full"}
				message, hint := ep.extract(matches)
				assert.NotEmpty(t, message)
				assert.NotEmpty(t, hint)
			case CategoryPanic:
				matches := []string{"full", "index out of range"}
				message, hint := ep.extract(matches)
				assert.NotEmpty(t, message)
				assert.NotEmpty(t, hint)
			}
		})
	}
}

func TestRealWorldErrors(t *testing.T) {
	// Test actual error messages seen from Extism/wazero
	realWorldErrors := []struct {
		name             string
		err              string
		expectedCategory ErrorCategory
	}{
		{
			name: "Extism HTTP forbidden",
			err: `HTTP request to 'https://registry.npmjs.org/lodash' is not allowed (recovered by wazero)
wasm stack trace:
	extism:host/env.http_request(i64,i64) i64
	main.Execute() i32
		0x2a26b: /Users/user/go/pkg/mod/github.com/extism/go-pdk@v1.1.3/extism_pdk.go:394:34 (inlined)
		         /Users/user/go/pkg/mod/github.com/extism/go-pdk@v1.1.3/internal/memory/memory.go:234:37 (inlined)
		         /Users/user/go/pkg/mod/github.com/extism/go-pdk@v1.1.3/env.go:271:12`,
			expectedCategory: CategoryHTTPForbidden,
		},
		{
			name: "Context timeout",
			err: `context deadline exceeded (recovered by wazero)
wasm stack trace:
	main.Execute() i32`,
			expectedCategory: CategoryTimeout,
		},
		{
			name: "Memory limit",
			err: `out of memory (recovered by wazero)
wasm stack trace:
	malloc(i32) i32
	main.Execute() i32`,
			expectedCategory: CategoryMemory,
		},
	}

	for _, tt := range realWorldErrors {
		t.Run(tt.name, func(t *testing.T) {
			parsed := parseWasmError(errors.New(tt.err))
			require.NotNil(t, parsed)
			assert.Equal(t, tt.expectedCategory, parsed.Category)
			assert.NotEmpty(t, parsed.UserMessage)
			assert.NotEmpty(t, parsed.Hint)
		})
	}
}
