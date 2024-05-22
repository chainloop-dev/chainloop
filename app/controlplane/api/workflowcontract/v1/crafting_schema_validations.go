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
	"fmt"
	"strings"
)

// CraftingMaterialInValidationOrder all type of CraftingMaterial that are available for automatic
// detection. The order of the list is important as it defines the order of the
// detection process. Normally from most common one to the least common one and weaker validation method.
var CraftingMaterialInValidationOrder = []CraftingSchema_Material_MaterialType{
	CraftingSchema_Material_OPENVEX,
	CraftingSchema_Material_SBOM_CYCLONEDX_JSON,
	CraftingSchema_Material_SBOM_SPDX_JSON,
	CraftingSchema_Material_CSAF_VEX,
	CraftingSchema_Material_CSAF_INFORMATIONAL_ADVISORY,
	CraftingSchema_Material_CSAF_SECURITY_ADVISORY,
	CraftingSchema_Material_CSAF_SECURITY_INCIDENT_RESPONSE,
	CraftingSchema_Material_JUNIT_XML,
	CraftingSchema_Material_HELM_CHART,
	CraftingSchema_Material_CONTAINER_IMAGE,
	CraftingSchema_Material_SARIF,
	CraftingSchema_Material_ATTESTATION,
	CraftingSchema_Material_ARTIFACT,
}

// ListAvailableMaterialKind returns a list of available material kinds
func ListAvailableMaterialKind() []string {
	var res []string
	for k := range CraftingSchema_Material_MaterialType_value {
		if k != "MATERIAL_TYPE_UNSPECIFIED" {
			res = append(res, strings.Replace(k, "MATERIAL_TYPE_", "", 1))
		}
	}

	return res
}

// Custom validations

// ValidateUniqueMaterialName validates that only one material definition
// with the same ID is present in the schema
func (schema *CraftingSchema) ValidateUniqueMaterialName() error {
	materialNames := make(map[string]bool)
	for _, m := range schema.Materials {
		if _, found := materialNames[m.Name]; found {
			return fmt.Errorf("material with name=%s is duplicated", m.Name)
		}

		materialNames[m.Name] = true
	}

	return nil
}
