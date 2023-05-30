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
	"time"

	api "github.com/chainloop-dev/chainloop/app/cli/api/attestation/v1"
	schemaapi "github.com/chainloop-dev/chainloop/app/controlplane/api/workflowcontract/v1"
	"github.com/chainloop-dev/chainloop/internal/casclient"
	"github.com/rs/zerolog"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type ArtifactCrafter struct {
	*crafterCommon
	uploader casclient.Uploader
}

func NewArtifactCrafter(schema *schemaapi.CraftingSchema_Material, uploader casclient.Uploader, l *zerolog.Logger) (*ArtifactCrafter, error) {
	if schema.Type != schemaapi.CraftingSchema_Material_ARTIFACT {
		return nil, fmt.Errorf("material type is not artifact")
	}

	craftCommon := &crafterCommon{logger: l, input: schema}
	return &ArtifactCrafter{uploader: uploader, crafterCommon: craftCommon}, nil
}

func (i *ArtifactCrafter) Craft(ctx context.Context, artifactPath string) (*api.Attestation_Material, error) {
	result, err := i.uploader.UploadFile(ctx, artifactPath)
	if err != nil {
		i.logger.Debug().Err(err)
		return nil, err
	}

	res := &api.Attestation_Material{
		AddedAt:      timestamppb.New(time.Now()),
		MaterialType: i.input.Type,
		M: &api.Attestation_Material_Artifact_{
			Artifact: &api.Attestation_Material_Artifact{
				Id: i.input.Name, Digest: result.Digest, Name: result.Filename, IsSubject: i.input.Output,
			},
		},
	}

	return res, nil
}
