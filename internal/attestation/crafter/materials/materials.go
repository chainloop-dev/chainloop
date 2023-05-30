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

// ErrInvalidMaterialType is returned when the provided material type
// is not from the kind we are expecting
var ErrInvalidMaterialType = fmt.Errorf("unexpected material type")

type crafterCommon struct {
	logger *zerolog.Logger
	input  *schemaapi.CraftingSchema_Material
}

type crafterUploader struct {
	*crafterCommon
	uploader casclient.Uploader
}

func (i *crafterUploader) craft(ctx context.Context, artifactPath string) (*api.Attestation_Material, error) {
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

type Craftable interface {
	Craft(ctx context.Context, value string) (*api.Attestation_Material, error)
}

func Craft(ctx context.Context, materialSchema *schemaapi.CraftingSchema_Material, value string, uploader casclient.Uploader, logger *zerolog.Logger) (*api.Attestation_Material, error) {
	var crafter Craftable
	var err error

	switch materialSchema.Type {
	case schemaapi.CraftingSchema_Material_STRING:
		crafter, err = NewStringCrafter(materialSchema)
	case schemaapi.CraftingSchema_Material_CONTAINER_IMAGE:
		crafter, err = NewOCIImageCrafter(materialSchema, logger)
	case schemaapi.CraftingSchema_Material_ARTIFACT:
		crafter, err = NewArtifactCrafter(materialSchema, uploader, logger)
	case schemaapi.CraftingSchema_Material_SBOM_CYCLONEDX_JSON:
		crafter, err = NewCyclonedxJSONCrafter(materialSchema, uploader, logger)
	case schemaapi.CraftingSchema_Material_SBOM_SPDX_JSON:
		crafter, err = NewSPDXJSONCrafter(materialSchema, uploader, logger)
	case schemaapi.CraftingSchema_Material_JUNIT_XML:
		crafter, err = NewJUnitXMLCrafter(materialSchema, uploader, logger)
	default:
		return nil, fmt.Errorf("material of type %q not supported yet", materialSchema.Type)
	}

	if err != nil {
		return nil, err
	}

	return crafter.Craft(ctx, value)
}
