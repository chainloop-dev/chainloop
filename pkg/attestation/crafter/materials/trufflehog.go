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
	"fmt"
	"os"

	schemaapi "github.com/chainloop-dev/chainloop/app/controlplane/api/workflowcontract/v1"
	api "github.com/chainloop-dev/chainloop/pkg/attestation/crafter/api/attestation/v1"
	"github.com/chainloop-dev/chainloop/pkg/attestation/crafter/materials/trufflehog"
	"github.com/chainloop-dev/chainloop/pkg/casclient"
	"github.com/rs/zerolog"
)

const trufflehogToolName = "trufflehog"

// TrufflehogCrafter crafts a TRUFFLEHOG_JSON material out of TruffleHog's
// --json output, which is JSONL (one JSON finding per line).
type TrufflehogCrafter struct {
	*crafterCommon
	backend *casclient.CASBackend
}

func NewTrufflehogCrafter(schema *schemaapi.CraftingSchema_Material, backend *casclient.CASBackend, l *zerolog.Logger) (*TrufflehogCrafter, error) {
	if schema.Type != schemaapi.CraftingSchema_Material_TRUFFLEHOG_JSON {
		return nil, fmt.Errorf("material type is not a TruffleHog report")
	}
	return &TrufflehogCrafter{
		backend:       backend,
		crafterCommon: &crafterCommon{logger: l, input: schema},
	}, nil
}

func (i *TrufflehogCrafter) Craft(ctx context.Context, filePath string) (*api.Attestation_Material, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("can't open the file: %w", err)
	}
	defer f.Close()

	findings, err := trufflehog.Parse(f)
	if err != nil {
		return nil, fmt.Errorf("invalid trufflehog report file: %w", ErrInvalidMaterialType)
	}

	// A clean scan (no secrets found) is a valid, meaningful result. TruffleHog
	// emits nothing in that case, leaving a zero-byte file. Rather than reject
	// it (or store an empty artifact, whose universal empty-file digest would
	// collide with unrelated empty materials), craft a canonical empty findings
	// report, "[]", so a passing scan is attestable and projects to an empty
	// findings list for policy evaluation.
	var craftOpts []uploadAndCraftOption
	if len(findings) == 0 {
		i.logger.Debug().Msg("Accepting an empty report (no secrets found).")
		craftOpts = append(craftOpts, withEmptyContentFallback(trufflehog.CanonicalEmpty))
	} else {
		finding := findings[0]
		// TruffleHog findings always carry a DetectorName. If the first parsed
		// line lacks it, this is valid JSONL of some other kind, not a report.
		if finding.DetectorName == "" {
			return nil, fmt.Errorf("'DetectorName' field not found in trufflehog report: %w", ErrInvalidMaterialType)
		}
	}

	m, err := uploadAndCraft(ctx, i.input, i.backend, filePath, i.logger, craftOpts...)
	if err != nil {
		return nil, err
	}

	if m.Annotations == nil {
		m.Annotations = make(map[string]string)
	}
	m.Annotations[AnnotationToolNameKey] = trufflehogToolName

	return m, nil
}
