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

package v1_test

import (
	"testing"

	"buf.build/go/protovalidate"
	v1 "github.com/chainloop-dev/chainloop/app/controlplane/api/controlplane/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Spec issue-3644: the level of an update request is optional. When it is set, it is a project
// reference or an organization level that must be true.
func TestWorkflowContractServiceUpdateRequestScope(t *testing.T) {
	const contractName = "my-contract"
	projectName := "my-project"

	testCases := []struct {
		desc    string
		req     *v1.WorkflowContractServiceUpdateRequest
		wantErr string
	}{
		{
			desc: "no scope keeps the level of the contract",
			req:  &v1.WorkflowContractServiceUpdateRequest{Name: contractName},
		},
		{
			desc: "organization scoped set to true",
			req:  &v1.WorkflowContractServiceUpdateRequest{Name: contractName, Scope: &v1.WorkflowContractServiceUpdateRequest_OrganizationScoped{OrganizationScoped: true}},
		},
		{
			desc:    "organization scoped set to false is rejected",
			req:     &v1.WorkflowContractServiceUpdateRequest{Name: contractName, Scope: &v1.WorkflowContractServiceUpdateRequest_OrganizationScoped{OrganizationScoped: false}},
			wantErr: "organization_scoped",
		},
		{
			desc: "a project reference",
			req:  &v1.WorkflowContractServiceUpdateRequest{Name: contractName, Scope: &v1.WorkflowContractServiceUpdateRequest_ProjectReference{ProjectReference: &v1.IdentityReference{Name: &projectName}}},
		},
		{
			desc:    "an empty project reference is rejected",
			req:     &v1.WorkflowContractServiceUpdateRequest{Name: contractName, Scope: &v1.WorkflowContractServiceUpdateRequest_ProjectReference{ProjectReference: &v1.IdentityReference{}}},
			wantErr: "project_reference",
		},
	}

	validator, err := protovalidate.New()
	require.NoError(t, err)

	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			err := validator.Validate(tc.req)
			if tc.wantErr != "" {
				assert.ErrorContains(t, err, tc.wantErr)
				return
			}

			assert.NoError(t, err)
		})
	}
}
