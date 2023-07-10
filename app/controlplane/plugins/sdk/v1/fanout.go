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
	"sort"

	crv1 "github.com/google/go-containerregistry/pkg/v1"

	schemaapi "github.com/chainloop-dev/chainloop/app/controlplane/api/workflowcontract/v1"
	"github.com/chainloop-dev/chainloop/internal/attestation/renderer/chainloop"
	"github.com/chainloop-dev/chainloop/internal/servicelogger"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/in-toto/in-toto-golang/in_toto"
	"github.com/invopop/jsonschema"
	schema_validator "github.com/santhosh-tekuri/jsonschema/v5"

	"github.com/secure-systems-lab/go-securesystemslib/dsse"
)

// FanOutIntegration represents an plugin point for integrations to be able to
// fanOut subscribed inputs
type FanOutIntegration struct {
	// Identifier of the integration
	id string
	// Integration version
	version string
	// Optional description
	description string
	// Material types an integration expect as part of the execution
	subscribedMaterials []*InputMaterial
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
	FanOutPlugin
}

// Implemented by the core struct
type Core interface {
	fmt.Stringer
	// Return information about the integration
	Describe() *IntegrationInfo
	ValidateRegistrationRequest(jsonPayload []byte) error
	ValidateAttachmentRequest(jsonPayload []byte) error
	// Return if the integration is subscribed to the material type
	IsSubscribedTo(materialType string) bool
}

// To be implemented per integration
type FanOutPlugin interface {
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
	WorkflowID      string
	WorkflowName    string
	WorkflowProject string

	WorkflowRunID string
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
	Attestation *ExecuteAttestation
	Materials   []*ExecuteMaterial
}

type ExecuteAttestation struct {
	Envelope *dsse.Envelope
	// Hash of the envelope
	Hash      crv1.Hash
	Statement *in_toto.Statement
	Predicate chainloop.NormalizablePredicate
}

type ExecuteMaterial struct {
	*chainloop.NormalizedMaterial
	// Content of the material already downloaded
	Content []byte
}

type Credentials struct {
	URL, Username, Password string
}

type InputMaterial struct {
	// Name of the material kind that the integration expects
	Type schemaapi.CraftingSchema_Material_MaterialType
}

type NewParams struct {
	ID, Version, Description string
	Logger                   log.Logger
	InputSchema              *InputSchema
}

func NewFanOut(p *NewParams, opts ...NewOpt) (*FanOutIntegration, error) {
	c := &FanOutIntegration{
		id:                  p.ID,
		version:             p.Version,
		description:         p.Description,
		log:                 p.Logger,
		subscribedMaterials: []*InputMaterial{},
	}

	if c.log == nil {
		c.log = log.NewStdLogger(io.Discard)
	}

	c.Logger = servicelogger.ScopedHelper(c.log, fmt.Sprintf("plugins/%s", p.ID))

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

	for _, m := range c.subscribedMaterials {
		if m.Type == schemaapi.CraftingSchema_Material_MATERIAL_TYPE_UNSPECIFIED {
			return fmt.Errorf("%s is not a valid material type", m.Type)
		}
	}

	return nil
}

// List of loaded integrations
type AvailablePlugins []*FanOutP
type FanOutP struct {
	FanOut
	DisposeFunc func()
}

type FanOutFactory = func(l log.Logger) (FanOut, error)

// FindByID returns the integration with the given ID from the list of available integrations
// If not found, an error is returned
func (i AvailablePlugins) FindByID(id string) (FanOut, error) {
	for _, integration := range i {
		if integration.Describe().ID == id {
			return integration, nil
		}
	}

	return nil, fmt.Errorf("integration %q not found", id)
}

func (i AvailablePlugins) Cleanup() {
	for _, plugin := range i {
		if plugin.DisposeFunc != nil {
			plugin.DisposeFunc()
		}
	}
}

