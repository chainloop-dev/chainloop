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

//nolint:dupl
package materials_test

import (
	"context"
	"testing"

	contractAPI "github.com/chainloop-dev/chainloop/app/controlplane/api/workflowcontract/v1"
	"github.com/chainloop-dev/chainloop/pkg/attestation/crafter/materials"
	"github.com/chainloop-dev/chainloop/pkg/casclient"
	mUploader "github.com/chainloop-dev/chainloop/pkg/casclient/mocks"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestNewCheckmarxCrafter(t *testing.T) {
	testCases := []struct {
		name    string
		input   *contractAPI.CraftingSchema_Material
		wantErr bool
	}{
		{
			name: "happy path",
			input: &contractAPI.CraftingSchema_Material{
				Type: contractAPI.CraftingSchema_Material_CHECKMARX_JSON,
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
			_, err := materials.NewCheckmarxCrafter(tc.input, nil, nil)
			if tc.wantErr {
				assert.Error(t, err)
				return
			}

			assert.NoError(t, err)
		})
	}
}

func TestCheckmarxCrafter_Craft(t *testing.T) {
	testCases := []struct {
		name        string
		filePath    string
		wantErr     string
		annotations map[string]string
	}{
		{
			name:     "invalid path",
			filePath: "./testdata/non-existing.json",
			wantErr:  "no such file or directory",
		},
		{
			name:     "empty file",
			filePath: "./testdata/empty.txt",
			wantErr:  "invalid Checkmarx native JSON report",
		},
		{
			name:     "wrong content",
			filePath: "./testdata/sbom-spdx.json",
			wantErr:  "missing required Checkmarx report fields",
		},
		{
			name:     "clean scan (empty results array)",
			filePath: "./testdata/checkmarx-clean.json",
			annotations: map[string]string{
				"chainloop.material.tool.name": "checkmarx",
			},
		},
		{
			// The ast-cli serializer emits "results": null (not []) when a
			// filter/engine path rebuilds an empty (nil) slice. A present-but-null
			// results key is still a valid clean report and must be accepted.
			name:     "clean scan (null results)",
			filePath: "./testdata/checkmarx-null-results.json",
			annotations: map[string]string{
				"chainloop.material.tool.name": "checkmarx",
			},
		},
		{
			// Real ast-cli output bundling multiple engines in one file (2 sast,
			// 1 sca, 1 kics). SCA results carry a package-based data shape while
			// SAST/KICS carry query-based data; the crafter must accept them all.
			name:     "multi-engine report (sast + sca + kics)",
			filePath: "./testdata/checkmarx.json",
			annotations: map[string]string{
				"chainloop.material.tool.name": "checkmarx",
			},
		},
	}

	schema := &contractAPI.CraftingSchema_Material{
		Name: "test",
		Type: contractAPI.CraftingSchema_Material_CHECKMARX_JSON,
	}

	l := zerolog.Nop()
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Mock uploader
			uploader := mUploader.NewUploader(t)
			if tc.wantErr == "" {
				uploader.On("Upload", context.TODO(), mock.Anything, mock.Anything, mock.Anything).
					Return(&casclient.UpDownStatus{}, nil)
			}

			backend := &casclient.CASBackend{Uploader: uploader}
			crafter, err := materials.NewCheckmarxCrafter(schema, backend, &l)
			require.NoError(t, err)

			got, err := crafter.Craft(context.TODO(), tc.filePath)
			if tc.wantErr != "" {
				assert.ErrorContains(t, err, tc.wantErr)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, contractAPI.CraftingSchema_Material_CHECKMARX_JSON.String(), got.MaterialType.String())
			assert.True(t, got.UploadedToCas)

			if tc.annotations != nil {
				for k, v := range tc.annotations {
					assert.Equal(t, v, got.Annotations[k])
				}
			}
		})
	}
}
