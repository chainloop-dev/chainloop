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

func TestNewAsyncAPICrafter(t *testing.T) {
	testCases := []struct {
		name    string
		input   *contractAPI.CraftingSchema_Material
		wantErr bool
	}{
		{
			name: "happy path",
			input: &contractAPI.CraftingSchema_Material{
				Type: contractAPI.CraftingSchema_Material_ASYNCAPI_SPEC,
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
			_, err := materials.NewAsyncAPICrafter(tc.input, nil, nil)
			if tc.wantErr {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)
		})
	}
}

func TestAsyncAPICraft(t *testing.T) {
	testCases := []struct {
		name        string
		filePath    string
		wantErr     string
		schema      *contractAPI.CraftingSchema_Material
		annotations map[string]string
	}{
		{
			name:     "invalid path",
			filePath: "./testdata/non-existing.json",
			wantErr:  "unexpected material type",
			schema: &contractAPI.CraftingSchema_Material{
				Name: "test",
				Type: contractAPI.CraftingSchema_Material_ASYNCAPI_SPEC,
			},
		},
		{
			name:     "non-JSON/YAML file",
			filePath: "./testdata/simple.txt",
			wantErr:  "unexpected material type",
			schema: &contractAPI.CraftingSchema_Material{
				Name: "test",
				Type: contractAPI.CraftingSchema_Material_ASYNCAPI_SPEC,
			},
		},
		{
			name:     "random JSON without asyncapi field",
			filePath: "./testdata/random.json",
			wantErr:  "unexpected material type",
			schema: &contractAPI.CraftingSchema_Material{
				Name: "test",
				Type: contractAPI.CraftingSchema_Material_ASYNCAPI_SPEC,
			},
		},
		{
			name:     "invalid AsyncAPI spec",
			filePath: "./testdata/asyncapi-invalid.json",
			wantErr:  "unexpected material type",
			schema: &contractAPI.CraftingSchema_Material{
				Name: "test",
				Type: contractAPI.CraftingSchema_Material_ASYNCAPI_SPEC,
			},
		},
		{
			name:     "valid AsyncAPI 2.6 JSON",
			filePath: "./testdata/asyncapi-2.6.json",
			schema: &contractAPI.CraftingSchema_Material{
				Name: "test",
				Type: contractAPI.CraftingSchema_Material_ASYNCAPI_SPEC,
			},
			annotations: map[string]string{
				"chainloop.material.tool.name":    "User Signup Service",
				"chainloop.material.tool.version": "2.6.0",
				"chainloop.material.api.version":  "1.0.0",
				"chainloop.material.api.protocol": "amqp",
			},
		},
		{
			name:     "valid AsyncAPI 3.0 YAML",
			filePath: "./testdata/asyncapi-3.0.yaml",
			schema: &contractAPI.CraftingSchema_Material{
				Name: "test",
				Type: contractAPI.CraftingSchema_Material_ASYNCAPI_SPEC,
			},
			annotations: map[string]string{
				"chainloop.material.tool.name":    "IoT Sensor Service",
				"chainloop.material.tool.version": "3.0.0",
				"chainloop.material.api.version":  "2.0.0",
				"chainloop.material.api.protocol": "mqtt",
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
			crafter, err := materials.NewAsyncAPICrafter(tc.schema, backend, &l)
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

func TestAsyncAPICraftNoStrictValidation(t *testing.T) {
	l := zerolog.Nop()
	uploader := mUploader.NewUploader(t)
	uploader.On("UploadFile", context.TODO(), "./testdata/asyncapi-invalid.json").
		Return(&casclient.UpDownStatus{
			Digest:   "deadbeef",
			Filename: "spec.json",
		}, nil)

	backend := &casclient.CASBackend{Uploader: uploader}
	schema := &contractAPI.CraftingSchema_Material{
		Name: "test",
		Type: contractAPI.CraftingSchema_Material_ASYNCAPI_SPEC,
	}

	crafter, err := materials.NewAsyncAPICrafter(schema, backend, &l, materials.WithAsyncAPINoStrictValidation(true))
	require.NoError(t, err)

	got, err := crafter.Craft(context.TODO(), "./testdata/asyncapi-invalid.json")
	require.NoError(t, err)
	assert.NotNil(t, got)
	assert.Equal(t, schema.Type.String(), got.MaterialType.String())
}
