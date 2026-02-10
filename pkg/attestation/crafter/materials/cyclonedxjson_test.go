//
// Copyright 2023-2025 The Chainloop Authors.
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

package materials_test

import (
	"context"
	"testing"

	contractAPI "github.com/chainloop-dev/chainloop/app/controlplane/api/workflowcontract/v1"
	attestationApi "github.com/chainloop-dev/chainloop/pkg/attestation/crafter/api/attestation/v1"
	"github.com/chainloop-dev/chainloop/pkg/attestation/crafter/materials"
	"github.com/chainloop-dev/chainloop/pkg/casclient"
	mUploader "github.com/chainloop-dev/chainloop/pkg/casclient/mocks"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewCyclonedxJSONCrafter(t *testing.T) {
	testCases := []struct {
		name    string
		input   *contractAPI.CraftingSchema_Material
		wantErr bool
	}{
		{
			name: "happy path",
			input: &contractAPI.CraftingSchema_Material{
				Type: contractAPI.CraftingSchema_Material_SBOM_CYCLONEDX_JSON,
			},
		},
		{
			name: "wrong type",
			input: &contractAPI.CraftingSchema_Material{
				Type: contractAPI.CraftingSchema_Material_CONTAINER_IMAGE,
			},
			wantErr: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := materials.NewCyclonedxJSONCrafter(tc.input, nil, nil)
			if tc.wantErr {
				assert.Error(t, err)
				return
			}

			assert.NoError(t, err)
		})
	}
}

