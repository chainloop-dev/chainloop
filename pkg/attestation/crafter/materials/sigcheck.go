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
	"github.com/chainloop-dev/chainloop/pkg/attestation/crafter/materials/sigcheck"
	"github.com/chainloop-dev/chainloop/pkg/casclient"
	"github.com/rs/zerolog"
)

type SigcheckCrafter struct {
	*crafterCommon
	backend *casclient.CASBackend
}

func NewSigcheckCrafter(schema *schemaapi.CraftingSchema_Material, backend *casclient.CASBackend, l *zerolog.Logger) (*SigcheckCrafter, error) {
	if schema.Type != schemaapi.CraftingSchema_Material_SYSINTERNALS_SIGCHECK {
		return nil, fmt.Errorf("material type is not a sigcheck report")
	}
	craftCommon := &crafterCommon{logger: l, input: schema}
	return &SigcheckCrafter{backend: backend, crafterCommon: craftCommon}, nil
}

func (i *SigcheckCrafter) Craft(ctx context.Context, filePath string) (*api.Attestation_Material, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("can't open the file: %w", err)
	}

	report, err := sigcheck.Parse(data)
	if err != nil {
		return nil, fmt.Errorf("invalid sigcheck report: %w", ErrInvalidMaterialType)
	}

	// Structural fingerprint: sigcheck always emits at least Path and Verified
	// columns. This is heuristic but rejects unrelated CSV/JSON files.
	if !report.HasColumns("Path", "Verified") {
		return nil, fmt.Errorf("missing required sigcheck columns (Path, Verified): %w", ErrInvalidMaterialType)
	}

	// A header-only report means a clean scan with no files listed. Accept it.
	if len(report.Rows) == 0 {
		i.logger.Debug().Msg("Accepting an empty sigcheck report.")
	}

	m, err := uploadAndCraft(ctx, i.input, i.backend, filePath, i.logger)
	if err != nil {
		return nil, err
	}

	i.injectAnnotations(m)

	return m, nil
}

func (i *SigcheckCrafter) injectAnnotations(m *api.Attestation_Material) {
	if m.Annotations == nil {
		m.Annotations = make(map[string]string)
	}
	m.Annotations[AnnotationToolNameKey] = "sigcheck"
}
