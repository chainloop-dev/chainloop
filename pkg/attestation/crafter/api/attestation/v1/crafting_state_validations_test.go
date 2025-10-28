//
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

package v1

import (
	"testing"

	workflowcontract "github.com/chainloop-dev/chainloop/app/controlplane/api/workflowcontract/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCraftingStateGetEnvAllowList(t *testing.T) {
	testCases := []struct {
		name         string
		state        *CraftingState
		expectedVars []string
	}{
		{
			name: "V1 schema with env vars",
			state: &CraftingState{
				Schema: &CraftingState_InputSchema{
					InputSchema: &workflowcontract.CraftingSchema{
						EnvAllowList: []string{"ENV1", "ENV2", "BUILD_NUMBER"},
					},
				},
			},
			expectedVars: []string{"ENV1", "ENV2", "BUILD_NUMBER"},
		},
		{
			name: "V2 schema with env vars",
			state: &CraftingState{
				Schema: &CraftingState_SchemaV2{
					SchemaV2: &workflowcontract.CraftingSchemaV2{
						Spec: &workflowcontract.CraftingSchemaV2Spec{
							EnvAllowList: []string{"CI_COMMIT_SHA", "CUSTOM_VAR"},
						},
					},
				},
			},
			expectedVars: []string{"CI_COMMIT_SHA", "CUSTOM_VAR"},
		},
		{
			name: "V1 schema with no env vars",
			state: &CraftingState{
				Schema: &CraftingState_InputSchema{
					InputSchema: &workflowcontract.CraftingSchema{
						EnvAllowList: []string{},
					},
				},
			},
			expectedVars: []string{},
		},
		{
			name: "V2 schema with no env vars",
			state: &CraftingState{
				Schema: &CraftingState_SchemaV2{
					SchemaV2: &workflowcontract.CraftingSchemaV2{
						Spec: &workflowcontract.CraftingSchemaV2Spec{
							EnvAllowList: []string{},
						},
					},
				},
			},
			expectedVars: []string{},
		},
		{
			name:         "nil schema",
			state:        &CraftingState{},
			expectedVars: nil,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := tc.state.GetEnvAllowList()
			assert.Equal(t, tc.expectedVars, result)
		})
	}
}

func TestCraftingStateGetMaterials(t *testing.T) {
	v1Materials := []*workflowcontract.CraftingSchema_Material{
		{
			Type: workflowcontract.CraftingSchema_Material_ARTIFACT,
			Name: "test-artifact",
		},
		{
			Type: workflowcontract.CraftingSchema_Material_CONTAINER_IMAGE,
			Name: "test-image",
		},
	}

	v2Materials := []*workflowcontract.CraftingSchema_Material{
		{
			Type:     workflowcontract.CraftingSchema_Material_SBOM_CYCLONEDX_JSON,
			Name:     "test-sbom",
			Optional: true,
		},
	}

	testCases := []struct {
		name              string
		state             *CraftingState
		expectedMaterials []*workflowcontract.CraftingSchema_Material
	}{
		{
			name: "V1 schema with materials",
			state: &CraftingState{
				Schema: &CraftingState_InputSchema{
					InputSchema: &workflowcontract.CraftingSchema{
						Materials: v1Materials,
					},
				},
			},
			expectedMaterials: v1Materials,
		},
		{
			name: "V2 schema with materials",
			state: &CraftingState{
				Schema: &CraftingState_SchemaV2{
					SchemaV2: &workflowcontract.CraftingSchemaV2{
						Spec: &workflowcontract.CraftingSchemaV2Spec{
							Materials: v2Materials,
						},
					},
				},
			},
			expectedMaterials: v2Materials,
		},
		{
			name: "V1 schema with no materials",
			state: &CraftingState{
				Schema: &CraftingState_InputSchema{
					InputSchema: &workflowcontract.CraftingSchema{
						Materials: []*workflowcontract.CraftingSchema_Material{},
					},
				},
			},
			expectedMaterials: []*workflowcontract.CraftingSchema_Material{},
		},
		{
			name:              "nil schema",
			state:             &CraftingState{},
			expectedMaterials: nil,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := tc.state.GetMaterials()
			assert.Equal(t, tc.expectedMaterials, result)
		})
	}
}

