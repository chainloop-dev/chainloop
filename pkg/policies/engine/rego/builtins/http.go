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
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/open-policy-agent/opa/v1/ast"
	"github.com/open-policy-agent/opa/v1/topdown"
	"github.com/open-policy-agent/opa/v1/types"
)

const (
	// httpWithAuthBuiltinName is the name of the chainloop.http_with_auth built-in
	//nolint:gosec // False positive: this is a function name, not a credential
	httpWithAuthBuiltinName = "chainloop.http_with_auth"

	// Default timeout for HTTP requests
	defaultHTTPTimeout = 30 * time.Second
)

// HTTPClient interface allows for dependency injection and testing
type HTTPClient interface {
	Do(req *http.Request) (*http.Response, error)
}

// httpClientProvider is a function that returns an HTTP client
// This allows for lazy initialization and dependency injection
type httpClientProvider func() HTTPClient

var (
	// defaultHTTPClient is the default HTTP client provider
	defaultHTTPClient httpClientProvider = func() HTTPClient {
		return &http.Client{
			Timeout: defaultHTTPTimeout,
		}
	}
)

// SetHTTPClient sets a custom HTTP client provider for testing
func SetHTTPClient(provider httpClientProvider) {
	defaultHTTPClient = provider
}

// ResetHTTPClient resets the HTTP client to the default
func ResetHTTPClient() {
	defaultHTTPClient = func() HTTPClient {
		return &http.Client{
			Timeout: defaultHTTPTimeout,
		}
	}
}

// RegisterHTTPBuiltins registers all HTTP-related custom built-in functions
func RegisterHTTPBuiltins() error {
	return Register(&BuiltinDef{
		Name: httpWithAuthBuiltinName,
		Decl: &ast.Builtin{
			Name: httpWithAuthBuiltinName,
			Decl: types.NewFunction(
				types.Args(
					types.Named("url", types.S), // URL to fetch
					types.Named("headers", types.NewObject(nil, types.NewDynamicProperty(types.S, types.S))), // Headers object
				),
				types.Named("response", types.A), // Response as object
			),
		},
		Impl:          httpWithAuthImpl,
		SecurityLevel: SecurityLevelPermissive, // Only available in permissive mode
		Description: "Makes an HTTP GET request with custom authentication headers. " +
			"Returns response body parsed as JSON. Only available in permissive mode for local development.",
	})
}

// httpWithAuthImpl implements the chainloop.http_with_auth built-in function
func httpWithAuthImpl(bctx topdown.BuiltinContext, operands []*ast.Term, iter func(*ast.Term) error) error {
	// Extract URL
	urlStr, ok := operands[0].Value.(ast.String)
	if !ok {
		return fmt.Errorf("url must be a string")
	}

	// Extract headers
	headersObj, ok := operands[1].Value.(ast.Object)
	if !ok {
		return fmt.Errorf("headers must be an object")
	}

	// Convert AST object to map
	headers := make(map[string]string)
	err := headersObj.Iter(func(k, v *ast.Term) error {
		keyStr, ok := k.Value.(ast.String)
		if !ok {
			return fmt.Errorf("header key must be a string")
		}
		valStr, ok := v.Value.(ast.String)
		if !ok {
			return fmt.Errorf("header value must be a string")
		}
		headers[string(keyStr)] = string(valStr)
		return nil
	})
	if err != nil {
		return err
	}

	// Create HTTP request with context
	ctx := bctx.Context
	if ctx == nil {
		ctx = context.Background()
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, string(urlStr), nil)
	if err != nil {
		return fmt.Errorf("failed to create HTTP request: %w", err)
	}

	// Add custom headers
	for key, value := range headers {
		req.Header.Set(key, value)
	}

	// Execute request using the configured client
	client := defaultHTTPClient()
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to execute HTTP request: %w", err)
	}
	defer resp.Body.Close()

	// Read response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response body: %w", err)
	}

	// Parse response body as JSON
	var jsonData interface{}
	if err := json.Unmarshal(body, &jsonData); err != nil {
		// If not valid JSON, return as string
		result := map[string]interface{}{
			"status":      resp.StatusCode,
			"status_text": resp.Status,
			"body":        string(body),
			"headers":     flattenHeaders(resp.Header),
		}
		return iter(ast.NewTerm(ast.MustInterfaceToValue(result)))
	}

	// Return structured response
	result := map[string]interface{}{
		"status":      resp.StatusCode,
		"status_text": resp.Status,
		"body":        jsonData,
		"headers":     flattenHeaders(resp.Header),
	}

	return iter(ast.NewTerm(ast.MustInterfaceToValue(result)))
}

// flattenHeaders converts http.Header to a simple map[string]string
// taking the first value for each header
func flattenHeaders(headers http.Header) map[string]string {
	result := make(map[string]string)
	for key, values := range headers {
		if len(values) > 0 {
			result[key] = values[0]
		}
	}
	return result
}

// init registers HTTP built-ins on package initialization
func init() {
	if err := RegisterHTTPBuiltins(); err != nil {
		panic(fmt.Sprintf("failed to register HTTP built-ins: %v", err))
	}
}
