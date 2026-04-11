//
// Copyright 2023-2026 The Chainloop Authors.
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
	"fmt"
	"os"
	"strings"

	schemaapi "github.com/chainloop-dev/chainloop/app/controlplane/api/workflowcontract/v1"
	api "github.com/chainloop-dev/chainloop/pkg/attestation/crafter/api/attestation/v1"
	"github.com/chainloop-dev/chainloop/pkg/casclient"
	remotename "github.com/google/go-containerregistry/pkg/name"
	"github.com/rs/zerolog"
	"github.com/spdx/tools-golang/json"
	"github.com/spdx/tools-golang/spdx"
	"github.com/spdx/tools-golang/spdxlib"
)

type SPDXJSONCrafter struct {
	*crafterCommon
	backend *casclient.CASBackend
}

func NewSPDXJSONCrafter(materialSchema *schemaapi.CraftingSchema_Material, backend *casclient.CASBackend, l *zerolog.Logger) (*SPDXJSONCrafter, error) {
	if materialSchema.Type != schemaapi.CraftingSchema_Material_SBOM_SPDX_JSON {
		return nil, fmt.Errorf("material type is not spdx json")
	}

	return &SPDXJSONCrafter{
		backend:       backend,
		crafterCommon: &crafterCommon{logger: l, input: materialSchema},
	}, nil
}

func (i *SPDXJSONCrafter) Craft(ctx context.Context, filePath string) (*api.Attestation_Material, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("can't open the file: %w", err)
	}
	defer f.Close()

	// Decode the file to check it's a valid SPDX BOM
	doc, err := json.Read(f)
	if err != nil {
		i.logger.Debug().Err(err).Msg("error decoding file")
		return nil, fmt.Errorf("invalid spdx sbom file: %w", ErrInvalidMaterialType)
	}

	m, err := uploadAndCraft(ctx, i.input, i.backend, filePath, i.logger)
	if err != nil {
		return nil, err
	}

	res := m
	res.M = &api.Attestation_Material_SbomArtifact{
		SbomArtifact: &api.Attestation_Material_SBOMArtifact{
			Artifact: m.GetArtifact(),
		},
	}

	// Extract main component information from SPDX document
	if err := i.extractMainComponent(m, doc); err != nil {
		i.logger.Debug().Err(err).Msg("error extracting main component from spdx sbom, skipping...")
	}

	i.injectAnnotations(m, doc)

	return res, nil
}

// extractMainComponent inspects the SPDX document and extracts the main component if any.
// It uses the first described package (via DESCRIBES relationship). If multiple described
// packages exist, only the first is used and a warning is logged.
// NOTE: SPDX PrimaryPackagePurpose values (APPLICATION, CONTAINER, FRAMEWORK, LIBRARY, etc.)
// are lowercased for consistency with CycloneDX component types. The two specs have different
// vocabularies so consumers should handle both sets of values.
func (i *SPDXJSONCrafter) extractMainComponent(m *api.Attestation_Material, doc *spdx.Document) error {
	describedIDs, err := spdxlib.GetDescribedPackageIDs(doc)
	if err != nil {
		return fmt.Errorf("couldn't get described packages: %w", err)
	}

	if len(describedIDs) == 0 {
		return fmt.Errorf("no described packages found")
	}

	if len(describedIDs) > 1 {
		i.logger.Warn().Int("count", len(describedIDs)).Msg("multiple described packages found, using the first one")
	}

	// Use the first described package
	targetID := describedIDs[0]

	// Find the package by ID
	var describedPkg *spdx.Package
	for _, pkg := range doc.Packages {
		if pkg.PackageSPDXIdentifier == targetID {
			describedPkg = pkg
			break
		}
	}

	if describedPkg == nil {
		return fmt.Errorf("described package %q not found in packages list", targetID)
	}

	name := describedPkg.PackageName
	version := describedPkg.PackageVersion

	// PrimaryPackagePurpose is optional in SPDX 2.3. Best effort: return name
	// and version even if kind is unknown.
	kind := strings.ToLower(describedPkg.PrimaryPackagePurpose)

	// For container packages, standardize the name via go-containerregistry
	// to get the full repository name and strip any tag (matching CycloneDX behavior).
	// If parsing fails (e.g. missing registry credentials), continue with the original name.
	if kind == containerComponentKind {
		ref, err := remotename.ParseReference(name)
		if err != nil {
			i.logger.Debug().Err(err).Str("name", name).Msg("couldn't parse OCI image reference, using original name")
		} else {
			name = ref.Context().String()
		}
	}

	setMainComponent(m, name, kind, version)

	return nil
}

func (i *SPDXJSONCrafter) injectAnnotations(m *api.Attestation_Material, doc *spdx.Document) {
	if m.Annotations == nil {
		m.Annotations = make(map[string]string)
	}

	// Extract all tools from the creators array
	var tools []Tool
	for _, c := range doc.CreationInfo.Creators {
		if c.CreatorType == "Tool" {
			// try to extract the tool name and version
			// e.g. "myTool-1.0.0"
			name, version := c.Creator, ""
			if parts := strings.SplitN(c.Creator, "-", 2); len(parts) == 2 {
				name, version = parts[0], parts[1]
			}
			tools = append(tools, Tool{Name: name, Version: version})
		}
	}

	SetToolsAnnotation(m, tools)

	// Maintain backward compatibility - keep legacy keys for the first tool
	if len(tools) > 0 {
		if tools[0].Name != "" {
			m.Annotations[AnnotationToolNameKey] = tools[0].Name
		}
		if tools[0].Version != "" {
			m.Annotations[AnnotationToolVersionKey] = tools[0].Version
		}
	}
}
