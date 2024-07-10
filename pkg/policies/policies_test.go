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
	"io/fs"
	"testing"

	v12 "github.com/chainloop-dev/chainloop/app/controlplane/api/workflowcontract/v1"
	v1 "github.com/chainloop-dev/chainloop/internal/attestation/crafter/api/attestation/v1"
	"github.com/stretchr/testify/suite"
)

func (s *testSuite) TestVerifyAttestations() {
	cases := []struct {
		name       string
		state      *v1.CraftingState
		violations int
		wantErr    error
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
			wantErr: &fs.PathError{},
			state: &v1.CraftingState{
				InputSchema: &v12.CraftingSchema{
					Policies: []*v12.PolicyAttachment{
						{Policy: &v12.PolicyAttachment_Ref{Ref: "testdata/wrong_policy.yaml"}},
					},
				},
			},
		},
		{
			name:    "missing rego policy",
			wantErr: &fs.PathError{},
			state: &v1.CraftingState{
				InputSchema: &v12.CraftingSchema{
					Policies: []*v12.PolicyAttachment{
						{Policy: &v12.PolicyAttachment_Ref{Ref: "testdata/missing_rego.yaml"}},
					},
				},
			},
		},
	}

	for _, tc := range cases {
		s.Run(tc.name, func() {
			verifier := NewPolicyVerifier(tc.state, nil)
			res, err := verifier.Verify(context.TODO())
			if tc.wantErr != nil {
				// #nosec G601
				s.ErrorAs(err, &tc.wantErr)
				return
			}
			s.Require().NoError(err)
			if tc.violations > 0 {
				s.Len(res, tc.violations)
			}
		})
	}
}

func (s *testSuite) TestAttestationResult() {
	state := &v1.CraftingState{
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
	}

	verifier := NewPolicyVerifier(state, nil)
	res, err := verifier.Verify(context.TODO())
	s.Require().NoError(err)
	s.Len(res, 0)

	att := state.GetAttestation()
	s.Len(att.Policies, 1)

	p := att.Policies[0]
	s.Len(p.Violations, 0)
	s.Equal("testdata/workflow.yaml", p.Attachment.GetRef())
	s.Equal("workflow", p.Name)
	s.Contains(p.Body, "package main")
}

type testSuite struct {
	suite.Suite
}

func TestPolicyVerifier(t *testing.T) {
	suite.Run(t, new(testSuite))
}
