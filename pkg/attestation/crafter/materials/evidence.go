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

const (
	// Annotations for evidence metadata that will be extracted if the evidence is in JSON format
	annotationEvidenceID     = "chainloop.material.evidence.id"
	annotationEvidenceSchema = "chainloop.material.evidence.schema"
)

type EvidenceCrafter struct {
	*crafterCommon
	backend *casclient.CASBackend
}

// customEvidence represents the expected structure of a custom Evidence JSON file
type customEvidence struct {
	// ID is a unique identifier for the evidence
	// Deprecated: in favor of ChainloopID
	ID          string `json:"id"`
	ChainloopID string `json:"chainloop.material.evidence.id"`
	// Schema is an optional schema reference for the evidence validation
	Schema string `json:"schema"`
	// Data contains the actual evidence content
	Data json.RawMessage `json:"data"`
}

// NewEvidenceCrafter generates a new Evidence material.
// Pieces of evidences represent generic, additional context that don't fit
// into one of the well known material types. For example, a custom approval report (in json), ...
func NewEvidenceCrafter(schema *schemaapi.CraftingSchema_Material, backend *casclient.CASBackend, l *zerolog.Logger) (*EvidenceCrafter, error) {
	if schema.Type != schemaapi.CraftingSchema_Material_EVIDENCE {
		return nil, fmt.Errorf("material type is not evidence")
	}

	craftCommon := &crafterCommon{logger: l, input: schema}
	return &EvidenceCrafter{backend: backend, crafterCommon: craftCommon}, nil
}

// Craft will calculate the digest of the artifact, simulate an upload and return the material definition
// If the evidence is in JSON format with id, data (and optionally schema) fields,
// it will extract those as annotations
func (i *EvidenceCrafter) Craft(ctx context.Context, artifactPath string) (*api.Attestation_Material, error) {
	material, err := uploadAndCraft(ctx, i.input, i.backend, artifactPath, i.logger)
	if err != nil {
		return nil, err
	}

	// Try to parse as JSON and extract annotations
	i.tryExtractAnnotations(material, artifactPath)

	return material, nil
}

// tryExtractAnnotations attempts to parse the evidence as JSON and extract id/schema fields as annotations
func (i *EvidenceCrafter) tryExtractAnnotations(m *api.Attestation_Material, artifactPath string) {
	// Read the file content
	content, err := os.ReadFile(artifactPath)
	if err != nil {
		i.logger.Debug().Err(err).Msg("failed to read evidence file for annotation extraction")
		return
	}

	// Try to parse as JSON
	var evidence customEvidence

	if err := json.Unmarshal(content, &evidence); err != nil {
		i.logger.Debug().Err(err).Msg("evidence is not valid JSON, skipping annotation extraction")
		return
	}

	chainloopID := evidence.ChainloopID
	// fallback to deprecated id field
	if chainloopID == "" {
		chainloopID = evidence.ID
	}

	// Check if it has the required structure (id and data fields)
	if chainloopID == "" || len(evidence.Data) == 0 {
		i.logger.Debug().Msg("evidence JSON does not have required id and data fields, skipping annotation extraction")
		return
	}

	// Initialize annotations map if needed
	if m.Annotations == nil {
		m.Annotations = make(map[string]string)
	}

	// Extract id and schema as annotations
	m.Annotations[annotationEvidenceID] = chainloopID
	if evidence.Schema != "" {
		m.Annotations[annotationEvidenceSchema] = evidence.Schema
	}

	i.logger.Debug().Str("id", evidence.ID).Str("schema", evidence.Schema).Msg("extracted evidence annotations")
}
