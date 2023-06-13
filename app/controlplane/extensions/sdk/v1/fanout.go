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
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"

	schemaapi "github.com/chainloop-dev/chainloop/app/controlplane/api/workflowcontract/v1"
	"github.com/chainloop-dev/chainloop/internal/attestation/renderer/chainloop"
	"github.com/chainloop-dev/chainloop/internal/servicelogger"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/invopop/jsonschema"
	schema_validator "github.com/santhosh-tekuri/jsonschema/v5"

	"github.com/secure-systems-lab/go-securesystemslib/dsse"
)

// FanOutIntegration represents an extension point for integrations to be able to
// fanout subscribed inputs
type FanOutIntegration struct {
	// Identifier of the integration
	id string
	// Integration version
	version string
	// Kind of inputs does the integration expect as part of the execution
	subscribedInputs *Inputs
	// Rendered schema definitions
	// Generated from the schema definitions using https://github.com/invopop/jsonschema
	registrationJSONSchema []byte
	attachmentJSONSchema   []byte
	log                    log.Logger
	Logger                 *log.Helper
}

type InputSchema struct {
	// Structs defining the registration and attachment schemas
	Registration, Attachment any
}

// Interface required to be implemented by any integration
type FanOut interface {
	// Implemented by the fanout base
	Core
	// To be implemented per integration
	FanOutExtension
}

// Implemented by the core struct
type Core interface {
	fmt.Stringer
	// Return information about the integration
	Describe() *IntegrationInfo
	ValidateRegistrationRequest(jsonPayload []byte) error
	ValidateAttachmentRequest(jsonPayload []byte) error
}

// To be implemented per integration
type FanOutExtension interface {
	// Validate, marshall and return the configuration that needs to be persisted
	Register(ctx context.Context, req *RegistrationRequest) (*RegistrationResponse, error)
	// Validate that the attachment configuration is valid in the context of the provided registration
	Attach(ctx context.Context, req *AttachmentRequest) (*AttachmentResponse, error)
	// Execute the integration
	Execute(ctx context.Context, req *ExecutionRequest) error
}

type RegistrationRequest struct {
	// Custom Payload to be used by the integration
	Payload Configuration
}

type RegistrationResponse struct {
	// Credentials to be persisted in Credentials Manager
	// JSON serializable
	Credentials *Credentials
	// Configuration to be persisted in DB
	Configuration
}

type AttachmentRequest struct {
	Payload          Configuration
	RegistrationInfo *RegistrationResponse
}

type AttachmentResponse struct {
	// JSON serializable configuration to be persisted
	Configuration
}

type ChainloopMetadata struct {
	WorkflowID string
}

