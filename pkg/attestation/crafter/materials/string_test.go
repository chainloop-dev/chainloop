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

	contractAPI "github.com/chainloop-dev/chainloop/app/controlplane/api/workflowcontract/v1"
	attestationApi "github.com/chainloop-dev/chainloop/pkg/attestation/crafter/api/attestation/v1"
	"github.com/chainloop-dev/chainloop/pkg/attestation/crafter/materials"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewStringCrafter(t *testing.T) {
	testCases := []struct {
		name    string
		input   *contractAPI.CraftingSchema_Material
		wantErr bool
	}{
		{
			name: "happy path",
			input: &contractAPI.CraftingSchema_Material{
				Name: "test",
				Type: contractAPI.CraftingSchema_Material_STRING,
			},
		},
		{
			name: "wrong type",
			input: &contractAPI.CraftingSchema_Material{
				Name: "test",
				Type: contractAPI.CraftingSchema_Material_CONTAINER_IMAGE,
			},
			wantErr: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := materials.NewStringCrafter(tc.input)
			if tc.wantErr {
				assert.Error(t, err)
				return
			}

			assert.NoError(t, err)
		})
	}
}

func TestStringCraft(t *testing.T) {
	assert := assert.New(t)
	schema := &contractAPI.CraftingSchema_Material{
		Name: "test",
		Type: contractAPI.CraftingSchema_Material_STRING,
	}

	crafter, err := materials.NewStringCrafter(schema)
	require.NoError(t, err)

	got, err := crafter.Craft(context.TODO(), "value")
	assert.NoError(err)
	assert.Equal(contractAPI.CraftingSchema_Material_STRING, got.MaterialType)
	assert.False(got.UploadedToCas)
	assert.Equal(got.GetString_(), &attestationApi.Attestation_Material_KeyVal{
		Id: "test", Value: "value", Digest: "sha256:cd42404d52ad55ccfa9aca4adc828aa5800ad9d385a0671fbcbf724118320619",
	})
}
