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
	"os"
	"slices"
	"testing"

	pb "github.com/chainloop-dev/chainloop/app/controlplane/api/controlplane/v1"
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

func TestParseContractV2(t *testing.T) {
	testCases := []struct {
		name           string
		contractFile   string
		format         pb.WorkflowContractVersionItem_RawBody_Format
		expectV2Schema bool
		expectName     string
		expectError    bool
	}{
		{
			name:           "valid V2 YAML contract",
			contractFile:   "testdata/contract_v2.yaml",
			format:         pb.WorkflowContractVersionItem_RawBody_FORMAT_YAML,
			expectV2Schema: true,
			expectName:     "test-contract-v2",
		},
		{
			name:           "V1 contract should fail V2 parsing",
			contractFile:   "testdata/contract_v1.yaml",
			format:         pb.WorkflowContractVersionItem_RawBody_FORMAT_YAML,
			expectV2Schema: false,
		},
		{
			name:           "invalid contract data",
			contractFile:   "testdata/invalid_contract.yaml",
			format:         pb.WorkflowContractVersionItem_RawBody_FORMAT_YAML,
			expectV2Schema: false,
		},
		{
			name:           "nil raw contract",
			contractFile:   "",
			format:         pb.WorkflowContractVersionItem_RawBody_FORMAT_YAML,
			expectV2Schema: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var rawContract *pb.WorkflowContractVersionItem_RawBody

			if tc.contractFile != "" {
				// Load contract file
				data, err := os.ReadFile(tc.contractFile)
				if err != nil {
					if tc.expectV2Schema {
						require.NoError(t, err, "Failed to load contract file")
					}
					// For non-existent files, test with nil
					rawContract = nil
				} else {
					rawContract = &pb.WorkflowContractVersionItem_RawBody{
						Body:   data,
						Format: tc.format,
					}
				}
			}

			result := parseContractV2(rawContract)

			if tc.expectV2Schema {
				require.NotNil(t, result, "Expected V2 schema to be parsed successfully")
				assert.Equal(t, "chainloop.dev/v1", result.GetApiVersion())
				assert.Equal(t, "Contract", result.GetKind())
				if tc.expectName != "" {
					assert.Equal(t, tc.expectName, result.GetMetadata().GetName())
				}

				// Verify spec fields exist
				spec := result.GetSpec()
				require.NotNil(t, spec, "Spec should not be nil")
				assert.Greater(t, len(spec.GetMaterials()), 0, "Should have materials")
				assert.Greater(t, len(spec.GetEnvAllowList()), 0, "Should have env allow list")
				assert.NotNil(t, spec.GetRunner(), "Should have runner config")

				// Verify annotations in metadata
				annotations := result.GetMetadata().GetAnnotations()
				assert.NotEmpty(t, annotations, "Should have metadata annotations")
			} else {
				assert.Nil(t, result, "Expected V2 schema parsing to fail")
			}
		})
	}
}