// ExecutionRequest is the request to execute the integration
type ExecutionRequest struct {
	*ChainloopMetadata
	Input            *ExecuteInput
	RegistrationInfo *RegistrationResponse
	AttachmentInfo   *AttachmentResponse
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

type Credentials struct {
	URL, Username, Password string
}

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

type NewParams struct {
	ID, Version string
	Logger      log.Logger
	InputSchema *InputSchema
}

func NewFanOut(p *NewParams, opts ...NewOpt) (*FanOutIntegration, error) {
	c := &FanOutIntegration{
		id:               p.ID,
		version:          p.Version,
		log:              p.Logger,
		subscribedInputs: &Inputs{},
	}

	if c.log == nil {
		c.log = log.NewStdLogger(io.Discard)
	}

	c.Logger = servicelogger.ScopedHelper(c.log, fmt.Sprintf("extensions/%s", p.ID))

	for _, opt := range opts {
		opt(c)
	}

	if err := validateInputs(c); err != nil {
		return nil, err
	}

	if err := validateAndMarshalSchema(p, c); err != nil {
		return nil, err
	}

	return c, nil
}

func validateAndMarshalSchema(p *NewParams, c *FanOutIntegration) error {
	// Schema
	if p.InputSchema == nil {
		return fmt.Errorf("input schema is required")
	}

	// Registration schema
	if p.InputSchema.Registration == nil {
		return fmt.Errorf("registration schema is required")
	}

	// Attachment schema
	if p.InputSchema.Attachment == nil {
		return fmt.Errorf("attachment schema is required")
	}

	// Try to generate JSON schemas
	var err error
	if c.registrationJSONSchema, err = generateJSONSchema(p.InputSchema.Registration); err != nil {
		return fmt.Errorf("failed to generate registration schema: %w", err)
	}

	if c.attachmentJSONSchema, err = generateJSONSchema(p.InputSchema.Attachment); err != nil {
		return fmt.Errorf("failed to generate attachment schema: %w", err)
	}

	return nil
}

func validateInputs(c *FanOutIntegration) error {
	if c.id == "" {
		return fmt.Errorf("id is required")
	}

	if c.version == "" {
		return fmt.Errorf("version is required")
	}

	// Subscribed inputs
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

// List of loaded integrations
type AvailableExtensions []FanOut
type FanOutFactory = func(l log.Logger) (FanOut, error)

// FindByID returns the integration with the given ID from the list of available integrations
// If not found, an error is returned
func (i AvailableExtensions) FindByID(id string) (FanOut, error) {
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
	// Kind of inputs does the integration expect as part of the execution
	SubscribedInputs *Inputs
	// Schemas in JSON schema format
	RegistrationJSONSchema, AttachmentJSONSchema []byte
}

func (i *FanOutIntegration) Describe() *IntegrationInfo {
	return &IntegrationInfo{
		ID:                     i.id,
		Version:                i.version,
		SubscribedInputs:       i.subscribedInputs,
		RegistrationJSONSchema: i.registrationJSONSchema,
		AttachmentJSONSchema:   i.attachmentJSONSchema,
	}
}

// Validate the registration payload against the registration JSON schema
func (i *FanOutIntegration) ValidateRegistrationRequest(jsonPayload []byte) error {
	return validatePayloadAgainstJSONSchema(jsonPayload, i.registrationJSONSchema)
}

// Validate the attachment payload against the attachment JSON schema
func (i *FanOutIntegration) ValidateAttachmentRequest(jsonPayload []byte) error {
	return validatePayloadAgainstJSONSchema(jsonPayload, i.attachmentJSONSchema)
}

func validatePayloadAgainstJSONSchema(jsonPayload []byte, jsonSchema []byte) error {
	compiler := schema_validator.NewCompiler()
	// Enable format validation
	compiler.AssertFormat = true

	if err := compiler.AddResource("schema.json", bytes.NewReader(jsonSchema)); err != nil {
		return fmt.Errorf("failed to compile schema: %w", err)
	}

	schema, err := compiler.Compile("schema.json")
	if err != nil {
		return fmt.Errorf("failed to compile schema: %w", err)
	}

	var v any
	if err := json.Unmarshal(jsonPayload, &v); err != nil {
		return fmt.Errorf("failed to unmarshal payload: %w", err)
	}

	if err = schema.Validate(v); err != nil {
		var validationError *schema_validator.ValidationError

		// Return only the last error to avoid giving the user context about the schema used.
		// The last error usually shows the information about the actual property not matching the schema
		// for example "missing property apiKey"
		if ok := errors.As(err, &validationError); ok {
			validationErrors := validationError.BasicOutput().Errors
			return errors.New(validationErrors[len(validationErrors)-1].Error)
		}

		return err
	}

	return nil
}

func (i *FanOutIntegration) String() string {
	inputs := i.subscribedInputs

	subscribedMaterials := make([]string, len(inputs.Materials))
	for i, m := range inputs.Materials {
		subscribedMaterials[i] = m.Type.String()
	}

	return fmt.Sprintf("id=%s, version=%s, expectsEnvelope=%t, expectedMaterials=%s", i.id, i.version, inputs.DSSEnvelope, subscribedMaterials)
}

type NewOpt func(*FanOutIntegration)

func WithEnvelope() NewOpt {
	return func(c *FanOutIntegration) {
		if c.subscribedInputs == nil {
			c.subscribedInputs = &Inputs{DSSEnvelope: true}
		} else {
			c.subscribedInputs.DSSEnvelope = true
		}
	}
}

func WithInputMaterial(materialType schemaapi.CraftingSchema_Material_MaterialType) NewOpt {
	return func(c *FanOutIntegration) {
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

// Configuration represents any raw configuration to be stored in the DB
// This wrapper is just a way to clearly indicate that the content needs to be JSON serializable
type Configuration []byte

func ToConfig(m any) (Configuration, error) {
	return json.Marshal(m)
}

func FromConfig(data Configuration, v any) error {
	return json.Unmarshal(data, v)
}

// generate a JSON schema from a struct, see
func generateJSONSchema(schema any) ([]byte, error) {
	s := jsonschema.Reflect(schema)
	return json.Marshal(s)
}
