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

package sdk_test

import (
	"encoding/json"
	"fmt"
	"os"
	"testing"

	schemaapi "github.com/chainloop-dev/chainloop/app/controlplane/api/workflowcontract/v1"
	"github.com/chainloop-dev/chainloop/app/controlplane/plugins/sdk/v1"
	"github.com/chainloop-dev/chainloop/app/controlplane/plugins/sdk/v1/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type schema struct {
	TestProperty string `json:"testProperty"`
}

var inputSchema = &sdk.InputSchema{
	Registration: schema{},
	Attachment:   schema{},
}

func TestNewBaseIntegration(t *testing.T) {
	testCases := []struct {
		name          string
		id            string
		version       string
		description   string
		opts          []sdk.NewOpt
		errMsg        string
		wantMaterials []*sdk.InputMaterial
		schema        *sdk.InputSchema
	}{
		{name: "invalid - missing id", description: "desc", errMsg: "id is required"},
		{name: "invalid - missing version", id: "id", description: "desc", errMsg: "version is required"},
		{name: "invalid - missing schema", id: "id", version: "123", description: "description", errMsg: "input schema is required"},
		{name: "ok - subscribed to no materials", id: "id", version: "123", description: "description", wantMaterials: []*sdk.InputMaterial{}, schema: inputSchema},
		{
			name: "ok - specific material", id: "id", version: "123", description: "description",
			opts: []sdk.NewOpt{
				sdk.WithInputMaterial(schemaapi.CraftingSchema_Material_JUNIT_XML),
			},
			wantMaterials: []*sdk.InputMaterial{
				{
					Type: schemaapi.CraftingSchema_Material_JUNIT_XML,
				},
			},
			schema: inputSchema,
		},
		{
			name: "ok - multiple materials", id: "id", version: "123", description: "description",
			opts: []sdk.NewOpt{
				sdk.WithInputMaterial(schemaapi.CraftingSchema_Material_JUNIT_XML),
				sdk.WithInputMaterial(schemaapi.CraftingSchema_Material_CONTAINER_IMAGE),
			},
			wantMaterials: []*sdk.InputMaterial{
				{
					Type: schemaapi.CraftingSchema_Material_JUNIT_XML,
				},
				{
					Type: schemaapi.CraftingSchema_Material_CONTAINER_IMAGE,
				},
			},
			schema: inputSchema,
		},
		{
			name: "ok - cant have both generic/all materials", id: "id", version: "123", description: "description",
			opts: []sdk.NewOpt{
				sdk.WithInputMaterial(schemaapi.CraftingSchema_Material_MATERIAL_TYPE_UNSPECIFIED),
			},
			errMsg: "is not a valid material type",
			schema: inputSchema,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := sdk.NewFanOut(
				&sdk.NewParams{
					ID:          tc.id,
					Version:     tc.version,
					InputSchema: tc.schema,
				}, tc.opts...)
			if tc.errMsg != "" {
				assert.ErrorContains(t, err, tc.errMsg)
			} else {
				require.NoError(t, err)
				d := got.Describe()
				assert.Equal(t, tc.wantMaterials, d.SubscribedMaterials)
				assert.Equal(t, tc.id, d.ID)
			}
		})
	}
}

func TestFindByID(t *testing.T) {
	want := mocks.NewFanOut(t)
	want.On("Describe").Return(&sdk.IntegrationInfo{ID: "id"})
	want2 := mocks.NewFanOut(t)
	want2.On("Describe").Return(&sdk.IntegrationInfo{ID: "id2"})

	var available sdk.AvailablePlugins = []*sdk.FanOutP{{FanOut: want}, {FanOut: want2}}

	got, err := available.FindByID("id")
	assert.NoError(t, err)
	assert.Equal(t, want.Describe().ID, got.Describe().ID)

	got, err = available.FindByID("id2")
	assert.NoError(t, err)
	assert.Equal(t, want2.Describe().ID, got.Describe().ID)

	// Not found
	got, err = available.FindByID("not-found")
	assert.Error(t, err)
	assert.Nil(t, got)
}

