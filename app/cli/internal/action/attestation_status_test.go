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

package action

import (
	"testing"

	craftingpb "github.com/chainloop-dev/chainloop/app/controlplane/api/workflowcontract/v1"
	v1 "github.com/chainloop-dev/chainloop/pkg/attestation/crafter/api/attestation/v1"
	"github.com/stretchr/testify/assert"
)

func TestPopulateContractMaterials(t *testing.T) {
	tests := []struct {
		name           string
		craftingState  *v1.CraftingState
		attRes         *AttestationStatusResult
		totalMaterials int
		wantErr        bool
	}{
		{
			name: "empty attestation result",
		},
		{
			name:           "materials on contract",
			totalMaterials: 1,
			craftingState: &v1.CraftingState{
				InputSchema: &craftingpb.CraftingSchema{
					SchemaVersion: "v1",
					Materials: []*craftingpb.CraftingSchema_Material{
						{
							Type:   craftingpb.CraftingSchema_Material_CSAF_VEX,
							Name:   "vex-file",
							Output: true,
						},
					},
				},
			},
		},
		{
			name:           "materials in contract and outside contract",
			totalMaterials: 2,
			craftingState: &v1.CraftingState{
				InputSchema: &craftingpb.CraftingSchema{
					SchemaVersion: "v1",
					Materials: []*craftingpb.CraftingSchema_Material{
						{
							Type:   craftingpb.CraftingSchema_Material_CSAF_VEX,
							Name:   "vex-file",
							Output: true,
						},
					},
				},
				Attestation: &v1.Attestation{
					Materials: map[string]*v1.Attestation_Material{
						"vex-file": {
							Id: "random",
							M: &v1.Attestation_Material_Artifact_{
								Artifact: &v1.Attestation_Material_Artifact{
									Name:    "vex-file",
									Digest:  "random-digest",
									Content: []byte("random-content"),
								},
							},
							MaterialType:  craftingpb.CraftingSchema_Material_CSAF_VEX,
							UploadedToCas: true,
						},
						"other-file": {
							Id: "random",
							M: &v1.Attestation_Material_Artifact_{
								Artifact: &v1.Attestation_Material_Artifact{
									Name:    "other-file",
									Digest:  "random-digest-2",
									Content: []byte("random-content-2"),
								},
							},
							MaterialType:  craftingpb.CraftingSchema_Material_CSAF_SECURITY_ADVISORY,
							UploadedToCas: true,
						},
					},
				},
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tc.attRes = &AttestationStatusResult{}

			err := populateMaterials(tc.craftingState, tc.attRes)
			assert.NoError(t, err)

			for _, m := range tc.attRes.Materials {
				assert.NotNil(t, m)
				assert.NotEmpty(t, m.Name)
				assert.NotEmpty(t, m.Type)
			}
			assert.Equal(t, tc.totalMaterials, len(tc.attRes.Materials))
		})
	}
}
