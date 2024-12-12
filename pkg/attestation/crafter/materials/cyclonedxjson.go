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

package materials

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"

	schemaapi "github.com/chainloop-dev/chainloop/app/controlplane/api/workflowcontract/v1"
	"github.com/chainloop-dev/chainloop/internal/casclient"
	"github.com/chainloop-dev/chainloop/internal/schemavalidators"
	api "github.com/chainloop-dev/chainloop/pkg/attestation/crafter/api/attestation/v1"
	remotename "github.com/google/go-containerregistry/pkg/name"

	"github.com/rs/zerolog"
)

// containerComponentKind is the kind of the main component when it's a container
const containerComponentKind = "container"

type CyclonedxJSONCrafter struct {
	backend *casclient.CASBackend
	*crafterCommon
}

func NewCyclonedxJSONCrafter(materialSchema *schemaapi.CraftingSchema_Material, backend *casclient.CASBackend, l *zerolog.Logger) (*CyclonedxJSONCrafter, error) {
	if materialSchema.Type != schemaapi.CraftingSchema_Material_SBOM_CYCLONEDX_JSON {
		return nil, fmt.Errorf("material type is not cyclonedx json")
	}

	return &CyclonedxJSONCrafter{
		backend:       backend,
		crafterCommon: &crafterCommon{logger: l, input: materialSchema},
	}, nil
}

func (i *CyclonedxJSONCrafter) Craft(ctx context.Context, filePath string) (*api.Attestation_Material, error) {
	f, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("can't open the file: %w", err)
	}

	var v interface{}
	if err := json.Unmarshal(f, &v); err != nil {
		i.logger.Debug().Err(err).Msg("error decoding file")
		return nil, fmt.Errorf("invalid cyclonedx sbom file: %w", ErrInvalidMaterialType)
	}

	// Setting the version to empty string to validate against the latest version of the schema
	err = schemavalidators.ValidateCycloneDX(v, "")
	if err != nil {
		i.logger.Debug().Err(err).Msgf("error decoding file: %#v", err)
		return nil, fmt.Errorf("invalid cyclonedx sbom file: %w", ErrInvalidMaterialType)
	}

	m, err := uploadAndCraft(ctx, i.input, i.backend, filePath, i.logger)
	if err != nil {
		return nil, fmt.Errorf("error crafting material: %w", err)
	}

	// Include the main component information if available
	mainComponent, err := i.extractMainComponent(v)
	if err != nil {
		i.logger.Debug().Err(err).Msg("error extracting main component from sbom, skipping...")
	}

	res := m
	res.M = &api.Attestation_Material_SbomArtifact{
		SbomArtifact: &api.Attestation_Material_SBOMArtifact{
			Artifact:      m.GetArtifact(),
			MainComponent: mainComponent,
		},
	}

	return res, nil
}

// extractMainComponent inspects the SBOM and extracts the main component if any and available
func (i *CyclonedxJSONCrafter) extractMainComponent(v interface{}) (string, error) {
	// Helper function to extract and validate keys from the map
	extractKey := func(m map[string]interface{}, key string) (map[string]interface{}, error) {
		val, ok := m[key].(map[string]interface{})
		if !ok {
			return nil, errors.New("invalid cyclonedx sbom file")
		}
		return val, nil
	}

	// Cast the incoming interface to a map
	m, ok := v.(map[string]interface{})
	if !ok {
		return "", errors.New("invalid cyclonedx sbom file")
	}

	// Extract metadata and component maps. The metadata field is not required
	metadata, err := extractKey(m, "metadata")
	if err != nil {
		return "", err
	}
	component, err := extractKey(metadata, "component")
	if err != nil {
		return "", err
	}

	// Extract and validate name
	name, ok := component["name"].(string)
	if !ok {
		return "", errors.New("couldn't extract name from metadata")
	}
	kind, ok := component["type"].(string)
	if !ok {
		return "", errors.New("couldn't extract type from metadata")
	}

	// If the main component is not a container, return the name as is
	if kind != containerComponentKind {
		return name, nil
	}

	// Standardize the name to have the full repository name including the registry and
	// sanitize the name to remove the possible tag from the repository name
	stdName, err := remotename.NewRepository(strings.Split(name, ":")[0])
	if err != nil {
		return "", fmt.Errorf("couldn't parse OCI image repository name: %w", err)
	}

	return stdName.String(), nil
}
