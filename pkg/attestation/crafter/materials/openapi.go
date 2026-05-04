//
// Copyright 2026 The Chainloop Authors.
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

package materials

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	schemaapi "github.com/chainloop-dev/chainloop/app/controlplane/api/workflowcontract/v1"
	"github.com/chainloop-dev/chainloop/internal/schemavalidators"
	api "github.com/chainloop-dev/chainloop/pkg/attestation/crafter/api/attestation/v1"
	"github.com/chainloop-dev/chainloop/pkg/casclient"
	"github.com/rs/zerolog"
	"gopkg.in/yaml.v3"
)

type OpenAPICrafter struct {
	backend            *casclient.CASBackend
	noStrictValidation bool
	*crafterCommon
}

type OpenAPICraftOpt func(*OpenAPICrafter)

func WithOpenAPINoStrictValidation(noStrict bool) OpenAPICraftOpt {
	return func(c *OpenAPICrafter) {
		c.noStrictValidation = noStrict
	}
}

func NewOpenAPICrafter(materialSchema *schemaapi.CraftingSchema_Material, backend *casclient.CASBackend, l *zerolog.Logger, opts ...OpenAPICraftOpt) (*OpenAPICrafter, error) {
	if materialSchema.Type != schemaapi.CraftingSchema_Material_OPENAPI_SPEC {
		return nil, fmt.Errorf("material type is not OPENAPI_SPEC format")
	}

	crafter := &OpenAPICrafter{
		backend:       backend,
		crafterCommon: &crafterCommon{logger: l, input: materialSchema},
	}

	for _, opt := range opts {
		opt(crafter)
	}

	return crafter, nil
}

func (i *OpenAPICrafter) Craft(ctx context.Context, filepath string) (*api.Attestation_Material, error) {
	i.logger.Debug().Str("path", filepath).Msg("decoding OpenAPI spec file")

	data, err := os.ReadFile(filepath)
	if err != nil {
		return nil, fmt.Errorf("can't open the file: %w", err)
	}

	var v interface{}
	if err := json.Unmarshal(data, &v); err != nil {
		if err := yaml.Unmarshal(data, &v); err != nil {
			i.logger.Debug().Err(err).Msg("error decoding file")
			return nil, fmt.Errorf("invalid OpenAPI spec file: %w", ErrInvalidMaterialType)
		}
	}

	doc, ok := v.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid OpenAPI spec file: %w", ErrInvalidMaterialType)
	}

	if _, ok := doc["openapi"].(string); !ok {
		return nil, fmt.Errorf("invalid OpenAPI spec file: %w", ErrInvalidMaterialType)
	}

	if err := schemavalidators.ValidateOpenAPI(v); err != nil {
		if i.noStrictValidation {
			i.logger.Warn().Err(err).Msg("OpenAPI spec validation failed but strict validation is disabled, continuing")
		} else {
			i.logger.Info().Msg("if the OpenAPI spec is valid but does not pass strict schema validation, consider using --no-strict-validation")
			return nil, fmt.Errorf("invalid OpenAPI spec file: %w", err)
		}
	}

	m, err := uploadAndCraft(ctx, i.input, i.backend, filepath, i.logger)
	if err != nil {
		return nil, err
	}

	i.injectAnnotations(m, doc)

	return m, nil
}

func (i *OpenAPICrafter) injectAnnotations(m *api.Attestation_Material, doc map[string]interface{}) {
	if m.Annotations == nil {
		m.Annotations = make(map[string]string)
	}

	if specVersion, ok := doc["openapi"].(string); ok && specVersion != "" {
		m.Annotations[AnnotationToolVersionKey] = specVersion
	}

	if info, ok := doc["info"].(map[string]interface{}); ok {
		if title, ok := info["title"].(string); ok && title != "" {
			m.Annotations[AnnotationToolNameKey] = title
		}
		if version, ok := info["version"].(string); ok && version != "" {
			m.Annotations["chainloop.material.api.version"] = version
		}
	}
}
