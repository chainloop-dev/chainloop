//
// Copyright 2024-2025 The Chainloop Authors.
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
	"path"
	"testing"

	"code.cloudfoundry.org/bytefmt"
	contractAPI "github.com/chainloop-dev/chainloop/app/controlplane/api/workflowcontract/v1"
	"github.com/chainloop-dev/chainloop/pkg/attestation/crafter/materials"
	"github.com/chainloop-dev/chainloop/pkg/casclient"
	mUploader "github.com/chainloop-dev/chainloop/pkg/casclient/mocks"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestNewAttestationCrafter(t *testing.T) {
	testCases := []struct {
		name    string
		input   *contractAPI.CraftingSchema_Material
		wantErr bool
	}{
		{
			name: "happy path",
			input: &contractAPI.CraftingSchema_Material{
				Type: contractAPI.CraftingSchema_Material_ATTESTATION,
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
			_, err := materials.NewAttestationCrafter(tc.input, nil, nil)
			if tc.wantErr {
				assert.Error(t, err)
				return
			}

			assert.NoError(t, err)
		})
	}
}

func TestInvalidAttestation(t *testing.T) {
	assert := assert.New(t)
	schema := &contractAPI.CraftingSchema_Material{
		Name: "test",
		Type: contractAPI.CraftingSchema_Material_ATTESTATION,
	}

	l := zerolog.Nop()

	// Mock uploader
	uploader := mUploader.NewUploader(t)
	backend := &casclient.CASBackend{Uploader: uploader}
	crafter, _ := materials.NewAttestationCrafter(schema, backend, &l)

	t.Run("wrong format", func(_ *testing.T) {
		// Invalid payload
		_, err := crafter.Craft(context.TODO(), "./testdata/simple.txt")
		assert.Error(err)
	})

	t.Run("wrong payload", func(_ *testing.T) {
		// Invalid payload
		_, err := crafter.Craft(context.TODO(), "./testdata/attestation-invalid-payload.json")
		assert.Contains(err.Error(), "unable to base64 decode payload")
	})

	t.Run("wrong in-toto statement", func(_ *testing.T) {
		// Invalid payload
		_, err := crafter.Craft(context.TODO(), "./testdata/attestation-invalid-intoto.json")
		assert.Contains(err.Error(), "failed to parse the DSSE payload")
	})
}

func TestAttestationCraft(t *testing.T) {
	var testCases = []struct {
		name      string
		filePath  string
		digest    string
		expectErr bool
	}{
		{
			name:     "DSSE envelope",
			filePath: "./testdata/attestation-dsse.json",
			digest:   "sha256:3911ab20e43d801d35459c53168f6cba66d50af99dcc9e12aeb84a95c0d231df",
		},
		{
			name:     "Sigstore bundle",
			filePath: "./testdata/attestation-bundle.json",
			digest:   "sha256:fa7165a16cc1efdd24457a12dda613bbbfc903d3a2538a5ce8779b157d39b04c",
		},
		{
			name:      "Invalid payload type",
			filePath:  "./testdata/attestation-dsse-invalidtype.json",
			expectErr: true,
		},
	}

	assert := assert.New(t)
	l := zerolog.Nop()

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			schema := &contractAPI.CraftingSchema_Material{
				Name: "test",
				Type: contractAPI.CraftingSchema_Material_ATTESTATION,
			}

			// Mock uploader
			uploader := mUploader.NewUploader(t)
			if !tc.expectErr {
				uploader.On("UploadFile", context.Background(), mock.Anything).
					Return(&casclient.UpDownStatus{
						Digest:   "deadbeef",
						Filename: "attestation.json",
					}, nil)
			}

			backend := &casclient.CASBackend{Uploader: uploader}

			crafter, err := materials.NewAttestationCrafter(schema, backend, &l)
			require.NoError(t, err)

			got, err := crafter.Craft(context.Background(), tc.filePath)
			if tc.expectErr {
				assert.Error(err)
				return
			}
			assert.NoError(err)
			assert.Equal(contractAPI.CraftingSchema_Material_ATTESTATION.String(), got.MaterialType.String())
			assert.True(got.UploadedToCas)

			// The result includes the name and digest reference
			assert.Equal(path.Base(tc.filePath), got.GetArtifact().GetName())
			assert.Equal(tc.digest, got.GetArtifact().GetDigest())

			uploader.AssertExpectations(t)
		})
	}

}

func TestAttestationCraftInline(t *testing.T) {
	assert := assert.New(t)
	schema := &contractAPI.CraftingSchema_Material{
		Name: "test",
		Type: contractAPI.CraftingSchema_Material_ATTESTATION,
	}
	l := zerolog.Nop()

	t.Run("inline without size limit", func(t *testing.T) {
		backend := &casclient.CASBackend{}

		crafter, err := materials.NewAttestationCrafter(schema, backend, &l)
		require.NoError(t, err)

		got, err := crafter.Craft(context.TODO(), "./testdata/attestation-dsse.json")
		assert.NoError(err)

		assert.NotNil(got)
	})

	t.Run("backend with small size limit", func(t *testing.T) {
		backend := &casclient.CASBackend{
			MaxSize: 100 * bytefmt.BYTE,
		}

		crafter, err := materials.NewAttestationCrafter(schema, backend, &l)
		require.NoError(t, err)

		_, err = crafter.Craft(context.TODO(), "./testdata/attestation-dsse.json")
		assert.Error(err)
	})
}
