//
// Copyright 2026 The Chainloop Authors.
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
	"github.com/chainloop-dev/chainloop/internal/schemavalidators"
	api "github.com/chainloop-dev/chainloop/pkg/attestation/crafter/api/attestation/v1"
	"github.com/chainloop-dev/chainloop/pkg/attestation/crafter/materials/aicodingsession"
	"github.com/chainloop-dev/chainloop/pkg/casclient"

	"github.com/rs/zerolog"
)

var annotationAICodingModel = api.CreateAnnotation("material.aiagent.model")

type ChainloopAICodingSessionCrafter struct {
	*crafterCommon
	backend *casclient.CASBackend
}

// NewChainloopAICodingSessionCrafter generates a new CHAINLOOP_AI_CODING_SESSION material.
// This material type contains AI coding session telemetry collected during attestation.
func NewChainloopAICodingSessionCrafter(schema *schemaapi.CraftingSchema_Material, backend *casclient.CASBackend, l *zerolog.Logger) (*ChainloopAICodingSessionCrafter, error) {
	if schema.Type != schemaapi.CraftingSchema_Material_CHAINLOOP_AI_CODING_SESSION {
		return nil, fmt.Errorf("material type is not chainloop_ai_coding_session")
	}

	craftCommon := &crafterCommon{logger: l, input: schema}
	return &ChainloopAICodingSessionCrafter{backend: backend, crafterCommon: craftCommon}, nil
}

// Craft validates the AI coding session against the JSON schema, calculates the digest,
// uploads it and returns the material definition.
func (c *ChainloopAICodingSessionCrafter) Craft(ctx context.Context, artifactPath string) (*api.Attestation_Material, error) {
	f, err := os.ReadFile(artifactPath)
	if err != nil {
		return nil, fmt.Errorf("can't open the file: %w", err)
	}

	// Unmarshal envelope, keeping data as raw JSON for schema validation
	var envelope struct {
		Data json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(f, &envelope); err != nil {
		c.logger.Debug().Err(err).Msg("error decoding file")
		return nil, fmt.Errorf("invalid JSON format: %w", err)
	}

	// Unmarshal data into typed struct for annotation extraction
	var data aicodingsession.Data
	if err := json.Unmarshal(envelope.Data, &data); err != nil {
		c.logger.Debug().Err(err).Msg("error decoding data field")
		return nil, fmt.Errorf("failed to unmarshal data: %w", err)
	}

	// Validate using raw JSON to preserve unknown fields for strict schema validation
	var rawData any
	if err := json.Unmarshal(envelope.Data, &rawData); err != nil {
		return nil, fmt.Errorf("failed to unmarshal data for validation: %w", err)
	}

	if err := schemavalidators.ValidateAICodingSession(rawData, schemavalidators.AICodingSessionVersion0_1); err != nil {
		c.logger.Debug().Err(err).Msg("schema validation failed")
		return nil, fmt.Errorf("AI coding session validation failed: %w", err)
	}

	material, err := uploadAndCraft(ctx, c.input, c.backend, artifactPath, c.logger)
	if err != nil {
		return nil, err
	}

	// Surface agent name as an annotation
	if data.Agent.Name != "" {
		material.Annotations[annotationAIAgentName] = data.Agent.Name
	}

	// Surface primary model as an annotation
	if data.Model != nil && data.Model.Primary != "" {
		material.Annotations[annotationAICodingModel] = data.Model.Primary
	}

	return material, nil
}