func TestCyclonedxJSONCraft(t *testing.T) {
	testCases := []struct {
		name                     string
		filePath                 string
		wantErr                  string
		wantFilename             string
		wantDigest               string
		wantMainComponent        string
		wantMainComponentKind    string
		wantMainComponentVersion string
		annotations              map[string]string
	}{
		{
			name:     "invalid path",
			filePath: "./testdata/non-existing.json",
			wantErr:  "no such file or directory",
		},
		{
			name:     "invalid sbom format",
			filePath: "./testdata/sbom-spdx.json",
			wantErr:  "unexpected material type",
		},
		{
			name:     "invalid artifact type",
			filePath: "./testdata/simple.txt",
			wantErr:  "unexpected material type",
		},
		{
			name:                  "1.4 version",
			filePath:              "./testdata/sbom.cyclonedx.json",
			wantDigest:            "sha256:16159bb881eb4ab7eb5d8afc5350b0feeed1e31c0a268e355e74f9ccbe885e0c",
			wantFilename:          "sbom.cyclonedx.json",
			wantMainComponent:     ".",
			wantMainComponentKind: "file",
			annotations: map[string]string{
				"chainloop.material.tool.name":    "syft",
				"chainloop.material.tool.version": "0.73.0",
			},
		},
		{
			name:                     "1.5 version",
			filePath:                 "./testdata/sbom.cyclonedx-1.5.json",
			wantDigest:               "sha256:5ca3508f02893b0419b266927f66c7b9dd8b11dbea7faf7cdb9169df8f69d8e3",
			wantFilename:             "sbom.cyclonedx-1.5.json",
			wantMainComponent:        "ghcr.io/chainloop-dev/chainloop/control-plane",
			wantMainComponentKind:    "container",
			wantMainComponentVersion: "v0.55.0",
			annotations: map[string]string{
				"chainloop.material.tool.name":    "syft",
				"chainloop.material.tool.version": "0.101.1",
			},
		},
		{
			name:                     "1.5 version with legacy tools",
			filePath:                 "./testdata/sbom.cyclonedx-1.5-legacy-tools.json",
			wantDigest:               "sha256:7bcc88d02bc19447f3fbe6bb76f12bf0f3788b3796b401716c1d62735f9c8c88",
			wantFilename:             "sbom.cyclonedx-1.5-legacy-tools.json",
			wantMainComponent:        "ghcr.io/chainloop-dev/chainloop/control-plane",
			wantMainComponentKind:    "container",
			wantMainComponentVersion: "v0.55.0",
			annotations: map[string]string{
				"chainloop.material.tool.name":    "syft",
				"chainloop.material.tool.version": "0.73.0",
			},
		},
		{
			name:                     "1.5 version with vulnerabilities",
			filePath:                 "./testdata/sbom.cyclonedx-1.5-vulnerabilities.json",
			wantDigest:               "sha256:16248b84917cee938826bd4de98b84b243715891524bf5e6ebfc33f2c499e60b",
			wantFilename:             "sbom.cyclonedx-1.5-vulnerabilities.json",
			wantMainComponent:        "ghcr.io/chainloop-dev/chainloop/control-plane",
			wantMainComponentKind:    "container",
			wantMainComponentVersion: "v0.55.0",
			annotations: map[string]string{
				"chainloop.material.tool.name":                   "syft",
				"chainloop.material.tool.version":                "0.101.1",
				"chainloop.material.sbom.vulnerabilities_report": "true",
			},
		},
		{
			name:                     "1.5 version with vulnerability with null cwes",
			filePath:                 "./testdata/sbom.cyclonedx-1.5-null-cwes.json",
			wantDigest:               "sha256:0b3aef5f26a3c28da82cbc510cee7633cd5b2cb264d3fa25eebbc10795546ffb",
			wantFilename:             "sbom.cyclonedx-1.5-null-cwes.json",
			wantMainComponent:        "ghcr.io/chainloop-dev/chainloop/control-plane",
			wantMainComponentKind:    "container",
			wantMainComponentVersion: "v0.55.0",
			annotations: map[string]string{
				"chainloop.material.tool.name":                   "syft",
				"chainloop.material.tool.version":                "0.101.1",
				"chainloop.material.sbom.vulnerabilities_report": "true",
			},
		},
		{
			name:                     "1.5 version with multiple tools",
			filePath:                 "./testdata/sbom.cyclonedx-1.5-multiple-tools.json",
			wantDigest:               "sha256:56f82c99fb4740f952296705ceb2ee0c5c3c6a3309b35373d542d58878d65cd3",
			wantFilename:             "sbom.cyclonedx-1.5-multiple-tools.json",
			wantMainComponent:        "test-app",
			wantMainComponentKind:    "application",
			wantMainComponentVersion: "1.0.0",
			annotations: map[string]string{
				"chainloop.material.tool.name":    "Hub",
				"chainloop.material.tool.version": "2025.4.2",
				"chainloop.material.tools":        `["Hub@2025.4.2","cyclonedx-core-java@5.0.5"]`,
			},
		},
	}

	schema := &contractAPI.CraftingSchema_Material{
		Name: "test",
		Type: contractAPI.CraftingSchema_Material_SBOM_CYCLONEDX_JSON,
	}
	l := zerolog.Nop()
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ast := assert.New(t)
			// Mock uploader
			uploader := mUploader.NewUploader(t)
			if tc.wantErr == "" {
				uploader.On("UploadFile", context.TODO(), tc.filePath).
					Return(&casclient.UpDownStatus{}, nil)
			}

			backend := &casclient.CASBackend{Uploader: uploader}
			crafter, err := materials.NewCyclonedxJSONCrafter(schema, backend, &l)
			require.NoError(t, err)

			got, err := crafter.Craft(context.TODO(), tc.filePath)
			if tc.wantErr != "" {
				ast.ErrorContains(err, tc.wantErr)
				return
			}

			require.NoError(t, err)
			ast.Equal(contractAPI.CraftingSchema_Material_SBOM_CYCLONEDX_JSON.String(), got.MaterialType.String())
			ast.True(got.UploadedToCas)

			// The result includes the digest reference
			ast.Equal(
				&attestationApi.Attestation_Material_SBOMArtifact{
					Artifact: &attestationApi.Attestation_Material_Artifact{
						Id: "test", Digest: tc.wantDigest, Name: tc.wantFilename,
					},
					MainComponent: &attestationApi.Attestation_Material_SBOMArtifact_MainComponent{
						Name:    tc.wantMainComponent,
						Kind:    tc.wantMainComponentKind,
						Version: tc.wantMainComponentVersion,
					},
				},
				got.GetSbomArtifact(),
			)

			if tc.annotations != nil {
				for k, v := range tc.annotations {
					ast.Equal(v, got.Annotations[k])
				}
			}
		})
	}
}

