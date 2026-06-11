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
	"github.com/chainloop-dev/chainloop/pkg/attestation/crafter/materials/accesschk"
	"github.com/chainloop-dev/chainloop/pkg/casclient"
	"github.com/rs/zerolog"
)

// AccessChkCrafter stores the text output of the Sysinternals AccessChk tool as
// supply-chain evidence. The raw text is stored as-is; the text-to-JSON
// projection used by the policy engine happens later, at evaluation time.
type AccessChkCrafter struct {
	*crafterCommon
	backend *casclient.CASBackend
}

func NewAccessChkCrafter(schema *schemaapi.CraftingSchema_Material, backend *casclient.CASBackend, l *zerolog.Logger) (*AccessChkCrafter, error) {
	if schema.Type != schemaapi.CraftingSchema_Material_SYSINTERNALS_ACCESSCHK {
		return nil, fmt.Errorf("material type is not an accesschk output")
	}
	craftCommon := &crafterCommon{logger: l, input: schema}
	return &AccessChkCrafter{backend: backend, crafterCommon: craftCommon}, nil
}

func (i *AccessChkCrafter) Craft(ctx context.Context, filePath string) (*api.Attestation_Material, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("can't open the file: %w", err)
	}

	// Soft fingerprint: AccessChk emits free-form text, so we only require that
	// the input is valid text that resembles AccessChk output (a banner, at
	// least one access entry, or an SDDL/descriptor marker). The raw text is
	// stored unchanged; it is projected to JSON later for policy evaluation.
	report, err := accesschk.Parse(data)
	if err != nil {
		return nil, fmt.Errorf("invalid accesschk output: %w", ErrInvalidMaterialType)
	}

	if !report.LooksLikeAccessChk() {
		return nil, fmt.Errorf("input does not look like accesschk output: %w", ErrInvalidMaterialType)
	}

	m, err := uploadAndCraft(ctx, i.input, i.backend, filePath, i.logger)
	if err != nil {
		return nil, err
	}

	i.injectAnnotations(m, report)

	return m, nil
}

func (i *AccessChkCrafter) injectAnnotations(m *api.Attestation_Material, report *accesschk.Report) {
	if m.Annotations == nil {
		m.Annotations = make(map[string]string)
	}
	m.Annotations[AnnotationToolNameKey] = report.Tool.Name
	if report.Tool.Version != "" {
		m.Annotations[AnnotationToolVersionKey] = report.Tool.Version
	}
}
