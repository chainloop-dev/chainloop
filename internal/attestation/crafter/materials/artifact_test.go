//
// Copyright 2023 The Chainloop Authors.
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
	"time"

	attestationApi "github.com/chainloop-dev/chainloop/app/cli/api/attestation/v1"
	contractAPI "github.com/chainloop-dev/chainloop/app/controlplane/api/workflowcontract/v1"
	"github.com/chainloop-dev/chainloop/internal/attestation/crafter/materials"
	"github.com/chainloop-dev/chainloop/internal/casclient"
	mUploader "github.com/chainloop-dev/chainloop/internal/casclient/mocks"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewArtifactCrafter(t *testing.T) {
	testCases := []struct {
		name    string
		input   *contractAPI.CraftingSchema_Material
		wantErr bool
	}{
		{
			name: "happy path",
			input: &contractAPI.CraftingSchema_Material{
				Type: contractAPI.CraftingSchema_Material_ARTIFACT,
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
			_, err := materials.NewArtifactCrafter(tc.input, nil, nil)
			if tc.wantErr {
				assert.Error(t, err)
				return
			}

			assert.NoError(t, err)
		})
	}
}

func TestArtifacftCraft(t *testing.T) {
	assert := assert.New(t)
	schema := &contractAPI.CraftingSchema_Material{
		Name: "test",
		Type: contractAPI.CraftingSchema_Material_ARTIFACT,
	}

	l := zerolog.Nop()

	// Mock uploader
	uploader := mUploader.NewUploader(t)
	uploader.On("Upload", context.TODO(), "file.txt").
		Return(&casclient.UpDownStatus{
			Digest:   "deadbeef",
			Filename: "file.txt",
		}, nil)

	crafter, err := materials.NewArtifactCrafter(schema, uploader, &l)
	require.NoError(t, err)

	got, err := crafter.Craft(context.TODO(), "file.txt")
	assert.NoError(err)
	assert.Equal(contractAPI.CraftingSchema_Material_ARTIFACT.String(), got.MaterialType.String())
	assert.WithinDuration(time.Now(), got.AddedAt.AsTime(), 5*time.Second)

	// The result includes the digest reference
	assert.Equal(got.GetArtifact(), &attestationApi.Attestation_Material_Artifact{
		Id: "test", Digest: "deadbeef", Name: "file.txt",
	})
}
