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
	"testing"

	schemaapi "github.com/chainloop-dev/chainloop/app/controlplane/api/workflowcontract/v1"
	"github.com/chainloop-dev/chainloop/app/controlplane/integrations/sdk/v1"
	"github.com/chainloop-dev/chainloop/app/controlplane/integrations/sdk/v1/mocks"
	"github.com/stretchr/testify/assert"
)

func TestNewBaseIntegration(t *testing.T) {
	testCases := []struct {
		name        string
		id          string
		version     string
		description string
		opts        []sdk.NewOpt
		errMsg      string
		wantInput   *sdk.Inputs
	}{
		{name: "invalid - missing id", description: "desc", errMsg: "id and description are required"},
		{name: "invalid - missing description", id: "id", errMsg: "id and description are required"},
		{name: "invalid - missing version", id: "id", description: "desc", errMsg: "version is required"},
		{name: "invalid - need one input", id: "id", version: "123", description: "description", errMsg: "at least one input"},
		{name: "ok - has envelope", id: "id", version: "123", description: "description", opts: []sdk.NewOpt{sdk.WithEnvelope()}, wantInput: &sdk.Inputs{DSSEnvelope: true}},
		{name: "ok - generic material", id: "id", version: "123", description: "description",
			opts:      []sdk.NewOpt{sdk.WithInputMaterial(schemaapi.CraftingSchema_Material_MATERIAL_TYPE_UNSPECIFIED)},
			wantInput: &sdk.Inputs{Materials: []*sdk.InputMaterial{{Type: schemaapi.CraftingSchema_Material_MATERIAL_TYPE_UNSPECIFIED}}},
		},
		{
			name: "ok - specific material", id: "id", version: "123", description: "description",
			opts: []sdk.NewOpt{
				sdk.WithInputMaterial(schemaapi.CraftingSchema_Material_JUNIT_XML),
			},
			wantInput: &sdk.Inputs{Materials: []*sdk.InputMaterial{
				{
					Type: schemaapi.CraftingSchema_Material_JUNIT_XML,
				},
			}},
		},
		{
			name: "ok - both material and envelope", id: "id", version: "123", description: "description",
			opts: []sdk.NewOpt{
				sdk.WithInputMaterial(schemaapi.CraftingSchema_Material_JUNIT_XML),
				sdk.WithEnvelope(),
			},
			wantInput: &sdk.Inputs{Materials: []*sdk.InputMaterial{
				{
					Type: schemaapi.CraftingSchema_Material_JUNIT_XML,
				},
			}, DSSEnvelope: true},
		},
		{
			name: "ok - multiple materials and envelope", id: "id", version: "123", description: "description",
			opts: []sdk.NewOpt{
				sdk.WithInputMaterial(schemaapi.CraftingSchema_Material_JUNIT_XML),
				sdk.WithInputMaterial(schemaapi.CraftingSchema_Material_CONTAINER_IMAGE),
				sdk.WithEnvelope(),
			},
			wantInput: &sdk.Inputs{Materials: []*sdk.InputMaterial{
				{
					Type: schemaapi.CraftingSchema_Material_JUNIT_XML,
				},
				{
					Type: schemaapi.CraftingSchema_Material_CONTAINER_IMAGE,
				},
			}, DSSEnvelope: true},
		},
		{
			name: "ok - cant have both generic and specific", id: "id", version: "123", description: "description",
			opts: []sdk.NewOpt{
				sdk.WithInputMaterial(schemaapi.CraftingSchema_Material_MATERIAL_TYPE_UNSPECIFIED),
				sdk.WithInputMaterial(schemaapi.CraftingSchema_Material_CONTAINER_IMAGE),
				sdk.WithEnvelope(),
			},
			errMsg: "can't subscribe to specific material",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := sdk.NewBaseIntegration(tc.id, tc.version, tc.description, tc.opts...)
			if tc.errMsg != "" {
				assert.ErrorContains(t, err, tc.errMsg)
			} else {
				assert.NoError(t, err)
				d := got.Describe()
				assert.Equal(t, tc.wantInput, d.SubscribedInputs)
				assert.Equal(t, tc.id, d.ID)
				assert.Equal(t, tc.description, d.Description)
			}
		})
	}
}

func TestFindByID(t *testing.T) {
	want := mocks.NewFanOut(t)
	want.On("Describe").Return(&sdk.IntegrationInfo{ID: "id"})
	want2 := mocks.NewFanOut(t)
	want2.On("Describe").Return(&sdk.IntegrationInfo{ID: "id2"})

	var available sdk.Initialized = []sdk.FanOut{want, want2}
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
			opts: []sdk.NewOpt{sdk.WithEnvelope()},
			want: "id=id, version=123, expectsEnvelope=true, expectedMaterials=[]",
		},
		{
			name: "only material", id: "id", version: "234",
			opts: []sdk.NewOpt{sdk.WithInputMaterial(schemaapi.CraftingSchema_Material_CONTAINER_IMAGE)},
			want: "id=id, version=234, expectsEnvelope=false, expectedMaterials=[CONTAINER_IMAGE]",
		},
		{
			name: "both material and envelope", id: "id", version: "123",
			opts: []sdk.NewOpt{sdk.WithEnvelope(), sdk.WithInputMaterial(schemaapi.CraftingSchema_Material_CONTAINER_IMAGE)},
			want: "id=id, version=123, expectsEnvelope=true, expectedMaterials=[CONTAINER_IMAGE]",
		},
		{
			name: "multiple materials", id: "id", version: "123",
			opts: []sdk.NewOpt{sdk.WithInputMaterial(schemaapi.CraftingSchema_Material_CONTAINER_IMAGE), sdk.WithInputMaterial(schemaapi.CraftingSchema_Material_JUNIT_XML)},
			want: "id=id, version=123, expectsEnvelope=false, expectedMaterials=[CONTAINER_IMAGE JUNIT_XML]",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := sdk.NewBaseIntegration(tc.id, tc.version, "desc", tc.opts...)
			assert.NoError(t, err)
			assert.Equal(t, tc.want, got.String())
		})
	}
}
