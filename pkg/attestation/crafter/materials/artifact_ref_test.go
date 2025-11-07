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
	attestationApi "github.com/chainloop-dev/chainloop/pkg/attestation/crafter/api/attestation/v1"
	"github.com/chainloop-dev/chainloop/pkg/attestation/crafter/materials"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewArtifactRefCrafter(t *testing.T) {
	testCases := []struct {
		name    string
		input   *contractAPI.CraftingSchema_Material
		wantErr bool
	}{
		{
			name: "happy path",
			input: &contractAPI.CraftingSchema_Material{
				Type: contractAPI.CraftingSchema_Material_ARTIFACT_REF,
			},
		},
		{
			name: "wrong type",
			input: &contractAPI.CraftingSchema_Material{
				Type: contractAPI.CraftingSchema_Material_ARTIFACT,
			},
			wantErr: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := materials.NewArtifactRefCrafter(tc.input, nil)
			if tc.wantErr {
				assert.Error(t, err)
				return
			}

			assert.NoError(t, err)
		})
	}
}

func TestArtifactRefCraft(t *testing.T) {
	assert := assert.New(t)
	schema := &contractAPI.CraftingSchema_Material{
		Name: "test",
		Type: contractAPI.CraftingSchema_Material_ARTIFACT_REF,
	}

	l := zerolog.Nop()

	crafter, err := materials.NewArtifactRefCrafter(schema, &l)
	require.NoError(t, err)

	got, err := crafter.Craft(context.TODO(), "./testdata/simple.txt")
	assert.NoError(err)
	assert.Equal(contractAPI.CraftingSchema_Material_ARTIFACT_REF.String(), got.MaterialType.String())

	// Should not be uploaded to CAS
	assert.False(got.UploadedToCas)
	assert.False(got.InlineCas)

	// The result includes the digest reference but no content
	assert.Equal(got.GetArtifact(), &attestationApi.Attestation_Material_Artifact{
		Id: "test", Digest: "sha256:54181dfe59340b318253e59f7695f547c5c10d071cb75001170a389061349918", Name: "simple.txt",
	})
}

func TestArtifactRefCraftEmptyFile(t *testing.T) {
	assert := assert.New(t)
	schema := &contractAPI.CraftingSchema_Material{
		Name: "test",
		Type: contractAPI.CraftingSchema_Material_ARTIFACT_REF,
	}

	l := zerolog.Nop()

	crafter, err := materials.NewArtifactRefCrafter(schema, &l)
	require.NoError(t, err)

	_, err = crafter.Craft(context.TODO(), "./testdata/empty.txt")
	assert.Error(err)
	assert.Contains(err.Error(), "file is empty")
}

func TestArtifactRefCraftNonExistentFile(t *testing.T) {
	assert := assert.New(t)
	schema := &contractAPI.CraftingSchema_Material{
		Name: "test",
		Type: contractAPI.CraftingSchema_Material_ARTIFACT_REF,
	}

	l := zerolog.Nop()

	crafter, err := materials.NewArtifactRefCrafter(schema, &l)
	require.NoError(t, err)

	_, err = crafter.Craft(context.TODO(), "./testdata/nonexistent.txt")
	assert.Error(err)
}
