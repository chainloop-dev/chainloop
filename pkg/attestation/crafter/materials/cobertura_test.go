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
	attestationApi "github.com/chainloop-dev/chainloop/pkg/attestation/crafter/api/attestation/v1"
	"github.com/chainloop-dev/chainloop/pkg/attestation/crafter/materials"
	"github.com/chainloop-dev/chainloop/pkg/casclient"
	mUploader "github.com/chainloop-dev/chainloop/pkg/casclient/mocks"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestCoberturaCraft(t *testing.T) {
	testCases := []struct {
		name     string
		filePath string
		wantErr  string
	}{
		{
			name:     "invalid path",
			filePath: "./testdata/non-existing.xml",
			wantErr:  "no such file or directory",
		},
		{
			name:     "invalid artifact type",
			filePath: "./testdata/simple.txt",
			wantErr:  "unexpected material type",
		},
		{
			name:     "wrong xml root (jacoco report)",
			filePath: "./testdata/jacoco.xml",
			wantErr:  "unexpected material type",
		},
		{
			name:     "wrong xml root (junit testsuite)",
			filePath: "./testdata/junit.xml",
			wantErr:  "unexpected material type",
		},
		{
			name:     "valid artifact type",
			filePath: "./testdata/cobertura.xml",
		},
	}

	assert := assert.New(t)
	schema := &contractAPI.CraftingSchema_Material{
		Name: "test",
		Type: contractAPI.CraftingSchema_Material_COBERTURA_XML,
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
						Filename: "cobertura.xml",
					}, nil)
			}
			backend := &casclient.CASBackend{Uploader: uploader}
			crafter := materials.NewCoberturaCrafter(schema, backend, &l)

			got, err := crafter.Craft(context.TODO(), tc.filePath)
			if tc.wantErr != "" {
				assert.ErrorContains(err, tc.wantErr)
				return
			}

			require.NoError(t, err)
			assert.Equal(contractAPI.CraftingSchema_Material_COBERTURA_XML.String(), got.MaterialType.String())
			assert.True(got.UploadedToCas)

			// The result includes the digest reference
			assert.Equal(&attestationApi.Attestation_Material_Artifact{
				Id: "test", Digest: "sha256:00b1d466b66effb5b42b02adcaa63c99fcf3df4d7e54f66e0e481bf4a15fdd38", Name: "cobertura.xml",
			}, got.GetArtifact())
		})
	}
}

// TestCoberturaCraftEmptyReport asserts that a legitimate empty coverage report
// (a service with no measurable lines, where the tool emits line-rate="NaN")
// is accepted, not rejected. Empty coverage is valid evidence; it must be
// attestable so a downstream policy can decide it is not a violation.
func TestCoberturaCraftEmptyReport(t *testing.T) {
	schema := &contractAPI.CraftingSchema_Material{
		Name: "test",
		Type: contractAPI.CraftingSchema_Material_COBERTURA_XML,
	}
	l := zerolog.Nop()
	uploader := mUploader.NewUploader(t)
	uploader.On("Upload", context.TODO(), mock.Anything, mock.Anything, mock.Anything).
		Return(&casclient.UpDownStatus{Digest: "deadbeef", Filename: "cobertura-empty.xml"}, nil)
	backend := &casclient.CASBackend{Uploader: uploader}
	crafter := materials.NewCoberturaCrafter(schema, backend, &l)

	got, err := crafter.Craft(context.TODO(), "./testdata/cobertura-empty.xml")
	require.NoError(t, err, "an empty-but-valid cobertura report must be accepted")
	require.NotNil(t, got)
	assert.Equal(t, contractAPI.CraftingSchema_Material_COBERTURA_XML.String(), got.MaterialType.String())
}
