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
	"strings"

	schemaapi "github.com/chainloop-dev/chainloop/app/controlplane/api/workflowcontract/v1"
	"github.com/chainloop-dev/chainloop/internal/redaction"
	"github.com/chainloop-dev/chainloop/internal/schemavalidators"
	api "github.com/chainloop-dev/chainloop/pkg/attestation/crafter/api/attestation/v1"
	"github.com/chainloop-dev/chainloop/pkg/attestation/crafter/materials/aicodingsession"
	"github.com/chainloop-dev/chainloop/pkg/casclient"

	"github.com/rs/zerolog"
)

var annotationAICodingModel = api.CreateAnnotation("material.aiagent.model")

type ChainloopAICodingSessionCrafter struct {
	*crafterCommon
	backend       *casclient.CASBackend
	skipRedaction bool
}

// AICodingSessionCraftOpt tunes how an AI coding session is crafted.
type AICodingSessionCraftOpt func(*ChainloopAICodingSessionCrafter)

// WithAICodingSessionSkipRedaction uploads the session exactly as captured,
// without stripping secrets out of it first.
func WithAICodingSessionSkipRedaction(skip bool) AICodingSessionCraftOpt {
	return func(c *ChainloopAICodingSessionCrafter) { c.skipRedaction = skip }
}

// NewChainloopAICodingSessionCrafter generates a new CHAINLOOP_AI_CODING_SESSION material.
// This material type contains AI coding session telemetry collected during attestation.
func NewChainloopAICodingSessionCrafter(schema *schemaapi.CraftingSchema_Material, backend *casclient.CASBackend, l *zerolog.Logger, opts ...AICodingSessionCraftOpt) (*ChainloopAICodingSessionCrafter, error) {
	if schema.Type != schemaapi.CraftingSchema_Material_CHAINLOOP_AI_CODING_SESSION {
		return nil, fmt.Errorf("material type is not chainloop_ai_coding_session")
	}

	craftCommon := &crafterCommon{logger: l, input: schema}
	c := &ChainloopAICodingSessionCrafter{backend: backend, crafterCommon: craftCommon}
	for _, o := range opts {
		o(c)
	}
	return c, nil
}

// Craft validates the AI coding session against the JSON schema, redacts any
// secrets found in it, calculates the digest, uploads it and returns the
// material definition.
//
// The file on disk is left untouched, so it is no longer the stored content once
// anything was redacted. The sanitized copy is returned as CraftResult.Content
// for whoever needs to read the artifact back — today policy evaluation, which
// must not be handed the credentials the session captured.
func (c *ChainloopAICodingSessionCrafter) Craft(ctx context.Context, artifactPath string) (*CraftResult, error) {
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

	redacted, report, err := c.redact(ctx, f)
	if err != nil {
		return nil, err
	}

	// Substituting the stored content only when something was actually replaced
	// keeps a clean session's digest reproducible from its source file.
	var craftOpts []uploadAndCraftOption
	if redacted != nil {
		craftOpts = append(craftOpts, withContentOverride(redacted))
	}

	material, err := uploadAndCraft(ctx, c.input, c.backend, artifactPath, c.logger, craftOpts...)
	if err != nil {
		return nil, err
	}

	c.annotateRedaction(material, report)

	// Surface agent name as an annotation
	if data.Agent.Name != "" {
		material.Annotations[annotationAIAgentName] = data.Agent.Name
	}

	// Surface primary model as an annotation
	if data.Model != nil && data.Model.Primary != "" {
		material.Annotations[annotationAICodingModel] = data.Model.Primary
	}

	return &CraftResult{Material: material, Content: redacted}, nil
}

// redact strips secrets out of the session content, returning the sanitized copy
// to store in place of the file on disk, or nil when nothing was replaced and the
// file itself is what gets stored.
//
// Redaction fails closed: if the content cannot be scanned or the result no
// longer matches the schema, the material is not crafted at all rather than
// uploaded unscanned. The operator's escape hatch is --skip-secret-redaction,
// whose use is recorded in the attestation.
func (c *ChainloopAICodingSessionCrafter) redact(ctx context.Context, content []byte) ([]byte, *redaction.Report, error) {
	if c.skipRedaction {
		c.logger.Warn().Msg("secret redaction is DISABLED: the AI coding session will be stored exactly as captured")
		return nil, nil, nil
	}

	redacted, report, err := aicodingsession.Redact(ctx, content)
	if err != nil {
		return nil, nil, fmt.Errorf("redacting secrets from the AI coding session: %w", err)
	}

	if !report.Changed() {
		return nil, report, nil
	}

	return redacted, report, nil
}

// annotateRedaction records what redaction did, so that it is visible in the
// attestation and actionable by policies rather than an invisible rewrite.
func (c *ChainloopAICodingSessionCrafter) annotateRedaction(material *api.Attestation_Material, report *redaction.Report) {
	if c.skipRedaction {
		material.Annotations[api.AnnotationMaterialRedactionSkipped] = api.AnnotationValueTrue
		return
	}

	if report == nil {
		return
	}

	if len(report.Unlocated) > 0 {
		// Detected in the document as a whole but not attributable to any
		// rewritable field: either a protected field or a match spanning the
		// boundary between two of them.
		c.logger.Warn().Interface("rules", report.Unlocated).
			Msg("some detected secrets could not be redacted")
	}

	if !report.Changed() {
		return
	}

	rules := report.RuleIDs()
	material.Annotations[api.AnnotationMaterialRedacted] = api.AnnotationValueTrue
	material.Annotations[api.AnnotationMaterialRedactionCount] = strconv.Itoa(report.Replacements)
	material.Annotations[api.AnnotationMaterialRedactionRules] = strings.Join(rules, ",")

	c.logger.Info().Int("count", report.Replacements).Strs("rules", rules).
		Msg("redacted secrets from the AI coding session before upload")
}
