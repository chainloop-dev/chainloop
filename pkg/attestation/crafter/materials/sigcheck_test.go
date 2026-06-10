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

func TestNewSigcheckCrafter(t *testing.T) {
	testCases := []struct {
		name    string
		input   *contractAPI.CraftingSchema_Material
		wantErr bool
	}{
		{
			name:  "happy path",
			input: &contractAPI.CraftingSchema_Material{Type: contractAPI.CraftingSchema_Material_SYSINTERNALS_SIGCHECK},
		},
		{
			name:    "wrong type",
			input:   &contractAPI.CraftingSchema_Material{Type: contractAPI.CraftingSchema_Material_CONTAINER_IMAGE},
			wantErr: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := materials.NewSigcheckCrafter(tc.input, nil, nil)
			if tc.wantErr {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)
		})
	}
}

func TestSigcheckCrafter_Craft(t *testing.T) {
	testCases := []struct {
		name        string
		filePath    string
		wantErr     string
		annotations map[string]string
	}{
		{
			name:     "invalid path",
			filePath: "./testdata/non-existing.csv",
			wantErr:  "no such file or directory",
		},
		{
			name:     "wrong content",
			filePath: "./testdata/sbom-spdx.json",
			wantErr:  "missing required sigcheck columns",
		},
		{
			name:     "valid report",
			filePath: "./testdata/sigcheck-report.csv",
			annotations: map[string]string{
				"chainloop.material.tool.name": "sigcheck",
			},
		},
		{
			name:     "tab separated report",
			filePath: "./testdata/sigcheck-report-tab.csv",
			annotations: map[string]string{
				"chainloop.material.tool.name": "sigcheck",
			},
		},
		{
			name:     "header-only report accepted",
			filePath: "./testdata/sigcheck-report-empty.csv",
			annotations: map[string]string{
				"chainloop.material.tool.name": "sigcheck",
			},
		},
	}

	schema := &contractAPI.CraftingSchema_Material{
		Name: "test",
		Type: contractAPI.CraftingSchema_Material_SYSINTERNALS_SIGCHECK,
	}

	l := zerolog.Nop()
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			uploader := mUploader.NewUploader(t)
			if tc.wantErr == "" {
				uploader.On("Upload", context.TODO(), mock.Anything, mock.Anything, mock.Anything).
					Return(&casclient.UpDownStatus{}, nil)
			}

			backend := &casclient.CASBackend{Uploader: uploader}
			crafter, err := materials.NewSigcheckCrafter(schema, backend, &l)
			require.NoError(t, err)

			got, err := crafter.Craft(context.TODO(), tc.filePath)
			if tc.wantErr != "" {
				assert.ErrorContains(t, err, tc.wantErr)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, contractAPI.CraftingSchema_Material_SYSINTERNALS_SIGCHECK.String(), got.MaterialType.String())
			assert.True(t, got.UploadedToCas)
			for k, v := range tc.annotations {
				assert.Equal(t, v, got.Annotations[k])
			}
		})
	}
}
