//
// Copyright 2023-2026 The Chainloop Authors.
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
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestNewSARIFCrafter(t *testing.T) {
	testCases := []struct {
		name    string
		input   *contractAPI.CraftingSchema_Material
		wantErr bool
	}{
		{
			name: "happy path",
			input: &contractAPI.CraftingSchema_Material{
				Type: contractAPI.CraftingSchema_Material_SARIF,
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
			_, err := materials.NewSARIFCrafter(tc.input, nil, nil)
			if tc.wantErr {
				assert.Error(t, err)
				return
			}

			assert.NoError(t, err)
		})
	}
}

func TestSARIFCraft(t *testing.T) {
	testCases := []struct {
		name           string
		filePath       string
		wantErr        string
		expectedDigest string
		expectedName   string
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
	}

	assert := assert.New(t)
	schema := &contractAPI.CraftingSchema_Material{
		Name: "test",
		Type: contractAPI.CraftingSchema_Material_SARIF,
	}
	l := zerolog.Nop()
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Mock uploader
			uploader := mUploader.NewUploader(t)
			if tc.wantErr == "" {
				uploader.On("Upload", context.TODO(), mock.Anything, mock.Anything, mock.Anything).
					Return(&casclient.UpDownStatus{
						Digest:   "deadbeef",
						Filename: "report.sarif",
					}, nil)
			}

			backend := &casclient.CASBackend{Uploader: uploader}
			crafter, err := materials.NewSARIFCrafter(schema, backend, &l)
			require.NoError(t, err)

			got, err := crafter.Craft(context.TODO(), tc.filePath)
			if tc.wantErr != "" {
				assert.ErrorContains(err, tc.wantErr)
				return
			}
			require.NoError(t, err)

			assert.Equal(contractAPI.CraftingSchema_Material_SARIF.String(), got.MaterialType.String())
			assert.True(got.UploadedToCas)

			if tc.expectedDigest != "" {
				assert.Equal(&attestationApi.Attestation_Material_Artifact{
					Id: "test", Digest: tc.expectedDigest, Name: tc.expectedName,
				}, got.GetArtifact())
			}
		})
	}
}

func TestSARIFCraft_ScanTypes(t *testing.T) {
	testCases := []struct {
		name     string
		filePath string
		// annotations lists annotation keys that must be set to the given value.
		annotations map[string]string
		// absentAnnotations lists annotation keys that must NOT be set. A
		// non-Checkmarx SARIF (or one with only unrecognized engines) advertises no
		// engine types, so scan.types must be omitted (fail closed) rather than set
		// to an empty value.
		absentAnnotations []string
	}{
		{
			// Checkmarx One SARIF bundling multiple engines under a single driver.
			// Engine types are read from rules[].properties.tags / the "(engine)"
			// ruleId suffix and normalized to the canonical vocabulary (kics -> iac),
			// sorted and comma-joined.
			name:     "checkmarx multi-engine SARIF",
			filePath: "./testdata/checkmarx.sarif",
			annotations: map[string]string{
				"chainloop.material.scan.types": "iac,sast,sca",
			},
		},
		{
			// containers -> container and sscs -> supply-chain map to the canonical
			// vocabulary; an unmapped engine ("future-engine") is dropped so no
			// vendor-specific value leaks into the annotation.
			name:     "checkmarx SARIF with extra + unmapped engines",
			filePath: "./testdata/checkmarx-extra-engines.sarif",
			annotations: map[string]string{
				"chainloop.material.scan.types": "container,supply-chain",
			},
		},
		{
			// A non-Checkmarx SARIF (tfsec) must never get a scan.types annotation:
			// the engine normalization is Checkmarx-specific, so recognition fails
			// closed for other tools rather than over-claiming.
			name:              "non-checkmarx SARIF gets no scan.types",
			filePath:          "./testdata/report.sarif",
			absentAnnotations: []string{"chainloop.material.scan.types"},
		},
	}

	l := zerolog.Nop()
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			uploader := mUploader.NewUploader(t)
			uploader.On("Upload", context.TODO(), mock.Anything, mock.Anything, mock.Anything).
				Return(&casclient.UpDownStatus{
					Digest:   "deadbeef",
					Filename: "report.sarif",
				}, nil)

			schema := &contractAPI.CraftingSchema_Material{
				Name: "test",
				Type: contractAPI.CraftingSchema_Material_SARIF,
			}
			backend := &casclient.CASBackend{Uploader: uploader}
			crafter, err := materials.NewSARIFCrafter(schema, backend, &l)
			require.NoError(t, err)

			got, err := crafter.Craft(context.TODO(), tc.filePath)
			require.NoError(t, err)

			for k, v := range tc.annotations {
				assert.Equal(t, v, got.Annotations[k], "annotation %q", k)
			}
			for _, k := range tc.absentAnnotations {
				_, ok := got.Annotations[k]
				assert.False(t, ok, "annotation %q must not be set", k)
			}
		})
	}
}

func TestSARIFCraft_SkipUpload(t *testing.T) {
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

	assert := assert.New(t)
	l := zerolog.Nop()
	filePath := "./testdata/report.sarif"

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			schema := &contractAPI.CraftingSchema_Material{
				Name:       "test",
				Type:       contractAPI.CraftingSchema_Material_SARIF,
				SkipUpload: tc.skipUpload,
			}

			// Mock uploader - only expect upload call if not skipped
			uploader := mUploader.NewUploader(t)
			if tc.wantUpload {
				uploader.On("Upload", context.TODO(), mock.Anything, mock.Anything, mock.Anything).
					Return(&casclient.UpDownStatus{
						Digest:   "deadbeef",
						Filename: "report.sarif",
					}, nil)
			}

			backend := &casclient.CASBackend{Uploader: uploader}
			crafter, err := materials.NewSARIFCrafter(schema, backend, &l)
			require.NoError(t, err)

			got, err := crafter.Craft(context.TODO(), filePath)
			require.NoError(t, err)

			assert.Equal(contractAPI.CraftingSchema_Material_SARIF.String(), got.MaterialType.String())

			// Verify upload behavior matches expectation
			if tc.wantUpload {
				assert.True(got.UploadedToCas, "material should be uploaded when skip_upload is false")
			} else {
				assert.False(got.UploadedToCas, "material should not be uploaded when skip_upload is true")
			}

			// Verify digest is always computed regardless of upload
			assert.Equal("sha256:c4a63494f9289dd9fd44f841efb4f5b52765c2de6332f2d86e5f6c0340b40a95", got.GetArtifact().Digest)
			assert.Equal("report.sarif", got.GetArtifact().Name)
		})
	}
}
