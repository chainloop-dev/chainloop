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
	"github.com/chainloop-dev/chainloop/pkg/attestation/crafter/materials/dranzer"
	"github.com/chainloop-dev/chainloop/pkg/casclient"
	"github.com/rs/zerolog"
)

// DranzerCrafter stores the text report of the CERT/CC dranzer ActiveX/COM
// control tester as supply-chain evidence. The raw text is stored as-is; the
// text-to-JSON projection used by the policy engine happens later, at
// evaluation time.
type DranzerCrafter struct {
	*crafterCommon
	backend *casclient.CASBackend
}

func NewDranzerCrafter(schema *schemaapi.CraftingSchema_Material, backend *casclient.CASBackend, l *zerolog.Logger) (*DranzerCrafter, error) {
	if schema.Type != schemaapi.CraftingSchema_Material_CERTCC_DRANZER {
		return nil, fmt.Errorf("material type is not a dranzer report")
	}
	craftCommon := &crafterCommon{logger: l, input: schema}
	return &DranzerCrafter{backend: backend, crafterCommon: craftCommon}, nil
}

func (i *DranzerCrafter) Craft(ctx context.Context, filePath string) (*api.Attestation_Material, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("can't open the file: %w", err)
	}

	// Soft fingerprint: dranzer emits free-form text, so we only require that
	// the input is valid text that resembles dranzer output (a test-object
	// banner or the test-engine version line). The raw text is stored unchanged;
	// it is projected to JSON later for policy evaluation.
	report, err := dranzer.Parse(data)
	if err != nil {
		return nil, fmt.Errorf("invalid dranzer output: %w", ErrInvalidMaterialType)
	}

	if !report.LooksLikeDranzer() {
		return nil, fmt.Errorf("input does not look like dranzer output: %w", ErrInvalidMaterialType)
	}

	m, err := uploadAndCraft(ctx, i.input, i.backend, filePath, i.logger)
	if err != nil {
		return nil, err
	}

	i.injectAnnotations(m, report)

	return m, nil
}

func (i *DranzerCrafter) injectAnnotations(m *api.Attestation_Material, report *dranzer.Report) {
	if m.Annotations == nil {
		m.Annotations = make(map[string]string)
	}
	m.Annotations[AnnotationToolNameKey] = report.Tool.Name
	if report.Tool.Version != "" {
		m.Annotations[AnnotationToolVersionKey] = report.Tool.Version
	}
}
