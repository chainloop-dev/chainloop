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

//nolint:dupl
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

func TestNewSPDXJSONCrafter(t *testing.T) {
	testCases := []struct {
		name    string
		input   *contractAPI.CraftingSchema_Material
		wantErr bool
	}{
		{
			name: "happy path",
			input: &contractAPI.CraftingSchema_Material{
				Type: contractAPI.CraftingSchema_Material_SBOM_SPDX_JSON,
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
			_, err := materials.NewSPDXJSONCrafter(tc.input, nil, nil)
			if tc.wantErr {
				assert.Error(t, err)
				return
			}

			assert.NoError(t, err)
		})
	}
}

func TestSPDXJSONCraft(t *testing.T) {
	testCases := []struct {
		name         string
		filePath     string
		wantErr      string
		wantDigest   string
		wantFilename string
		annotations  map[string]string
	}{
		{
			name:     "invalid sbom format",
			filePath: "./testdata/sbom.cyclonedx.json",
			wantErr:  "unexpected material type",
		},
		{
			name:     "invalid path",
			filePath: "./testdata/non-existing.json",
			wantErr:  "no such file or directory",
		},
		{
			name:     "invalid artifact type",
			filePath: "./testdata/simple.txt",
			wantErr:  "unexpected material type",
		},
		{
			name:         "valid artifact type",
			filePath:     "./testdata/sbom-spdx.json",
			wantDigest:   "sha256:fe2636fb6c698a29a315278b762b2000efd5959afe776ee4f79f1ed523365a33",
			wantFilename: "sbom-spdx.json",
			annotations: map[string]string{
				"chainloop.material.tool.name":    "syft",
				"chainloop.material.tool.version": "0.73.0",
				"chainloop.material.tools":        `["syft@0.73.0"]`,
			},
		},
		{
			name:         "multiple tools",
			filePath:     "./testdata/sbom-spdx-multiple-tools.json",
			wantDigest:   "sha256:c1a61566c7c0224ac02ad9cd21d90234e5a71de26971e33df2205c1a2eb319fc",
			wantFilename: "sbom-spdx-multiple-tools.json",
			annotations: map[string]string{
				"chainloop.material.tool.name":    "spdxgen",
				"chainloop.material.tool.version": "1.0.0",
				"chainloop.material.tools":        `["spdxgen@1.0.0","scanner@2.1.5"]`,
			},
		},
	}

	assert := assert.New(t)
	schema := &contractAPI.CraftingSchema_Material{
		Name: "test",
		Type: contractAPI.CraftingSchema_Material_SBOM_SPDX_JSON,
	}
	l := zerolog.Nop()
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Mock uploader
			uploader := mUploader.NewUploader(t)
			if tc.wantErr == "" {
				uploader.On("UploadFile", context.TODO(), tc.filePath).
					Return(&casclient.UpDownStatus{
						Digest:   "deadbeef",
						Filename: "sbom-spdx.json",
					}, nil)
			}

			backend := &casclient.CASBackend{Uploader: uploader}
			crafter, err := materials.NewSPDXJSONCrafter(schema, backend, &l)
			require.NoError(t, err)

			got, err := crafter.Craft(context.TODO(), tc.filePath)
			if tc.wantErr != "" {
				assert.ErrorContains(err, tc.wantErr)
				return
			}
			require.NoError(t, err)

			assert.Equal(contractAPI.CraftingSchema_Material_SBOM_SPDX_JSON.String(), got.MaterialType.String())
			assert.True(got.UploadedToCas)

			// The result includes the digest reference
			assert.Equal(got.GetArtifact(), &attestationApi.Attestation_Material_Artifact{
				Id: "test", Digest: tc.wantDigest, Name: tc.wantFilename,
			})

			// Validate annotations if specified
			if tc.annotations != nil {
				for k, v := range tc.annotations {
					assert.Equal(v, got.Annotations[k])
				}
			}
		})
	}
}
