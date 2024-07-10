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

package policies

import (
	"context"
	"testing"

	v12 "github.com/chainloop-dev/chainloop/app/controlplane/api/workflowcontract/v1"
	v1 "github.com/chainloop-dev/chainloop/internal/attestation/crafter/api/attestation/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPolicyVerifier_Verify(t *testing.T) {
	cases := []struct {
		name       string
		state      *v1.CraftingState
		violations int
		wantErr    bool
	}{
		{
			name: "happy path, test attestation properties",
			state: &v1.CraftingState{
				InputSchema: &v12.CraftingSchema{
					Policies: []*v12.PolicyAttachment{
						{Policy: &v12.PolicyAttachment_Ref{Ref: "testdata/workflow.yaml"}},
					},
				},
				Attestation: &v1.Attestation{
					Workflow: &v1.WorkflowMetadata{
						Name: "policytest",
					},
					RunnerType: v12.CraftingSchema_Runner_GITHUB_ACTION,
				},
			},
		},
		{
			name:       "wrong runner",
			violations: 1,
			state: &v1.CraftingState{
				InputSchema: &v12.CraftingSchema{
					Policies: []*v12.PolicyAttachment{
						{Policy: &v12.PolicyAttachment_Ref{Ref: "testdata/workflow.yaml"}},
					},
				},
				Attestation: &v1.Attestation{
					Workflow: &v1.WorkflowMetadata{
						Name: "policytest",
					},
					RunnerType: v12.CraftingSchema_Runner_DAGGER_PIPELINE,
				},
			},
		},
		{
			name:       "missing runner",
			violations: 1,
			state: &v1.CraftingState{
				InputSchema: &v12.CraftingSchema{
					Policies: []*v12.PolicyAttachment{
						{Policy: &v12.PolicyAttachment_Ref{Ref: "testdata/workflow.yaml"}},
					},
				},
				Attestation: &v1.Attestation{
					Workflow: &v1.WorkflowMetadata{
						Name: "policytest",
					},
				},
			},
		},
		{
			name:    "wrong policy",
			wantErr: true,
			state: &v1.CraftingState{
				InputSchema: &v12.CraftingSchema{
					Policies: []*v12.PolicyAttachment{
						{Policy: &v12.PolicyAttachment_Ref{Ref: "testdata/wrong_policy.yaml"}},
					},
				},
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			verifier := NewPolicyVerifier(tc.state, nil)
			res, err := verifier.Verify(context.TODO())
			if tc.wantErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			if tc.violations > 0 {
				assert.Len(t, res, tc.violations)
			}
		})
	}
}
