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
	"os"
	"time"

	cdx "github.com/CycloneDX/cyclonedx-go"
	api "github.com/chainloop-dev/chainloop/app/cli/api/attestation/v1"
	schemaapi "github.com/chainloop-dev/chainloop/app/controlplane/api/workflowcontract/v1"
	"github.com/rs/zerolog"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type CyclonedxJSONCrafter struct {
	uploader Uploader
	*crafterCommon
}

func NewCyclonedxJSONCrafter(materialSchema *schemaapi.CraftingSchema_Material, uploader Uploader, l *zerolog.Logger) (*CyclonedxJSONCrafter, error) {
	if materialSchema.Type != schemaapi.CraftingSchema_Material_SBOM_CYCLONEDX_JSON {
		return nil, fmt.Errorf("material type is not cyclonedx json")
	}

	return &CyclonedxJSONCrafter{
		uploader:      uploader,
		crafterCommon: &crafterCommon{logger: l, input: materialSchema},
	}, nil
}

func (i *CyclonedxJSONCrafter) Craft(ctx context.Context, filePath string) (*api.Attestation_Material, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("can't open the file: %w", err)
	}
	defer f.Close()

	// Decode the file to check it's a valid CycloneDX BOM
	bom := new(cdx.BOM)
	decoder := cdx.NewBOMDecoder(f, cdx.BOMFileFormatJSON)
	if err = decoder.Decode(bom); err != nil {
		i.logger.Debug().Err(err).Msg("error decoding file")
		return nil, fmt.Errorf("invalid cyclonedx sbom file: %w", ErrInvalidMaterialType)
	}

	if bom.Metadata == nil {
		return nil, fmt.Errorf("invalid cyclonedx sbom file: %w", ErrInvalidMaterialType)
	}

	result, err := i.uploader.Upload(ctx, filePath)
	if err != nil {
		i.logger.Debug().Err(err)
		return nil, err
	}

	res := &api.Attestation_Material{
		AddedAt:      timestamppb.New(time.Now()),
		MaterialType: i.input.Type,
		M: &api.Attestation_Material_Artifact_{
			Artifact: &api.Attestation_Material_Artifact{
				Id: i.input.Name, Digest: result.Digest, Name: "sbom.cyclonedx.json",
			},
		},
	}

	return res, nil
}
