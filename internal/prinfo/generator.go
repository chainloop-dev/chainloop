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

package prinfo

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/invopop/jsonschema"
)

// Generator handles the generation of JSON schemas for PR info.
type Generator struct {
}

// NewGenerator creates a new schema generator
func NewGenerator() *Generator {
	return &Generator{}
}

// GeneratePRInfoSchema generates a JSON schema for the PR/MR info data.
func (g *Generator) GeneratePRInfoSchema(version string) *jsonschema.Schema {
	r := &jsonschema.Reflector{
		DoNotReference:             true,
		ExpandedStruct:             true,
		RequiredFromJSONSchemaTags: true,
		// Set to false to allow additional properties by default
		AllowAdditionalProperties: false,
	}

	schema := r.Reflect(&Data{})

	schema.ID = jsonschema.ID(fmt.Sprintf("https://schemas.chainloop.dev/prinfo/%s/pr-info.schema.json", version))
	schema.Title = "Pull Request / Merge Request Information"
	schema.Description = "Schema for Pull Request or Merge Request metadata collected during attestation"
	// we want to have a specific version of the schema to avoid compatibility issues
	schema.Version = "http://json-schema.org/draft-07/schema#"

	return schema
}

// Save writes the schema to a file
func (g *Generator) Save(schema *jsonschema.Schema, outputDir, version string) error {
	schemaJSON, err := json.MarshalIndent(schema, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal schema to JSON: %w", err)
	}

	outputFile := fmt.Sprintf("%s/pr-info-%s.schema.json", outputDir, version)
	if err := os.WriteFile(outputFile, schemaJSON, 0644); err != nil {
		return fmt.Errorf("failed to write schema to file: %w", err)
	}

	return nil
}
