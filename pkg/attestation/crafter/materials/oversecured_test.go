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

func TestNewOversecuredCrafter(t *testing.T) {
	testCases := []struct {
		name    string
		input   *contractAPI.CraftingSchema_Material
		wantErr bool
	}{
		{
			name: "happy path",
			input: &contractAPI.CraftingSchema_Material{
				Type: contractAPI.CraftingSchema_Material_OVERSECURED_JSON,
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
			_, err := materials.NewOversecuredCrafter(tc.input, nil, nil)
			if tc.wantErr {
				assert.Error(t, err)
				return
			}

			assert.NoError(t, err)
		})
	}
}

func TestOversecuredCrafter_Craft(t *testing.T) {
	testCases := []struct {
		name        string
		filePath    string
		wantErr     string
		annotations map[string]string
		// absentAnnotations lists annotation keys that must NOT be set. The export
		// carries no per-finding hasSast/hasDast flags and no scanner version, so
		// neither scan.types nor tool.version can be derived from it.
		absentAnnotations []string
	}{
		{
			name:     "invalid path",
			filePath: "./testdata/non-existing.json",
			wantErr:  "no such file or directory",
		},
		{
			name:     "empty file",
			filePath: "./testdata/empty.txt",
			wantErr:  "invalid Oversecured JSON report",
		},
		{
			name:     "wrong content",
			filePath: "./testdata/sbom-spdx.json",
			wantErr:  "missing scan id",
		},
		{
			// The real vendor response most likely to be mistaken for an export.
			name:     "paginated findings list, not an export",
			filePath: "./testdata/oversecured-findings-list.json",
			wantErr:  "missing scan id",
		},
		{
			name:     "header without vulnerabilities",
			filePath: "./testdata/oversecured-no-vulnerabilities.json",
			wantErr:  "missing vulnerabilities",
		},
		{
			name:     "vulnerabilities is not an array",
			filePath: "./testdata/oversecured-vulnerabilities-not-array.json",
			wantErr:  "vulnerabilities is not an array",
		},
		{
			// Every Oversecured export names the platform it scanned, so its
			// absence means this is not one of them.
			name:     "no app platform",
			filePath: "./testdata/oversecured-no-platform.json",
			wantErr:  "missing app platform",
		},
		{
			// A platform outside the android/ios the vendor scans today is logged
			// but accepted: pinning the vocabulary would make a new vendor platform
			// break every pipeline until Chainloop ships a release.
			name:     "unfamiliar app platform is accepted",
			filePath: "./testdata/oversecured-unfamiliar-platform.json",
			annotations: map[string]string{
				"chainloop.material.tool.name": "oversecured",
			},
		},
		{
			name:     "clean scan (empty vulnerabilities array)",
			filePath: "./testdata/oversecured-clean.json",
			annotations: map[string]string{
				"chainloop.material.tool.name": "oversecured",
			},
			absentAnnotations: []string{
				"chainloop.material.scan.types",
				"chainloop.material.tool.version",
			},
		},
		{
			name:     "clean scan (null vulnerabilities)",
			filePath: "./testdata/oversecured-null-vulnerabilities.json",
			annotations: map[string]string{
				"chainloop.material.tool.name": "oversecured",
			},
			absentAnnotations: []string{
				"chainloop.material.scan.types",
				"chainloop.material.tool.version",
			},
		},
		{
			// Trimmed real export (`oversecured report <scanId> --app <appId>
			// --format json`). Only the envelope is validated, so the findings it
			// keeps are there to pin the shape, not to add coverage: one SAST
			// finding with an AI write-up, one DAST finding carrying runtime stack
			// traces and no code[], and one plain low-severity one.
			name:     "real export",
			filePath: "./testdata/oversecured.json",
			annotations: map[string]string{
				"chainloop.material.tool.name": "oversecured",
			},
			absentAnnotations: []string{
				"chainloop.material.scan.types",
				"chainloop.material.tool.version",
			},
		},
	}

	schema := &contractAPI.CraftingSchema_Material{
		Name: "test",
		Type: contractAPI.CraftingSchema_Material_OVERSECURED_JSON,
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
			crafter, err := materials.NewOversecuredCrafter(schema, backend, &l)
			require.NoError(t, err)

			got, err := craftedMaterial(crafter.Craft(context.TODO(), tc.filePath))
			if tc.wantErr != "" {
				assert.ErrorContains(t, err, tc.wantErr)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, contractAPI.CraftingSchema_Material_OVERSECURED_JSON.String(), got.MaterialType.String())
			assert.True(t, got.UploadedToCas)

			for k, v := range tc.annotations {
				assert.Equal(t, v, got.Annotations[k])
			}

			for _, k := range tc.absentAnnotations {
				_, ok := got.Annotations[k]
				assert.False(t, ok, "annotation %q must not be set", k)
			}
		})
	}
}
