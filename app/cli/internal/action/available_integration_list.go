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

	pb "github.com/chainloop-dev/chainloop/app/controlplane/api/controlplane/v1"
	"github.com/santhosh-tekuri/jsonschema/v5"
)

type AvailableIntegrationList struct {
	cfg *ActionsOpts
}

type AvailableIntegrationItem struct {
	ID           string      `json:"id"`
	Version      string      `json:"version"`
	Registration *JSONSchema `json:"registration"`
	Attachment   *JSONSchema `json:"attachment"`
}

type JSONSchema struct {
	// Show it as raw string so the json output contains it
	Raw string `json:"schema"`
	// Parsed schema so it can be used for validation or other purposes
	// It's not shown in the json output
	Parsed *jsonschema.Schema `json:"-"`
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

func pbAvailableIntegrationItemToAction(in *pb.IntegrationsServiceListAvailableResponse_Integration) (*AvailableIntegrationItem, error) {
	if in == nil {
		return nil, errors.New("nil input")
	}

	i := &AvailableIntegrationItem{
		ID: in.GetId(), Version: in.GetVersion(),
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
