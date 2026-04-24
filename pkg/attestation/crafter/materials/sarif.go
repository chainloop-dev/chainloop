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

	schemaapi "github.com/chainloop-dev/chainloop/app/controlplane/api/workflowcontract/v1"
	api "github.com/chainloop-dev/chainloop/pkg/attestation/crafter/api/attestation/v1"
	"github.com/chainloop-dev/chainloop/pkg/casclient"
	sarif "github.com/owenrumney/go-sarif/v3/pkg/report/v210/sarif"
	"github.com/rs/zerolog"
)

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
}
