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

//nolint:dupl
package materials_test

import (
	"context"
	"testing"

	"code.cloudfoundry.org/bytefmt"
	contractAPI "github.com/chainloop-dev/chainloop/app/controlplane/api/workflowcontract/v1"
	attestationApi "github.com/chainloop-dev/chainloop/internal/attestation/crafter/api/attestation/v1"
	"github.com/chainloop-dev/chainloop/internal/attestation/crafter/materials"
	"github.com/chainloop-dev/chainloop/internal/casclient"
	mUploader "github.com/chainloop-dev/chainloop/internal/casclient/mocks"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewEvidenceCrafter(t *testing.T) {
	testCases := []struct {
		name    string
		input   *contractAPI.CraftingSchema_Material
		wantErr bool
	}{
		{
			name: "happy path",
			input: &contractAPI.CraftingSchema_Material{
				Type: contractAPI.CraftingSchema_Material_EVIDENCE,
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
			_, err := materials.NewEvidenceCrafter(tc.input, nil, nil)
			if tc.wantErr {
				assert.Error(t, err)
				return
			}

			assert.NoError(t, err)
		})
	}
}

func TestEvidenceCraft(t *testing.T) {
	assert := assert.New(t)
	schema := &contractAPI.CraftingSchema_Material{
		Name: "test",
		Type: contractAPI.CraftingSchema_Material_EVIDENCE,
	}

	l := zerolog.Nop()

	// Mock uploader
	uploader := mUploader.NewUploader(t)
	uploader.On("UploadFile", context.TODO(), "./testdata/simple.txt").
		Return(&casclient.UpDownStatus{
			Digest:   "deadbeef",
			Filename: "simple.txt",
		}, nil)

	backend := &casclient.CASBackend{Uploader: uploader}

	crafter, err := materials.NewEvidenceCrafter(schema, backend, &l)
	require.NoError(t, err)

	got, err := crafter.Craft(context.TODO(), "./testdata/simple.txt")
	assert.NoError(err)
	assert.Equal(contractAPI.CraftingSchema_Material_EVIDENCE.String(), got.MaterialType.String())
	assert.True(got.UploadedToCas)

	// The result includes the digest reference
	assert.Equal(got.GetArtifact(), &attestationApi.Attestation_Material_Artifact{
		Id: "test", Digest: "sha256:54181dfe59340b318253e59f7695f547c5c10d071cb75001170a389061349918", Name: "simple.txt",
	})
}

func TestEvidenceCraftInline(t *testing.T) {
	assert := assert.New(t)
	schema := &contractAPI.CraftingSchema_Material{
		Name: "test",
		Type: contractAPI.CraftingSchema_Material_EVIDENCE,
	}
	l := zerolog.Nop()

	t.Run("inline without size limit", func(t *testing.T) {
		backend := &casclient.CASBackend{}

		crafter, err := materials.NewEvidenceCrafter(schema, backend, &l)
		require.NoError(t, err)

		got, err := crafter.Craft(context.TODO(), "./testdata/simple.txt")
		assert.NoError(err)
		assertEvidenceMaterial(t, got)
	})

	t.Run("backend with size limit", func(t *testing.T) {
		backend := &casclient.CASBackend{
			MaxSize: 100 * bytefmt.BYTE,
		}

		crafter, err := materials.NewEvidenceCrafter(schema, backend, &l)
		require.NoError(t, err)

		got, err := crafter.Craft(context.TODO(), "./testdata/simple.txt")
		assert.NoError(err)
		assertEvidenceMaterial(t, got)
	})

	t.Run("backend with size limit too small", func(t *testing.T) {
		backend := &casclient.CASBackend{
			MaxSize: bytefmt.BYTE,
		}

		crafter, err := materials.NewEvidenceCrafter(schema, backend, &l)
		require.NoError(t, err)

		_, err = crafter.Craft(context.TODO(), "./testdata/simple.txt")
		assert.Error(err)
	})
}

func assertEvidenceMaterial(t *testing.T, got *attestationApi.Attestation_Material) {
	assert := assert.New(t)
	// Not uploaded to CAS
	assert.False(got.UploadedToCas)
	// The result includes the digest and inline content
	assert.Equal(got.GetArtifact(), &attestationApi.Attestation_Material_Artifact{
		Id: "test", Digest: "sha256:54181dfe59340b318253e59f7695f547c5c10d071cb75001170a389061349918", Name: "simple.txt",
		// Inline content
		Content: []byte("txt file"),
	})
}
