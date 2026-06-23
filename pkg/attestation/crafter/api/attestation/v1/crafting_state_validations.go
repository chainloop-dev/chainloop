//
// Copyright 2024-2026 The Chainloop Authors.
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
	"errors"
	"fmt"
	"strings"

	"buf.build/go/protovalidate"
)

// Custom validations

// ValidateComplete makes sure that the crafting state has been completed
// before it gets passed to the renderer
func (state *CraftingState) ValidateComplete(dryRun bool) error {
	// We do not want to validate the schema of the state if we are just doing a dry run
	// since it's known to not to contain the workflow metadata information
	if !dryRun {
		if err := protovalidate.Validate(state); err != nil {
			return fmt.Errorf("invalid crafting state: %w", err)
		}
	}

	// Semantic errors
	// It has values for all the defined, non optional materials
	var missing []string
	expectedMaterials := state.GetMaterials()
	craftedMaterials := state.GetAttestation().GetMaterials()

	// Choke groups: materials sharing the same non-empty group form an
	// "at least one of" set. We track the members of each group and whether
	// at least one of them has been crafted.
	groupMembers := make(map[string][]string)
	groupSatisfied := make(map[string]bool)
	// Preserve a stable order of groups for deterministic error messages
	var groupOrder []string

	// Iterate on the expected materials
	for _, m := range expectedMaterials {
		_, crafted := craftedMaterials[m.Name]

		// Grouped materials are enforced at the group level, not individually.
		if m.Group != "" {
			if _, seen := groupMembers[m.Group]; !seen {
				groupOrder = append(groupOrder, m.Group)
			}
			groupMembers[m.Group] = append(groupMembers[m.Group], m.Name)
			if crafted {
				groupSatisfied[m.Group] = true
			}
			continue
		}

		if !crafted && !m.Optional {
			missing = append(missing, m.Name)
		}
	}

	var errs []string
	if len(missing) > 0 {
		errs = append(errs, fmt.Sprintf("some materials have not been crafted yet: %s", strings.Join(missing, ", ")))
	}
	for _, group := range groupOrder {
		if !groupSatisfied[group] {
			errs = append(errs, fmt.Sprintf("at least one material from group %q is required: %s", group, strings.Join(groupMembers[group], ", ")))
		}
	}

	if len(errs) > 0 {
		return errors.New(strings.Join(errs, "; "))
	}

	return nil
}