func TestString(t *testing.T) {
	testCases := []struct {
		name    string
		id      string
		version string
		opts    []sdk.NewOpt
		want    string
	}{
		{
			name: "with envelope", id: "id", version: "123",
			want: "id=id, version=123, expectedMaterials=[]",
		},
		{
			name: "only material", id: "id", version: "234",
			opts: []sdk.NewOpt{sdk.WithInputMaterial(schemaapi.CraftingSchema_Material_CONTAINER_IMAGE)},
			want: "id=id, version=234, expectedMaterials=[CONTAINER_IMAGE]",
		},
		{
			name: "both material and envelope", id: "id", version: "123",
			opts: []sdk.NewOpt{sdk.WithInputMaterial(schemaapi.CraftingSchema_Material_CONTAINER_IMAGE)},
			want: "id=id, version=123, expectedMaterials=[CONTAINER_IMAGE]",
		},
		{
			name: "multiple materials", id: "id", version: "123",
			opts: []sdk.NewOpt{sdk.WithInputMaterial(schemaapi.CraftingSchema_Material_CONTAINER_IMAGE), sdk.WithInputMaterial(schemaapi.CraftingSchema_Material_JUNIT_XML)},
			want: "id=id, version=123, expectedMaterials=[CONTAINER_IMAGE JUNIT_XML]",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := sdk.NewFanOut(&sdk.NewParams{ID: tc.id, Version: tc.version, InputSchema: inputSchema}, tc.opts...)
			require.NoError(t, err)
			assert.Equal(t, tc.want, got.String())
		})
	}
}

type registrationSchema struct {
	Username string `json:"username"`
	Email    string `json:"email" jsonschema:"format=email"`
	Optional int    `json:"optional,omitempty"`
}

