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
	"testing"

	v1 "github.com/chainloop-dev/chainloop/app/controlplane/api/workflowcontract/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	materialSBOM  = "sbom"
	materialImage = "image"
)

func TestValidateMaterialAttachment(t *testing.T) {
	kindless := &v1.Policy{
		Metadata: &v1.Metadata{Name: "upstream-workflow-attested"},
		Spec:     &v1.PolicySpec{},
	}
	kindlessV2 := &v1.Policy{
		Metadata: &v1.Metadata{Name: "kindless-v2"},
		Spec: &v1.PolicySpec{
			Policies: []*v1.PolicySpecV2{{Source: &v1.PolicySpecV2_Path{Path: "p.rego"}}},
		},
	}
	sbomPolicy := &v1.Policy{
		Metadata: &v1.Metadata{Name: "sbom-policy"},
		Spec: &v1.PolicySpec{
			Policies: []*v1.PolicySpecV2{
				{Kind: v1.CraftingSchema_Material_SBOM_SPDX_JSON},
				{Kind: v1.CraftingSchema_Material_SBOM_CYCLONEDX_JSON},
			},
		},
	}

	materials := []*v1.CraftingSchema_Material{
		{Name: materialSBOM, Type: v1.CraftingSchema_Material_SBOM_CYCLONEDX_JSON},
		{Name: materialImage, Type: v1.CraftingSchema_Material_CONTAINER_IMAGE},
	}

	testCases := []struct {
		name      string
		policy    *v1.Policy
		selector  *v1.PolicyAttachment_MaterialSelector
		wantErrIs error
		wantMsg   string
	}{
		{
			name:      "kind-less policy without selector",
			policy:    kindless,
			wantErrIs: ErrAttachmentNeverApplies,
			wantMsg:   `policy attachment never applies: policy "upstream-workflow-attested" applies to any material type and requires a material selector (selector.name)`,
		},
		{
			name:      "kind-less v2 policy without selector",
			policy:    kindlessV2,
			wantErrIs: ErrAttachmentNeverApplies,
		},
		{
			name:      "kind-less policy with empty selector name",
			policy:    kindless,
			selector:  &v1.PolicyAttachment_MaterialSelector{},
			wantErrIs: ErrAttachmentNeverApplies,
		},
		{
			name:     "kind-less policy with selector",
			policy:   kindless,
			selector: &v1.PolicyAttachment_MaterialSelector{Name: materialImage},
		},
		{
			name:     "kind-less policy with selector of a material not in the contract",
			policy:   kindless,
			selector: &v1.PolicyAttachment_MaterialSelector{Name: "added-at-runtime"},
		},
		{
			name:   "typed policy without selector",
			policy: sbomPolicy,
		},
		{
			name:     "typed policy with selector of a supported material",
			policy:   sbomPolicy,
			selector: &v1.PolicyAttachment_MaterialSelector{Name: materialSBOM},
		},
		{
			name:      "typed policy with selector of an unsupported contract material",
			policy:    sbomPolicy,
			selector:  &v1.PolicyAttachment_MaterialSelector{Name: materialImage},
			wantErrIs: ErrAttachmentNeverApplies,
			wantMsg:   `policy attachment never applies: policy "sbom-policy" does not support material "image" of type CONTAINER_IMAGE (supported types: [SBOM_SPDX_JSON SBOM_CYCLONEDX_JSON])`,
		},
		{
			name:      "typed policy with explicit exact selector of an unsupported contract material",
			policy:    sbomPolicy,
			selector:  &v1.PolicyAttachment_MaterialSelector{Name: materialImage, MatchMode: v1.PolicyAttachment_MaterialSelector_EXACT},
			wantErrIs: ErrAttachmentNeverApplies,
		},
		{
			name:     "typed policy with prefix selector is not checked against contract materials",
			policy:   sbomPolicy,
			selector: &v1.PolicyAttachment_MaterialSelector{Name: materialImage, MatchMode: v1.PolicyAttachment_MaterialSelector_PREFIX},
		},
		{
			name:     "typed policy with selector of a material not in the contract",
			policy:   sbomPolicy,
			selector: &v1.PolicyAttachment_MaterialSelector{Name: "added-at-runtime"},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			att := &v1.PolicyAttachment{
				Policy:   &v1.PolicyAttachment_Ref{Ref: "chainloop://some-policy"},
				Selector: tc.selector,
			}

			err := ValidateMaterialAttachment(tc.policy, att, materials)
			if tc.wantErrIs == nil {
				assert.NoError(t, err)
				return
			}

			require.Error(t, err)
			assert.ErrorIs(t, err, tc.wantErrIs)
			if tc.wantMsg != "" {
				assert.EqualError(t, err, tc.wantMsg)
			}
		})
	}
}
