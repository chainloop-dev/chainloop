//
// Copyright 2023 The Chainloop Authors.
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

	cdx "github.com/CycloneDX/cyclonedx-go"
	schemaapi "github.com/chainloop-dev/chainloop/app/controlplane/api/workflowcontract/v1"
	api "github.com/chainloop-dev/chainloop/internal/attestation/crafter/api/attestation/v1"
	"github.com/chainloop-dev/chainloop/internal/casclient"
	"github.com/rs/zerolog"
)

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
	f, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("can't open the file: %w", err)
	}
	defer f.Close()

	// Decode the file to check it's a valid CycloneDX BOM
	bom := new(cdx.BOM)
	decoder := cdx.NewBOMDecoder(f, cdx.BOMFileFormatJSON)
	if err = decoder.Decode(bom); err != nil {
		i.logger.Debug().Err(err).Msg("error decoding file")
		return nil, fmt.Errorf("invalid cyclonedx sbom file: %w", ErrInvalidMaterialType)
	}

	if bom.Metadata == nil {
		return nil, fmt.Errorf("invalid cyclonedx sbom file: %w", ErrInvalidMaterialType)
	}

	return uploadAndCraft(ctx, i.input, i.backend, filePath, i.logger)
}
