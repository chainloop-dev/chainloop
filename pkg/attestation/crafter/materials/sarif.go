//
// Copyright 2023-2026 The Chainloop Authors.
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
	"maps"
	"slices"
	"strings"

	schemaapi "github.com/chainloop-dev/chainloop/app/controlplane/api/workflowcontract/v1"
	api "github.com/chainloop-dev/chainloop/pkg/attestation/crafter/api/attestation/v1"
	"github.com/chainloop-dev/chainloop/pkg/casclient"
	sarif "github.com/owenrumney/go-sarif/v3/pkg/report/v210/sarif"
	"github.com/rs/zerolog"
)

// checkmarxVendorTag is the lower-cased vendor marker Checkmarx One stamps on
// every SARIF rule (properties.tags == ["security","checkmarx","<engine>"]) and
// embeds in its driver name ("Checkmarx One"). Either signal identifies a
// Checkmarx SARIF export.
const checkmarxVendorTag = "checkmarx"

type SARIFCrafter struct {
	backend *casclient.CASBackend
	*crafterCommon
}

func NewSARIFCrafter(materialSchema *schemaapi.CraftingSchema_Material, backend *casclient.CASBackend, l *zerolog.Logger) (*SARIFCrafter, error) {
	if materialSchema.Type != schemaapi.CraftingSchema_Material_SARIF {
		return nil, fmt.Errorf("material type is not SARIF format")
	}

	return &SARIFCrafter{
		backend:       backend,
		crafterCommon: &crafterCommon{logger: l, input: materialSchema},
	}, nil
}

func (i *SARIFCrafter) Craft(ctx context.Context, filepath string) (*api.Attestation_Material, error) {
	i.logger.Debug().Str("path", filepath).Msg("decoding SARIF file")

	// sarif.Open will take care of checkif if the file exists or not and unmarshal it, we just need to check if the schema is present to validate that it's a valid SARIF file
	doc, err := sarif.Open(filepath)
	if err != nil || doc.Schema == "" {
		if err != nil {
			i.logger.Debug().Err(err).Msg("error decoding file")
		}

		return nil, fmt.Errorf("invalid SARIF file (%w): %w", err, ErrInvalidMaterialType)
	}

	m, err := uploadAndCraft(ctx, i.input, i.backend, filepath, i.logger)
	if err != nil {
		return nil, err
	}

	i.injectAnnotations(m, doc)

	return m, nil
}

func (i *SARIFCrafter) injectAnnotations(m *api.Attestation_Material, doc *sarif.Report) {
	if len(doc.Runs) == 0 {
		return
	}

	run := doc.Runs[0]
	if run == nil || run.Tool == nil || run.Tool.Driver == nil {
		return
	}

	m.Annotations = make(map[string]string)
	driver := run.Tool.Driver

	if driver.Name != nil && *driver.Name != "" {
		m.Annotations[AnnotationToolNameKey] = *driver.Name
	}
	if driver.Version != nil && *driver.Version != "" {
		m.Annotations[AnnotationToolVersionKey] = *driver.Version
	}

	// Checkmarx One exports every engine (sast, sca, kics, containers, sscs) under
	// a single driver, so the driver name alone cannot tell attestation-level
	// policies which analyses actually ran. Record the distinct engine types on the
	// shared scan.types annotation, mirroring the native CHECKMARX_JSON crafter, so
	// those policies (e.g. *-scan-present) match uniformly across material kinds.
	if scanTypes := i.checkmarxScanTypes(doc); scanTypes != "" {
		m.Annotations[AnnotationScanTypesKey] = scanTypes
	}
}

// checkmarxScanTypes inspects a SARIF report and returns the distinct scan types
// produced by its Checkmarx runs, normalized onto the canonical scan-type
// vocabulary and formatted for the AnnotationScanTypesKey annotation (sorted,
// comma-joined; e.g. "iac,sast,sca"). It returns "" when no Checkmarx run is
// present or none of its engines can be classified, so recognition fails closed
// and never over-claims for other tools.
//
// Detection and extraction are both per run: a SARIF document may bundle several
// runs (e.g. an aggregated report mixing tools), and only a Checkmarx run's
// "(<engine>)" suffixes use the vocabulary we normalize. Gating extraction on the
// individual run keeps another tool's rule ids from being attributed to Checkmarx.
//
// The engine is read from each finding's ruleId, not the driver's rule catalog:
// Checkmarx's SARIF export carries no dedicated engine field (the EngineID
// property only exists in its sonar export), but ast-cli appends a "(<engine>)"
// suffix to every result ruleId (e.g. "Reflected_XSS (sast)"), verified against
// ast-cli's findRuleID. Reading findings rather than tool.driver.rules (a catalog
// that need not correspond to findings) keeps the annotation findings-based,
// consistent with the native CHECKMARX_JSON crafter.
func (i *SARIFCrafter) checkmarxScanTypes(doc *sarif.Report) string {
	scanTypes := map[string]struct{}{}
	for _, run := range doc.Runs {
		if !isCheckmarxRun(run) {
			continue
		}
		for _, result := range run.Results {
			if result == nil || result.RuleID == nil {
				continue
			}
			engine := ruleIDEngineSuffix(*result.RuleID)
			if engine == "" {
				continue
			}
			scanType, ok := checkmarxEngineToScanType[strings.ToLower(engine)]
			if !ok {
				// Fail closed: an engine we cannot classify is dropped so no
				// vendor-specific value leaks into the annotation.
				i.logger.Debug().Str("engine", engine).Msg("unrecognized Checkmarx engine type, omitting from scan.types annotation")
				continue
			}
			scanTypes[scanType] = struct{}{}
		}
	}

	if len(scanTypes) == 0 {
		return ""
	}
	return strings.Join(slices.Sorted(maps.Keys(scanTypes)), ",")
}

// isCheckmarxRun reports whether a single SARIF run looks like a Checkmarx One
// export. Checkmarx stamps its driver name ("Checkmarx One") and tags every rule
// with "checkmarx"; either signal is enough.
func isCheckmarxRun(run *sarif.Run) bool {
	if run == nil || run.Tool == nil || run.Tool.Driver == nil {
		return false
	}
	driver := run.Tool.Driver
	if driver.Name != nil && strings.Contains(strings.ToLower(*driver.Name), checkmarxVendorTag) {
		return true
	}
	for _, rule := range driver.Rules {
		if rule == nil || rule.Properties == nil {
			continue
		}
		for _, tag := range rule.Properties.Tags {
			if strings.ToLower(tag) == checkmarxVendorTag {
				return true
			}
		}
	}
	return false
}

// ruleIDEngineSuffix extracts the engine identifier from the trailing
// "(<engine>)" that ast-cli appends to every SARIF rule id (e.g.
// "Reflected_XSS (sast)" -> "sast"). It returns "" when no such suffix is present.
func ruleIDEngineSuffix(id string) string {
	id = strings.TrimSpace(id)
	if !strings.HasSuffix(id, ")") {
		return ""
	}
	open := strings.LastIndex(id, "(")
	if open < 0 {
		return ""
	}
	return strings.TrimSpace(id[open+1 : len(id)-1])
}
