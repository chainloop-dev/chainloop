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

func TestNewOSSFScorecardCrafter(t *testing.T) {
	testCases := []struct {
		name    string
		input   *contractAPI.CraftingSchema_Material
		wantErr bool
	}{
		{
			name: "happy path",
			input: &contractAPI.CraftingSchema_Material{
				Type: contractAPI.CraftingSchema_Material_OSSF_SCORECARD_JSON,
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
			_, err := materials.NewOSSFScorecardCrafter(tc.input, nil, nil)
			if tc.wantErr {
				assert.Error(t, err)
				return
			}

			assert.NoError(t, err)
		})
	}
}

func TestOSSFScorecardCrafter_Craft(t *testing.T) {
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
			wantErr:  "invalid OpenSSF Scorecard report file",
		},
		{
			name:     "wrong content",
			filePath: "./testdata/scorecard-invalid.json",
			wantErr:  "invalid OpenSSF Scorecard report file",
		},
		{
			name:     "non-scorecard json (sbom)",
			filePath: "./testdata/sbom-spdx.json",
			wantErr:  "invalid OpenSSF Scorecard report file",
		},
		{
			name:     "valid report with high score",
			filePath: "./testdata/scorecard-chainloop.json",
			annotations: map[string]string{
				"chainloop.material.tool.name":       "scorecard",
				"chainloop.material.tool.version":    "v5.5.0",
				"chainloop.material.scorecard.score": "8.2",
			},
		},
		{
			name:     "valid report with low score and inconclusive checks",
			filePath: "./testdata/scorecard-low.json",
			annotations: map[string]string{
				"chainloop.material.tool.name":       "scorecard",
				"chainloop.material.tool.version":    "v5.5.0",
				"chainloop.material.scorecard.score": "2.9",
			},
		},
	}

	schema := &contractAPI.CraftingSchema_Material{
		Name: "test",
		Type: contractAPI.CraftingSchema_Material_OSSF_SCORECARD_JSON,
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
			crafter, err := materials.NewOSSFScorecardCrafter(schema, backend, &l)
			require.NoError(t, err)

			got, err := crafter.Craft(context.TODO(), tc.filePath)
			if tc.wantErr != "" {
				assert.ErrorContains(t, err, tc.wantErr)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, contractAPI.CraftingSchema_Material_OSSF_SCORECARD_JSON.String(), got.MaterialType.String())
			assert.True(t, got.UploadedToCas)

			for k, v := range tc.annotations {
				assert.Equal(t, v, got.Annotations[k])
			}
		})
	}
}

// TestOSSFScorecardCrafter_Craft_NoStrictValidation ensures that even with
// strict schema validation disabled, the discriminating-field guard still
// rejects arbitrary JSON, so non-Scorecard files are not misclassified.
func TestOSSFScorecardCrafter_Craft_NoStrictValidation(t *testing.T) {
	testCases := []struct {
		name string
		// noScoreAnnotation asserts the score annotation is absent (report had no score).
		filePath          string
		wantErr           string
		noScoreAnnotation bool
	}{
		{
			name:     "non-scorecard json still rejected",
			filePath: "./testdata/sbom-spdx.json",
			wantErr:  "invalid OpenSSF Scorecard report file",
		},
		{
			name:     "valid scorecard accepted",
			filePath: "./testdata/scorecard-chainloop.json",
		},
		{
			name:              "report without score is not annotated as score 0",
			filePath:          "./testdata/scorecard-no-score.json",
			noScoreAnnotation: true,
		},
	}

	schema := &contractAPI.CraftingSchema_Material{
		Name: "test",
		Type: contractAPI.CraftingSchema_Material_OSSF_SCORECARD_JSON,
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
			crafter, err := materials.NewOSSFScorecardCrafter(schema, backend, &l, materials.WithOSSFScorecardNoStrictValidation(true))
			require.NoError(t, err)

			got, err := crafter.Craft(context.TODO(), tc.filePath)
			if tc.wantErr != "" {
				assert.ErrorContains(t, err, tc.wantErr)
				return
			}

			require.NoError(t, err)
			if tc.noScoreAnnotation {
				_, ok := got.Annotations["chainloop.material.scorecard.score"]
				assert.False(t, ok, "score annotation should be absent when report has no score")
			}
		})
	}
}
