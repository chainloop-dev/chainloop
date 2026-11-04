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

package crafter_test

import (
	"context"
	"testing"

	schemaapi "github.com/chainloop-dev/chainloop/app/controlplane/api/workflowcontract/v1"
	v1 "github.com/chainloop-dev/chainloop/pkg/attestation/crafter/api/attestation/v1"
	"github.com/chainloop-dev/chainloop/pkg/policies"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidateMaterialPolicyAttachments(t *testing.T) {
	const rego = "package main\nimport rego.v1\nviolations contains msg if { false; msg := \"\" }\n"

	kindless := &schemaapi.Policy{
		ApiVersion: "chainloop.dev/v1",
		Kind:       "Policy",
		Metadata:   &schemaapi.Metadata{Name: "kindless"},
		Spec: &schemaapi.PolicySpec{
			Policies: []*schemaapi.PolicySpecV2{{Source: &schemaapi.PolicySpecV2_Embedded{Embedded: rego}}},
		},
	}

	testCases := []struct {
		name     string
		selector *schemaapi.PolicyAttachment_MaterialSelector
		wantErrs int
	}{
		{
			name:     "kind-less policy without selector",
			wantErrs: 1,
		},
		{
			name:     "kind-less policy with selector",
			selector: &schemaapi.PolicyAttachment_MaterialSelector{Name: "image"},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			contract := &schemaapi.CraftingSchemaV2{
				ApiVersion: "chainloop.dev/v1",
				Kind:       "Contract",
				Metadata:   &schemaapi.Metadata{Name: "contract"},
				Spec: &schemaapi.CraftingSchemaV2Spec{
					Materials: []*schemaapi.CraftingSchema_Material{
						{Name: "image", Type: schemaapi.CraftingSchema_Material_CONTAINER_IMAGE},
					},
					Policies: &schemaapi.Policies{
						Materials: []*schemaapi.PolicyAttachment{
							{Policy: &schemaapi.PolicyAttachment_Embedded{Embedded: kindless}, Selector: tc.selector},
						},
					},
				},
			}

			c, err := newInitializedCrafterV2(t, contract, &v1.WorkflowMetadata{Name: "wf"}, true, "", nil)
			require.NoError(t, err)

			errs := c.ValidateMaterialPolicyAttachments(context.Background())
			assert.Len(t, errs, tc.wantErrs)
			for _, err := range errs {
				assert.ErrorIs(t, err, policies.ErrAttachmentNeverApplies)
			}
		})
	}
}
