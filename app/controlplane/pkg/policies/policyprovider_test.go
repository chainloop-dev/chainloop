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
	"encoding/json"
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

// TestResolve_CP9_CrossOrgPolicyFetch_Documentation documents the control-plane
// side of CP-9: the control plane forwards the attacker-controlled orgName as
// the organization_name query param, while the caller's actual org goes only in
// the Chainloop-Organization header. The control plane does not cross-check
// these values. However, the exploit is NOT reachable because the backend
// policy provider (platform/backend/internal/service/policy.go:73-78) rejects
// the request with PermissionDenied when organization_name != the caller's
// authenticated org. This test demonstrates the control-plane gap (the two
// values differ on the wire) for defense-in-depth documentation only.
func TestResolve_CP9_CrossOrgPolicyFetch_Documentation(t *testing.T) {
	var capturedRequest struct {
		queryOrgName string
		orgHeader    string
		authHeader   string
	}

	// Fake policy provider that returns a policy and records what the
	// control plane sent it.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedRequest.queryOrgName = r.URL.Query().Get(orgNameParam)
		capturedRequest.orgHeader = r.Header.Get(organizationHeader)
		capturedRequest.authHeader = r.Header.Get("Authorization")

		resp := ProviderResponse{
			Digest: "sha256:abc123",
			Raw: &RawMessage{
				Format: "FORMAT_JSON",
				Body:   []byte(`{"apiVersion":"workflowcontract.chainloop.dev/v1","kind":"Policy","metadata":{"name":"secret-policy"},"spec":{"policies":[{"kind":"CONTAINER_IMAGE","embedded":"package main"}]}}`),
			},
			OrganizationName: "victim-org",
		}
		w.Header().Set("Content-Type", "application/json")
		err := json.NewEncoder(w).Encode(resp)
		require.NoError(t, err)
	}))
	defer srv.Close()

	srvURL, err := url.Parse(srv.URL)
	require.NoError(t, err)

	provider := &PolicyProvider{name: "test", url: srvURL.String()}

	// Simulate a caller in "attacker-org" requesting a policy from "victim-org".
	// - policyOrgName = "victim-org"  (attacker-controlled via req.GetOrgName())
	// - authOpts.OrgName = "attacker-org"  (caller's actual org from requireCurrentOrg)
	policy, ref, err := provider.Resolve(
		"secret-policy", // policyName
		"victim-org",    // policyOrgName — attacker-controlled
		ProviderAuthOpts{ // authOpts — derived from the caller's actual org
			Token:   "attacker-jwt",
			OrgName: "attacker-org",
		},
	)
	require.NoError(t, err)

	// The control plane sent the attacker's org name as the namespace to read
	// from (organization_name query param) and the caller's actual org only as
	// the Chainloop-Organization header. These differ — the control plane did
	// not validate that policyOrgName matches the caller's org.
	assert.Equal(t, "victim-org", capturedRequest.queryOrgName,
		"the organization_name query param is the attacker-controlled value")
	assert.Equal(t, "attacker-org", capturedRequest.orgHeader,
		"the Chainloop-Organization header is the caller's actual org")
	assert.NotEqual(t, capturedRequest.queryOrgName, capturedRequest.orgHeader,
		"CP-9 control-plane gap: the namespace query param and the caller's org header differ")

	// NOTE: In production, the backend (platform/backend) blocks this request at
	// policy.go:73-78 with PermissionDenied because organization_name !=
	// membership.OrgName. The policy content returned here is only received
	// because this test uses a mock provider that doesn't implement that check.
	require.NotNil(t, policy)
	assert.Equal(t, "secret-policy", policy.Metadata.Name)
	require.NotNil(t, ref)
	assert.Contains(t, ref.URL, "victim-org")
}
