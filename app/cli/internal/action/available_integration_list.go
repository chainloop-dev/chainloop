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

package action

import (
	"context"
	"errors"
	"fmt"

	pb "github.com/chainloop-dev/chainloop/app/controlplane/api/controlplane/v1"
	"github.com/chainloop-dev/chainloop/app/controlplane/plugins/sdk/v1"
	"github.com/santhosh-tekuri/jsonschema/v5"
)

type AvailableIntegrationList struct {
	cfg *ActionsOpts
}

type AvailableIntegrationItem struct {
	ID           string      `json:"id"`
	Version      string      `json:"version"`
	Description  string      `json:"description,omitempty"`
	Registration *JSONSchema `json:"registration"`
	Attachment   *JSONSchema `json:"attachment"`
	// Subscribed inputs (material types)
	SubscribedInputs []string `json:"subscribedInputs"`
}

type JSONSchema struct {
	// Show it as raw string so the json output contains it
	Raw string `json:"schema"`
	// Parsed schema so it can be used for validation or other purposes
	// It's not shown in the json output
	Parsed     *jsonschema.Schema      `json:"-"`
	Properties sdk.SchemaPropertiesMap `json:"-"`
}

func NewAvailableIntegrationList(cfg *ActionsOpts) *AvailableIntegrationList {
	return &AvailableIntegrationList{cfg}
}

func (action *AvailableIntegrationList) Run() ([]*AvailableIntegrationItem, error) {
	client := pb.NewIntegrationsServiceClient(action.cfg.CPConnection)
	resp, err := client.ListAvailable(context.Background(), &pb.IntegrationsServiceListAvailableRequest{})
	if err != nil {
		return nil, err
	}

	result := make([]*AvailableIntegrationItem, 0, len(resp.Result))
	for _, p := range resp.Result {
		i, err := pbAvailableIntegrationItemToAction(p)
		if err != nil {
			return nil, err
		}

		result = append(result, i)
	}

	return result, nil
}

func pbAvailableIntegrationItemToAction(in *pb.IntegrationAvailableItem) (*AvailableIntegrationItem, error) {
	if in == nil {
		return nil, errors.New("nil input")
	}

	if in.GetFanout() == nil {
		fmt.Printf("skipping integration %s, type not supported\n", in.GetId())
		return nil, nil
	}

	foType := in.GetFanout()

	i := &AvailableIntegrationItem{
		ID: in.GetId(), Version: in.GetVersion(), Description: in.GetDescription(),
		Registration: &JSONSchema{Raw: string(foType.GetRegistrationSchema())},
		Attachment:   &JSONSchema{Raw: string(foType.GetAttachmentSchema())},
	}

	// Parse the schemas so they can be used for validation or other purposes
	var err error
	i.Registration.Parsed, err = sdk.CompileJSONSchema(foType.GetRegistrationSchema())
	if err != nil {
		return nil, fmt.Errorf("failed to compile registration schema: %w", err)
	}

	i.Attachment.Parsed, err = sdk.CompileJSONSchema(foType.GetAttachmentSchema())
	if err != nil {
		return nil, fmt.Errorf("failed to compile registration schema: %w", err)
	}

	// Calculate the properties map
	i.Registration.Properties = make(sdk.SchemaPropertiesMap)
	if err := sdk.CalculatePropertiesMap(i.Registration.Parsed, &i.Registration.Properties); err != nil {
		return nil, fmt.Errorf("failed to calculate registration properties: %w", err)
	}

	i.Attachment.Properties = make(sdk.SchemaPropertiesMap)
	if err := sdk.CalculatePropertiesMap(i.Attachment.Parsed, &i.Attachment.Properties); err != nil {
		return nil, fmt.Errorf("failed to calculate attachment properties: %w", err)
	}

	// Subscribed inputs
	i.SubscribedInputs = append(i.SubscribedInputs, foType.GetSubscribedMaterials()...)

	return i, nil
}
