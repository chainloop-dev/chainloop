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
	"github.com/stretchr/testify/require"
)

func TestBlackduckJSONCraft(t *testing.T) {
	testCases := []struct {
		name         string
		filePath     string
		wantErr      string
		wantFilename string
		wantDigest   string
	}{
		{
			name:     "invalid path",
			filePath: "./testdata/non-existing.json",
			wantErr:  "no such file or directory",
		},
		{
			name:     "unrecognized json type",
			filePath: "./testdata/sbom-spdx.json",
			wantErr:  "unexpected material type",
		},
		{
			name:     "invalid artifact type",
			filePath: "./testdata/simple.txt",
			wantErr:  "unexpected material type",
		},
		{
			name:     "missing fields",
			filePath: "./testdata/blackduck_sca_missing_fields.json",
			wantErr:  "unexpected material type",
		},
		{
			name:         "valid report",
			filePath:     "./testdata/blackduck_sca.json",
			wantDigest:   "sha256:62d3c6f91c3988db5f76d0f5e915e850771651f12aee97da35a5e65b65d27d09",
			wantFilename: "blackduck_sca.json",
		},
	}

	assert := assert.New(t)
	schema := &contractAPI.CraftingSchema_Material{
		Name: "test",
		Type: contractAPI.CraftingSchema_Material_BLACKDUCK_SCA_JSON,
	}

	l := zerolog.Nop()
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Mock uploader
			uploader := mUploader.NewUploader(t)
			if tc.wantErr == "" {
				uploader.On("UploadFile", context.TODO(), tc.filePath).
					Return(&casclient.UpDownStatus{}, nil)
			}

			backend := &casclient.CASBackend{Uploader: uploader}
			crafter, err := materials.NewBlackduckSCAJSONCrafter(schema, backend, &l)
			require.NoError(t, err)

			got, err := crafter.Craft(context.TODO(), tc.filePath)
			if tc.wantErr != "" {
				assert.ErrorContains(err, tc.wantErr)
				return
			}

			require.NoError(t, err)
			assert.Equal(contractAPI.CraftingSchema_Material_BLACKDUCK_SCA_JSON.String(), got.MaterialType.String())
			assert.True(got.UploadedToCas)

			// The result includes the digest reference
			assert.Equal(&attestationApi.Attestation_Material_Artifact{
				Digest: tc.wantDigest, Name: tc.wantFilename,
			}, got.GetArtifact())
		})
	}
}
