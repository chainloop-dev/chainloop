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
	"github.com/chainloop-dev/chainloop/internal/aiagentconfig"
	"github.com/chainloop-dev/chainloop/internal/schemavalidators"
	api "github.com/chainloop-dev/chainloop/pkg/attestation/crafter/api/attestation/v1"
	"github.com/chainloop-dev/chainloop/pkg/casclient"

	"github.com/rs/zerolog"
)

var annotationAIAgentName = api.CreateAnnotation("material.aiagent.name")

type ChainloopAIAgentConfigCrafter struct {
	*crafterCommon
	backend *casclient.CASBackend
}

// NewChainloopAIAgentConfigCrafter generates a new CHAINLOOP_AI_AGENT_CONFIG material.
// This material type contains AI agent configuration data collected during attestation.
func NewChainloopAIAgentConfigCrafter(schema *schemaapi.CraftingSchema_Material, backend *casclient.CASBackend, l *zerolog.Logger) (*ChainloopAIAgentConfigCrafter, error) {
	if schema.Type != schemaapi.CraftingSchema_Material_CHAINLOOP_AI_AGENT_CONFIG {
		return nil, fmt.Errorf("material type is not chainloop_ai_agent_config")
	}

	craftCommon := &crafterCommon{logger: l, input: schema}
	return &ChainloopAIAgentConfigCrafter{backend: backend, crafterCommon: craftCommon}, nil
}

// Craft validates the AI agent config against the JSON schema, calculates the digest,
// uploads it and returns the material definition.
func (c *ChainloopAIAgentConfigCrafter) Craft(ctx context.Context, artifactPath string) (*api.Attestation_Material, error) {
	f, err := os.ReadFile(artifactPath)
	if err != nil {
		return nil, fmt.Errorf("can't open the file: %w", err)
	}

	// Unmarshal envelope, keeping data as raw JSON for schema validation
	var envelope struct {
		Schema string          `json:"schema"`
		Data   json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(f, &envelope); err != nil {
		c.logger.Debug().Err(err).Msg("error decoding file")
		return nil, fmt.Errorf("invalid JSON format: %w", err)
	}

	// Unmarshal data into typed struct for agent name extraction
	var data aiagentconfig.Data
	if err := json.Unmarshal(envelope.Data, &data); err != nil {
		c.logger.Debug().Err(err).Msg("error decoding data field")
		return nil, fmt.Errorf("failed to unmarshal data: %w", err)
	}

	// Validate using raw JSON to preserve unknown fields for strict schema validation
	var rawData any
	if err := json.Unmarshal(envelope.Data, &rawData); err != nil {
		return nil, fmt.Errorf("failed to unmarshal data for validation: %w", err)
	}

	if err := schemavalidators.ValidateAIAgentConfig(rawData, schemavalidators.AIAgentConfigVersion0_1); err != nil {
		c.logger.Debug().Err(err).Msg("schema validation failed")
		return nil, fmt.Errorf("AI agent config validation failed: %w", err)
	}

	material, err := uploadAndCraft(ctx, c.input, c.backend, artifactPath, c.logger)
	if err != nil {
		return nil, err
	}

	// Surface schema as an annotation
	if envelope.Schema != "" {
		material.Annotations[annotationEvidenceSchema] = envelope.Schema
	}

	// Surface agent name as an annotation
	if data.Agent.Name != "" {
		material.Annotations[annotationAIAgentName] = data.Agent.Name
	}

	return material, nil
}
