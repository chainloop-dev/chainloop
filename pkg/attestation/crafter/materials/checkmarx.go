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
	"maps"
	"os"
	"slices"
	"strings"

	schemaapi "github.com/chainloop-dev/chainloop/app/controlplane/api/workflowcontract/v1"
	api "github.com/chainloop-dev/chainloop/pkg/attestation/crafter/api/attestation/v1"
	"github.com/chainloop-dev/chainloop/pkg/casclient"
	"github.com/rs/zerolog"
)

type CheckmarxCrafter struct {
	*crafterCommon
	backend *casclient.CASBackend
}

// checkmarxScanResults is the subset of the Checkmarx One native JSON report
// (the ast-cli ScanResultsCollection produced by `cx ... --report-format json`)
// used to validate its structure.
// https://github.com/Checkmarx/ast-cli/blob/main/internal/wrappers/results-json.go
//
// Results is kept as a json.RawMessage so we can tell an absent "results" key
// (look-alike JSON) apart from a present-but-null or empty one (a valid clean
// scan). encoding/json decodes both an absent key and an explicit null into a
// nil typed slice, so a typed field alone can't distinguish them. The ast-cli
// serializer marshals ScanResultsCollection with no omitempty, and several CLI
// code paths rebuild the slice as nil (marshalling to `"results": null`) when a
// scan or filter yields no findings.
type checkmarxScanResults struct {
	ScanID     string          `json:"scanID"`
	TotalCount int             `json:"totalCount"`
	Results    json.RawMessage `json:"results"`
}

// checkmarxResult is the subset of a single Checkmarx result entry we fingerprint.
// A native report bundles multiple engines (sast, sca, kics, containers, sscs)
// distinguished by Type; every real result carries a Type and a Data payload.
type checkmarxResult struct {
	Type         string          `json:"type"`
	SimilarityID string          `json:"similarityId"`
	Data         json.RawMessage `json:"data"`
}

// checkmarxEngineToScanType maps Checkmarx's raw engine identifiers onto the
// canonical scan-type vocabulary. Checkmarx names some engines by product
// (e.g. "kics") rather than by category, so we normalize them here to keep the
// scan.types annotation consistent with other material kinds. An engine absent
// from this map is dropped from the annotation (fail closed): recognition never
// fires on a type we cannot classify, and no vendor-specific name leaks out.
var checkmarxEngineToScanType = map[string]string{
	"sast":       ScanTypeSAST,
	"sca":        ScanTypeSCA,
	"kics":       ScanTypeIaC,
	"containers": ScanTypeContainer,
	"sscs":       ScanTypeSupplyChain,
}

func NewCheckmarxCrafter(schema *schemaapi.CraftingSchema_Material, backend *casclient.CASBackend, l *zerolog.Logger) (*CheckmarxCrafter, error) {
	if schema.Type != schemaapi.CraftingSchema_Material_CHECKMARX_JSON {
		return nil, fmt.Errorf("material type is not a Checkmarx native JSON report")
	}
	craftCommon := &crafterCommon{logger: l, input: schema}
	return &CheckmarxCrafter{backend: backend, crafterCommon: craftCommon}, nil
}

func (i *CheckmarxCrafter) Craft(ctx context.Context, filePath string) (*api.Attestation_Material, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("can't open the file: %w", err)
	}

	var report checkmarxScanResults
	if err = json.Unmarshal(data, &report); err != nil {
		return nil, fmt.Errorf("invalid Checkmarx native JSON report: %w", ErrInvalidMaterialType)
	}

	// Structural fingerprint check. A Checkmarx native report always carries a
	// scanID and a results key (an array that may be empty, or null, when nothing
	// was found). Auto-detection is intentionally disabled for this kind, but we
	// still reject look-alike JSON so an explicit --kind CHECKMARX_JSON fails
	// loudly on the wrong file. We anchor on fields that Checkmarx reliably
	// populates: the top-level scanID and the presence of the results key.
	if report.ScanID == "" || report.Results == nil {
		return nil, fmt.Errorf("missing required Checkmarx report fields: %w", ErrInvalidMaterialType)
	}

	// Parse the results array. A present-but-null ("null") or empty ("[]") array
	// is a valid clean scan (or one whose findings were all filtered out at the
	// CLI); both decode to a nil/empty slice without error.
	var results []checkmarxResult
	if err = json.Unmarshal(report.Results, &results); err != nil {
		return nil, fmt.Errorf("invalid Checkmarx results array: %w", ErrInvalidMaterialType)
	}

	// Each real result carries a type and a data payload, regardless of engine
	// (sast, sca, kics, containers, sscs). similarityId carries omitempty in the
	// ast-cli structs and is left as a supporting signal only, to avoid rejecting
	// valid reports. We also collect the distinct engine types so attestation-level
	// policies can tell which engines actually produced findings.
	typeSet := map[string]struct{}{}
	for idx, r := range results {
		if r.Type == "" || r.Data == nil {
			return nil, fmt.Errorf("checkmarx result %d is missing type or data: %w", idx, ErrInvalidMaterialType)
		}
		typeSet[strings.ToLower(r.Type)] = struct{}{}
	}

	if len(results) == 0 {
		i.logger.Debug().Msg("Accepting an empty report.")
	}

	// Call uploadAndCraft with the path of the JSON report file
	m, err := uploadAndCraft(ctx, i.input, i.backend, filePath, i.logger)
	if err != nil {
		return nil, err
	}

	i.injectAnnotations(m, typeSet)

	return m, nil
}

func (i *CheckmarxCrafter) injectAnnotations(m *api.Attestation_Material, typeSet map[string]struct{}) {
	if m.Annotations == nil {
		m.Annotations = make(map[string]string)
	}
	m.Annotations[AnnotationToolNameKey] = "checkmarx"

	// Normalize the raw Checkmarx engine identifiers onto the canonical scan-type
	// vocabulary, dropping any engine we cannot classify so no vendor-specific
	// name leaks into the annotation.
	scanTypes := map[string]struct{}{}
	for raw := range typeSet {
		scanType, ok := checkmarxEngineToScanType[raw]
		if !ok {
			i.logger.Debug().Str("engine", raw).Msg("unrecognized Checkmarx engine type, omitting from scan.types annotation")
			continue
		}
		scanTypes[scanType] = struct{}{}
	}

	// Advertise the distinct scan types found in the report (sorted, comma-joined;
	// e.g. "iac,sast,sca"). A clean/null report (or one with only unrecognized
	// engines) yields no types, so the annotation is omitted rather than set
	// empty: recognition then fails closed, the safe choice for a compliance gate.
	if len(scanTypes) > 0 {
		types := slices.Sorted(maps.Keys(scanTypes))
		m.Annotations[AnnotationScanTypesKey] = strings.Join(types, ",")
	}
}
