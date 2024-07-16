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

// limitations under the License.

package v1_test

import (
	"testing"

	"github.com/bufbuild/protovalidate-go"
	v1 "github.com/chainloop-dev/chainloop/app/controlplane/api/workflowcontract/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidatePolicy(t *testing.T) {
	testCases := []struct {
		desc      string
		policy    *v1.Policy
		wantErr   bool
		violation string
	}{
		{
			desc:      "empty policy",
			policy:    &v1.Policy{},
			wantErr:   true,
			violation: "api_version",
		},
		{
			desc:      "wrong api version",
			policy:    &v1.Policy{ApiVersion: "wrong", Kind: "Policy"},
			wantErr:   true,
			violation: "api_version",
		},
		{
			desc:      "wrong kind",
			policy:    &v1.Policy{ApiVersion: "workflowcontract.chainloop.dev/v1", Kind: "wrong"},
			wantErr:   true,
			violation: "kind",
		},
		{
			desc:      "missing metadata",
			policy:    &v1.Policy{ApiVersion: "workflowcontract.chainloop.dev/v1", Kind: "Policy"},
			wantErr:   true,
			violation: "metadata",
		},
		{
			desc:      "missing name",
			policy:    &v1.Policy{ApiVersion: "workflowcontract.chainloop.dev/v1", Kind: "Policy", Metadata: &v1.Metadata{}},
			wantErr:   true,
			violation: "metadata.name",
		},
		{
			desc:      "non DNS name",
			policy:    &v1.Policy{ApiVersion: "workflowcontract.chainloop.dev/v1", Kind: "Policy", Metadata: &v1.Metadata{Name: "--asdf--"}},
			wantErr:   true,
			violation: "metadata.name",
		},
		{
			desc: "empty spec",
			policy: &v1.Policy{ApiVersion: "workflowcontract.chainloop.dev/v1", Kind: "Policy",
				Metadata: &v1.Metadata{Name: "my-policy"}, Spec: &v1.PolicySpec{}},
			wantErr:   true,
			violation: "spec.source",
		},
		{
			desc: "correct spec",
			policy: &v1.Policy{ApiVersion: "workflowcontract.chainloop.dev/v1", Kind: "Policy",
				Metadata: &v1.Metadata{Name: "my-policy"}, Spec: &v1.PolicySpec{Source: &v1.PolicySpec_Path{Path: "policy.rego"}}},
			wantErr: false,
		},
		{
			desc: "filter material type",
			policy: &v1.Policy{ApiVersion: "workflowcontract.chainloop.dev/v1", Kind: "Policy",
				Metadata: &v1.Metadata{Name: "my-policy"}, Spec: &v1.PolicySpec{Source: &v1.PolicySpec_Path{Path: "policy.rego"}, Type: v1.CraftingSchema_Material_ATTESTATION}},
			wantErr: false,
		},
	}

	validator, err := protovalidate.New()
	require.NoError(t, err)

	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			err := validator.Validate(tc.policy)
			if tc.wantErr {
				assert.Error(t, err)

				if tc.violation != "" {
					assert.Contains(t, err.Error(), tc.violation)
				}
				return
			}

			assert.NoError(t, err)
		})
	}
}
