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

	schemaapi "github.com/chainloop-dev/chainloop/app/controlplane/api/workflowcontract/v1"
	"github.com/chainloop-dev/chainloop/internal/casclient"
	api "github.com/chainloop-dev/chainloop/pkg/attestation/crafter/api/attestation/v1"
	"github.com/owenrumney/go-sarif/sarif"
	"github.com/rs/zerolog"
)

type SARIFCrafter struct {
	backend *casclient.CASBackend
	*crafterCommon
}

func NewSARIFCrafter(materialSchema *schemaapi.CraftingSchema_Material, backend *casclient.CASBackend, l *zerolog.Logger) (*SARIFCrafter, error) {
	if materialSchema.Type != schemaapi.CraftingSchema_Material_SARIF {
		return nil, fmt.Errorf("material type is not SARIF format")
	}

	return &SARIFCrafter{
		backend:       backend,
		crafterCommon: &crafterCommon{logger: l, input: materialSchema},
	}, nil
}

func (i *SARIFCrafter) Craft(ctx context.Context, filepath string) (*api.Attestation_Material, error) {
	i.logger.Debug().Str("path", filepath).Msg("decoding SARIF file")
	doc, err := sarif.Open(filepath)
	// parse doesn't fail if the provided file is a valid JSON, but not a valid CSAF VEX file
	if err != nil || doc.Schema == "" {
		if err != nil {
			i.logger.Debug().Err(err).Msg("error decoding file")
		}

		return nil, fmt.Errorf("invalid SARIF file: %w", ErrInvalidMaterialType)
	}

	return uploadAndCraft(ctx, i.input, i.backend, filepath, i.logger)
}
