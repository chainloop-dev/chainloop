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
	"errors"
	"fmt"
	"strconv"

	schemaapi "github.com/chainloop-dev/chainloop/app/controlplane/api/workflowcontract/v1"
	api "github.com/chainloop-dev/chainloop/pkg/attestation/crafter/api/attestation/v1"
	"github.com/chainloop-dev/chainloop/pkg/attestation/crafter/materials/dranzer"
	"github.com/chainloop-dev/chainloop/pkg/casclient"
	"github.com/rs/zerolog"
)

// AnnotationDranzerReportsCount is the annotation holding the number of dranzer
// reports recorded in a CERTCC_DRANZER material: 1 for a single report, or the
// number of report entries found in a bundle.
const AnnotationDranzerReportsCount = "chainloop.material.dranzer.reports.count"

// DranzerCrafter stores the text report of the CERT/CC dranzer ActiveX/COM
// control tester as supply-chain evidence. The raw text is stored as-is; the
// text-to-JSON projection used by the policy engine happens later, at
// evaluation time.
//
// A single dranzer run produces one report per test mode (-b, -p, -s, -t), so the
// value may also be an archive holding several of them. The archive is recorded
// whole — it is the artifact the customer produced — and the projection
// aggregates its entries at evaluation time.
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

func (i *DranzerCrafter) Craft(ctx context.Context, filePath string) (*CraftResult, error) {
	// dranzer emits free-form text, so the fingerprint is soft: the input only has
	// to resemble dranzer output (a test-engine version banner, a parsed object or
	// finding, or the run-summary line). Inspect accepts a single report or an
	// archive of them, applying the same recognition predicate the policy-input
	// projection uses later.
	inspection, err := dranzer.Inspect(filePath)
	if err != nil {
		switch {
		case errors.Is(err, dranzer.ErrNoReports):
			return nil, fmt.Errorf("input does not look like dranzer output: %w", ErrInvalidMaterialType)
		case errors.Is(err, ErrTooManyEntries), errors.Is(err, ErrArchiveTooLarge):
			// The bundle limits are sized for one run's per-mode reports, so hitting
			// them usually means a whole output directory was archived. Say what to
			// provide instead rather than only reporting the limit.
			return nil, fmt.Errorf("%w: provide an archive holding just the dranzer reports of a single run", err)
		}
		return nil, err
	}

	// The value is stored unchanged — for a bundle that means the archive as the
	// customer produced it — and projected to JSON later for policy evaluation.
	m, err := uploadAndCraft(ctx, i.input, i.backend, filePath, i.logger)
	if err != nil {
		return nil, err
	}

	i.injectAnnotations(m, inspection)

	return craftResult(m, nil)
}

func (i *DranzerCrafter) injectAnnotations(m *api.Attestation_Material, inspection dranzer.Inspection) {
	if m.Annotations == nil {
		m.Annotations = make(map[string]string)
	}
	m.Annotations[AnnotationToolNameKey] = dranzer.ToolName
	if inspection.Version != "" {
		m.Annotations[AnnotationToolVersionKey] = inspection.Version
	}
	m.Annotations[AnnotationDranzerReportsCount] = strconv.Itoa(inspection.Reports)
}
