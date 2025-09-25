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
		args        map[string]string
		expectErr   bool
		nMaterials  int
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
		},
		{
			name:        "empty materials in schema",
			materials:   []*v1.CraftingSchema_Material{},
			policyGroup: "file://testdata/policy_group.yaml",
			nMaterials:  2,
		},
		{
			name:        "wrong policy group",
			materials:   []*v1.CraftingSchema_Material{},
			policyGroup: "file://testdata/idontexist.yaml",
			expectErr:   true,
		},
		{
			name:        "name-less materials are not added",
			materials:   []*v1.CraftingSchema_Material{},
			policyGroup: "file://testdata/policy_group_no_name.yaml",
			nMaterials:  0,
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
			// find "sbom" material
			if tc.nMaterials > 0 {
				assert.True(t, slices.ContainsFunc(schema.Materials, func(m *v1.CraftingSchema_Material) bool {
					return m.Name == "sbom"
				}))
			}
		})
	}
}

func TestTemplatedGroups(t *testing.T) {
	cases := []struct {
		name             string
		materials        []*v1.CraftingSchema_Material
		groupFile        string
		args             map[string]string
		nMaterials       int
		materialName     string
		materialOptional bool
	}{
		{
			name:         "interpolates material names, with defaults",
			materials:    []*v1.CraftingSchema_Material{},
			groupFile:    "file://testdata/policy_group_with_arguments.yaml",
			nMaterials:   1,
			materialName: "sbom",
		},
		{
			name:         "interpolates material names, custom material name",
			materials:    []*v1.CraftingSchema_Material{},
			groupFile:    "file://testdata/policy_group_with_arguments.yaml",
			args:         map[string]string{"sbom_name": "foo"},
			nMaterials:   1,
			materialName: "foo",
		},
		{
			name:       "allows empty material name, making it anonymous",
			materials:  []*v1.CraftingSchema_Material{},
			groupFile:  "file://testdata/policy_group_with_arguments.yaml",
			args:       map[string]string{"sbom_name": ""},
			nMaterials: 0,
		},
		{
			name: "interpolates material names, custom name, with material override",
			materials: []*v1.CraftingSchema_Material{{
				Type:     v1.CraftingSchema_Material_SBOM_SPDX_JSON,
				Name:     "foo",
				Optional: true,
			},
			},
			groupFile:        "file://testdata/policy_group_with_arguments.yaml",
			args:             map[string]string{"sbom_name": "foo"},
			nMaterials:       1,
			materialName:     "foo",
			materialOptional: true,
		},
	}

	l := zerolog.Nop()
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			schema := v1.CraftingSchema{
				Materials: tc.materials,
				PolicyGroups: []*v1.PolicyGroupAttachment{
					{
						Ref:  tc.groupFile,
						With: tc.args,
					},
				},
			}
			err := enrichContractMaterials(context.TODO(), &schema, nil, &l)
			assert.NoError(t, err)
			assert.Len(t, schema.Materials, tc.nMaterials)
			if tc.nMaterials > 0 {
				assert.Equal(t, tc.materialName, schema.Materials[0].Name)
				assert.Equal(t, tc.materialOptional, schema.Materials[0].Optional)
			}
		})
	}
}
