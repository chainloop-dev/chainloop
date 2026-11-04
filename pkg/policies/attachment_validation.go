//
// Copyright 2026 The Chainloop Authors.
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

package policies

import (
	"errors"
	"fmt"

	v1 "github.com/chainloop-dev/chainloop/app/controlplane/api/workflowcontract/v1"
)

// ErrAttachmentNeverApplies is returned when a material policy attachment can
// never be evaluated on any material, so it would silently be a no-op.
var ErrAttachmentNeverApplies = errors.New("policy attachment never applies")

// ValidateMaterialAttachment checks that a material policy attachment can apply
// to at least one material, using the same rule as the policy evaluation:
//   - a policy without material types applies only to the material named by
//     the attachment selector, so the selector is required.
//   - a typed policy whose selector names, by exact match, a contract material
//     of a type the policy does not support never applies.
//
// materials are the materials declared in the contract. A selector that names a
// material not declared in the contract is accepted, because that material can
// be added at attestation time.
func ValidateMaterialAttachment(policy *v1.Policy, att *v1.PolicyAttachment, materials []*v1.CraftingSchema_Material) error {
	policyName := policy.GetMetadata().GetName()
	if policyName == "" {
		policyName = att.GetRef()
	}

	specTypes := getPolicyTypes(policy)
	selector := att.GetSelector()

	if len(specTypes) == 0 && selector.GetName() == "" {
		return fmt.Errorf("%w: policy %q applies to any material type and requires a material selector (selector.name)", ErrAttachmentNeverApplies, policyName)
	}

	// a prefix selector can match materials added at attestation time
	if selector.GetName() == "" || selector.GetMatchMode() == v1.PolicyAttachment_MaterialSelector_PREFIX {
		return nil
	}

	for _, m := range materials {
		if m.GetName() == selector.GetName() && !attachmentApplies(specTypes, selector, m.GetName(), m.GetType()) {
			return fmt.Errorf("%w: policy %q does not support material %q of type %s (supported types: %v)",
				ErrAttachmentNeverApplies, policyName, m.GetName(), m.GetType(), specTypes)
		}
	}

	return nil
}
