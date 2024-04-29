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

	schemaapi "github.com/chainloop-dev/chainloop/app/controlplane/api/workflowcontract/v1"
	api "github.com/chainloop-dev/chainloop/internal/attestation/crafter/api/attestation/v1"
	"github.com/chainloop-dev/chainloop/internal/casclient"
	"github.com/openvex/go-vex/pkg/vex"
	"github.com/rs/zerolog"
)

type OpenVEXCrafter struct {
	backend *casclient.CASBackend
	*crafterCommon
}

func NewOpenVEXCrafter(materialSchema *schemaapi.CraftingSchema_Material, backend *casclient.CASBackend, l *zerolog.Logger) (*OpenVEXCrafter, error) {
	if materialSchema.Type != schemaapi.CraftingSchema_Material_OPENVEX {
		return nil, fmt.Errorf("material type is not OpenVEX format")
	}

	return &OpenVEXCrafter{
		backend:       backend,
		crafterCommon: &crafterCommon{logger: l, input: materialSchema},
	}, nil
}

func (i *OpenVEXCrafter) Craft(ctx context.Context, filePath string) (*api.Attestation_Material, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("can't open the file: %w", err)
	}

	i.logger.Debug().Str("path", filePath).Msg("decoding OpenVex file")
	doc, err := vex.Parse(data)
	// parse doesn't fail if the provided file is a valid JSON, but not a valid OpenVEX file
	if err != nil || doc.ID == "" {
		if err != nil {
			i.logger.Debug().Err(err).Msg("error decoding file")
		}

		return nil, fmt.Errorf("invalid OpenVEX file: %w", ErrInvalidMaterialType)
	}

	return uploadAndCraftFromFile(ctx, i.input, i.backend, filePath, i.logger)
}
