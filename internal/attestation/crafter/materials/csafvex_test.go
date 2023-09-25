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

//nolint:dupl
package materials_test

import (
	"context"
	"testing"

	attestationApi "github.com/chainloop-dev/chainloop/app/cli/api/attestation/v1"
	contractAPI "github.com/chainloop-dev/chainloop/app/controlplane/api/workflowcontract/v1"
	"github.com/chainloop-dev/chainloop/internal/attestation/crafter/materials"
	"github.com/chainloop-dev/chainloop/internal/casclient"
	mUploader "github.com/chainloop-dev/chainloop/internal/casclient/mocks"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewCSAFVEXCrafter(t *testing.T) {
	testCases := []struct {
		name    string
		input   *contractAPI.CraftingSchema_Material
		wantErr bool
	}{
		{
			name: "happy path",
			input: &contractAPI.CraftingSchema_Material{
				Type: contractAPI.CraftingSchema_Material_CSAF_VEX,
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
			_, err := materials.NewCSAFVEXCrafter(tc.input, nil, nil)
			if tc.wantErr {
				assert.Error(t, err)
				return
			}

			assert.NoError(t, err)
		})
	}
}

func TestCSAFVEXCraft(t *testing.T) {
	testCases := []struct {
		name     string
		filePath string
		wantErr  string
	}{
		{
			name:     "non-expected json file",
			filePath: "./testdata/sbom.cyclonedx.json",
			wantErr:  "unexpected material type",
		},
		{
			name:     "invalid path",
			filePath: "./testdata/non-existing.json",
			wantErr:  "unexpected material type",
		},
		{
			name:     "invalid artifact type",
			filePath: "./testdata/simple.txt",
			wantErr:  "unexpected material type",
		},
		{
			name:     "valid artifact type",
			filePath: "./testdata/csaf_vex_v0.2.0.json",
		},
	}

	assert := assert.New(t)
	schema := &contractAPI.CraftingSchema_Material{
		Name: "test",
		Type: contractAPI.CraftingSchema_Material_CSAF_VEX,
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
						Filename: "vex.json",
					}, nil)
			}

			backend := &casclient.CASBackend{Uploader: uploader}
			crafter, err := materials.NewCSAFVEXCrafter(schema, backend, &l)
			require.NoError(t, err)

			got, err := crafter.Craft(context.TODO(), tc.filePath)
			if tc.wantErr != "" {
				assert.ErrorContains(err, tc.wantErr)
				return
			}
			require.NoError(t, err)

			assert.Equal(contractAPI.CraftingSchema_Material_CSAF_VEX.String(), got.MaterialType.String())
			assert.True(got.UploadedToCas)

			// // The result includes the digest reference
			assert.Equal(&attestationApi.Attestation_Material_Artifact{
				Id: "test", Digest: "sha256:c27087147fa040909e0ef1b522386608af545b0a163c30c9f11c3d753676fa44", Name: "csaf_vex_v0.2.0.json",
			}, got.GetArtifact())
		})
	}
}
