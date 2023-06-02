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

package integrations

import (
	"context"
	"fmt"

	schemaapi "github.com/chainloop-dev/chainloop/app/controlplane/api/workflowcontract/v1"
	"github.com/chainloop-dev/chainloop/internal/attestation/renderer/chainloop"
	"github.com/secure-systems-lab/go-securesystemslib/dsse"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"
)

type Inputs struct {
	DSSEnvelope   bool
	InputMaterial *InputMaterial
}

type InputMaterial struct {
	// Name of the material kind that the integration expects
	Type schemaapi.CraftingSchema_Material_MaterialType
}

// BaseIntegration integration struct
type BaseIntegration struct {
	// Identifier of the integration
	id string
	// Brief description of what the integration does
	description string
	// Kind of inputs does the integration expect as part of the execution
	subscribedInputs *Inputs
}

type NewOpt func(*BaseIntegration)

func WithEnvelope() NewOpt {
	return func(c *BaseIntegration) {
		c.subscribedInputs.DSSEnvelope = true
	}
}

func WithInputMaterial(materialType schemaapi.CraftingSchema_Material_MaterialType) NewOpt {
	return func(c *BaseIntegration) {
		c.subscribedInputs.InputMaterial = &InputMaterial{
			Type: materialType,
		}
	}
}

func NewBaseIntegration(id, description string, opts ...NewOpt) (*BaseIntegration, error) {
	if id == "" || description == "" {
		return nil, fmt.Errorf("id and description are required")
	}

	c := &BaseIntegration{
		id:          id,
		description: description,
	}

	for _, opt := range opts {
		opt(c)
	}

	if c.subscribedInputs == nil || (!c.subscribedInputs.DSSEnvelope && c.subscribedInputs.InputMaterial == nil) {
		return nil, fmt.Errorf("the integration needs to subscribe to at least one input type. An envelope and/or a material")
	}

	return c, nil
}

type FanOut interface {
	// Implemented by the core struct
	CoreI
	// To be implemented per integration
	Custom
}

// Implemented by the core struct
type CoreI interface {
	// Return information about the integration
	Describe() *IntegrationInfo
	fmt.Stringer
}

// To be implemented per integration
type Custom interface {
	// Validate, marshall and return the configuration that needs to be persisted
	PreRegister(ctx context.Context, req *anypb.Any) (*PreRegistration, error)
	// Validate that the attachment configuration is valid in the context of the provided registration
	PreAttach(ctx context.Context, c *BundledConfig) (*PreAttachment, error)
	// Execute the integration
	Execute(ctx context.Context, opts *ExecuteReq) error
}

type PreRegistration struct {
	// Credentials to be persisted in Credentials Manager
	// JSON serializable
	Credentials *Credentials
	// Configuration to be persisted in DB
	Configuration proto.Message
	// registration kind
	Kind string
}

type PreAttachment struct {
	// Configuration to be persisted
	Configuration proto.Message
}

// ExecuteReq is the request to execute the integration
type ExecuteReq struct {
	Config *BundledConfig
	Input  *ExecuteInput
}

// An execute method will receive either the envelope or a material as input
// The material will contain its content as well as the metadata
type ExecuteInput struct {
	DSSEnvelope *dsse.Envelope
	Material    *ExecuteMaterial
}

type ExecuteMaterial struct {
	*chainloop.NormalizedMaterial
	// Content of the material already downloaded
	Content []byte
}

// BundledConfig is the collection of the registration and attachment configuration
type BundledConfig struct {
	// Registration configuration
	Registration *anypb.Any
	// Attachment configuration
	Attachment *anypb.Any
	// Stored credentials
	Credentials *Credentials
	// Chainloop Metadata
	WorkflowID string
}

type Credentials struct {
	URL, Username, Password string
}

// List of initialized integrations
type Initialized []FanOut

// FindByID returns the integration with the given ID from the list of available integrations
// If not found, an error is returned
func (i Initialized) FindByID(id string) (FanOut, error) {
	for _, integration := range i {
		if integration.Describe().ID == id {
			return integration, nil
		}
	}

	return nil, fmt.Errorf("integration %q not found", id)
}

type IntegrationInfo struct {
	// Identifier of the integration
	ID string
	// Brief description of what the integration does
	Description string
	// Kind of inputs does the integration expect as part of the execution
	SubscribedInputs *Inputs
}

func (i *BaseIntegration) Describe() *IntegrationInfo {
	return &IntegrationInfo{
		ID:               i.id,
		Description:      i.description,
		SubscribedInputs: i.subscribedInputs,
	}
}

func (i *BaseIntegration) String() string {
	inputs := i.subscribedInputs

	materialType := "none"
	if inputs.InputMaterial != nil {
		materialType = inputs.InputMaterial.Type.String()
	}

	return fmt.Sprintf("id=%s, expectsEnvelope=%t, expectedMaterial=%s", i.id, inputs.DSSEnvelope, materialType)
}
