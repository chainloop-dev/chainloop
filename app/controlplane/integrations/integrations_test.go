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

package integrations_test

import (
	"testing"

	schemaapi "github.com/chainloop-dev/chainloop/app/controlplane/api/workflowcontract/v1"
	"github.com/chainloop-dev/chainloop/app/controlplane/integrations"
	"github.com/chainloop-dev/chainloop/app/controlplane/integrations/mocks"
	"github.com/stretchr/testify/assert"
)

func TestNewBaseIntegration(t *testing.T) {
	testCases := []struct {
		name        string
		id          string
		description string
		opts        []integrations.NewOpt
		errMsg      string
		wantInput   *integrations.Inputs
	}{
		{name: "invalid - missing id", description: "desc", errMsg: "id and description are required"},
		{name: "invalid - missing description", id: "id", errMsg: "id and description are required"},
		{name: "invalid - need one input", id: "id", description: "description", errMsg: "at least one input"},
		{name: "ok - has envelope", id: "id", description: "description", opts: []integrations.NewOpt{integrations.WithEnvelope()}, wantInput: &integrations.Inputs{DSSEnvelope: true}},
		{name: "ok - generic material", id: "id", description: "description",
			opts:      []integrations.NewOpt{integrations.WithInputMaterial(schemaapi.CraftingSchema_Material_MATERIAL_TYPE_UNSPECIFIED)},
			wantInput: &integrations.Inputs{Materials: []*integrations.InputMaterial{{Type: schemaapi.CraftingSchema_Material_MATERIAL_TYPE_UNSPECIFIED}}},
		},
		{
			name: "ok - specific material", id: "id", description: "description",
			opts: []integrations.NewOpt{
				integrations.WithInputMaterial(schemaapi.CraftingSchema_Material_JUNIT_XML),
			},
			wantInput: &integrations.Inputs{Materials: []*integrations.InputMaterial{
				{
					Type: schemaapi.CraftingSchema_Material_JUNIT_XML,
				},
			}},
		},
		{
			name: "ok - both material and envelope", id: "id", description: "description",
			opts: []integrations.NewOpt{
				integrations.WithInputMaterial(schemaapi.CraftingSchema_Material_JUNIT_XML),
				integrations.WithEnvelope(),
			},
			wantInput: &integrations.Inputs{Materials: []*integrations.InputMaterial{
				{
					Type: schemaapi.CraftingSchema_Material_JUNIT_XML,
				},
			}, DSSEnvelope: true},
		},
		{
			name: "ok - multiple materials and envelope", id: "id", description: "description",
			opts: []integrations.NewOpt{
				integrations.WithInputMaterial(schemaapi.CraftingSchema_Material_JUNIT_XML),
				integrations.WithInputMaterial(schemaapi.CraftingSchema_Material_CONTAINER_IMAGE),
				integrations.WithEnvelope(),
			},
			wantInput: &integrations.Inputs{Materials: []*integrations.InputMaterial{
				{
					Type: schemaapi.CraftingSchema_Material_JUNIT_XML,
				},
				{
					Type: schemaapi.CraftingSchema_Material_CONTAINER_IMAGE,
				},
			}, DSSEnvelope: true},
		},
		{
			name: "ok - cant have both generic and specific", id: "id", description: "description",
			opts: []integrations.NewOpt{
				integrations.WithInputMaterial(schemaapi.CraftingSchema_Material_MATERIAL_TYPE_UNSPECIFIED),
				integrations.WithInputMaterial(schemaapi.CraftingSchema_Material_CONTAINER_IMAGE),
				integrations.WithEnvelope(),
			},
			errMsg: "can't subscribe to specific material",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := integrations.NewBaseIntegration(tc.id, tc.description, tc.opts...)
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
	want.On("Describe").Return(&integrations.IntegrationInfo{ID: "id"})
	want2 := mocks.NewFanOut(t)
	want2.On("Describe").Return(&integrations.IntegrationInfo{ID: "id2"})

	var available integrations.Initialized = []integrations.FanOut{want, want2}
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
		name string
		id   string
		opts []integrations.NewOpt
		want string
	}{
		{
			name: "with envelope", id: "id",
			opts: []integrations.NewOpt{integrations.WithEnvelope()},
			want: "id=id, expectsEnvelope=true, expectedMaterials=[]",
		},
		{
			name: "only material", id: "id",
			opts: []integrations.NewOpt{integrations.WithInputMaterial(schemaapi.CraftingSchema_Material_CONTAINER_IMAGE)},
			want: "id=id, expectsEnvelope=false, expectedMaterials=[CONTAINER_IMAGE]",
		},
		{
			name: "both material and envelope", id: "id",
			opts: []integrations.NewOpt{integrations.WithEnvelope(), integrations.WithInputMaterial(schemaapi.CraftingSchema_Material_CONTAINER_IMAGE)},
			want: "id=id, expectsEnvelope=true, expectedMaterials=[CONTAINER_IMAGE]",
		},
		{
			name: "multiple materials", id: "id",
			opts: []integrations.NewOpt{integrations.WithInputMaterial(schemaapi.CraftingSchema_Material_CONTAINER_IMAGE), integrations.WithInputMaterial(schemaapi.CraftingSchema_Material_JUNIT_XML)},
			want: "id=id, expectsEnvelope=false, expectedMaterials=[CONTAINER_IMAGE JUNIT_XML]",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := integrations.NewBaseIntegration(tc.id, "desc", tc.opts...)
			assert.NoError(t, err)
			assert.Equal(t, tc.want, got.String())
		})
	}
}
