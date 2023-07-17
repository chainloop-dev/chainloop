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

package v1

import (
	schemaapi "github.com/chainloop-dev/chainloop/app/controlplane/api/workflowcontract/v1"
)

type NormalizedMaterialOutput struct {
	Name, Digest string
	IsOutput     bool
	Content      []byte
}

func (m *Attestation_Material) NormalizedOutput() *NormalizedMaterialOutput {
	switch m.MaterialType {
	case schemaapi.CraftingSchema_Material_ARTIFACT, schemaapi.CraftingSchema_Material_SBOM_CYCLONEDX_JSON, schemaapi.CraftingSchema_Material_SBOM_SPDX_JSON:
		a := m.GetArtifact()
		return &NormalizedMaterialOutput{a.Name, a.Digest, a.IsSubject, a.Content}
	case schemaapi.CraftingSchema_Material_CONTAINER_IMAGE:
		a := m.GetContainerImage()
		return &NormalizedMaterialOutput{a.Name, a.Digest, a.IsSubject, nil}
	case schemaapi.CraftingSchema_Material_STRING:
		a := m.GetString_()
		return &NormalizedMaterialOutput{Content: []byte(a.Value)}
	}

	return nil
}
