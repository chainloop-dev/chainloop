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
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	schemaapi "github.com/chainloop-dev/chainloop/app/controlplane/api/workflowcontract/v1"
	api "github.com/chainloop-dev/chainloop/internal/attestation/crafter/api/attestation/v1"
	"github.com/chainloop-dev/chainloop/internal/attestation/renderer/chainloop"
	"github.com/chainloop-dev/chainloop/internal/casclient"
	"github.com/rs/zerolog"
	"github.com/secure-systems-lab/go-securesystemslib/dsse"
)

type AttestationCrafter struct {
	*crafterCommon
	backend *casclient.CASBackend
}

// NewAttestationCrafter generates a new Attestation material.
// Attestation materials represent a chainloop attestation submitted in a different workflow. This is useful to link
// related workflow runs. For instance, the deployment of different microservices coming from a common build workflow.
func NewAttestationCrafter(schema *schemaapi.CraftingSchema_Material, backend *casclient.CASBackend, l *zerolog.Logger) (*AttestationCrafter, error) {
	if schema.Type != schemaapi.CraftingSchema_Material_ATTESTATION {
		return nil, fmt.Errorf("material type is not attestation")
	}

	craftCommon := &crafterCommon{logger: l, input: schema}
	return &AttestationCrafter{backend: backend, crafterCommon: craftCommon}, nil
}

// Craft will calculate the digest of the artifact, simulate an upload and return the material definition
func (i *AttestationCrafter) Craft(ctx context.Context, artifactPath string) (*api.Attestation_Material, error) {
	data, err := os.ReadFile(artifactPath)
	if err != nil {
		return nil, fmt.Errorf("artifact file cannot be read: %w", err)
	}
	var dsseEnvelope dsse.Envelope
	if err := json.Unmarshal(data, &dsseEnvelope); err != nil {
		return nil, fmt.Errorf("artifact is not a valid DSEE Envelope: %w", err)
	}

	_, err = chainloop.ExtractPredicate(&dsseEnvelope)
	if err != nil {
		return nil, fmt.Errorf("extracting predicate from envelope: %w", err)
	}

	jsonContent, h, err := JSONEnvelopeWithDigest(&dsseEnvelope)
	if err != nil {
		return nil, fmt.Errorf("creating CAS payload: %w", err)
	}

	return uploadAndCraftFromBytes(ctx, i.input, i.backend, filepath.Base(artifactPath), jsonContent, h, i.logger)
}
