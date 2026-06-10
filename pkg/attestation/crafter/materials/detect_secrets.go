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
	api "github.com/chainloop-dev/chainloop/pkg/attestation/crafter/api/attestation/v1"
	"github.com/chainloop-dev/chainloop/pkg/casclient"
	"github.com/rs/zerolog"
)

type DetectSecretsCrafter struct {
	*crafterCommon
	backend *casclient.CASBackend
}

// detectSecretsBaseline represents the subset of the detect-secrets baseline
// file used to validate its structure and extract metadata.
// https://github.com/Yelp/detect-secrets
type detectSecretsBaseline struct {
	Version     string                      `json:"version"`
	PluginsUsed []map[string]any            `json:"plugins_used"`
	Results     map[string][]map[string]any `json:"results"`
}

func NewDetectSecretsCrafter(schema *schemaapi.CraftingSchema_Material, backend *casclient.CASBackend, l *zerolog.Logger) (*DetectSecretsCrafter, error) {
	if schema.Type != schemaapi.CraftingSchema_Material_YELP_DETECT_SECRETS_BASELINE {
		return nil, fmt.Errorf("material type is not a detect-secrets baseline")
	}
	craftCommon := &crafterCommon{logger: l, input: schema}
	return &DetectSecretsCrafter{backend: backend, crafterCommon: craftCommon}, nil
}

func (i *DetectSecretsCrafter) Craft(ctx context.Context, filePath string) (*api.Attestation_Material, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("can't open the file: %w", err)
	}

	var baseline detectSecretsBaseline
	if err = json.Unmarshal(data, &baseline); err != nil {
		return nil, fmt.Errorf("invalid detect-secrets baseline file: %w", ErrInvalidMaterialType)
	}

	// Structural fingerprint check. A detect-secrets baseline always carries a
	// version, the list of plugins it ran, and a results map (which may be empty
	// when no secrets were found). Reject anything missing these fields.
	if baseline.Version == "" || baseline.PluginsUsed == nil || baseline.Results == nil {
		return nil, fmt.Errorf("missing required detect-secrets baseline fields: %w", ErrInvalidMaterialType)
	}

	// An empty results map means a clean scan. It's ambiguous, but we accept it.
	if len(baseline.Results) == 0 {
		i.logger.Debug().Msg("Accepting an empty report.")
	}

	// Call uploadAndCraft with the path of the JSON baseline file
	m, err := uploadAndCraft(ctx, i.input, i.backend, filePath, i.logger)
	if err != nil {
		return nil, err
	}

	i.injectAnnotations(m, &baseline)

	return m, nil
}

func (i *DetectSecretsCrafter) injectAnnotations(m *api.Attestation_Material, baseline *detectSecretsBaseline) {
	if m.Annotations == nil {
		m.Annotations = make(map[string]string)
	}
	m.Annotations[AnnotationToolNameKey] = "detect-secrets"
	m.Annotations[AnnotationToolVersionKey] = baseline.Version
}
