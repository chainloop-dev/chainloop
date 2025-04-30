//
// Copyright 2025 The Chainloop Authors.
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

func TestInvalidSLSAProvenance(t *testing.T) {
	assert := assert.New(t)
	schema := &contractAPI.CraftingSchema_Material{
		Name: "test",
		Type: contractAPI.CraftingSchema_Material_SLSA_PROVENANCE,
	}

	l := zerolog.Nop()

	// Mock uploader
	uploader := mUploader.NewUploader(t)
	backend := &casclient.CASBackend{Uploader: uploader}
	crafter, _ := materials.NewSLSAProvenanceCrafter(schema, backend, &l)

	t.Run("wrong format", func(_ *testing.T) {
		// Invalid payload
		_, err := crafter.Craft(context.TODO(), "./testdata/simple.txt")
		assert.Error(err)
	})

	t.Run("is not a sigstore bundle but a DSSE envelope", func(_ *testing.T) {
		// Invalid payload
		_, err := crafter.Craft(context.TODO(), "./testdata/attestation-dsse.json")
		assert.Contains(err.Error(), "failed to unmarshal bundle")
	})

	t.Run("it is a bundle but not a valid SLSA Provenance", func(_ *testing.T) {
		// Invalid payload
		_, err := crafter.Craft(context.TODO(), "./testdata/chainloop_attestation.sigstore.json")
		assert.Contains(err.Error(), "the provided predicate is not a valid SLSA Provenance")
	})
}

func TestSLSAProvenanceCraft(t *testing.T) {
	assert := assert.New(t)
	schema := &contractAPI.CraftingSchema_Material{
		Name: "test",
		Type: contractAPI.CraftingSchema_Material_SLSA_PROVENANCE,
	}

	l := zerolog.Nop()

	// Mock uploader
	uploader := mUploader.NewUploader(t)
	uploader.On("UploadFile", context.TODO(), mock.Anything).
		Return(&casclient.UpDownStatus{
			Digest:   "deadbeef",
			Filename: "attestation.json",
		}, nil)

	backend := &casclient.CASBackend{Uploader: uploader}

	crafter, err := materials.NewSLSAProvenanceCrafter(schema, backend, &l)
	require.NoError(t, err)

	got, err := crafter.Craft(context.TODO(), "./testdata/slsa_provenance.sigstore.json")
	assert.NoError(err)
	assert.Equal(contractAPI.CraftingSchema_Material_SLSA_PROVENANCE.String(), got.MaterialType.String())
	assert.True(got.UploadedToCas)

	// The result includes the digest reference
	assert.Equal("test", got.GetArtifact().Id)
	assert.Equal("sha256:38dc33ded482dcb3efccccea977e24e11bd063bccf499da1cc46487255ff6857", got.GetArtifact().Digest)

	uploader.AssertExpectations(t)
}
