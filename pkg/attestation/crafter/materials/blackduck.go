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
	"fmt"
	"os"

	schemaapi "github.com/chainloop-dev/chainloop/app/controlplane/api/workflowcontract/v1"
	"github.com/chainloop-dev/chainloop/internal/casclient"
	api "github.com/chainloop-dev/chainloop/pkg/attestation/crafter/api/attestation/v1"
	"github.com/rs/zerolog"
)

type BlackduckSCAJSONCrafter struct {
	backend *casclient.CASBackend
	*crafterCommon
}

func NewBlackduckSCAJSONCrafter(materialSchema *schemaapi.CraftingSchema_Material, backend *casclient.CASBackend, l *zerolog.Logger) (*BlackduckSCAJSONCrafter, error) {
	if materialSchema.Type != schemaapi.CraftingSchema_Material_BLACKDUCK_SCA_JSON {
		return nil, fmt.Errorf("material type is not Blackduck SCA report in JSON format")
	}

	return &BlackduckSCAJSONCrafter{
		backend:       backend,
		crafterCommon: &crafterCommon{logger: l, input: materialSchema},
	}, nil
}

func (i *BlackduckSCAJSONCrafter) Craft(ctx context.Context, filePath string) (*api.Attestation_Material, error) {
	_, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("can't open the file: %w", err)
	}

	return uploadAndCraft(ctx, i.input, i.backend, filePath, i.logger)
}
