//
// Copyright 2024 The Chainloop Authors.
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
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	v1 "github.com/chainloop-dev/chainloop/app/controlplane/api/workflowcontract/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCreateRef(t *testing.T) {
	testCases := []struct {
		name       string
		policyURL  string
		policyName string
		digest     string
		orgName    string
		want       *PolicyReference
	}{
		{
			name:       "base",
			policyURL:  "https://p1host.com/foo",
			policyName: "my-policy",
			digest:     "my-digest",
			want:       &PolicyReference{URL: "chainloop://p1host.com/my-policy", Digest: "my-digest"},
		},
		{
			name:       "with org",
			policyURL:  "https://p1host.com/foo",
			policyName: "my-policy",
			digest:     "my-digest",
			orgName:    "my-org",
			want:       &PolicyReference{URL: "chainloop://p1host.com/my-policy?org=my-org", Digest: "my-digest"},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			policyURL, err := url.Parse(tc.policyURL)
			require.NoError(t, err)
			got := createRef(policyURL, tc.policyName, tc.digest, tc.orgName)

			assert.Equal(t, tc.want, got)
		})
	}
}

func TestResolveHTTPErrors(t *testing.T) {
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
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(tc.statusCode)
			}))
			defer srv.Close()

			provider := &PolicyProvider{
				name: "test",
				url:  srv.URL,
			}

			_, _, err := provider.Resolve("my-policy", "", ProviderAuthOpts{Token: "test-token"})
			require.Error(t, err)

			if tc.wantErr != nil {
				assert.True(t, errors.Is(err, tc.wantErr), fmt.Sprintf("expected error wrapping %v, got: %v", tc.wantErr, err))
			} else {
				assert.False(t, errors.Is(err, ErrUnauthorized))
				assert.False(t, errors.Is(err, ErrNotFound))
			}
		})
	}
}

func TestUnmarshalFromRaw(t *testing.T) {
	cases := []struct {
		name    string
		raw     *RawMessage
		wantErr bool
	}{
		{
			name:    "raw from json",
			raw:     &RawMessage{Format: "FORMAT_JSON", Body: []byte("{\"apiVersion\": \"workflowcontract.chainloop.dev/v1\",\"kind\": \"Policy\",\"metadata\": {\"name\": \"policy-workflow\" },\"spec\": {\"policies\": [{\"kind\": \"CONTAINER_IMAGE\",\"embedded\": \"\"}]}}")},
			wantErr: false,
		},
		{
			name:    "raw from yaml",
			raw:     &RawMessage{Format: "FORMAT_YAML", Body: []byte("apiVersion: workflowcontract.chainloop.dev/v1\nkind: Policy\nmetadata:\n  name: policy-workflow\nspec:\n  policies:\n    - kind: CONTAINER_IMAGE\n      embedded: |\n        package main\n        import rego.v1\n        result := {\"violations\":[], \"skipped\": true, \"skip_reason\":\"hello world\"}")},
			wantErr: false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var policy v1.Policy
			err := unmarshalFromRaw(tc.raw, &policy)
			if tc.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, "policy-workflow", policy.Metadata.Name)
		})
	}
}
