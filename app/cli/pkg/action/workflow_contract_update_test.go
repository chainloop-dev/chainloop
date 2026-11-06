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

package action

import (
	"testing"

	pb "github.com/chainloop-dev/chainloop/app/controlplane/api/controlplane/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
)

func TestWorkflowContractUpdateScopeApply(t *testing.T) {
	projectName := "contract-project"

	testCases := []struct {
		name    string
		scope   WorkflowContractUpdateScope
		want    *pb.WorkflowContractServiceUpdateRequest
		wantErr string
	}{
		{
			name: "no scope keeps the level",
			want: &pb.WorkflowContractServiceUpdateRequest{},
		},
		{
			name:  "to a project",
			scope: WorkflowContractUpdateScope{ProjectName: projectName},
			want: &pb.WorkflowContractServiceUpdateRequest{
				Scope: &pb.WorkflowContractServiceUpdateRequest_ProjectReference{ProjectReference: &pb.IdentityReference{Name: &projectName}},
			},
		},
		{
			name:  "to the organization",
			scope: WorkflowContractUpdateScope{Organization: true},
			want: &pb.WorkflowContractServiceUpdateRequest{
				Scope: &pb.WorkflowContractServiceUpdateRequest_OrganizationScoped{OrganizationScoped: true},
			},
		},
		{
			name:    "both",
			scope:   WorkflowContractUpdateScope{ProjectName: projectName, Organization: true},
			wantErr: "only one",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			req := &pb.WorkflowContractServiceUpdateRequest{}
			err := tc.scope.apply(req)
			if tc.wantErr != "" {
				assert.ErrorContains(t, err, tc.wantErr)
				return
			}

			require.NoError(t, err)
			assert.True(t, proto.Equal(tc.want, req), "got %v", req)
		})
	}
}
