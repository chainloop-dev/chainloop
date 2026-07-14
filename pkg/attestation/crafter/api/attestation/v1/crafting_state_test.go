//
// Copyright 2023-2026 The Chainloop Authors.
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
	"github.com/stretchr/testify/require"
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
		Id:           "id",
		M: &Attestation_Material_String_{
			String_: &Attestation_Material_KeyVal{
				Value: "value",
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
		{
			name: "cobertura xml material projected to json",
			material: &Attestation_Material{
				MaterialType: schemaapi.CraftingSchema_Material_COBERTURA_XML,
				M: &Attestation_Material_Artifact_{
					Artifact: &Attestation_Material_Artifact{
						Name: "name", Digest: "sha256:deadbeef", IsSubject: true,
					},
				},
			},
			filename:  "testdata/cobertura.xml",
			testField: "packages",
		},
		{
			name: "sigcheck csv material",
			material: &Attestation_Material{
				MaterialType: schemaapi.CraftingSchema_Material_SYSINTERNALS_SIGCHECK,
				M: &Attestation_Material_Artifact_{
					Artifact: &Attestation_Material_Artifact{
						Name: "name", Digest: "sha256:deadbeef", IsSubject: true,
					},
				},
			},
			filename:  "testdata/sigcheck-report.csv",
			testField: "elements",
		},
		{
			name: "accesschk text material projected to json",
			material: &Attestation_Material{
				MaterialType: schemaapi.CraftingSchema_Material_SYSINTERNALS_ACCESSCHK,
				M: &Attestation_Material_Artifact_{
					Artifact: &Attestation_Material_Artifact{
						Name: "name", Digest: "sha256:deadbeef", IsSubject: true,
						Content: []byte("c:\\windows\\system32\\notepad.exe\n  RW BUILTIN\\Administrators\n"),
					},
				},
				InlineCas: true,
			},
			testField: "objects",
		},
		{
			name: "dranzer text material projected to json",
			material: &Attestation_Material{
				MaterialType: schemaapi.CraftingSchema_Material_CERTCC_DRANZER,
				M: &Attestation_Material_Artifact_{
					Artifact: &Attestation_Material_Artifact{
						Name: "name", Digest: "sha256:deadbeef", IsSubject: true,
						Content: []byte("Testing COM Object - {11111111-2222-3333-4444-555555555555} Example.WidgetControl\nCOM Object Filename : example.ocx\n"),
					},
				},
				InlineCas: true,
			},
			testField: "objects",
		},
		{
			name: "radamsa report -M log projected to elements",
			material: &Attestation_Material{
				MaterialType: schemaapi.CraftingSchema_Material_RADAMSA_REPORT,
				M: &Attestation_Material_Artifact_{
					Artifact: &Attestation_Material_Artifact{
						Name: "name", Digest: "sha256:deadbeef", IsSubject: true,
					},
				},
			},
			filename:  "testdata/radamsa-meta.txt",
			testField: "elements",
		},
		{
			name: "trufflehog JSONL projected to elements",
			material: &Attestation_Material{
				MaterialType: schemaapi.CraftingSchema_Material_TRUFFLEHOG_JSON,
				M: &Attestation_Material_Artifact_{
					Artifact: &Attestation_Material_Artifact{
						Name: "name", Digest: "sha256:deadbeef", IsSubject: true,
					},
				},
			},
			filename:  "testdata/trufflehog-report.json",
			testField: "elements",
		},
		{
			// metadata-only: the (non-existent) crashes path must NOT be read.
			name: "radamsa crashes metadata only",
			material: &Attestation_Material{
				MaterialType: schemaapi.CraftingSchema_Material_RADAMSA_CRASHES,
				M: &Attestation_Material_Artifact_{
					Artifact: &Attestation_Material_Artifact{
						Name: "name", Digest: "sha256:deadbeef", IsSubject: true,
					},
				},
				Annotations: map[string]string{"chainloop.material.radamsa.crashes.count": "0"},
			},
			filename: "testdata/this-crashes-file-does-not-exist.tar.gz",
		},
		{
			// inline binary crash content must NOT be parsed as JSON; it is
			// metadata-only regardless of how the content is sourced.
			name: "radamsa crashes inline binary content",
			material: &Attestation_Material{
				MaterialType: schemaapi.CraftingSchema_Material_RADAMSA_CRASHES,
				M: &Attestation_Material_Artifact_{
					Artifact: &Attestation_Material_Artifact{
						Name: "name", Digest: "sha256:deadbeef", IsSubject: true,
						Content: []byte("\x1f\x8b\x08\x00rawcrashingbytes"),
					},
				},
				InlineCas:   true,
				Annotations: map[string]string{"chainloop.material.radamsa.crashes.count": "1"},
			},
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

// TestCoberturaEmptyReportIsEvaluable guards the requirement that a legitimate
// empty coverage report (line-rate="NaN", no packages) projects to valid JSON
// the policy engine can evaluate — instead of failing with a NaN marshal error,
// which a policy would surface as a violation. line-rate is null and
// lines-valid is 0, so a policy can guard on lines-valid > 0 and treat the
// report as valid (no violations).
func TestCoberturaEmptyReportIsEvaluable(t *testing.T) {
	m := &Attestation_Material{
		MaterialType: schemaapi.CraftingSchema_Material_COBERTURA_XML,
		M: &Attestation_Material_Artifact_{
			Artifact: &Attestation_Material_Artifact{Name: "coverage", Digest: "sha256:deadbeef"},
		},
	}

	content, err := m.GetEvaluableContent("testdata/cobertura-empty.xml")
	require.NoError(t, err, "empty report must be evaluable, not error on NaN")

	var decoded map[string]any
	require.NoError(t, json.NewDecoder(bytes.NewReader(content)).Decode(&decoded))
	assert.Nil(t, decoded["line-rate"], "NaN line-rate must project as null")
	assert.EqualValues(t, 0, decoded["lines-valid"], "lines-valid stays 0 so a policy can detect an empty report")
}

// TestTruffleHogCleanScanIsEvaluable guards that a clean scan (TruffleHog found
// no secrets, leaving a zero-byte file) projects to valid policy input with an
// empty findings list, so a secrets policy sees "no secrets -> no violation"
// rather than an evaluation error.
func TestTruffleHogCleanScanIsEvaluable(t *testing.T) {
	m := &Attestation_Material{
		MaterialType: schemaapi.CraftingSchema_Material_TRUFFLEHOG_JSON,
		M: &Attestation_Material_Artifact_{
			Artifact: &Attestation_Material_Artifact{Name: "secrets", Digest: "sha256:deadbeef"},
		},
	}

	content, err := m.GetEvaluableContent("testdata/trufflehog-clean-scan.jsonl")
	require.NoError(t, err, "clean scan must be evaluable")

	var decoded map[string]any
	require.NoError(t, json.NewDecoder(bytes.NewReader(content)).Decode(&decoded))
	elements, ok := decoded["elements"].([]any)
	require.True(t, ok, "clean scan must project to an elements array")
	assert.Empty(t, elements, "a clean scan has zero findings")
}