type IntegrationInfo struct {
	// Identifier of the integration
	ID string
	// Integration version
	Version string
	// Integration description
	Description string
	// Kind of inputs does the integration expect as part of the execution
	SubscribedMaterials []*InputMaterial
	// Schemas in JSON schema format
	RegistrationJSONSchema, AttachmentJSONSchema []byte
}

func (i *FanOutIntegration) Describe() *IntegrationInfo {
	return &IntegrationInfo{
		ID:                     i.id,
		Version:                i.version,
		Description:            i.description,
		SubscribedMaterials:    i.subscribedMaterials,
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

func (i *FanOutIntegration) IsSubscribedTo(m string) bool {
	if i.subscribedMaterials == nil {
		return false
	}

	for _, material := range i.subscribedMaterials {
		if material.Type.String() == m {
			return true
		}
	}

	return false
}

func validatePayloadAgainstJSONSchema(jsonPayload []byte, jsonSchema []byte) error {
	schema, err := CompileJSONSchema(jsonSchema)
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
	inputs := i.subscribedMaterials

	subscribedMaterials := make([]string, len(inputs))
	for i, m := range inputs {
		subscribedMaterials[i] = m.Type.String()
	}

	return fmt.Sprintf("id=%s, version=%s, expectedMaterials=%s", i.id, i.version, subscribedMaterials)
}

type NewOpt func(*FanOutIntegration)

func WithInputMaterial(materialType schemaapi.CraftingSchema_Material_MaterialType) NewOpt {
	return func(c *FanOutIntegration) {
		material := &InputMaterial{Type: materialType}

		switch {
		case len(c.subscribedMaterials) == 0: // Materials struct is empty
			c.subscribedMaterials = []*InputMaterial{material}
		default: // Materials struct contains data
			c.subscribedMaterials = append(c.subscribedMaterials, material)
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

// generate a flat JSON schema from a struct using https://github.com/invopop/jsonschema
// We've put some limitations on the kind of input structs we support, for example:
// - Nested schemas are not supported
// - Array based properties are not supported

func generateJSONSchema(schema any) ([]byte, error) {
	if schema == nil {
		return nil, fmt.Errorf("schema is nil")
	}

	r := &jsonschema.Reflector{}
	// Set top-level properties flattened
	// https://github.com/invopop/jsonschema#expandedstruct
	r.ExpandedStruct = true

	s := r.Reflect(schema)

	// Double check that the schema is valid
	// Nested schemas are not supported
	if len(s.Definitions) > 0 {
		return nil, fmt.Errorf("nested schemas are not supported")
	}

	// Array based properties are not supported
	for _, k := range s.Properties.Keys() {
		p, _ := s.Properties.Get(k)
		s := p.(*jsonschema.Schema)
		if s.Items != nil {
			return nil, fmt.Errorf("array based properties are not supported")
		}
	}

	return json.Marshal(s)
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

func CompileJSONSchema(in []byte) (*schema_validator.Schema, error) {
	// Parse the schemas
	compiler := schema_validator.NewCompiler()
	// Enable format validation
	compiler.AssertFormat = true
	// Show description
	compiler.ExtractAnnotations = true

	if err := compiler.AddResource("schema.json", bytes.NewReader(in)); err != nil {
		return nil, fmt.Errorf("failed to compile schema: %w", err)
	}

	return compiler.Compile("schema.json")
}

// Denormalize the properties of a json schema
func CalculatePropertiesMap(s *schema_validator.Schema, m *SchemaPropertiesMap) error {
	if m == nil {
		return nil
	}

	// Schema with reference
	if s.Ref != nil {
		return CalculatePropertiesMap(s.Ref, m)
	}

	// Appended schemas
	if s.AllOf != nil {
		for _, s := range s.AllOf {
			if err := CalculatePropertiesMap(s, m); err != nil {
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
			if err := CalculatePropertiesMap(v, m); err != nil {
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
