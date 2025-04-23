//
// Copyright 2023-2025 The Chainloop Authors.
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
	"github.com/spdx/tools-golang/json"
	"github.com/spdx/tools-golang/spdx"

	"github.com/rs/zerolog"
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

	i.injectAnnotations(m, doc)

	return m, nil
}

func (i *SPDXJSONCrafter) injectAnnotations(m *api.Attestation_Material, doc *spdx.Document) {
	for _, c := range doc.CreationInfo.Creators {
		if c.CreatorType == "Tool" {
			m.Annotations = make(map[string]string)
			m.Annotations[AnnotationToolNameKey] = c.Creator

			// try to extract the tool name and version
			// e.g. "myTool-1.0.0"
			parts := strings.SplitN(c.Creator, "-", 2)
			if len(parts) == 2 {
				m.Annotations[AnnotationToolNameKey] = parts[0]
				m.Annotations[AnnotationToolVersionKey] = parts[1]
			}
			break
		}
	}
}
