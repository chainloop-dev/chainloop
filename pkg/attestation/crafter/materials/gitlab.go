//
// Copyright 2024-2025 The Chainloop Authors.
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
	"slices"

	schemaapi "github.com/chainloop-dev/chainloop/app/controlplane/api/workflowcontract/v1"
	api "github.com/chainloop-dev/chainloop/pkg/attestation/crafter/api/attestation/v1"
	"github.com/chainloop-dev/chainloop/pkg/casclient"
	"github.com/rs/zerolog"
	"gitlab.com/gitlab-org/security-products/analyzers/report/v5"
)

var supportedTypes = []string{"sast", "dast", "api_fuzzing", "coverage_fuzzing", "secret_detection", "dependency_scanning", "container_scanning", "container_scanning_for_registry", "cluster_image_scanning"}

type GitlabCrafter struct {
	*crafterCommon
	backend *casclient.CASBackend
}

func NewGitlabCrafter(schema *schemaapi.CraftingSchema_Material, backend *casclient.CASBackend, l *zerolog.Logger) (*GitlabCrafter, error) {
	if schema.Type != schemaapi.CraftingSchema_Material_GITLAB_SECURITY_REPORT {
		return nil, fmt.Errorf("material type is not a Gitlab Security Report")
	}
	craftCommon := &crafterCommon{logger: l, input: schema}
	return &GitlabCrafter{backend: backend, crafterCommon: craftCommon}, nil
}

func (i *GitlabCrafter) Craft(ctx context.Context, filePath string) (*api.Attestation_Material, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("can't open the file: %w", err)
	}

	var glReport report.Report
	if err = json.Unmarshal(data, &glReport); err != nil {
		return nil, fmt.Errorf("error unmarshalling report file: %w", err)
	}

	if !slices.Contains(supportedTypes, string(glReport.Scan.Type)) {
		return nil, fmt.Errorf("error loading Gitlab report. Missing scan type")
	}

	m, err := uploadAndCraft(ctx, i.input, i.backend, filePath, i.logger)
	if err != nil {
		return nil, err
	}

	i.injectAnnotations(m, &glReport)

	return m, nil
}

func (i *GitlabCrafter) injectAnnotations(m *api.Attestation_Material, glReport *report.Report) {
	// Prefer scanner (the actual security tool) over analyzer (the wrapper/integration layer)
	toolName := glReport.Scan.Scanner.Name
	toolVersion := glReport.Scan.Scanner.Version

	// Fallback to analyzer if scanner information is not available
	if toolName == "" {
		toolName = glReport.Scan.Analyzer.Name
		toolVersion = glReport.Scan.Analyzer.Version
	}

	if toolName != "" {
		if m.Annotations == nil {
			m.Annotations = make(map[string]string)
		}
		m.Annotations[AnnotationToolNameKey] = toolName
		if toolVersion != "" {
			m.Annotations[AnnotationToolVersionKey] = toolVersion
		}
	}
}
