//
// Copyright 2024 The Chainloop Authors.
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
	api "github.com/chainloop-dev/chainloop/internal/attestation/crafter/api/attestation/v1"
	"github.com/chainloop-dev/chainloop/internal/casclient"
	"github.com/rs/zerolog"
)

type EvidenceCrafter struct {
	*crafterCommon
	backend *casclient.CASBackend
}

// NewEvidenceCrafter generates a new Evidence material.
// Pieces of evidences represent generic, additional context that don't fit
// into one of the well known material types. For example, a custom approval report (in json), ...
func NewEvidenceCrafter(schema *schemaapi.CraftingSchema_Material, backend *casclient.CASBackend, l *zerolog.Logger) (*EvidenceCrafter, error) {
	if schema.Type != schemaapi.CraftingSchema_Material_EVIDENCE {
		return nil, fmt.Errorf("material type is not evidence")
	}

	craftCommon := &crafterCommon{logger: l, input: schema}
	return &EvidenceCrafter{backend: backend, crafterCommon: craftCommon}, nil
}

// Craft will calculate the digest of the artifact, simulate an upload and return the material definition
func (i *EvidenceCrafter) Craft(ctx context.Context, artifactPath string) (*api.Attestation_Material, error) {
	return uploadAndCraft(ctx, i.input, i.backend, artifactPath, i.logger)
}
