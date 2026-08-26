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
	"path/filepath"
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

func TestPitestCraft(t *testing.T) {
	testCases := []struct {
		name       string
		filePath   string
		wantErr    string
		wantDigest string
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
			name:     "wrong xml root (cobertura coverage)",
			filePath: "./testdata/cobertura.xml",
			wantErr:  "unexpected material type",
		},
		{
			name:     "wrong xml root (junit testsuite)",
			filePath: "./testdata/junit.xml",
			wantErr:  "unexpected material type",
		},
		{
			name:       "valid artifact type",
			filePath:   "./testdata/pitest.xml",
			wantDigest: "sha256:32bce80d562307c69f3e60eadc2aaa56be2f9d47d4aff354aa35e7c4d6bb909f",
		},
		{
			name:       "valid full mutation matrix report",
			filePath:   "./testdata/pitest-full-matrix.xml",
			wantDigest: "sha256:ed5677f32b319ee35064e92b2fcc0d03db266cb1484e209f293fc007f8aed494",
		},
	}

	assert := assert.New(t)
	schema := &contractAPI.CraftingSchema_Material{
		Name: "test",
		Type: contractAPI.CraftingSchema_Material_PITEST_XML,
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
						Filename: "pitest.xml",
					}, nil)
			}
			backend := &casclient.CASBackend{Uploader: uploader}
			crafter := materials.NewPitestCrafter(schema, backend, &l)

			got, err := crafter.Craft(context.TODO(), tc.filePath)
			if tc.wantErr != "" {
				assert.ErrorContains(err, tc.wantErr)
				return
			}

			require.NoError(t, err)
			assert.Equal(contractAPI.CraftingSchema_Material_PITEST_XML.String(), got.MaterialType.String())
			assert.True(got.UploadedToCas)

			// The result includes the digest reference
			assert.Equal(&attestationApi.Attestation_Material_Artifact{
				Id: "test", Digest: tc.wantDigest, Name: filepath.Base(tc.filePath),
			}, got.GetArtifact())
		})
	}
}

// TestPitestCraftEmptyReport asserts that a report with a valid <mutations>
// root but no <mutation> records is rejected: it means no mutants were
// analyzed, not a 0% or 100% result, and downstream score calculations would
// otherwise divide by zero. The file must not be uploaded to the CAS.
func TestPitestCraftEmptyReport(t *testing.T) {
	schema := &contractAPI.CraftingSchema_Material{
		Name: "test",
		Type: contractAPI.CraftingSchema_Material_PITEST_XML,
	}
	l := zerolog.Nop()
	// No Upload expectation: the mock fails the test if an upload happens.
	uploader := mUploader.NewUploader(t)
	backend := &casclient.CASBackend{Uploader: uploader}
	crafter := materials.NewPitestCrafter(schema, backend, &l)

	_, err := crafter.Craft(context.TODO(), "./testdata/pitest-empty.xml")
	require.Error(t, err)
	assert.ErrorContains(t, err, "invalid PIT report file, no mutations found")
	assert.ErrorIs(t, err, materials.ErrInvalidMaterialType)
}
