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
	"google.golang.org/protobuf/types/known/timestamppb"
)

// TestNewWorkflowCreateOptsToRequest pins that the create options, including the optional
// workflow template reference, are forwarded verbatim to the API request.
func TestNewWorkflowCreateOptsToRequest(t *testing.T) {
	testCases := []struct {
		desc string
		opts *NewWorkflowCreateOpts
	}{
		{
			desc: "without a template reference",
			opts: &NewWorkflowCreateOpts{
				Name:    "my-workflow",
				Project: "my-project",
			},
		},
		{
			desc: "with a template reference",
			opts: &NewWorkflowCreateOpts{
				Name:               "my-workflow",
				Project:            "my-project",
				Team:               "my-team",
				Description:        "a description",
				ContractName:       "my-contract",
				ContractBytes:      []byte("schemaVersion: v1"),
				WorkflowTemplateID: "1b4c6f8a-2d3e-4f5a-8b9c-0d1e2f3a4b5c",
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			got := newWorkflowCreateRequest(tc.opts)

			assert.Equal(t, tc.opts.Name, got.GetName())
			assert.Equal(t, tc.opts.Project, got.GetProjectName())
			assert.Equal(t, tc.opts.Team, got.GetTeam())
			assert.Equal(t, tc.opts.Description, got.GetDescription())
			assert.Equal(t, tc.opts.ContractName, got.GetContractName())
			assert.Equal(t, tc.opts.ContractBytes, got.GetContractBytes())
			assert.Equal(t, tc.opts.WorkflowTemplateID, got.GetWorkflowTemplateId())
		})
	}
}

func TestPbWorkflowItemToActionWorkflowTemplateID(t *testing.T) {
	testCases := []struct {
		desc string
		item *pb.WorkflowItem
		want string
	}{
		{
			desc: "unbound workflow leaves the template reference empty",
			item: &pb.WorkflowItem{
				Id:        "wf-id",
				Name:      "my-workflow",
				CreatedAt: timestamppb.Now(),
			},
			want: "",
		},
		{
			desc: "template-backed workflow carries the template reference",
			item: &pb.WorkflowItem{
				Id:                 "wf-id",
				Name:               "my-workflow",
				CreatedAt:          timestamppb.Now(),
				WorkflowTemplateId: "1b4c6f8a-2d3e-4f5a-8b9c-0d1e2f3a4b5c",
			},
			want: "1b4c6f8a-2d3e-4f5a-8b9c-0d1e2f3a4b5c",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			got := pbWorkflowItemToAction(tc.item)

			assert.Equal(t, tc.want, got.WorkflowTemplateID)
		})
	}
}
