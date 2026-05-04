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

package materials_test

import (
	"context"
	"strings"
	"testing"

	contractAPI "github.com/chainloop-dev/chainloop/app/controlplane/api/workflowcontract/v1"
	attestationApi "github.com/chainloop-dev/chainloop/pkg/attestation/crafter/api/attestation/v1"
	"github.com/chainloop-dev/chainloop/pkg/attestation/crafter/materials"
	"github.com/chainloop-dev/chainloop/pkg/casclient"
	mUploader "github.com/chainloop-dev/chainloop/pkg/casclient/mocks"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewGraphQLCrafter(t *testing.T) {
	testCases := []struct {
		name    string
		input   *contractAPI.CraftingSchema_Material
		wantErr bool
	}{
		{
			name: "happy path",
			input: &contractAPI.CraftingSchema_Material{
				Type: contractAPI.CraftingSchema_Material_GRAPHQL_SPEC,
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
			_, err := materials.NewGraphQLCrafter(tc.input, nil, nil)
			if tc.wantErr {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)
		})
	}
}

func TestGraphQLCraft(t *testing.T) {
	testCases := []struct {
		name        string
		filePath    string
		wantErr     string
		schema      *contractAPI.CraftingSchema_Material
		annotations map[string]string
	}{
		{
			name:     "invalid path",
			filePath: "./testdata/non-existing.graphql",
			wantErr:  "can't open the file",
			schema: &contractAPI.CraftingSchema_Material{
				Name: "test",
				Type: contractAPI.CraftingSchema_Material_GRAPHQL_SPEC,
			},
		},
		{
			name:     "non-GraphQL file",
			filePath: "./testdata/simple.txt",
			wantErr:  "unexpected material type",
			schema: &contractAPI.CraftingSchema_Material{
				Name: "test",
				Type: contractAPI.CraftingSchema_Material_GRAPHQL_SPEC,
			},
		},
		{
			name:     "invalid GraphQL SDL",
			filePath: "./testdata/invalid.graphql",
			wantErr:  "unexpected material type",
			schema: &contractAPI.CraftingSchema_Material{
				Name: "test",
				Type: contractAPI.CraftingSchema_Material_GRAPHQL_SPEC,
			},
		},
		{
			name:     "valid GraphQL SDL",
			filePath: "./testdata/schema.graphql",
			schema: &contractAPI.CraftingSchema_Material{
				Name: "test",
				Type: contractAPI.CraftingSchema_Material_GRAPHQL_SPEC,
			},
			annotations: map[string]string{
				"chainloop.material.graphql.type_count": "3",
				"chainloop.material.graphql.directives": "auth,deprecated",
			},
		},
	}

	assert := assert.New(t)
	l := zerolog.Nop()
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			uploader := mUploader.NewUploader(t)
			if tc.wantErr == "" {
				uploader.On("UploadFile", context.TODO(), tc.filePath).
					Return(&casclient.UpDownStatus{
						Digest:   "deadbeef",
						Filename: "schema.graphql",
					}, nil)
			}

			backend := &casclient.CASBackend{Uploader: uploader}
			crafter, err := materials.NewGraphQLCrafter(tc.schema, backend, &l)
			require.NoError(t, err)

			got, err := crafter.Craft(context.TODO(), tc.filePath)
			if tc.wantErr != "" {
				assert.ErrorContains(err, tc.wantErr)
				return
			}
			require.NoError(t, err)

			assert.Equal(tc.schema.Type.String(), got.MaterialType.String())
			assert.True(got.UploadedToCas)

			assert.Equal(&attestationApi.Attestation_Material_Artifact{
				Id: "test", Digest: got.GetArtifact().Digest, Name: strings.Split(tc.filePath, "/")[2],
			}, got.GetArtifact())

			for k, v := range tc.annotations {
				assert.Equal(v, got.Annotations[k])
			}
		})
	}
}
