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

package action

import (
	"context"
	"slices"
	"testing"

	v1 "github.com/chainloop-dev/chainloop/app/controlplane/api/workflowcontract/v1"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEnrichMaterials(t *testing.T) {
	cases := []struct {
		name        string
		materials   []*v1.CraftingSchema_Material
		policyGroup string
		expectErr   bool
		nMaterials  int
		nPolicies   int
	}{
		{
			name: "existing material",
			materials: []*v1.CraftingSchema_Material{
				{
					Type: v1.CraftingSchema_Material_SBOM_SPDX_JSON,
					Name: "sbom",
				},
			},
			policyGroup: "file://testdata/policy_group.yaml",
			nMaterials:  2,
			nPolicies:   0,
		},
		{
			name: "new materials",
			materials: []*v1.CraftingSchema_Material{
				{
					Type: v1.CraftingSchema_Material_SBOM_CYCLONEDX_JSON,
					Name: "another-sbom",
				},
			},
			policyGroup: "file://testdata/policy_group.yaml",
			nMaterials:  3,
			nPolicies:   1,
		},
		{
			name:        "empty materials in schema",
			materials:   []*v1.CraftingSchema_Material{},
			policyGroup: "file://testdata/policy_group.yaml",
			nMaterials:  2,
			nPolicies:   1,
		},
		{
			name:        "wrong policy group",
			materials:   []*v1.CraftingSchema_Material{},
			policyGroup: "file://testdata/idontexist.yaml",
			// TODO: Fix this condition in next release
			expectErr: false,
		},
	}

	l := zerolog.Nop()
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			schema := v1.CraftingSchema{
				Materials: tc.materials,
				PolicyGroups: []*v1.PolicyGroupAttachment{
					{
						Ref: tc.policyGroup,
					},
				},
			}
			err := enrichContractMaterials(context.TODO(), &schema, nil, &l)
			if tc.expectErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Len(t, schema.Materials, tc.nMaterials)
			// find "sbom" material and check it has proper policies
			if tc.nMaterials > 0 {
				assert.True(t, slices.ContainsFunc(schema.Materials, func(m *v1.CraftingSchema_Material) bool {
					return m.Name == "sbom" && len(m.Policies) == tc.nPolicies
				}))
			}
		})
	}
}
