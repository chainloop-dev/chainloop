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

func TestNewOpenAPICrafter(t *testing.T) {
	testCases := []struct {
		name    string
		input   *contractAPI.CraftingSchema_Material
		wantErr bool
	}{
		{
			name: "happy path",
			input: &contractAPI.CraftingSchema_Material{
				Type: contractAPI.CraftingSchema_Material_OPENAPI_SPEC,
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
			_, err := materials.NewOpenAPICrafter(tc.input, nil, nil)
			if tc.wantErr {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)
		})
	}
}

func TestOpenAPICraft(t *testing.T) {
	testCases := []struct {
		name        string
		filePath    string
		wantErr     string
		digest      string
		schema      *contractAPI.CraftingSchema_Material
		annotations map[string]string
	}{
		{
			name:     "invalid path",
			filePath: "./testdata/non-existing.json",
			wantErr:  "can't open the file",
			schema: &contractAPI.CraftingSchema_Material{
				Name: "test",
				Type: contractAPI.CraftingSchema_Material_OPENAPI_SPEC,
			},
		},
		{
			name:     "non-JSON/YAML file",
			filePath: "./testdata/simple.txt",
			wantErr:  "unexpected material type",
			schema: &contractAPI.CraftingSchema_Material{
				Name: "test",
				Type: contractAPI.CraftingSchema_Material_OPENAPI_SPEC,
			},
		},
		{
			name:     "random JSON without openapi field",
			filePath: "./testdata/random.json",
			wantErr:  "unexpected material type",
			schema: &contractAPI.CraftingSchema_Material{
				Name: "test",
				Type: contractAPI.CraftingSchema_Material_OPENAPI_SPEC,
			},
		},
		{
			name:     "invalid OpenAPI spec",
			filePath: "./testdata/openapi-invalid.json",
			wantErr:  "invalid OpenAPI spec file",
			schema: &contractAPI.CraftingSchema_Material{
				Name: "test",
				Type: contractAPI.CraftingSchema_Material_OPENAPI_SPEC,
			},
		},
		{
			name:     "valid OpenAPI 3.0 JSON",
			filePath: "./testdata/openapi-3.0.json",
			schema: &contractAPI.CraftingSchema_Material{
				Name: "test",
				Type: contractAPI.CraftingSchema_Material_OPENAPI_SPEC,
			},
			annotations: map[string]string{
				"chainloop.material.api.name":         "Petstore API",
				"chainloop.material.api.spec_version": "3.0.3",
				"chainloop.material.api.version":      "1.0.0",
			},
		},
		{
			name:     "valid OpenAPI 3.1 YAML",
			filePath: "./testdata/openapi-3.1.yaml",
			schema: &contractAPI.CraftingSchema_Material{
				Name: "test",
				Type: contractAPI.CraftingSchema_Material_OPENAPI_SPEC,
			},
			annotations: map[string]string{
				"chainloop.material.api.name":         "Bookstore API",
				"chainloop.material.api.spec_version": "3.1.0",
				"chainloop.material.api.version":      "2.0.0",
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
						Filename: "spec.json",
					}, nil)
			}

			backend := &casclient.CASBackend{Uploader: uploader}
			crafter, err := materials.NewOpenAPICrafter(tc.schema, backend, &l)
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

func TestOpenAPICraftNoStrictValidation(t *testing.T) {
	l := zerolog.Nop()
	uploader := mUploader.NewUploader(t)
	uploader.On("UploadFile", context.TODO(), "./testdata/openapi-invalid.json").
		Return(&casclient.UpDownStatus{
			Digest:   "deadbeef",
			Filename: "spec.json",
		}, nil)

	backend := &casclient.CASBackend{Uploader: uploader}
	schema := &contractAPI.CraftingSchema_Material{
		Name: "test",
		Type: contractAPI.CraftingSchema_Material_OPENAPI_SPEC,
	}

	crafter, err := materials.NewOpenAPICrafter(schema, backend, &l, materials.WithOpenAPINoStrictValidation(true))
	require.NoError(t, err)

	got, err := crafter.Craft(context.TODO(), "./testdata/openapi-invalid.json")
	require.NoError(t, err)
	assert.NotNil(t, got)
	assert.Equal(t, schema.Type.String(), got.MaterialType.String())
}
