//
// Copyright 2023 The Chainloop Authors.
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
	"github.com/chainloop-dev/chainloop/internal/casclient"
	api "github.com/chainloop-dev/chainloop/pkg/attestation/crafter/api/attestation/v1"
	materialsjunit "github.com/chainloop-dev/chainloop/pkg/attestation/crafter/materials/junit"
	"github.com/rs/zerolog"
)

type JUnitXMLCrafter struct {
	*crafterCommon
	backend *casclient.CASBackend
}

func NewJUnitXMLCrafter(schema *schemaapi.CraftingSchema_Material, backend *casclient.CASBackend, l *zerolog.Logger) (*JUnitXMLCrafter, error) {
	if schema.Type != schemaapi.CraftingSchema_Material_JUNIT_XML {
		return nil, fmt.Errorf("material type is not JUnit XML")
	}
	craftCommon := &crafterCommon{logger: l, input: schema}
	return &JUnitXMLCrafter{backend: backend, crafterCommon: craftCommon}, nil
}

func (i *JUnitXMLCrafter) Craft(ctx context.Context, filePath string) (*api.Attestation_Material, error) {
	if err := i.validate(filePath); err != nil {
		return nil, err
	}

	return uploadAndCraft(ctx, i.input, i.backend, filePath, i.logger)
}

func (i *JUnitXMLCrafter) validate(filePath string) error {
	suites, err := materialsjunit.Ingest(filePath)
	if err != nil {
		return fmt.Errorf("failed to ingest JUnit XML: %w", err)
	}

	if len(suites) == 0 {
		return fmt.Errorf("invalid JUnit XML file, no suites found: %w", ErrInvalidMaterialType)
	}

	return nil
}
