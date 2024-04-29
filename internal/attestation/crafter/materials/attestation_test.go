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
		assert.Contains(err.Error(), "un-marshaling predicate")
	})
}

func TestAttestationCraft(t *testing.T) {
	assert := assert.New(t)
	schema := &contractAPI.CraftingSchema_Material{
		Name: "test",
		Type: contractAPI.CraftingSchema_Material_ATTESTATION,
	}

	l := zerolog.Nop()

	// Mock uploader
	uploader := mUploader.NewUploader(t)
	uploader.On("UploadFile", context.TODO(), "./testdata/attestation.json").
		Return(&casclient.UpDownStatus{
			Digest:   "deadbeef",
			Filename: "attestation.json",
		}, nil)

	backend := &casclient.CASBackend{Uploader: uploader}

	crafter, err := materials.NewAttestationCrafter(schema, backend, &l)
	require.NoError(t, err)

	got, err := crafter.Craft(context.TODO(), "./testdata/attestation.json")
	assert.NoError(err)
	assert.Equal(contractAPI.CraftingSchema_Material_ATTESTATION.String(), got.MaterialType.String())
	assert.True(got.UploadedToCas)

	// The result includes the digest reference
	assert.Equal(&attestationApi.Attestation_Material_Artifact{
		Id: "test", Digest: "sha256:3911ab20e43d801d35459c53168f6cba66d50af99dcc9e12aeb84a95c0d231df", Name: "attestation.json",
	}, got.GetArtifact())
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

		got, err := crafter.Craft(context.TODO(), "./testdata/attestation.json")
		assert.NoError(err)

		assert.NotNil(got)
	})

	t.Run("backend with small size limit", func(t *testing.T) {
		backend := &casclient.CASBackend{
			MaxSize: 100 * bytefmt.BYTE,
		}

		crafter, err := materials.NewAttestationCrafter(schema, backend, &l)
		require.NoError(t, err)

		_, err = crafter.Craft(context.TODO(), "./testdata/attestation.json")
		assert.Error(err)
	})
}
