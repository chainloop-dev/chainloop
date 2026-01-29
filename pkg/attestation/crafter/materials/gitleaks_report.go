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

	schemaapi "github.com/chainloop-dev/chainloop/app/controlplane/api/workflowcontract/v1"
	api "github.com/chainloop-dev/chainloop/pkg/attestation/crafter/api/attestation/v1"
	"github.com/chainloop-dev/chainloop/pkg/casclient"
	"github.com/rs/zerolog"
)

type GitleaksReportCrafter struct {
	*crafterCommon
	backend *casclient.CASBackend
}

// GitleaksFinding represents a single finding in the gitleaks JSON report
type GitleaksFinding struct {
	RuleID      string `json:"RuleID"`
	Description string `json:"Description"`
	StartLine   int    `json:"StartLine"`
	EndLine     int    `json:"EndLine"`
	File        string `json:"File"`
	Secret      string `json:"Secret"`
	Match       string `json:"Match"`
}

func NewGitleaksReportCrafter(schema *schemaapi.CraftingSchema_Material, backend *casclient.CASBackend, l *zerolog.Logger) (*GitleaksReportCrafter, error) {
	if schema.Type != schemaapi.CraftingSchema_Material_GITLEAKS_REPORT {
		return nil, fmt.Errorf("material type is not a Gitleaks Report")
	}
	craftCommon := &crafterCommon{logger: l, input: schema}
	return &GitleaksReportCrafter{backend: backend, crafterCommon: craftCommon}, nil
}

func (i *GitleaksReportCrafter) Craft(ctx context.Context, filePath string) (*api.Attestation_Material, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("can't open the file: %w", err)
	}

	var findings []GitleaksFinding
	if err = json.Unmarshal(data, &findings); err != nil {
		return nil, fmt.Errorf("error unmarshalling gitleaks report: %w", err)
	}

	// Validate structure - must be an array
	// Empty array is valid (clean scan with no secrets found)
	if findings == nil {
		return nil, fmt.Errorf("invalid gitleaks report: expected JSON array")
	}

	// If there are findings, validate that at least one has the required fields
	if len(findings) > 0 {
		hasValidStructure := false
		for _, finding := range findings {
			if finding.RuleID != "" && finding.File != "" {
				hasValidStructure = true
				break
			}
		}
		if !hasValidStructure {
			return nil, fmt.Errorf("invalid gitleaks report: missing required fields (RuleID, File)")
		}
	}

	m, err := uploadAndCraft(ctx, i.input, i.backend, filePath, i.logger)
	if err != nil {
		return nil, err
	}

	i.injectAnnotations(m)

	return m, nil
}

func (i *GitleaksReportCrafter) injectAnnotations(m *api.Attestation_Material) {
	// Gitleaks doesn't include version information in the JSON output
	// Set the tool name annotation
	if m.Annotations == nil {
		m.Annotations = make(map[string]string)
	}
	m.Annotations[AnnotationToolNameKey] = "Gitleaks"
}
