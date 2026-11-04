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

package biz

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path"
	"testing"

	cpolicies "github.com/chainloop-dev/chainloop/app/controlplane/pkg/policies"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// remote policies served by the fake policy provider, by name
var providerPolicies = map[string]string{
	"kindless": `apiVersion: chainloop.dev/v1
kind: Policy
metadata:
  name: kindless
spec:
  policies:
    - embedded: |
        package main
        import rego.v1
        violations contains msg if { false; msg := "" }
`,
	"sbom-only": `apiVersion: chainloop.dev/v1
kind: Policy
metadata:
  name: sbom-only
spec:
  policies:
    - kind: SBOM_CYCLONEDX_JSON
      embedded: |
        package main
        import rego.v1
        violations contains msg if { false; msg := "" }
`,
}

func newFakePolicyProvider(t *testing.T) *cpolicies.Registry {
	t.Helper()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// the validate endpoint is optional for providers, a 404 skips it
		raw, ok := providerPolicies[path.Base(r.URL.Path)]
		if !ok {
			w.WriteHeader(http.StatusNotFound)
			return
		}

		_ = json.NewEncoder(w).Encode(cpolicies.ProviderResponse{
			Digest: "sha256:4a1b2c3d4e5f60718293a4b5c6d7e8f90a1b2c3d4e5f60718293a4b5c6d7e8f9",
			Raw:    &cpolicies.RawMessage{Body: []byte(raw), Format: "FORMAT_YAML"},
		})
	}))
	t.Cleanup(server.Close)

	registry, err := cpolicies.NewRegistry(log.DefaultLogger, &cpolicies.NewRegistryConfig{Name: "builtin", URL: server.URL, Default: true})
	require.NoError(t, err)

	return registry
}

func contractWithMaterialPolicy(policyName, selector string) []byte {
	return fmt.Appendf(nil, `apiVersion: chainloop.dev/v1
kind: Contract
metadata:
  name: my-contract
spec:
  materials:
    - name: my-image
      type: CONTAINER_IMAGE
    - name: sbom
      type: SBOM_CYCLONEDX_JSON
  policies:
    materials:
      - ref: %s
        %s
`, policyName, selector)
}

func TestValidateContractPoliciesMaterialApplicability(t *testing.T) {
	uc := NewWorkflowContractUseCase(nil, newFakePolicyProvider(t), nil, log.DefaultLogger)

	testCases := []struct {
		name     string
		contract []byte
		wantErr  string
	}{
		{
			name:     "kind-less policy without selector",
			contract: contractWithMaterialPolicy("kindless", ""),
			wantErr:  `policy "kindless" applies to any material type and requires a material selector (selector.name)`,
		},
		{
			name:     "kind-less policy with selector",
			contract: contractWithMaterialPolicy("kindless", "selector: {name: my-image}"),
		},
		{
			name:     "typed policy with selector of an unsupported material",
			contract: contractWithMaterialPolicy("sbom-only", "selector: {name: my-image}"),
			wantErr:  `policy "sbom-only" does not support material "my-image" of type CONTAINER_IMAGE (supported types: [SBOM_CYCLONEDX_JSON])`,
		},
		{
			name: "v1 contract with kind-less policy without selector",
			contract: []byte(`schemaVersion: v1
materials:
  - name: my-image
    type: CONTAINER_IMAGE
policies:
  materials:
    - ref: kindless
`),
			wantErr: `policy "kindless" applies to any material type and requires a material selector (selector.name)`,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := uc.ValidateContractPolicies(context.Background(), tc.contract, "token", nil, nil)
			if tc.wantErr == "" {
				assert.NoError(t, err)
				return
			}

			require.Error(t, err)
			assert.True(t, IsErrValidation(err))
			assert.ErrorContains(t, err, tc.wantErr)
		})
	}
}
