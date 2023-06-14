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
	"bytes"
	"context"
	"errors"
	"fmt"
	"sort"

	pb "github.com/chainloop-dev/chainloop/app/controlplane/api/controlplane/v1"
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
}

type JSONSchema struct {
	// Show it as raw string so the json output contains it
	Raw string `json:"schema"`
	// Parsed schema so it can be used for validation or other purposes
	// It's not shown in the json output
	Parsed     *jsonschema.Schema  `json:"-"`
	Properties SchemaPropertiesMap `json:"-"`
}

type SchemaPropertiesMap map[string]*SchemaProperty
type SchemaProperty struct {
	// Name of the property
	Name string
	// optional description
	Description string
	// Type of the property (string, boolean, number)
	Type string
	// If the property is required
	Required bool
	// Optional format (email, host)
	Format string
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

	i := &AvailableIntegrationItem{
		ID: in.GetId(), Version: in.GetVersion(), Description: in.GetDescription(),
		Registration: &JSONSchema{Raw: string(in.GetRegistrationSchema())},
		Attachment:   &JSONSchema{Raw: string(in.GetAttachmentSchema())},
	}

	// Parse the schemas so they can be used for validation or other purposes
	var err error
	i.Registration.Parsed, err = compileJSONSchema(in.GetRegistrationSchema())
	if err != nil {
		return nil, fmt.Errorf("failed to compile registration schema: %w", err)
	}

	i.Attachment.Parsed, err = compileJSONSchema(in.GetAttachmentSchema())
	if err != nil {
		return nil, fmt.Errorf("failed to compile registration schema: %w", err)
	}

	// Calculate the properties map
	i.Registration.Properties = make(SchemaPropertiesMap)
	if err := calculatePropertiesMap(i.Registration.Parsed, &i.Registration.Properties); err != nil {
		return nil, fmt.Errorf("failed to calculate registration properties: %w", err)
	}

	i.Attachment.Properties = make(SchemaPropertiesMap)
	if err := calculatePropertiesMap(i.Attachment.Parsed, &i.Attachment.Properties); err != nil {
		return nil, fmt.Errorf("failed to calculate attachment properties: %w", err)
	}

	return i, nil
}

func compileJSONSchema(in []byte) (*jsonschema.Schema, error) {
	// Parse the schemas
	compiler := jsonschema.NewCompiler()
	// Enable format validation
	compiler.AssertFormat = true
	// Show description
	compiler.ExtractAnnotations = true

	if err := compiler.AddResource("schema.json", bytes.NewReader(in)); err != nil {
		return nil, fmt.Errorf("failed to compile schema: %w", err)
	}

	return compiler.Compile("schema.json")
}

// calculate a map with all the properties of a schema
func calculatePropertiesMap(s *jsonschema.Schema, m *SchemaPropertiesMap) error {
	if m == nil {
		return nil
	}

	// Schema with reference
	if s.Ref != nil {
		return calculatePropertiesMap(s.Ref, m)
	}

	// Appended schemas
	if s.AllOf != nil {
		for _, s := range s.AllOf {
			if err := calculatePropertiesMap(s, m); err != nil {
				return err
			}
		}
	}

	if s.Properties != nil {
		requiredMap := make(map[string]bool)
		for _, r := range s.Required {
			requiredMap[r] = true
		}

		for k, v := range s.Properties {
			if err := calculatePropertiesMap(v, m); err != nil {
				return err
			}

			var required = requiredMap[k]
			(*m)[k] = &SchemaProperty{
				Name:        k,
				Type:        v.Types[0],
				Required:    required,
				Description: v.Description,
				Format:      v.Format,
			}
		}
	}

	// We return the map sorted
	// This is not strictly necessary but it makes the output more readable
	// and it's easier to test

	// Sort the keys
	keys := make([]string, 0, len(*m))
	for k := range *m {
		keys = append(keys, k)
	}

	sort.Strings(keys)

	// Create a new map with the sorted keys
	newMap := make(SchemaPropertiesMap)
	for _, k := range keys {
		newMap[k] = (*m)[k]
	}

	*m = newMap

	return nil
}
