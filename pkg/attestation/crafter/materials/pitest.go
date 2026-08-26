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
	"encoding/xml"
	"fmt"
	"io"
	"os"

	schemaapi "github.com/chainloop-dev/chainloop/app/controlplane/api/workflowcontract/v1"
	api "github.com/chainloop-dev/chainloop/pkg/attestation/crafter/api/attestation/v1"
	"github.com/chainloop-dev/chainloop/pkg/attestation/crafter/materials/pitest"
	"github.com/chainloop-dev/chainloop/pkg/casclient"
	"github.com/rs/zerolog"
)

type PitestCrafter struct {
	*crafterCommon
	backend *casclient.CASBackend
}

func NewPitestCrafter(schema *schemaapi.CraftingSchema_Material, backend *casclient.CASBackend, l *zerolog.Logger) *PitestCrafter {
	return &PitestCrafter{
		crafterCommon: &crafterCommon{logger: l, input: schema},
		backend:       backend,
	}
}

func (c *PitestCrafter) Craft(ctx context.Context, filePath string) (*api.Attestation_Material, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("can't open the file: %w", err)
	}
	defer f.Close()

	data, err := io.ReadAll(f)
	if err != nil {
		return nil, fmt.Errorf("can't read the file: %w", err)
	}

	var report pitest.Report
	// Report pins its XMLName to "mutations", so xml.Unmarshal rejects a
	// mismatched root element (e.g. JaCoCo's <report>, Cobertura's <coverage>
	// or JUnit's <testsuite>).
	if err := xml.Unmarshal(data, &report); err != nil {
		return nil, fmt.Errorf("invalid PIT report file: %w", ErrInvalidMaterialType)
	}

	// An empty <mutations/> report means no mutants were analyzed, not a 0% or
	// 100% result; downstream score calculations would otherwise divide by
	// zero. Reject it instead of uploading contentless evidence.
	if len(report.Mutations) == 0 {
		return nil, fmt.Errorf("invalid PIT report file, no mutations found: %w", ErrInvalidMaterialType)
	}

	return uploadAndCraft(ctx, c.input, c.backend, filePath, c.logger)
}
