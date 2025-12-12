//
// Copyright 2025 The Chainloop Authors.
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
	"github.com/chainloop-dev/chainloop/internal/prinfo"
	"github.com/chainloop-dev/chainloop/internal/schemavalidators"
	api "github.com/chainloop-dev/chainloop/pkg/attestation/crafter/api/attestation/v1"
	"github.com/chainloop-dev/chainloop/pkg/casclient"

	"github.com/rs/zerolog"
)

type ChainloopPRInfoCrafter struct {
	*crafterCommon
	backend *casclient.CASBackend
}

// NewChainloopPRInfoCrafter generates a new CHAINLOOP_PR_INFO material.
// This material type contains Pull Request or Merge Request metadata
// collected automatically during attestation in a PR/MR context.
func NewChainloopPRInfoCrafter(schema *schemaapi.CraftingSchema_Material, backend *casclient.CASBackend, l *zerolog.Logger) (*ChainloopPRInfoCrafter, error) {
	if schema.Type != schemaapi.CraftingSchema_Material_CHAINLOOP_PR_INFO {
		return nil, fmt.Errorf("material type is not chainloop_pr_info")
	}

	craftCommon := &crafterCommon{logger: l, input: schema}
	return &ChainloopPRInfoCrafter{backend: backend, crafterCommon: craftCommon}, nil
}

// Craft will validate the PR info against the JSON schema, calculate the digest of the artifact,
// upload it and return the material definition.
func (i *ChainloopPRInfoCrafter) Craft(ctx context.Context, artifactPath string) (*api.Attestation_Material, error) {
	// Read the file
	f, err := os.ReadFile(artifactPath)
	if err != nil {
		return nil, fmt.Errorf("can't open the file: %w", err)
	}

	// Unmarshal into typed structure first
	var v prinfo.Evidence
	if err := json.Unmarshal(f, &v); err != nil {
		i.logger.Debug().Err(err).Msg("error decoding file")
		return nil, fmt.Errorf("invalid JSON format: %w", err)
	}

	// Marshal the data field to validate it
	dataBytes, err := json.Marshal(v.Data)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal data for validation: %w", err)
	}

	var rawData interface{}
	if err := json.Unmarshal(dataBytes, &rawData); err != nil {
		return nil, fmt.Errorf("failed to unmarshal data for validation: %w", err)
	}

	// Validate the data against JSON schema
	if err := schemavalidators.ValidatePRInfo(rawData, schemavalidators.PRInfoVersion1_0); err != nil {
		i.logger.Debug().Err(err).Msg("schema validation failed")
		return nil, fmt.Errorf("PR info validation failed: %w", err)
	}

	// Upload the artifact
	material, err := uploadAndCraft(ctx, i.input, i.backend, artifactPath, i.logger)
	if err != nil {
		return nil, err
	}

	return material, nil
}
