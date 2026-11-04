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
	"context"
	"testing"

	v1 "github.com/chainloop-dev/chainloop/app/controlplane/api/workflowcontract/v1"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
		{Name: "sbom", Type: v1.CraftingSchema_Material_SBOM_CYCLONEDX_JSON},
		{Name: "image", Type: v1.CraftingSchema_Material_CONTAINER_IMAGE},
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
			selector: &v1.PolicyAttachment_MaterialSelector{Name: "image"},
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
			selector: &v1.PolicyAttachment_MaterialSelector{Name: "sbom"},
		},
		{
			name:      "typed policy with selector of an unsupported contract material",
			policy:    sbomPolicy,
			selector:  &v1.PolicyAttachment_MaterialSelector{Name: "image"},
			wantErrIs: ErrAttachmentNeverApplies,
			wantMsg:   `policy attachment never applies: policy "sbom-policy" does not support material "image" of type CONTAINER_IMAGE (supported types: [SBOM_SPDX_JSON SBOM_CYCLONEDX_JSON])`,
		},
		{
			name:      "typed policy with explicit exact selector of an unsupported contract material",
			policy:    sbomPolicy,
			selector:  &v1.PolicyAttachment_MaterialSelector{Name: "image", MatchMode: v1.PolicyAttachment_MaterialSelector_EXACT},
			wantErrIs: ErrAttachmentNeverApplies,
		},
		{
			name:     "typed policy with prefix selector is not checked against contract materials",
			policy:   sbomPolicy,
			selector: &v1.PolicyAttachment_MaterialSelector{Name: "image", MatchMode: v1.PolicyAttachment_MaterialSelector_PREFIX},
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

func TestPolicyVerifierValidateMaterialAttachments(t *testing.T) {
	logger := zerolog.Nop()
	materials := []*v1.CraftingSchema_Material{
		{Name: "custom-material", Type: v1.CraftingSchema_Material_ARTIFACT},
	}

	testCases := []struct {
		name        string
		attachments []*v1.PolicyAttachment
		wantErrs    int
	}{
		{
			name: "valid attachments",
			attachments: []*v1.PolicyAttachment{
				{Policy: &v1.PolicyAttachment_Ref{Ref: "file://testdata/sbom_syft.yaml"}},
				{
					Policy:   &v1.PolicyAttachment_Ref{Ref: "file://testdata/sbom_syft_not_typed.yaml"},
					Selector: &v1.PolicyAttachment_MaterialSelector{Name: "custom-material"},
				},
			},
		},
		{
			name: "kind-less policy without selector",
			attachments: []*v1.PolicyAttachment{
				{Policy: &v1.PolicyAttachment_Ref{Ref: "file://testdata/sbom_syft.yaml"}},
				{Policy: &v1.PolicyAttachment_Ref{Ref: "file://testdata/sbom_syft_not_typed.yaml"}},
			},
			wantErrs: 1,
		},
		{
			name: "several attachments that never apply",
			attachments: []*v1.PolicyAttachment{
				{Policy: &v1.PolicyAttachment_Ref{Ref: "file://testdata/sbom_syft_not_typed.yaml"}},
				{
					Policy:   &v1.PolicyAttachment_Ref{Ref: "file://testdata/sbom_syft.yaml"},
					Selector: &v1.PolicyAttachment_MaterialSelector{Name: "custom-material"},
				},
			},
			wantErrs: 2,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			pv := NewPolicyVerifier(&v1.Policies{Materials: tc.attachments}, nil, &logger)
			errs := pv.ValidateMaterialAttachments(context.TODO(), materials)
			assert.Len(t, errs, tc.wantErrs)
			for _, err := range errs {
				assert.ErrorIs(t, err, ErrAttachmentNeverApplies)
			}
		})
	}
}
