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

package materials_test

import (
	"context"
	"testing"

	contractAPI "github.com/chainloop-dev/chainloop/app/controlplane/api/workflowcontract/v1"
	"github.com/chainloop-dev/chainloop/internal/casclient"
	mUploader "github.com/chainloop-dev/chainloop/internal/casclient/mocks"
	attestationApi "github.com/chainloop-dev/chainloop/pkg/attestation/crafter/api/attestation/v1"
	"github.com/chainloop-dev/chainloop/pkg/attestation/crafter/materials"
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
		name              string
		filePath          string
		wantErr           string
		wantFilename      string
		wantDigest        string
		wantMainComponent string
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
			name:         "1.4 version",
			filePath:     "./testdata/sbom.cyclonedx.json",
			wantDigest:   "sha256:16159bb881eb4ab7eb5d8afc5350b0feeed1e31c0a268e355e74f9ccbe885e0c",
			wantFilename: "sbom.cyclonedx.json",
		},
		{
			name:              "1.5 version",
			filePath:          "./testdata/sbom.cyclonedx-1.5.json",
			wantDigest:        "sha256:5ca3508f02893b0419b266927f66c7b9dd8b11dbea7faf7cdb9169df8f69d8e3",
			wantFilename:      "sbom.cyclonedx-1.5.json",
			wantMainComponent: "ghcr.io/chainloop-dev/chainloop/control-plane",
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
					MainComponent: tc.wantMainComponent,
				},
				got.GetSbomArtifact(),
			)
		})
	}
}