func TestValidateRegistrationRequest(t *testing.T) {
	testCases := []struct {
		name    string
		input   map[string]interface{}
		wantErr string
	}{
		{
			name: "ok all properties",
			input: map[string]interface{}{
				"username": "user",
				"email":    "foo@gmail.com",
				"optional": 1,
			},
		},
		{
			name: "ok all required properties",
			input: map[string]interface{}{
				"username": "user",
				"email":    "foo@gmail.com",
			},
		},
		{
			name: "invalid type",
			input: map[string]interface{}{
				"username": "user",
				"email":    "foo@gmail.com",
				"optional": "1",
			},
			wantErr: "expected integer, but got string",
		},
		{
			name: "invalid email",
			input: map[string]interface{}{
				"username": "user",
				"email":    "foo",
			},
			wantErr: "is not valid 'email'",
		},
		{
			name: "missing username",
			input: map[string]interface{}{
				"email": "foo@gmail.com",
			},
			wantErr: "missing properties: 'username'",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := sdk.NewFanOut(
				&sdk.NewParams{
					ID: "ID", Version: "123",
					InputSchema: &sdk.InputSchema{Registration: &registrationSchema{}, Attachment: &attachmentSchema{}},
				})

			require.NoError(t, err)
			payload, err := json.Marshal(tc.input)
			require.NoError(t, err)

			err = got.ValidateRegistrationRequest(payload)
			if tc.wantErr != "" {
				assert.ErrorContains(t, err, tc.wantErr)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestIsSubscribedTo(t *testing.T) {
	testCases := []struct {
		name                string
		subscribedMaterials []schemaapi.CraftingSchema_Material_MaterialType
		input               string
		want                bool
	}{
		{
			name:                "empty",
			subscribedMaterials: []schemaapi.CraftingSchema_Material_MaterialType{},
			input:               "foo",
			want:                false,
		},
		{
			name:                "not subscribed",
			subscribedMaterials: []schemaapi.CraftingSchema_Material_MaterialType{schemaapi.CraftingSchema_Material_CONTAINER_IMAGE},
			input:               "foo",
			want:                false,
		},
		{
			name:                "subscribed",
			subscribedMaterials: []schemaapi.CraftingSchema_Material_MaterialType{schemaapi.CraftingSchema_Material_CONTAINER_IMAGE},
			input:               "CONTAINER_IMAGE",
			want:                true,
		},
		{
			name:                "subscribed multiple",
			subscribedMaterials: []schemaapi.CraftingSchema_Material_MaterialType{schemaapi.CraftingSchema_Material_CONTAINER_IMAGE, schemaapi.CraftingSchema_Material_JUNIT_XML},
			input:               "CONTAINER_IMAGE",
			want:                true,
		},
		{
			name:                "subscribed multiple 2",
			subscribedMaterials: []schemaapi.CraftingSchema_Material_MaterialType{schemaapi.CraftingSchema_Material_CONTAINER_IMAGE, schemaapi.CraftingSchema_Material_JUNIT_XML},
			input:               "JUNIT_XML",
			want:                true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			opts := []sdk.NewOpt{}
			for _, m := range tc.subscribedMaterials {
				opts = append(opts, sdk.WithInputMaterial(m))
			}

			got, err := sdk.NewFanOut(
				&sdk.NewParams{
					ID: "ID", Version: "123",
					InputSchema: &sdk.InputSchema{Registration: &registrationSchema{}, Attachment: &attachmentSchema{}},
				}, opts...)

			require.NoError(t, err)
			assert.Equal(t, tc.want, got.IsSubscribedTo(tc.input))
		})
	}
}

type attachmentSchema struct {
	ProjectID   int    `json:"projectID,omitempty" jsonschema:"oneof_required=projectID,minLength=1"`
	ProjectName string `json:"projectName,omitempty" jsonschema:"oneof_required=projectName,minLength=1"`
}

func TestValidateAttachmentRequest(t *testing.T) {
	testCases := []struct {
		name    string
		input   map[string]interface{}
		wantErr string
	}{
		{
			name: "ok projectID set",
			input: map[string]interface{}{
				"projectID": 123,
			},
		},
		{
			name: "invalid projectID",
			input: map[string]interface{}{
				"projectID": []int{123},
			},
			wantErr: "expected integer, but got array",
		},
		{
			name: "ok projectName set",
			input: map[string]interface{}{
				"projectName": "my-project",
			},
		},
		{
			name:    "ko no properties set",
			input:   map[string]interface{}{},
			wantErr: "missing properties",
		},
		{
			name: "ko both properties set",
			input: map[string]interface{}{
				"projectID":   123,
				"projectName": "my-project",
			},
			wantErr: "valid against schemas at indexes 0 and 1",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := sdk.NewFanOut(
				&sdk.NewParams{
					ID: "ID", Version: "123",
					InputSchema: &sdk.InputSchema{Registration: &registrationSchema{}, Attachment: &attachmentSchema{}},
				})

			require.NoError(t, err)
			payload, err := json.Marshal(tc.input)
			require.NoError(t, err)

			err = got.ValidateAttachmentRequest(payload)
			if tc.wantErr != "" {
				assert.ErrorContains(t, err, tc.wantErr)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestCalculatePropertiesMap(t *testing.T) {
	testCases := []struct {
		schemaPath string
		want       sdk.SchemaPropertiesMap
	}{
		{
			"basic.json",
			sdk.SchemaPropertiesMap{
				"allowAutoCreate": {
					Name:        "allowAutoCreate",
					Description: "Support of creating projects on demand",
					Type:        "boolean",
					Required:    false,
				},
				"apiKey": {
					Name:        "apiKey",
					Description: "The API key to use for authentication",
					Type:        "string",
					Required:    true,
				},
				"instanceURI": {
					Name:        "instanceURI",
					Description: "The URL of the Dependency-Track instance",
					Type:        "string",
					Required:    true,
					Format:      "uri",
				},
				"port": {
					Name: "port",
					Type: "number",
				},
			},
		},
		{
			// NOTE: oneof work in the validation but are not shown in the map
			// This testCase is here to document this limitation
			"oneof_required.json",
			sdk.SchemaPropertiesMap{
				"projectID": {
					Name:        "projectID",
					Description: "The ID of the existing project to send the SBOMs to",
					Type:        "string",
				},
				"projectName": {
					Name:        "projectName",
					Description: "The name of the project to create and send the SBOMs to",
					Type:        "string",
				},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.schemaPath, func(t *testing.T) {
			schemaRaw, err := os.ReadFile(fmt.Sprintf("testdata/schemas/%s", tc.schemaPath))
			require.NoError(t, err)
			schema, err := sdk.CompileJSONSchema(schemaRaw)
			require.NoError(t, err)

			var got = make(sdk.SchemaPropertiesMap)
			err = sdk.CalculatePropertiesMap(schema, &got)
			assert.NoError(t, err)

			assert.Equal(t, tc.want, got)
		})
	}
}
