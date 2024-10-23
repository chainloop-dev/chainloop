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

type GHASDependencyScanCrafter struct {
	*crafterCommon
	backend *casclient.CASBackend
}

func NewGHASDependencyScanCrafter(materialSchema *schemaapi.CraftingSchema_Material, backend *casclient.CASBackend, l *zerolog.Logger) (*GHASDependencyScanCrafter, error) {
	if materialSchema.Type != schemaapi.CraftingSchema_Material_GHAS_DEPENDENCY_SCAN {
		return nil, fmt.Errorf("material type is not GHAS Dependency Scan file")
	}

	return &GHASDependencyScanCrafter{
		backend:       backend,
		crafterCommon: &crafterCommon{logger: l, input: materialSchema},
	}, nil
}

// Craft will validate the CodeScan alerts report and craft the material
func (i *GHASDependencyScanCrafter) Craft(ctx context.Context, filePath string) (*api.Attestation_Material, error) {
	var alerts []*github.DependabotAlert

	report, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("can't open the file: %w", err)
	}
	err = json.Unmarshal(report, &alerts)
	if err != nil {
		i.logger.Debug().Err(err).Msg("error decoding file")
		return nil, fmt.Errorf("invalid GHAS dependency scan file: %w", ErrInvalidMaterialType)
	}

	// if list is empty. It's ambiguous, but we accept it
	if len(alerts) == 0 {
		i.logger.Debug().Err(err).Msg("Accepting an empty report. Make sure it's a valid GHAS Dependency Scan report")
	} else {
		alert := alerts[0]
		// All dependabot scan alerts have a dependency. If this doesn't have it, it might be a different scan (secrets, code scan ...)
		if alert.Dependency == nil {
			return nil, fmt.Errorf("tool field not found in GHAS dependency scan: %w", ErrInvalidMaterialType)
		}
	}

	// Call uploadAndCraft with the path of the JSON report file
	return uploadAndCraft(ctx, i.input, i.backend, filePath, i.logger)
}
