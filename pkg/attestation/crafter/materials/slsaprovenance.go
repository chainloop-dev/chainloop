//
// Copyright 2025 The Chainloop Authors.
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
	"github.com/chainloop-dev/chainloop/pkg/casclient"
	intoto "github.com/in-toto/attestation/go/v1"
	intotoatt "github.com/in-toto/in-toto-golang/in_toto/slsa_provenance/v1"
	"github.com/rs/zerolog"
	protobundle "github.com/sigstore/protobuf-specs/gen/pb-go/bundle/v1"
	"google.golang.org/protobuf/encoding/protojson"
)

type SLSAProvenanceCrafter struct {
	*crafterCommon
	backend *casclient.CASBackend
}

// SLSA Provenance in the form of Sigstore Bundle
// https://slsa.dev/spec/v1.0/provenance
// https://docs.sigstore.dev/about/bundle/
func NewSLSAProvenanceCrafter(schema *schemaapi.CraftingSchema_Material, backend *casclient.CASBackend, l *zerolog.Logger) (*SLSAProvenanceCrafter, error) {
	if schema.Type != schemaapi.CraftingSchema_Material_SLSA_PROVENANCE {
		return nil, fmt.Errorf("material type is not SLSA Provenance")
	}

	craftCommon := &crafterCommon{logger: l, input: schema}
	return &SLSAProvenanceCrafter{backend: backend, crafterCommon: craftCommon}, nil
}

// Craft will calculate the digest of the artifact, simulate an upload and return the material definition
func (i *SLSAProvenanceCrafter) Craft(ctx context.Context, artifactPath string) (*api.Attestation_Material, error) {
	data, err := os.ReadFile(artifactPath)
	if err != nil {
		return nil, fmt.Errorf("artifact file cannot be read: %w", err)
	}

	bundle := &protobundle.Bundle{}
	if err := protojson.Unmarshal(data, bundle); err != nil {
		return nil, fmt.Errorf("failed to unmarshal bundle: %w", err)
	}

	statement := &intoto.Statement{}
	if err := protojson.Unmarshal(bundle.GetDsseEnvelope().GetPayload(), statement); err != nil {
		return nil, fmt.Errorf("failed to unmarshal statement: %w", err)
	}

	if p := statement.PredicateType; p != intotoatt.PredicateSLSAProvenance {
		return nil, fmt.Errorf("the provided predicate is not a valid SLSA Provenance: found=%q", p)
	}

	return uploadAndCraft(ctx, i.input, i.backend, artifactPath, i.logger)
}