func TestCycloneDXJSONCraftNoStrictValidation(t *testing.T) {
	testCases := []struct {
		name                 string
		filePath             string
		noStrictValidation bool
		wantErr              string
	}{
		{
			name:                 "invalid schema without skip flag fails",
			filePath:             "./testdata/sbom.cyclonedx-invalid-schema.json",
			noStrictValidation: false,
			wantErr:              "invalid cyclonedx sbom file",
		},
		{
			name:                 "invalid schema with skip flag succeeds",
			filePath:             "./testdata/sbom.cyclonedx-invalid-schema.json",
			noStrictValidation: true,
			wantErr:              "",
		},
		{
			name:                 "non-cyclonedx file fails even with skip flag",
			filePath:             "./testdata/random.json",
			noStrictValidation: true,
			wantErr:              "invalid cyclonedx sbom file",
		},
		{
			name:                 "valid file works without skip flag",
			filePath:             "./testdata/sbom.cyclonedx.json",
			noStrictValidation: false,
			wantErr:              "",
		},
	}

	schema := &contractAPI.CraftingSchema_Material{
		Name: "test",
		Type: contractAPI.CraftingSchema_Material_SBOM_CYCLONEDX_JSON,
	}
	l := zerolog.Nop()

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ast := assert.New(t)
			// Mock uploader
			uploader := mUploader.NewUploader(t)
			if tc.wantErr == "" {
				uploader.On("UploadFile", context.TODO(), tc.filePath).
					Return(&casclient.UpDownStatus{}, nil)
			}

			backend := &casclient.CASBackend{Uploader: uploader}
			crafter, err := materials.NewCyclonedxJSONCrafter(schema, backend, &l,
				materials.WithCycloneDXNoStrictValidation(tc.noStrictValidation))
			require.NoError(t, err)

			got, err := crafter.Craft(context.TODO(), tc.filePath)
			if tc.wantErr != "" {
				ast.ErrorContains(err, tc.wantErr)
				return
			}

			require.NoError(t, err)
			ast.Equal(contractAPI.CraftingSchema_Material_SBOM_CYCLONEDX_JSON.String(), got.MaterialType.String())
		})
	}
}

func TestCycloneDXJSONCraft_SkipUpload(t *testing.T) {
	testCases := []struct {
		name       string
		skipUpload bool
		wantUpload bool
	}{
		{
			name:       "upload enabled (default)",
			skipUpload: false,
			wantUpload: true,
		},
		{
			name:       "upload skipped via contract",
			skipUpload: true,
			wantUpload: false,
		},
	}

	ast := assert.New(t)
	l := zerolog.Nop()
	filePath := "./testdata/sbom.cyclonedx.json"

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			schema := &contractAPI.CraftingSchema_Material{
				Name:       "test",
				Type:       contractAPI.CraftingSchema_Material_SBOM_CYCLONEDX_JSON,
				SkipUpload: tc.skipUpload,
			}

			// Mock uploader - only expect upload call if not skipped
			uploader := mUploader.NewUploader(t)
			if tc.wantUpload {
				uploader.On("UploadFile", context.TODO(), filePath).
					Return(&casclient.UpDownStatus{
						Digest:   "deadbeef",
						Filename: "sbom.cyclonedx.json",
					}, nil)
			}

			backend := &casclient.CASBackend{Uploader: uploader}
			crafter, err := materials.NewCyclonedxJSONCrafter(schema, backend, &l)
			require.NoError(t, err)

			got, err := crafter.Craft(context.TODO(), filePath)
			require.NoError(t, err)

			ast.Equal(contractAPI.CraftingSchema_Material_SBOM_CYCLONEDX_JSON.String(), got.MaterialType.String())

			// Verify upload behavior matches expectation
			if tc.wantUpload {
				ast.True(got.UploadedToCas, "material should be uploaded when skip_upload is false")
			} else {
				ast.False(got.UploadedToCas, "material should not be uploaded when skip_upload is true")
			}

			// Verify digest is always computed regardless of upload
			ast.Equal("sha256:16159bb881eb4ab7eb5d8afc5350b0feeed1e31c0a268e355e74f9ccbe885e0c", got.GetSbomArtifact().Artifact.Digest)
			ast.Equal("sbom.cyclonedx.json", got.GetSbomArtifact().Artifact.Name)

			// Verify main component extraction still works
			ast.Equal(".", got.GetSbomArtifact().MainComponent.Name)
			ast.Equal("file", got.GetSbomArtifact().MainComponent.Kind)
		})
	}
}
