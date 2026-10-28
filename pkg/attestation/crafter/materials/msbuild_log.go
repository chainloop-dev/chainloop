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
	"slices"
	"strconv"

	schemaapi "github.com/chainloop-dev/chainloop/app/controlplane/api/workflowcontract/v1"
	api "github.com/chainloop-dev/chainloop/pkg/attestation/crafter/api/attestation/v1"
	"github.com/chainloop-dev/chainloop/pkg/attestation/crafter/materials/buildlog"
	"github.com/chainloop-dev/chainloop/pkg/casclient"
	"github.com/rs/zerolog"
)

const (
	// AnnotationMSBuildLogLogsCount is the number of build logs in an MSBUILD_LOG
	// material: 1 for a single log, or the number of logs found in an archive.
	AnnotationMSBuildLogLogsCount = "chainloop.material.msbuild_log.logs.count"
	// AnnotationMSBuildLogWarningsCount and AnnotationMSBuildLogErrorsCount are the
	// number of distinct warnings and errors in an MSBUILD_LOG material.
	AnnotationMSBuildLogWarningsCount = "chainloop.material.msbuild_log.warnings.count"
	AnnotationMSBuildLogErrorsCount   = "chainloop.material.msbuild_log.errors.count"
)

// MSBuildLogCrafter stores the log of a Microsoft build (MSBuild binary or text
// log, or a Visual C++ BuildLog.htm) as evidence of the warnings and errors the
// build produced. The log is stored as-is; its projection to the JSON the
// policy engine consumes happens later, at evaluation time.
//
// The value may also be an archive of logs, for builds that produce one binary
// log per solution. The archive is recorded whole and the projection
// aggregates its entries.
type MSBuildLogCrafter struct {
	*crafterCommon
	backend *casclient.CASBackend
}

func NewMSBuildLogCrafter(schema *schemaapi.CraftingSchema_Material, backend *casclient.CASBackend, l *zerolog.Logger) (*MSBuildLogCrafter, error) {
	if schema.Type != schemaapi.CraftingSchema_Material_MSBUILD_LOG {
		return nil, fmt.Errorf("material type is not a build log")
	}
	craftCommon := &crafterCommon{logger: l, input: schema}
	return &MSBuildLogCrafter{backend: backend, crafterCommon: craftCommon}, nil
}

func (i *MSBuildLogCrafter) Craft(ctx context.Context, filePath string) (*CraftResult, error) {
	// Parse with the same code the policy projection uses, so a material
	// accepted here is one the projection can read. Were the two to disagree, a
	// log would be attested and then fail, or worse pass, the policy gate.
	report, err := buildlog.ParseFile(filePath)
	if err != nil {
		if errors.Is(err, buildlog.ErrNoBuildLog) {
			return nil, fmt.Errorf("input does not look like a build log: %w", ErrInvalidMaterialType)
		}
		return nil, err
	}

	m, err := uploadAndCraft(ctx, i.input, i.backend, filePath, i.logger)
	if err != nil {
		return nil, err
	}

	i.injectAnnotations(m, report)

	return craftResult(m, nil)
}

func (i *MSBuildLogCrafter) injectAnnotations(m *api.Attestation_Material, report *buildlog.Report) {
	if m.Annotations == nil {
		m.Annotations = make(map[string]string)
	}
	m.Annotations[AnnotationToolNameKey] = report.Tool.Name
	if report.Tool.Version != "" {
		m.Annotations[AnnotationToolVersionKey] = report.Tool.Version
	}
	// An archive can hold the logs of several tools: list each of them.
	var tools []Tool
	for _, l := range report.Logs {
		tool := Tool{Name: l.Tool.Name, Version: l.Tool.Version}
		if !slices.Contains(tools, tool) {
			tools = append(tools, tool)
		}
	}
	SetToolsAnnotation(m, tools)
	m.Annotations[AnnotationMSBuildLogLogsCount] = strconv.Itoa(len(report.Logs))
	m.Annotations[AnnotationMSBuildLogWarningsCount] = strconv.Itoa(report.Summary.Warnings)
	m.Annotations[AnnotationMSBuildLogErrorsCount] = strconv.Itoa(report.Summary.Errors)
}
