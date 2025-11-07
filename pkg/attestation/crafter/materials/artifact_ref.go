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
	"errors"
	"fmt"

	schemaapi "github.com/chainloop-dev/chainloop/app/controlplane/api/workflowcontract/v1"
	api "github.com/chainloop-dev/chainloop/pkg/attestation/crafter/api/attestation/v1"
	"github.com/rs/zerolog"
)

type ArtifactRefCrafter struct {
	*crafterCommon
}

func NewArtifactRefCrafter(schema *schemaapi.CraftingSchema_Material, l *zerolog.Logger) (*ArtifactRefCrafter, error) {
	if schema.Type != schemaapi.CraftingSchema_Material_ARTIFACT_REF {
		return nil, fmt.Errorf("material type is not artifact_ref")
	}

	craftCommon := &crafterCommon{logger: l, input: schema}
	return &ArtifactRefCrafter{crafterCommon: craftCommon}, nil
}

// Craft will calculate the digest of the artifact without uploading to CAS
func (i *ArtifactRefCrafter) Craft(_ context.Context, artifactPath string) (*api.Attestation_Material, error) {
	// Get file stats including digest
	result, err := fileStats(artifactPath)
	if err != nil {
		return nil, fmt.Errorf("getting file stats: %w", err)
	}
	defer result.r.Close()

	if result.size == 0 {
		return nil, fmt.Errorf("%w: %w", ErrBaseUploadAndCraft, errors.New("file is empty"))
	}

	i.logger.Debug().
		Str("filename", result.filename).
		Str("digest", result.digest).
		Str("path", artifactPath).
		Int64("size", result.size).
		Msg("crafting artifact reference (no upload)")

	material := &api.Attestation_Material{
		MaterialType: i.input.Type,
		M: &api.Attestation_Material_Artifact_{
			Artifact: &api.Attestation_Material_Artifact{
				Id:        i.input.Name,
				Name:      result.filename,
				Digest:    result.digest,
				IsSubject: i.input.Output,
			},
		},
		// Not uploaded to CAS
		UploadedToCas: false,
		InlineCas:     false,
	}

	return material, nil
}
