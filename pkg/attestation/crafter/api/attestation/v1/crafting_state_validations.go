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

package v1

import (
	"fmt"
	"strings"

	"github.com/bufbuild/protovalidate-go"
)

// Custom validations

// ValidateComplete makes sure that the crafting state has been completed
// before it gets passed to the renderer
func (state *CraftingState) ValidateComplete(dryRun bool) error {
	validator, err := protovalidate.New()
	if err != nil {
		return fmt.Errorf("could not create validator: %w", err)
	}

	// We do not want to validate the schema of the state if we are just doing a dry run
	// since it's known to not to contain the workflow metadata information
	if !dryRun {
		if err := validator.Validate(state); err != nil {
			return fmt.Errorf("invalid crafting state: %w", err)
		}
	}

	// Semantic errors
	// It has values for all the defined, non optional materials
	var missing []string
	expectedMaterials := state.GetInputSchema().GetMaterials()
	craftedMaterials := state.GetAttestation().GetMaterials()
	// Iterate on the expected materials
	for _, m := range expectedMaterials {
		if _, ok := craftedMaterials[m.Name]; !ok && !m.Optional {
			missing = append(missing, m.Name)
		}
	}

	if len(missing) > 0 {
		return fmt.Errorf("some materials have not been crafted yet: %s", strings.Join(missing, ", "))
	}

	return nil
}
