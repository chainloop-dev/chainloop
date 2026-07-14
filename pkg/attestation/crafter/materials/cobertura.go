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
	"github.com/chainloop-dev/chainloop/pkg/attestation/crafter/materials/cobertura"
	"github.com/chainloop-dev/chainloop/pkg/casclient"
	"github.com/rs/zerolog"
)

type CoberturaCrafter struct {
	*crafterCommon
	backend *casclient.CASBackend
}

func NewCoberturaCrafter(schema *schemaapi.CraftingSchema_Material, backend *casclient.CASBackend, l *zerolog.Logger) *CoberturaCrafter {
	return &CoberturaCrafter{
		crafterCommon: &crafterCommon{logger: l, input: schema},
		backend:       backend,
	}
}

func (c *CoberturaCrafter) Craft(ctx context.Context, filePath string) (*api.Attestation_Material, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("can't open the file: %w", err)
	}
	defer f.Close()

	data, err := io.ReadAll(f)
	if err != nil {
		return nil, fmt.Errorf("can't read the file: %w", err)
	}

	var report cobertura.Coverage
	// Coverage pins its XMLName to "coverage", so xml.Unmarshal rejects a
	// mismatched root element (e.g. JaCoCo's <report> or JUnit's <testsuite>).
	if err := xml.Unmarshal(data, &report); err != nil {
		return nil, fmt.Errorf("invalid Cobertura report file: %w", ErrInvalidMaterialType)
	}

	// Reject only genuinely dataless input: a zero line-rate AND no packages.
	// Both legitimate low-data cases are still accepted, so we neither suppress
	// real results nor reject valid empty reports:
	//   - real 0% coverage has line-rate "0" but carries packages (the uncovered
	//     code), so len(Packages) > 0 keeps it — a policy can flag it;
	//   - an empty report (no measurable lines) has line-rate "NaN" (0/0), which
	//     is not == 0, so it is kept and projects to a null rate for a policy to
	//     treat as "nothing to measure" (distinct from 0% coverage).
	// The pinned <coverage> root checked above already excludes other formats.
	if report.LineRate == 0 && len(report.Packages) == 0 {
		return nil, fmt.Errorf("invalid Cobertura report file, missing coverage data: %w", ErrInvalidMaterialType)
	}

	return uploadAndCraft(ctx, c.input, c.backend, filePath, c.logger)
}
