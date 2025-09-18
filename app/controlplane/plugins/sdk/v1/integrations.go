//
// Copyright 2025 The Chainloop Authors.
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
	"sort"

	"github.com/invopop/jsonschema"
	schema_validator "github.com/santhosh-tekuri/jsonschema/v5"
)

// Integration is the basic interface for all integrations
type Integration interface {
	fmt.Stringer

	Describe() *IntegrationInfo
	// Register Validates, marshalls and returns the configuration that needs to be persisted
	Register(ctx context.Context, req *RegistrationRequest) (*RegistrationResponse, error)
	// Attach Validates that the attachment configuration is valid in the context of the provided registration
	Attach(ctx context.Context, req *AttachmentRequest) (*AttachmentResponse, error)
	// Execute runs the integration
	Execute(ctx context.Context, req any) error
}

type IntegrationInfo struct {
	// Identifier of the integration
	ID string
	// Friendly Name of the integration
	Name string
	// Integration version
	Version string
	// Integration description
	Description string
	// Type of integration (e.g. "notification", "task-manager", "fanout", etc.)
	Type string
	// Schemas in JSON schema format
	RegistrationJSONSchema, AttachmentJSONSchema []byte
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

// ValidateRegistrationRequest Validates the registration payload against the registration JSON schema
func ValidateRegistrationRequest(i Integration, jsonPayload []byte) error {
	return validatePayloadAgainstJSONSchema(jsonPayload, i.Describe().RegistrationJSONSchema)
}

// ValidateAttachmentRequest Validates the attachment payload against the attachment JSON schema
func ValidateAttachmentRequest(i Integration, jsonPayload []byte) error {
	return validatePayloadAgainstJSONSchema(jsonPayload, i.Describe().AttachmentJSONSchema)
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
	Format  string
	Default string
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

			var defaultVal string
			if v.Default != nil && !required {
				defaultVal = fmt.Sprintf("%v", v.Default)
			}

			(*m)[k] = &SchemaProperty{
				Name:        k,
				Type:        v.Types[0],
				Required:    required,
				Description: v.Description,
				Format:      v.Format,
				Default:     defaultVal,
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
