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

	contractAPI "github.com/chainloop-dev/chainloop/app/controlplane/api/workflowcontract/v1"
	attestationApi "github.com/chainloop-dev/chainloop/pkg/attestation/crafter/api/attestation/v1"
	"github.com/chainloop-dev/chainloop/pkg/attestation/crafter/materials"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCraft(t *testing.T) {
	assert := assert.New(t)
	schema := &contractAPI.CraftingSchema_Material{
		Name: "test",
		Type: contractAPI.CraftingSchema_Material_STRING,
		Annotations: []*contractAPI.Annotation{
			{
				Name:  "test",
				Value: "test",
			},
		},
	}

	got, err := materials.Craft(context.TODO(), schema, "test-value", nil, nil, nil)
	require.NoError(t, err)
	assert.Equal(contractAPI.CraftingSchema_Material_STRING, got.MaterialType)
	assert.False(got.UploadedToCas)
	assert.Equal(got.GetString_(), &attestationApi.Attestation_Material_KeyVal{
		Id: "test", Value: "test-value", Digest: "sha256:5b1406fffc9de5537eb35a845c99521f26fba0e772d58b42e09f4221b9e043ae",
	})

	// Timestamp
	assert.WithinDuration(time.Now(), got.AddedAt.AsTime(), 5*time.Second)
	// Annotations
	assert.Equal(map[string]string{"test": "test"}, got.Annotations)
}
