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
	"github.com/chainloop-dev/chainloop/pkg/casclient"
	"github.com/rs/zerolog"
)

type OSSFScorecardCrafter struct {
	backend            *casclient.CASBackend
	noStrictValidation bool
	*crafterCommon
}

type OSSFScorecardCraftOpt func(*OSSFScorecardCrafter)

func WithOSSFScorecardNoStrictValidation(noStrict bool) OSSFScorecardCraftOpt {
	return func(c *OSSFScorecardCrafter) {
		c.noStrictValidation = noStrict
	}
}

func NewOSSFScorecardCrafter(materialSchema *schemaapi.CraftingSchema_Material, backend *casclient.CASBackend, l *zerolog.Logger, opts ...OSSFScorecardCraftOpt) (*OSSFScorecardCrafter, error) {
	if materialSchema.Type != schemaapi.CraftingSchema_Material_OSSF_SCORECARD_JSON {
		return nil, fmt.Errorf("material type is not OSSF_SCORECARD_JSON")
	}

	c := &OSSFScorecardCrafter{
		backend:       backend,
		crafterCommon: &crafterCommon{logger: l, input: materialSchema},
	}

	for _, opt := range opts {
		opt(c)
	}

	return c, nil
}

func (i *OSSFScorecardCrafter) Craft(ctx context.Context, filepath string) (*api.Attestation_Material, error) {
	i.logger.Debug().Str("path", filepath).Msg("decoding OpenSSF Scorecard report")

	data, err := os.ReadFile(filepath)
	if err != nil {
		return nil, fmt.Errorf("can't open the file: %w", err)
	}

	var v interface{}
	if err := json.Unmarshal(data, &v); err != nil {
		i.logger.Debug().Err(err).Msg("error decoding file")
		return nil, fmt.Errorf("invalid OpenSSF Scorecard report file: %w", ErrInvalidMaterialType)
	}

	doc, ok := v.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid OpenSSF Scorecard report file: %w", ErrInvalidMaterialType)
	}

	if err := schemavalidators.ValidateOSSFScorecard(v, schemavalidators.ScorecardVersionV2); err != nil {
		if i.noStrictValidation {
			i.logger.Warn().Err(err).Msg("error validating OpenSSF Scorecard report, strict validation disabled, continuing")
		} else {
			i.logger.Debug().Err(err).Msg("error validating OpenSSF Scorecard report")
			i.logger.Info().Msg("you can disable strict validation to skip schema validation")
			return nil, fmt.Errorf("invalid OpenSSF Scorecard report file: %w", ErrInvalidMaterialType)
		}
	}

	m, err := uploadAndCraft(ctx, i.input, i.backend, filepath, i.logger)
	if err != nil {
		return nil, err
	}

	i.injectAnnotations(m, doc)

	return m, nil
}

func (i *OSSFScorecardCrafter) injectAnnotations(m *api.Attestation_Material, doc map[string]interface{}) {
	if m.Annotations == nil {
		m.Annotations = make(map[string]string)
	}

	m.Annotations[AnnotationToolNameKey] = "scorecard"

	if scorecard, ok := doc["scorecard"].(map[string]interface{}); ok {
		if version, ok := scorecard["version"].(string); ok && version != "" {
			m.Annotations[AnnotationToolVersionKey] = version
		}
	}

	if score, ok := doc["score"].(float64); ok {
		m.Annotations["chainloop.material.scorecard.score"] = strconv.FormatFloat(score, 'f', -1, 64)
	}
}
