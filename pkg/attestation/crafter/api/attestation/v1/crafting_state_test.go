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

package v1

import (
	"bytes"
	"encoding/json"
	"testing"

	schemaapi "github.com/chainloop-dev/chainloop/app/controlplane/api/workflowcontract/v1"
	"github.com/stretchr/testify/assert"
)

func TestNormalizeOutput(t *testing.T) {
	artifactBasedMaterial := &Attestation_Material{
		MaterialType: schemaapi.CraftingSchema_Material_SARIF,
		M: &Attestation_Material_Artifact_{
			Artifact: &Attestation_Material_Artifact{
				Name: "name", Digest: "deadbeef", IsSubject: true, Content: []byte("content"),
			},
		},
	}

	artifactBasedMaterialWant := &NormalizedMaterialOutput{
		Name: "name", Digest: "deadbeef", IsOutput: true, Content: []byte("content"),
	}

	containerMaterial := &Attestation_Material{
		MaterialType: schemaapi.CraftingSchema_Material_CONTAINER_IMAGE,
		M: &Attestation_Material_ContainerImage_{
			ContainerImage: &Attestation_Material_ContainerImage{
				Name: "name", Digest: "deadbeef", IsSubject: true,
			},
		},
	}

	containerMaterialWant := &NormalizedMaterialOutput{
		Name: "name", Digest: "deadbeef", IsOutput: true,
	}

	keyValMaterial := &Attestation_Material{
		MaterialType: schemaapi.CraftingSchema_Material_STRING,
		M: &Attestation_Material_String_{
			String_: &Attestation_Material_KeyVal{
				Id: "id", Value: "value",
			},
		},
	}

	sbomArtifactMaterial := &Attestation_Material{
		MaterialType: schemaapi.CraftingSchema_Material_SBOM_CYCLONEDX_JSON,
		M: &Attestation_Material_SbomArtifact{
			SbomArtifact: &Attestation_Material_SBOMArtifact{
				Artifact: &Attestation_Material_Artifact{
					Name: "name", Digest: "deadbeef", IsSubject: true, Content: []byte("content"),
				},
				MainComponent: &Attestation_Material_SBOMArtifact_MainComponent{
					Name: "the-main-component",
				},
			},
		},
	}

	sbomArtifactMaterialWant := &NormalizedMaterialOutput{
		Name: "name", Digest: "deadbeef", IsOutput: true, Content: []byte("content"),
	}

	keyValWant := &NormalizedMaterialOutput{
		Content: []byte("value"),
	}

	testCases := []struct {
		name     string
		material *Attestation_Material
		want     *NormalizedMaterialOutput
		wantErr  string
	}{
		{
			name:    "nil material",
			wantErr: "material not provided",
		},
		{
			name:     "empty material",
			material: &Attestation_Material{},
			wantErr:  "unknown material: MATERIAL_TYPE_UNSPECIFIED",
		},
		{
			name:     "artifact based material",
			material: artifactBasedMaterial,
			want:     artifactBasedMaterialWant,
		},
		{
			name:     "Container image material",
			material: containerMaterial,
			want:     containerMaterialWant,
		},
		{
			name:     "keyval material",
			material: keyValMaterial,
			want:     keyValWant,
		},
		{
			name:     "sbom artifact material",
			material: sbomArtifactMaterial,
			want:     sbomArtifactMaterialWant,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := (tc.material).NormalizedOutput()
			if tc.wantErr != "" {
				assert.EqualError(t, err, tc.wantErr)
				return
			}

			assert.NoError(t, err)
			assert.Equal(t, tc.want, got)
		})
	}
}

func TestGetEvaluableContentWithMetadata(t *testing.T) {
	cases := []struct {
		name      string
		filename  string
		material  *Attestation_Material
		testField string
	}{
		{
			name: "artifact based material",
			material: &Attestation_Material{
				MaterialType: schemaapi.CraftingSchema_Material_SARIF,
				M: &Attestation_Material_Artifact_{
					Artifact: &Attestation_Material_Artifact{
						Name: "name", Digest: "sha256:deadbeef", IsSubject: true, Content: []byte("{}"),
					},
				},
				InlineCas: true,
			},
		},
		{
			name: "artifact based material",
			material: &Attestation_Material{
				MaterialType: schemaapi.CraftingSchema_Material_CONTAINER_IMAGE,
				M: &Attestation_Material_ContainerImage_{
					ContainerImage: &Attestation_Material_ContainerImage{
						Name: "name", Digest: "sha256:deadbeef", IsSubject: true, Tag: "latest",
					},
				},
			},
		},
		{
			name: "sbom artifact material",
			material: &Attestation_Material{
				MaterialType: schemaapi.CraftingSchema_Material_SBOM_CYCLONEDX_JSON,
				M: &Attestation_Material_SbomArtifact{
					SbomArtifact: &Attestation_Material_SBOMArtifact{
						Artifact: &Attestation_Material_Artifact{
							Name: "name", Digest: "sha256:deadbeef", IsSubject: true, Content: []byte("{}"),
						},
						MainComponent: &Attestation_Material_SBOMArtifact_MainComponent{
							Name: "the-main-component",
						},
					},
				},
				InlineCas: true,
			},
		},
		{
			name: "sbom artifact material not inline",
			material: &Attestation_Material{
				MaterialType: schemaapi.CraftingSchema_Material_SBOM_CYCLONEDX_JSON,
				M: &Attestation_Material_SbomArtifact{
					SbomArtifact: &Attestation_Material_SBOMArtifact{
						Artifact: &Attestation_Material_Artifact{
							Name: "name", Digest: "sha256:deadbeef", IsSubject: true,
						},
						MainComponent: &Attestation_Material_SBOMArtifact_MainComponent{
							Name: "the-main-component",
						},
					},
				},
			},
			filename:  "testdata/sbom.cyclonedx.json",
			testField: "bomFormat",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			content, err := tc.material.GetEvaluableContent(tc.filename)
			assert.NoError(t, err)
			decoder := json.NewDecoder(bytes.NewReader(content))

			var decodedMaterial map[string]interface{}
			err = decoder.Decode(&decodedMaterial)
			assert.NoError(t, err)

			assert.Equal(t, decodedMaterial["chainloop_metadata"].(map[string]any)["name"], "name")

			if tc.testField != "" {
				assert.NotEmpty(t, decodedMaterial[tc.testField])
			}
		})
	}
}
