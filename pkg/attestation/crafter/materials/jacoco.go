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
	"encoding/xml"
	"fmt"
	"io"
	"os"
	"slices"

	schemaapi "github.com/chainloop-dev/chainloop/app/controlplane/api/workflowcontract/v1"
	api "github.com/chainloop-dev/chainloop/pkg/attestation/crafter/api/attestation/v1"
	"github.com/chainloop-dev/chainloop/pkg/attestation/crafter/materials/jacoco"
	"github.com/chainloop-dev/chainloop/pkg/casclient"
	"github.com/rs/zerolog"
)

type JacocoCrafter struct {
	*crafterCommon
	backend *casclient.CASBackend
}

func NewJacocoCrafter(schema *schemaapi.CraftingSchema_Material, backend *casclient.CASBackend, l *zerolog.Logger) *JacocoCrafter {
	return &JacocoCrafter{
		crafterCommon: &crafterCommon{logger: l, input: schema},
		backend:       backend,
	}
}

func (c *JacocoCrafter) Craft(ctx context.Context, filePath string) (*api.Attestation_Material, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("can't open the file: %w", err)
	}
	defer f.Close()

	bytes, err := io.ReadAll(f)
	if err != nil {
		return nil, fmt.Errorf("can't read the file: %w", err)
	}

	var report jacoco.Report

	if err := xml.Unmarshal(bytes, &report); err != nil {
		return nil, fmt.Errorf("invalid Jacoco report file: %w", ErrInvalidMaterialType)
	}

	if len(report.Counters) == 0 {
		return nil, fmt.Errorf("invalid Jacoco report file, no counters found:  %w", ErrInvalidMaterialType)
	}
	// At least "instruction" counter should be available according to the documentation
	// https://www.eclemma.org/jacoco/trunk/doc/counters.html
	if !slices.ContainsFunc(report.Counters, func(counter *jacoco.Counter) bool {
		return counter.Type == "INSTRUCTION"
	}) {
		return nil, fmt.Errorf("invalid Jacoco report file: %w", ErrInvalidMaterialType)
	}
	return uploadAndCraft(ctx, c.input, c.backend, filePath, c.logger)
}
