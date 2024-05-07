//
// Copyright 2024 The Chainloop Authors.
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

package schemavalidators

import (
	_ "embed"
	"errors"
	"strings"

	"github.com/santhosh-tekuri/jsonschema/v5"
)

// ErrInvalidJSONPayload represents an error for invalid JSON payload.
var ErrInvalidJSONPayload = errors.New("invalid JSON payload")

// CycloneDXVersion represents the version of CycloneDX schema.
type CycloneDXVersion string

const (
	// CycloneDXVersion1_5 represents CycloneDX version 1.5 schema.
	CycloneDXVersion1_5 CycloneDXVersion = "1.5"
	// CycloneDXVersion1_6 represents CycloneDX version 1.6 schema.
	CycloneDXVersion1_6 CycloneDXVersion = "1.6"
)

var (
	// Common schemas
	//go:embed external_schemas/cyclonedx/jsf-0.82.schema.json
	jsfSpecVersion0_82 string
	//go:embed external_schemas/cyclonedx/spdx.schema.json
	spdxSpec string
	//go:embed external_schemas/cyclonedx/bom-1.5.schema.json
	bomSpecVersion1_5 string
	//go:embed external_schemas/cyclonedx/bom-1.6.schema.json
	bomSpecVersion1_6 string
)

// SchemaURLMapping maps the schema URL to the schema content. This is used to compile the schema validators
// against the schemas on external_schemas/*. This is done in the init function.
// The keys are the URLs of the schemas and the values are the schema content that can be found in the embedded
// files.
var SchemaURLMapping = map[string]string{
	"http://cyclonedx.org/schema/jsf-0.82.schema.json": jsfSpecVersion0_82,
	"http://cyclonedx.org/schema/spdx.schema.json":     spdxSpec,
	"http://cyclonedx.org/schema/bom-1.5.schema.json":  bomSpecVersion1_5,
	"http://cyclonedx.org/schema/bom-1.6.schema.json":  bomSpecVersion1_6,
}

var compiledCycloneDxSchemas map[CycloneDXVersion]*jsonschema.Schema

func init() {
	compiler := jsonschema.NewCompiler()
	for url, schema := range SchemaURLMapping {
		_ = compiler.AddResource(url, strings.NewReader(schema))
	}

	compiledCycloneDxSchemas = make(map[CycloneDXVersion]*jsonschema.Schema)
	compiledCycloneDxSchemas[CycloneDXVersion1_5] = compiler.MustCompile("http://cyclonedx.org/schema/bom-1.5.schema.json")
	compiledCycloneDxSchemas[CycloneDXVersion1_6] = compiler.MustCompile("http://cyclonedx.org/schema/bom-1.6.schema.json")
}

// ValidateCycloneDX validates the given object against the specified CycloneDX schema version.
func ValidateCycloneDX(data interface{}, version CycloneDXVersion) error {
	if version == "" {
		version = CycloneDXVersion1_6
	}

	schema, ok := compiledCycloneDxSchemas[version]
	if !ok {
		return errors.New("invalid CycloneDX schema version")
	}

	if err := schema.Validate(data); err != nil {
		var invalidJSONTypeError jsonschema.InvalidJSONTypeError
		if errors.As(err, &invalidJSONTypeError) {
			return ErrInvalidJSONPayload
		}
		return err
	}

	return nil
}
