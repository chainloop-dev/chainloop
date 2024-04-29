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
	"encoding/xml"
	"fmt"
	"io"
	"os"

	schemaapi "github.com/chainloop-dev/chainloop/app/controlplane/api/workflowcontract/v1"
	api "github.com/chainloop-dev/chainloop/internal/attestation/crafter/api/attestation/v1"
	"github.com/chainloop-dev/chainloop/internal/casclient"
	junit "github.com/joshdk/go-junit"
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

	return uploadAndCraftFromFile(ctx, i.input, i.backend, filePath, i.logger)
}

func (i *JUnitXMLCrafter) validate(filePath string) error {
	f, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("can't open the file: %w", err)
	}
	defer f.Close()

	bytes, err := io.ReadAll(f)
	if err != nil {
		return fmt.Errorf("can't read the file: %w", err)
	}

	if err := xml.Unmarshal(bytes, &junit.Suite{}); err != nil {
		return fmt.Errorf("invalid JUnit XML file: %w", ErrInvalidMaterialType)
	}

	_, err = junit.IngestReader(f)
	if err != nil {
		i.logger.Debug().Err(err).Msgf("error decoding file: %s", filePath)
		return fmt.Errorf("invalid JUnit XML file: %w", ErrInvalidMaterialType)
	}

	return nil
}
