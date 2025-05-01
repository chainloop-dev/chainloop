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
	"fmt"
	"os"

	schemaapi "github.com/chainloop-dev/chainloop/app/controlplane/api/workflowcontract/v1"
	api "github.com/chainloop-dev/chainloop/pkg/attestation/crafter/api/attestation/v1"
	attestation2 "github.com/chainloop-dev/chainloop/pkg/attestation/crafter/materials/attestation"
	"github.com/chainloop-dev/chainloop/pkg/attestation/renderer/chainloop"
	"github.com/chainloop-dev/chainloop/pkg/casclient"
	intoto "github.com/in-toto/attestation/go/v1"
	"github.com/in-toto/in-toto-golang/in_toto"
	"github.com/rs/zerolog"
	"google.golang.org/protobuf/encoding/protojson"
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

	// extract the DSSE envelope (from the bundle if needed)
	dsseEnvelope, err := attestation2.ExtractDSSEEnvelope(data)
	if err != nil {
		return nil, fmt.Errorf("failed to parse the provided file as a DSSE envelope: %w", err)
	}

	// run some validations
	if dsseEnvelope.PayloadType != in_toto.PayloadType {
		return nil, fmt.Errorf("the payload %q is not of type in-toto", dsseEnvelope.PayloadType)
	}

	dec, err := dsseEnvelope.DecodeB64Payload()
	if err != nil {
		return nil, fmt.Errorf("failed to decode the DSSE payload: %w", err)
	}

	// decode the intoto payload
	var intotoStatement intoto.Statement
	if err = protojson.Unmarshal(dec, &intotoStatement); err != nil {
		return nil, fmt.Errorf("failed to parse the DSSE payload as an in-toto statement: %w", err)
	}
	// check if the statement predicate is of expected type
	if intotoStatement.PredicateType != chainloop.PredicateTypeV02 {
		return nil, fmt.Errorf("the provided predicate is not a valid chainloop attestation: found=%q", intotoStatement.PredicateType)
	}

	return uploadAndCraft(ctx, i.input, i.backend, artifactPath, i.logger)
}