func TestCraftingStateGetAnnotations(t *testing.T) {
	v1Annotations := []*workflowcontract.Annotation{
		{Name: "version", Value: "1.0.0"},
		{Name: "team", Value: "backend"},
	}

	testCases := []struct {
		name                string
		state               *CraftingState
		expectedAnnotations []*workflowcontract.Annotation
	}{
		{
			name: "V1 schema with annotations",
			state: &CraftingState{
				Schema: &CraftingState_InputSchema{
					InputSchema: &workflowcontract.CraftingSchema{
						Annotations: v1Annotations,
					},
				},
			},
			expectedAnnotations: v1Annotations,
		},
		{
			name: "V2 schema with annotations",
			state: &CraftingState{
				Schema: &CraftingState_SchemaV2{
					SchemaV2: &workflowcontract.CraftingSchemaV2{
						Metadata: &workflowcontract.Metadata{
							Annotations: map[string]string{
								"environment": "production",
								"service":     "api",
							},
						},
					},
				},
			},
			expectedAnnotations: []*workflowcontract.Annotation{
				{Name: "environment", Value: "production"},
				{Name: "service", Value: "api"},
			},
		},
		{
			name: "V2 schema with empty annotations",
			state: &CraftingState{
				Schema: &CraftingState_SchemaV2{
					SchemaV2: &workflowcontract.CraftingSchemaV2{
						Metadata: &workflowcontract.Metadata{
							Annotations: map[string]string{},
						},
					},
				},
			},
			expectedAnnotations: []*workflowcontract.Annotation{},
		},
		{
			name: "V1 schema with no annotations",
			state: &CraftingState{
				Schema: &CraftingState_InputSchema{
					InputSchema: &workflowcontract.CraftingSchema{
						Annotations: []*workflowcontract.Annotation{},
					},
				},
			},
			expectedAnnotations: []*workflowcontract.Annotation{},
		},
		{
			name:                "nil schema",
			state:               &CraftingState{},
			expectedAnnotations: nil,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := tc.state.GetAnnotations()

			if tc.expectedAnnotations == nil {
				assert.Nil(t, result)
				return
			}

			require.Len(t, result, len(tc.expectedAnnotations))

			// For V2 annotations, we need to compare by content since order may vary
			if len(tc.expectedAnnotations) > 0 && tc.state.GetSchema() != nil {
				if _, isV2 := tc.state.GetSchema().(*CraftingState_SchemaV2); isV2 {
					// Create maps for comparison
					expectedMap := make(map[string]string)
					actualMap := make(map[string]string)

					for _, ann := range tc.expectedAnnotations {
						expectedMap[ann.Name] = ann.Value
					}
					for _, ann := range result {
						actualMap[ann.Name] = ann.Value
					}

					assert.Equal(t, expectedMap, actualMap)
					return
				}
			}

			// For V1 annotations, direct comparison
			assert.Equal(t, tc.expectedAnnotations, result)
		})
	}
}

func TestCraftingStateGetPolicyGroups(t *testing.T) {
	v1PolicyGroups := []*workflowcontract.PolicyGroupAttachment{
		{Ref: "file://policy1.yaml"},
		{Ref: "file://policy2.yaml"},
	}

	v2PolicyGroups := []*workflowcontract.PolicyGroupAttachment{
		{Ref: "chainloop://test-policy"},
	}

	testCases := []struct {
		name                 string
		state                *CraftingState
		expectedPolicyGroups []*workflowcontract.PolicyGroupAttachment
	}{
		{
			name: "V1 schema with policy groups",
			state: &CraftingState{
				Schema: &CraftingState_InputSchema{
					InputSchema: &workflowcontract.CraftingSchema{
						PolicyGroups: v1PolicyGroups,
					},
				},
			},
			expectedPolicyGroups: v1PolicyGroups,
		},
		{
			name: "V2 schema with policy groups",
			state: &CraftingState{
				Schema: &CraftingState_SchemaV2{
					SchemaV2: &workflowcontract.CraftingSchemaV2{
						Spec: &workflowcontract.CraftingSchemaV2Spec{
							PolicyGroups: v2PolicyGroups,
						},
					},
				},
			},
			expectedPolicyGroups: v2PolicyGroups,
		},
		{
			name: "V1 schema with no policy groups",
			state: &CraftingState{
				Schema: &CraftingState_InputSchema{
					InputSchema: &workflowcontract.CraftingSchema{
						PolicyGroups: []*workflowcontract.PolicyGroupAttachment{},
					},
				},
			},
			expectedPolicyGroups: []*workflowcontract.PolicyGroupAttachment{},
		},
		{
			name:                 "nil schema",
			state:                &CraftingState{},
			expectedPolicyGroups: nil,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := tc.state.GetPolicyGroups()
			assert.Equal(t, tc.expectedPolicyGroups, result)
		})
	}
}

func TestCraftingStateGetPolicies(t *testing.T) {
	v1Policies := &workflowcontract.Policies{
		Materials: []*workflowcontract.PolicyAttachment{
			{Policy: &workflowcontract.PolicyAttachment_Ref{Ref: "file://material-policy.yaml"}},
		},
		Attestation: []*workflowcontract.PolicyAttachment{
			{Policy: &workflowcontract.PolicyAttachment_Ref{Ref: "file://attestation-policy.yaml"}},
		},
	}

	v2Policies := &workflowcontract.Policies{
		Materials: []*workflowcontract.PolicyAttachment{
			{Policy: &workflowcontract.PolicyAttachment_Ref{Ref: "chainloop://v2-policy"}},
		},
	}

	testCases := []struct {
		name             string
		state            *CraftingState
		expectedPolicies *workflowcontract.Policies
	}{
		{
			name: "V1 schema with policies",
			state: &CraftingState{
				Schema: &CraftingState_InputSchema{
					InputSchema: &workflowcontract.CraftingSchema{
						Policies: v1Policies,
					},
				},
			},
			expectedPolicies: v1Policies,
		},
		{
			name: "V2 schema with policies",
			state: &CraftingState{
				Schema: &CraftingState_SchemaV2{
					SchemaV2: &workflowcontract.CraftingSchemaV2{
						Spec: &workflowcontract.CraftingSchemaV2Spec{
							Policies: v2Policies,
						},
					},
				},
			},
			expectedPolicies: v2Policies,
		},
		{
			name: "V1 schema with nil policies",
			state: &CraftingState{
				Schema: &CraftingState_InputSchema{
					InputSchema: &workflowcontract.CraftingSchema{
						Policies: nil,
					},
				},
			},
			expectedPolicies: nil,
		},
		{
			name:             "nil schema",
			state:            &CraftingState{},
			expectedPolicies: nil,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := tc.state.GetPolicies()
			assert.Equal(t, tc.expectedPolicies, result)
		})
	}
}
