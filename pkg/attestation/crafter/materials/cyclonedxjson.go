//
// Copyright 2024-2025 The Chainloop Authors.
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
	"strings"

	schemaapi "github.com/chainloop-dev/chainloop/app/controlplane/api/workflowcontract/v1"
	"github.com/chainloop-dev/chainloop/internal/schemavalidators"
	api "github.com/chainloop-dev/chainloop/pkg/attestation/crafter/api/attestation/v1"
	"github.com/chainloop-dev/chainloop/pkg/casclient"
	remotename "github.com/google/go-containerregistry/pkg/name"
	"github.com/rs/zerolog"
)

const (
	// containerComponentKind is the kind of the main component when it's a container
	containerComponentKind = "container"
	// aquaTrivyRepoDigestPropertyKey is the key used by Aqua Trivy to store the repo digest
	aquaTrivyRepoDigestPropertyKey = "aquasecurity:trivy:RepoDigest"
)

type CyclonedxJSONCrafter struct {
	backend *casclient.CASBackend
	*crafterCommon
}

// cyclonedxDoc internal struct to unmarshall the incoming CycloneDX JSON
type cyclonedxDoc struct {
	SpecVersion string          `json:"specVersion"`
	Metadata    json.RawMessage `json:"metadata"`
}

type cyclonedxMetadataV14 struct {
	Tools []struct {
		Name    string `json:"name"`
		Version string `json:"version"`
	} `json:"tools"`
	Component cyclonedxComponent `json:"component"`
}

type cyclonedxComponent struct {
	Name       string `json:"name"`
	Type       string `json:"type"`
	Version    string `json:"version"`
	Properties []struct {
		Name  string `json:"name"`
		Value string `json:"value"`
	} `json:"properties"`
}

type cyclonedxMetadataV15 struct {
	Tools struct {
		Components []struct { // available from 1.5 onwards
			Name    string `json:"name"`
			Version string `json:"version"`
		} `json:"components"`
	} `json:"tools"`
	Component cyclonedxComponent `json:"component"`
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

	res := m
	res.M = &api.Attestation_Material_SbomArtifact{
		SbomArtifact: &api.Attestation_Material_SBOMArtifact{
			Artifact: m.GetArtifact(),
		},
	}

	// parse the file to extract the main information
	var doc cyclonedxDoc
	if err = json.Unmarshal(f, &doc); err != nil {
		i.logger.Debug().Err(err).Msg("error decoding file to extract main information, skipping ...")
	}

	switch doc.SpecVersion {
	case "1.4":
		var metaV14 cyclonedxMetadataV14
		if err = json.Unmarshal(doc.Metadata, &metaV14); err != nil {
			i.logger.Debug().Err(err).Msg("error decoding file to extract main information, skipping ...")
		} else {
			i.extractMetadata(m, &metaV14)
		}
	default: // 1.5 onwards
		var metaV15 cyclonedxMetadataV15
		if err = json.Unmarshal(doc.Metadata, &metaV15); err != nil {
			i.logger.Debug().Err(err).Msg("error decoding file to extract main information, skipping ...")
		} else {
			i.extractMetadata(m, &metaV15)
		}
	}

	return res, nil
}

func (i *CyclonedxJSONCrafter) extractMetadata(m *api.Attestation_Material, metadata any) {
	m.Annotations = make(map[string]string)

	switch meta := metadata.(type) {
	case *cyclonedxMetadataV14:
		if err := i.extractMainComponent(m, &meta.Component); err != nil {
			i.logger.Debug().Err(err).Msg("error extracting main component from sbom, skipping...")
		}

		if len(meta.Tools) > 0 {
			m.Annotations[AnnotationToolNameKey] = meta.Tools[0].Name
			m.Annotations[AnnotationToolVersionKey] = meta.Tools[0].Version
		}
	case *cyclonedxMetadataV15:
		if err := i.extractMainComponent(m, &meta.Component); err != nil {
			i.logger.Debug().Err(err).Msg("error extracting main component from sbom, skipping...")
		}

		if len(meta.Tools.Components) > 0 {
			m.Annotations[AnnotationToolNameKey] = meta.Tools.Components[0].Name
			m.Annotations[AnnotationToolVersionKey] = meta.Tools.Components[0].Version
		}
	default:
		i.logger.Debug().Msg("unknown metadata version")
	}
}

// extractMainComponent inspects the SBOM and extracts the main component if any and available
func (i *CyclonedxJSONCrafter) extractMainComponent(m *api.Attestation_Material, component *cyclonedxComponent) error {
	var mainComponent *SBOMMainComponentInfo

	// If the version is empty, try to extract it from the properties
	if component.Version == "" {
		for _, prop := range component.Properties {
			if prop.Name == aquaTrivyRepoDigestPropertyKey {
				if parts := strings.Split(prop.Value, "sha256:"); len(parts) == 2 {
					component.Version = fmt.Sprintf("sha256:%s", parts[1])
					break
				}
			}
		}
	}

	if component.Type != containerComponentKind {
		mainComponent = &SBOMMainComponentInfo{
			name:    component.Name,
			kind:    component.Type,
			version: component.Version,
		}
	} else {
		// Standardize the name to have the full repository name including the registry and
		// sanitize the name to remove the possible tag from the repository name
		ref, err := remotename.ParseReference(component.Name)
		if err != nil {
			return fmt.Errorf("couldn't parse OCI image repository name: %w", err)
		}

		mainComponent = &SBOMMainComponentInfo{
			name:    ref.Context().String(),
			kind:    component.Type,
			version: component.Version,
		}
	}

	// If the main component is available, include it in the material
	m.M.(*api.Attestation_Material_SbomArtifact).SbomArtifact.MainComponent = &api.Attestation_Material_SBOMArtifact_MainComponent{
		Name:    mainComponent.name,
		Kind:    mainComponent.kind,
		Version: mainComponent.version,
	}

	return nil
}
