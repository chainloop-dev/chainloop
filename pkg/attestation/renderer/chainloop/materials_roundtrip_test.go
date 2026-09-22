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

package chainloop

import (
	"encoding/json"
	"testing"

	schemaapi "github.com/chainloop-dev/chainloop/app/controlplane/api/workflowcontract/v1"
	attestationapi "github.com/chainloop-dev/chainloop/pkg/attestation/crafter/api/attestation/v1"
	intoto "github.com/in-toto/attestation/go/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/encoding/protojson"
)

// The control plane validates an attestation against its contract by matching
// the names that GetMaterials() reports against the material names the contract
// declares. GetMaterials() silently drops any descriptor that normalizeMaterial
// rejects, so a material whose name fails to survive the crafting-state ->
// descriptor -> JSON -> predicate round trip would read as "not crafted" and
// wrongly fail the attestation.
//
// This guards that every material shape keeps its contract name all the way
// through, including over the protojson/json boundary the predicate crosses.
func TestMaterialNameSurvivesPredicateRoundTrip(t *testing.T) {
	const sha = "aa5f8b2b0dd7e13d54f5f9a9dd6a4c1efb0d4c3a5e1b8f7a6c9d2e3f4a5b6c7d"

	testCases := []struct {
		name         string
		contractName string
		material     *attestationapi.Attestation_Material
	}{
		{
			name:         "string material",
			contractName: "release-notes",
			material: &attestationapi.Attestation_Material{
				MaterialType: schemaapi.CraftingSchema_Material_STRING,
				M: &attestationapi.Attestation_Material_String_{
					String_: &attestationapi.Attestation_Material_KeyVal{
						Value: "some value", Digest: "sha256:" + sha,
					},
				},
			},
		},
		{
			name:         "container image",
			contractName: "image",
			material: &attestationapi.Attestation_Material{
				MaterialType: schemaapi.CraftingSchema_Material_CONTAINER_IMAGE,
				M: &attestationapi.Attestation_Material_ContainerImage_{
					ContainerImage: &attestationapi.Attestation_Material_ContainerImage{
						Name: "ghcr.io/chainloop-dev/chainloop/cli", Digest: "sha256:" + sha,
					},
				},
			},
		},
		{
			name:         "artifact uploaded to CAS",
			contractName: "binary",
			material: &attestationapi.Attestation_Material{
				MaterialType:  schemaapi.CraftingSchema_Material_ARTIFACT,
				UploadedToCas: true,
				M: &attestationapi.Attestation_Material_Artifact_{
					Artifact: &attestationapi.Attestation_Material_Artifact{
						Name: "chainloop-linux-amd64", Digest: "sha256:" + sha,
					},
				},
			},
		},
		{
			name:         "artifact embedded inline",
			contractName: "evidence",
			material: &attestationapi.Attestation_Material{
				MaterialType: schemaapi.CraftingSchema_Material_EVIDENCE,
				InlineCas:    true,
				M: &attestationapi.Attestation_Material_Artifact_{
					Artifact: &attestationapi.Attestation_Material_Artifact{
						Name: "approval.json", Digest: "sha256:" + sha, Content: []byte(`{"approved":true}`),
					},
				},
			},
		},
		{
			name:         "sarif artifact",
			contractName: "static-analysis",
			material: &attestationapi.Attestation_Material{
				MaterialType: schemaapi.CraftingSchema_Material_SARIF,
				M: &attestationapi.Attestation_Material_Artifact_{
					Artifact: &attestationapi.Attestation_Material_Artifact{
						Name: "report.sarif", Digest: "sha256:" + sha,
					},
				},
			},
		},
		{
			name:         "sbom artifact",
			contractName: "sbom",
			material: &attestationapi.Attestation_Material{
				MaterialType: schemaapi.CraftingSchema_Material_SBOM_CYCLONEDX_JSON,
				M: &attestationapi.Attestation_Material_SbomArtifact{
					SbomArtifact: &attestationapi.Attestation_Material_SBOMArtifact{
						Artifact: &attestationapi.Attestation_Material_Artifact{
							Name: "bom.json", Digest: "sha256:" + sha,
						},
					},
				},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			descriptor, err := tc.material.CraftingStateToIntotoDescriptor(tc.contractName)
			require.NoError(t, err)

			// Mimic the encoding the predicate actually goes through: the
			// statement is protojson-encoded into the DSSE payload and decoded
			// back into the predicate struct with encoding/json.
			raw, err := protojson.Marshal(descriptor)
			require.NoError(t, err)

			var decoded intoto.ResourceDescriptor
			require.NoError(t, json.Unmarshal(raw, &decoded))

			predicate := &ProvenancePredicateV02{Materials: []*intoto.ResourceDescriptor{&decoded}}

			materials := predicate.GetMaterials()
			require.Len(t, materials, 1, "material was dropped during normalization")
			assert.Equal(t, tc.contractName, materials[0].Name)
			assert.Equal(t, tc.material.MaterialType.String(), materials[0].Type)
		})
	}
}
