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
	"strconv"

	schemaapi "github.com/chainloop-dev/chainloop/app/controlplane/api/workflowcontract/v1"
	"github.com/chainloop-dev/chainloop/internal/schemavalidators"
	api "github.com/chainloop-dev/chainloop/pkg/attestation/crafter/api/attestation/v1"
	"github.com/chainloop-dev/chainloop/pkg/attestation/crafter/materials/aisecuritycontext"
	"github.com/chainloop-dev/chainloop/pkg/casclient"

	"github.com/rs/zerolog"
)

var (
	annotationSecurityContextHeadSHA      = api.CreateAnnotation("material.securitycontext.head_sha")
	annotationSecurityContextToolVersion  = api.CreateAnnotation("material.securitycontext.tool_version")
	annotationSecurityContextFingerprints = api.CreateAnnotation("material.securitycontext.fingerprints")
	annotationSecurityContextReconciles   = api.CreateAnnotation("material.securitycontext.reconciles")
)

type ChainloopAISecurityContextCrafter struct {
	*crafterCommon
	backend *casclient.CASBackend
}

// NewChainloopAISecurityContextCrafter generates a new CHAINLOOP_AI_SECURITY_CONTEXT material.
// This material type contains a security context compiled from a repository's fix history.
func NewChainloopAISecurityContextCrafter(schema *schemaapi.CraftingSchema_Material, backend *casclient.CASBackend, l *zerolog.Logger) (*ChainloopAISecurityContextCrafter, error) {
	if schema.Type != schemaapi.CraftingSchema_Material_CHAINLOOP_AI_SECURITY_CONTEXT {
		return nil, fmt.Errorf("material type is not chainloop_ai_security_context")
	}

	craftCommon := &crafterCommon{logger: l, input: schema}
	return &ChainloopAISecurityContextCrafter{backend: backend, crafterCommon: craftCommon}, nil
}

// Craft validates the AI security context against the JSON schema, calculates the
// digest, uploads it and returns the material definition.
func (c *ChainloopAISecurityContextCrafter) Craft(ctx context.Context, artifactPath string) (*api.Attestation_Material, error) {
	f, err := os.ReadFile(artifactPath)
	if err != nil {
		return nil, fmt.Errorf("can't open the file: %w", err)
	}

	// Unmarshal the envelope, keeping data as raw JSON for schema validation
	var envelope struct {
		ID     string          `json:"chainloop.material.evidence.id"`
		Schema string          `json:"schema"`
		Data   json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(f, &envelope); err != nil {
		c.logger.Debug().Err(err).Msg("error decoding file")
		return nil, fmt.Errorf("invalid JSON format: %w", err)
	}

	// The envelope is asserted here rather than by the schema, which is
	// payload-rooted like every other internal schema. Checked before the data
	// decode so that a bare payload or an unrelated JSON document names the
	// actual problem instead of failing on empty input.
	if err := validateSecurityContextEnvelope(envelope.ID, envelope.Schema, envelope.Data); err != nil {
		c.logger.Debug().Err(err).Msg("evidence envelope validation failed")
		return nil, err
	}

	// Unmarshal data into a typed struct for annotation extraction
	var data aisecuritycontext.Data
	if err := json.Unmarshal(envelope.Data, &data); err != nil {
		c.logger.Debug().Err(err).Msg("error decoding data field")
		return nil, fmt.Errorf("failed to unmarshal data: %w", err)
	}

	// Validate using raw JSON to preserve unknown fields for strict schema validation
	var rawData any
	if err := json.Unmarshal(envelope.Data, &rawData); err != nil {
		return nil, fmt.Errorf("failed to unmarshal data for validation: %w", err)
	}

	if err := schemavalidators.ValidateSecurityContext(rawData, schemavalidators.AISecurityContextVersion0_1); err != nil {
		c.logger.Debug().Err(err).Msg("schema validation failed")
		return nil, fmt.Errorf("AI security context validation failed: %w", err)
	}

	material, err := uploadAndCraft(ctx, c.input, c.backend, artifactPath, c.logger)
	if err != nil {
		return nil, err
	}

	c.annotate(material, &data)

	return material, nil
}

// validateSecurityContextEnvelope checks the two constant envelope fields and
// that a payload is present at all.
func validateSecurityContextEnvelope(id, schema string, data json.RawMessage) error {
	if len(data) == 0 {
		return fmt.Errorf("no `data` field: a %s artifact must be wrapped in the Chainloop evidence "+
			"envelope {chainloop.material.evidence.id, schema, data}", aisecuritycontext.EvidenceID)
	}
	if id != aisecuritycontext.EvidenceID {
		return fmt.Errorf("evidence id is %q, want %q", id, aisecuritycontext.EvidenceID)
	}
	if schema != aisecuritycontext.EvidenceSchemaURL {
		return fmt.Errorf("schema is %q, want %q", schema, aisecuritycontext.EvidenceSchemaURL)
	}
	return nil
}

// annotate surfaces the fields a policy is most likely to gate on, so that it
// does not have to download the payload from CAS to read them.
func (c *ChainloopAISecurityContextCrafter) annotate(material *api.Attestation_Material, data *aisecuritycontext.Data) {
	if data.Repo.HeadSHA != "" {
		material.Annotations[annotationSecurityContextHeadSHA] = data.Repo.HeadSHA
	}

	if v := data.Provenance.ToolVersion; v != "" {
		material.Annotations[annotationSecurityContextToolVersion] = v
		if v == "dev" {
			// A development build carries no version, so the context cannot be
			// traced back to the code that produced it.
			c.logger.Warn().Msg("the AI security context was produced by a 'dev' build of the scanner")
		}
	}

	material.Annotations[annotationSecurityContextFingerprints] = strconv.Itoa(len(data.Fingerprints))

	// A funnel that does not reconcile means adjudications vanished unattributed:
	// the scan is incomplete and must not be read as a clean result. Published so
	// a policy can reject it without reading the payload.
	material.Annotations[annotationSecurityContextReconciles] = strconv.FormatBool(data.Scan.Reconciles)
}
