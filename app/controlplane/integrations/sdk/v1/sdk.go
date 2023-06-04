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

package sdk

import (
	"context"
	"fmt"
	"io"

	schemaapi "github.com/chainloop-dev/chainloop/app/controlplane/api/workflowcontract/v1"
	"github.com/chainloop-dev/chainloop/internal/attestation/renderer/chainloop"
	"github.com/chainloop-dev/chainloop/internal/servicelogger"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/secure-systems-lab/go-securesystemslib/dsse"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"
)

// An integration can be subscribed to an envelope and/or a list of materials
// To subscribe to any material type it will use schemaapi.CraftingSchema_Material_MATERIAL_TYPE_UNSPECIFIED
type Inputs struct {
	DSSEnvelope bool
	Materials   []*InputMaterial
}

type InputMaterial struct {
	// Name of the material kind that the integration expects
	Type schemaapi.CraftingSchema_Material_MaterialType
}

type BaseIntegration struct {
	// Identifier of the integration
	id string
	// Integration version
	version string
	// Brief description of what the integration does
	description string
	// Kind of inputs does the integration expect as part of the execution
	subscribedInputs *Inputs
	log              log.Logger
	Logger           *log.Helper
}

func NewBaseIntegration(id, version, description string, opts ...NewOpt) (*BaseIntegration, error) {
	c := &BaseIntegration{
		id:               id,
		version:          version,
		description:      description,
		log:              log.NewStdLogger(io.Discard),
		subscribedInputs: &Inputs{},
	}

	for _, opt := range opts {
		opt(c)
	}

	if err := validateConstructor(c); err != nil {
		return nil, err
	}

	c.Logger = servicelogger.ScopedHelper(c.log, fmt.Sprintf("integrations/%s", id))

	return c, nil
}

func validateConstructor(c *BaseIntegration) error {
	if c.id == "" || c.description == "" {
		return fmt.Errorf("id and description are required")
	}

	if c.version == "" {
		return fmt.Errorf("version is required")
	}

	if c.subscribedInputs == nil || (!c.subscribedInputs.DSSEnvelope && (c.subscribedInputs.Materials == nil || len(c.subscribedInputs.Materials) == 0)) {
		return fmt.Errorf("the integration needs to subscribe to at least one input type. An envelope and/or a material")
	}

	// If you subscribe to a generic material type you can't subscribe to an specific one
	if c.subscribedInputs.Materials != nil && len(c.subscribedInputs.Materials) > 1 {
		for _, m := range c.subscribedInputs.Materials {
			if m.Type == schemaapi.CraftingSchema_Material_MATERIAL_TYPE_UNSPECIFIED {
				return fmt.Errorf("can't subscribe to specific material type since you are already subscribed to a generic one")
			}
		}
	}

	return nil
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
	// Integration version
	Version string
	// Brief description of what the integration does
	Description string
	// Kind of inputs does the integration expect as part of the execution
	SubscribedInputs *Inputs
}

func (i *BaseIntegration) Describe() *IntegrationInfo {
	return &IntegrationInfo{
		ID:               i.id,
		Version:          i.version,
		Description:      i.description,
		SubscribedInputs: i.subscribedInputs,
	}
}

func (i *BaseIntegration) String() string {
	inputs := i.subscribedInputs

	subscribedMaterials := make([]string, len(inputs.Materials))
	for i, m := range inputs.Materials {
		subscribedMaterials[i] = m.Type.String()
	}

	return fmt.Sprintf("id=%s, version=%s, expectsEnvelope=%t, expectedMaterials=%s", i.id, i.version, inputs.DSSEnvelope, subscribedMaterials)
}

type NewOpt func(*BaseIntegration)

// Set a logger only if provided
func WithLogger(logger log.Logger) NewOpt {
	return func(c *BaseIntegration) {
		if logger != nil {
			c.log = logger
		}
	}
}

func WithEnvelope() NewOpt {
	return func(c *BaseIntegration) {
		if c.subscribedInputs == nil {
			c.subscribedInputs = &Inputs{DSSEnvelope: true}
		} else {
			c.subscribedInputs.DSSEnvelope = true
		}
	}
}

func WithInputMaterial(materialType schemaapi.CraftingSchema_Material_MaterialType) NewOpt {
	return func(c *BaseIntegration) {
		material := &InputMaterial{Type: materialType}

		switch {
		case c.subscribedInputs == nil: // Inputs is not defined
			c.subscribedInputs = &Inputs{Materials: []*InputMaterial{material}}
		case len(c.subscribedInputs.Materials) == 0: // Materials struct is empty
			c.subscribedInputs.Materials = []*InputMaterial{material}
		default: // Materials struct contains data
			c.subscribedInputs.Materials = append(c.subscribedInputs.Materials, material)
		}
	}
}
