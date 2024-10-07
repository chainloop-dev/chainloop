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
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestNewZAPCrafter(t *testing.T) {
	testCases := []struct {
		name    string
		input   *contractAPI.CraftingSchema_Material
		wantErr bool
	}{
		{
			name: "happy path",
			input: &contractAPI.CraftingSchema_Material{
				Type: contractAPI.CraftingSchema_Material_ZAP_DAST_ZIP,
			},
		},
		{
			name: "wrong type",
			input: &contractAPI.CraftingSchema_Material{
				Type: contractAPI.CraftingSchema_Material_SBOM_SPDX_JSON,
			},
			wantErr: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := materials.NewZAPCrafter(tc.input, nil, nil)
			if tc.wantErr {
				assert.Error(t, err)
				return
			}
		})
	}
}

func TestNewZAPCraft(t *testing.T) {
	testCases := []struct {
		name     string
		filePath string
		wantErr  string
	}{
		{
			name:     "invalid ZAP format",
			filePath: "./testdata/sbom.cyclonedx.json",
			wantErr:  "invalid zip file: file is not a valid zip archive, detected content type",
		},
		{
			name:     "invalid path",
			filePath: "./testdata/non-existing.json",
			wantErr:  "no such file or directory",
		},
		{
			name:     "invalid artifact type",
			filePath: "./testdata/simple.txt",
			wantErr:  "invalid zip file: file is not a valid zip archive, detected content type",
		},
		{
			name:     "missing ZAP json report",
			filePath: "./testdata/zap_scan_wrong.zip",
			wantErr:  "zip file does not contain the ZAP report",
		},
		{
			name:     "valid artifact type",
			filePath: "./testdata/zap_scan.zip",
		},
	}

	assert := assert.New(t)
	schema := &contractAPI.CraftingSchema_Material{
		Name: "test",
		Type: contractAPI.CraftingSchema_Material_ZAP_DAST_ZIP,
	}
	l := zerolog.Nop()
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Mock uploader
			uploader := mUploader.NewUploader(t)
			if tc.wantErr == "" {
				uploader.On("UploadFile", context.TODO(), mock.Anything).
					Return(&casclient.UpDownStatus{
						Digest:   "deadbeef",
						Filename: "zap_dast_report.json",
					}, nil)
			}

			backend := &casclient.CASBackend{Uploader: uploader}
			crafter, err := materials.NewZAPCrafter(schema, backend, &l)
			require.NoError(t, err)

			got, err := crafter.Craft(context.TODO(), tc.filePath)
			if tc.wantErr != "" {
				assert.ErrorContains(err, tc.wantErr)
				return
			}
			require.NoError(t, err)

			assert.Equal(contractAPI.CraftingSchema_Material_ZAP_DAST_ZIP.String(), got.MaterialType.String())
			assert.True(got.UploadedToCas)

			// // The result includes the digest reference
			assert.Equal(got.GetArtifact(), &attestationApi.Attestation_Material_Artifact{
				Id: "test", Digest: "sha256:f61aa70510cb53530158c78dd2d50da90ec8190c11020fe4fafb42e1cce47b69", Name: "zap_dast_report.json",
			})
		})
	}
}
