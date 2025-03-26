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

	schemaapi "github.com/chainloop-dev/chainloop/app/controlplane/api/workflowcontract/v1"
	api "github.com/chainloop-dev/chainloop/pkg/attestation/crafter/api/attestation/v1"
	"github.com/chainloop-dev/chainloop/pkg/casclient"

	"github.com/rs/zerolog"
)

type twistCLIScanResult struct {
	Results    any    `json:"results"`
	ConsoleURL string `json:"consoleURL"`
}

type TwistCLIScanCrafter struct {
	backend *casclient.CASBackend
	*crafterCommon
}

func NewTwistCLIScanCrafter(materialSchema *schemaapi.CraftingSchema_Material, backend *casclient.CASBackend, l *zerolog.Logger) (*TwistCLIScanCrafter, error) {
	if materialSchema.Type != schemaapi.CraftingSchema_Material_TWISTCLI_SCAN_JSON {
		return nil, fmt.Errorf("material type is not a twistcli scan")
	}

	return &TwistCLIScanCrafter{
		backend:       backend,
		crafterCommon: &crafterCommon{logger: l, input: materialSchema},
	}, nil
}

func (i *TwistCLIScanCrafter) Craft(ctx context.Context, filePath string) (*api.Attestation_Material, error) {
	f, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("can't open the file: %w", err)
	}

	var v twistCLIScanResult
	if err := json.Unmarshal(f, &v); err != nil {
		i.logger.Debug().Err(err).Msg("error decoding file")
		return nil, fmt.Errorf("invalid twistcli scan file: %w", ErrInvalidMaterialType)
	}

	// Check the unmarshalled JSON contains a results and consoleURL fields
	if v.Results == nil || v.ConsoleURL == "" {
		return nil, fmt.Errorf("invalid twistcli scan file: %w", ErrInvalidMaterialType)
	}

	return uploadAndCraft(ctx, i.input, i.backend, filePath, i.logger)
}
