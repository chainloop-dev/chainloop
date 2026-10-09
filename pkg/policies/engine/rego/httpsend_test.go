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

package rego

import (
	"context"
	"fmt"
	"testing"

	"github.com/chainloop-dev/chainloop/pkg/policies/engine"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const httpSendPolicyTemplate = `package main

import rego.v1

result := {"skipped": false, "violations": violations}

violations contains "request sent" if {
	http.send(%[1]s)
}

matches_parameters if {
	http.send(%[1]s)
}
`

func TestRego_HTTPSendRestrictedOptions(t *testing.T) {
	const allowedURL = `"url": "https://www.chainloop.dev"`

	testCases := []struct {
		name    string
		request string
		wantErr string
	}{
		{
			name:    "tls_ca_cert_file",
			request: `{"method": "GET", ` + allowedURL + `, "tls_ca_cert_file": "/etc/ssl/ca.pem"}`,
			wantErr: `http.send: request option "tls_ca_cert_file" is not allowed`,
		},
		{
			name:    "tls_ca_cert_env_variable",
			request: `{"method": "GET", ` + allowedURL + `, "tls_ca_cert_env_variable": "CA_CERT"}`,
			wantErr: `http.send: request option "tls_ca_cert_env_variable" is not allowed`,
		},
		{
			name:    "tls_client_cert_file",
			request: `{"method": "GET", ` + allowedURL + `, "tls_client_cert_file": "/etc/ssl/cert.pem"}`,
			wantErr: `http.send: request option "tls_client_cert_file" is not allowed`,
		},
		{
			name:    "tls_client_cert_env_variable",
			request: `{"method": "GET", ` + allowedURL + `, "tls_client_cert_env_variable": "CLIENT_CERT"}`,
			wantErr: `http.send: request option "tls_client_cert_env_variable" is not allowed`,
		},
		{
			name:    "tls_client_key_file",
			request: `{"method": "GET", ` + allowedURL + `, "tls_client_key_file": "/etc/ssl/key.pem"}`,
			wantErr: `http.send: request option "tls_client_key_file" is not allowed`,
		},
		{
			name:    "tls_client_key_env_variable",
			request: `{"method": "GET", ` + allowedURL + `, "tls_client_key_env_variable": "CLIENT_KEY"}`,
			wantErr: `http.send: request option "tls_client_key_env_variable" is not allowed`,
		},
		{
			name:    "empty value is rejected",
			request: `{"method": "GET", ` + allowedURL + `, "tls_ca_cert_file": ""}`,
			wantErr: `http.send: request option "tls_ca_cert_file" is not allowed`,
		},
		{
			name:    "option key built at runtime",
			request: `object.union({"method": "GET", ` + allowedURL + `}, {concat("_", ["tls", "client", "key", "file"]): "/etc/ssl/key.pem"})`,
			wantErr: `http.send: request option "tls_client_key_file" is not allowed`,
		},
		{
			name:    "unix scheme with an allowed host",
			request: `{"method": "GET", "url": "unix://www.chainloop.dev/info?socket=%2Fvar%2Frun%2Fapp.sock"}`,
			wantErr: `http.send: url scheme "unix" is not allowed`,
		},
		{
			name:    "url without scheme",
			request: `{"method": "GET", "url": "www.chainloop.dev"}`,
			wantErr: `http.send: url scheme "" is not allowed`,
		},
		{
			name:    "upper case https scheme reaches http.send",
			request: `{"method": "GET", "url": "HTTPS://example.com"}`,
			wantErr: "http.send: disallowed host: example.com",
		},
		{
			name:    "request without restricted options reaches http.send",
			request: `{"method": "GET", "url": "https://example.com"}`,
			wantErr: "http.send: disallowed host: example.com",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			r := NewEngine()
			policy := &engine.Policy{
				Name:   "http-send",
				Source: fmt.Appendf(nil, httpSendPolicyTemplate, tc.request),
			}

			_, verifyErr := r.Verify(context.TODO(), policy, []byte(`{}`), nil)
			_, matchErr := r.MatchesParameters(context.TODO(), policy, nil, nil)

			for _, err := range []error{verifyErr, matchErr} {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tc.wantErr)
			}
		})
	}
}

func TestRego_HTTPSendPermissiveMode(t *testing.T) {
	// raise_error=false makes http.send return its own error in the response,
	// which shows that the request got past the restriction.
	const policySource = `package main

import rego.v1

result := {"skipped": false, "violations": violations}

violations contains resp.error.message if {
	resp := http.send({
		"method": "GET",
		"url": "unix://www.chainloop.dev/info?socket=%2Fnonexistent%2Fapp.sock",
		"tls_ca_cert_file": "/nonexistent/ca.pem",
		"raise_error": false,
	})
}
`

	r := NewEngine(engine.WithOperatingMode(int32(EnvironmentModePermissive)))
	result, err := r.Verify(context.TODO(), &engine.Policy{Name: "http-send", Source: []byte(policySource)}, []byte(`{}`), nil)
	require.NoError(t, err)
	require.Len(t, result.Violations, 1)
	assert.Contains(t, result.Violations[0].Violation, "/nonexistent/ca.pem")
	assert.NotContains(t, result.Violations[0].Violation, "is not allowed")
}
