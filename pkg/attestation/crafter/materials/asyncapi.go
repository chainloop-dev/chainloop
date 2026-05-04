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
	"sort"

	schemaapi "github.com/chainloop-dev/chainloop/app/controlplane/api/workflowcontract/v1"
	"github.com/chainloop-dev/chainloop/internal/schemavalidators"
	api "github.com/chainloop-dev/chainloop/pkg/attestation/crafter/api/attestation/v1"
	"github.com/chainloop-dev/chainloop/pkg/casclient"
	"github.com/rs/zerolog"
	"gopkg.in/yaml.v3"
)

type AsyncAPICrafter struct {
	backend            *casclient.CASBackend
	noStrictValidation bool
	*crafterCommon
}

type AsyncAPICraftOpt func(*AsyncAPICrafter)

func WithAsyncAPINoStrictValidation(noStrict bool) AsyncAPICraftOpt {
	return func(c *AsyncAPICrafter) {
		c.noStrictValidation = noStrict
	}
}

func NewAsyncAPICrafter(materialSchema *schemaapi.CraftingSchema_Material, backend *casclient.CASBackend, l *zerolog.Logger, opts ...AsyncAPICraftOpt) (*AsyncAPICrafter, error) {
	if materialSchema.Type != schemaapi.CraftingSchema_Material_ASYNCAPI_SPEC {
		return nil, fmt.Errorf("material type is not AsyncAPI spec")
	}

	c := &AsyncAPICrafter{
		backend:       backend,
		crafterCommon: &crafterCommon{logger: l, input: materialSchema},
	}

	for _, opt := range opts {
		opt(c)
	}

	return c, nil
}

func (i *AsyncAPICrafter) Craft(ctx context.Context, filepath string) (*api.Attestation_Material, error) {
	i.logger.Debug().Str("path", filepath).Msg("decoding AsyncAPI spec file")

	f, err := os.ReadFile(filepath)
	if err != nil {
		return nil, fmt.Errorf("can't open the file: %w - %w", err, ErrInvalidMaterialType)
	}

	var v interface{}
	if err := json.Unmarshal(f, &v); err != nil {
		if err := yaml.Unmarshal(f, &v); err != nil {
			i.logger.Debug().Err(err).Msg("error decoding file")
			return nil, fmt.Errorf("invalid AsyncAPI spec file: %w", ErrInvalidMaterialType)
		}
	}

	doc, ok := v.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid AsyncAPI spec file: %w", ErrInvalidMaterialType)
	}

	if _, ok := doc["asyncapi"].(string); !ok {
		return nil, fmt.Errorf("invalid AsyncAPI spec file: %w", ErrInvalidMaterialType)
	}

	if err := schemavalidators.ValidateAsyncAPI(v); err != nil {
		if i.noStrictValidation {
			i.logger.Warn().Err(err).Msg("error validating AsyncAPI spec, strict validation disabled, continuing")
		} else {
			i.logger.Debug().Err(err).Msg("error validating AsyncAPI spec")
			i.logger.Info().Msg("you can disable strict validation to skip schema validation")
			return nil, fmt.Errorf("invalid AsyncAPI spec file: %w", ErrInvalidMaterialType)
		}
	}

	m, err := uploadAndCraft(ctx, i.input, i.backend, filepath, i.logger)
	if err != nil {
		return nil, err
	}

	i.injectAnnotations(m, doc)

	return m, nil
}

func (i *AsyncAPICrafter) injectAnnotations(m *api.Attestation_Material, doc map[string]interface{}) {
	if m.Annotations == nil {
		m.Annotations = make(map[string]string)
	}

	if specVersion, ok := doc["asyncapi"].(string); ok && specVersion != "" {
		m.Annotations["chainloop.material.api.spec_version"] = specVersion
	}

	if info, ok := doc["info"].(map[string]interface{}); ok {
		if title, ok := info["title"].(string); ok && title != "" {
			m.Annotations["chainloop.material.api.name"] = title
		}
		if version, ok := info["version"].(string); ok && version != "" {
			m.Annotations["chainloop.material.api.version"] = version
		}
	}

	if servers, ok := doc["servers"].(map[string]interface{}); ok {
		serverNames := make([]string, 0, len(servers))
		for name := range servers {
			serverNames = append(serverNames, name)
		}
		sort.Strings(serverNames)

		for _, name := range serverNames {
			if server, ok := servers[name].(map[string]interface{}); ok {
				if protocol, ok := server["protocol"].(string); ok && protocol != "" {
					m.Annotations["chainloop.material.api.protocol"] = protocol
					break
				}
			}
		}
	}
}
