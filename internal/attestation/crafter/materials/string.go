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

	api "github.com/chainloop-dev/bedrock/app/cli/api/attestation/v1"
	schemaapi "github.com/chainloop-dev/bedrock/app/controlplane/api/workflowcontract/v1"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type StringCrafter struct {
	*crafterCommon
}

func NewStringCrafter(materialSchema *schemaapi.CraftingSchema_Material) (*StringCrafter, error) {
	if materialSchema.Type != schemaapi.CraftingSchema_Material_STRING {
		return nil, fmt.Errorf("material type is not string")
	}

	return &StringCrafter{
		&crafterCommon{input: materialSchema},
	}, nil
}

func (i *StringCrafter) Craft(ctx context.Context, value string) (*api.Attestation_Material, error) {
	return &api.Attestation_Material{
		AddedAt:      timestamppb.New(time.Now()),
		MaterialType: i.input.Type,
		M: &api.Attestation_Material_String_{
			String_: &api.Attestation_Material_KeyVal{
				Id: i.input.Name, Value: value,
			},
		},
	}, nil
}
