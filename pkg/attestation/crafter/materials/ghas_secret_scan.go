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
	"fmt"
	"os"

	schemaapi "github.com/chainloop-dev/chainloop/app/controlplane/api/workflowcontract/v1"
	"github.com/chainloop-dev/chainloop/internal/casclient"
	api "github.com/chainloop-dev/chainloop/pkg/attestation/crafter/api/attestation/v1"
	"github.com/google/go-github/v66/github"
	"github.com/rs/zerolog"
)

type GHASSecretScanCrafter struct {
	*crafterCommon
	backend *casclient.CASBackend
}

func NewGHASSecretScanCrafter(materialSchema *schemaapi.CraftingSchema_Material, backend *casclient.CASBackend, l *zerolog.Logger) (*GHASSecretScanCrafter, error) {
	if materialSchema.Type != schemaapi.CraftingSchema_Material_GHAS_SECRET_SCAN {
		return nil, fmt.Errorf("material type is not GHAS Secret Scan file")
	}

	return &GHASSecretScanCrafter{
		backend:       backend,
		crafterCommon: &crafterCommon{logger: l, input: materialSchema},
	}, nil
}

// Craft will validate the CodeScan alerts report and craft the material
func (i *GHASSecretScanCrafter) Craft(ctx context.Context, filePath string) (*api.Attestation_Material, error) {
	var alerts []*github.SecretScanningAlert

	report, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("can't open the file: %w", err)
	}
	err = json.Unmarshal(report, &alerts)
	if err != nil {
		i.logger.Debug().Err(err).Msg("error decoding file")
		return nil, fmt.Errorf("invalid GHAS secret scan file: %w", ErrInvalidMaterialType)
	}

	// if list is empty. It's ambiguous, but we accept it
	if len(alerts) == 0 {
		i.logger.Debug().Err(err).Msg("Accepting an empty report. Make sure it's a valid GHAS Secret Scan report")
	} else {
		alert := alerts[0]
		// All secret scan alerts have a secret type. If this doesn't have it, it might be a different scan (code, dependencies ...)
		// check https://docs.github.com/en/code-security/secret-scanning/introduction/supported-secret-scanning-patterns#supported-secrets for the different values
		if alert.SecretType == nil {
			return nil, fmt.Errorf("secret type field not found in GHAS secret scan: %w", ErrInvalidMaterialType)
		}
	}

	// Call uploadAndCraft with the path of the JSON report file
	return uploadAndCraft(ctx, i.input, i.backend, filePath, i.logger)
}
