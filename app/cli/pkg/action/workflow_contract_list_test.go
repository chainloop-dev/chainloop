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

func TestPbWorkflowContractItemToAction(t *testing.T) {
	now := timestamppb.Now()

	tests := []struct {
		name          string
		input         *pb.WorkflowContractItem
		expectedScope string
	}{
		{
			name: "global scope when scoped entity is nil",
			input: &pb.WorkflowContractItem{
				Id:                      "contract-id",
				Name:                    "my-contract",
				LatestRevision:          3,
				CreatedAt:               now,
				LatestRevisionCreatedAt: now,
			},
			expectedScope: "global",
		},
		{
			name: "project scope when scoped entity is set",
			input: &pb.WorkflowContractItem{
				Id:                      "contract-id",
				Name:                    "my-contract",
				LatestRevision:          1,
				CreatedAt:               now,
				LatestRevisionCreatedAt: now,
				ScopedEntity: &pb.ScopedEntity{
					Type: "project",
					Id:   "project-id",
					Name: "my-project",
				},
			},
			expectedScope: "project/my-project",
		},
		{
			name: "workflow refs are converted",
			input: &pb.WorkflowContractItem{
				Id:                      "contract-id",
				Name:                    "my-contract",
				CreatedAt:               now,
				LatestRevisionCreatedAt: now,
				WorkflowRefs: []*pb.WorkflowRef{
					{Id: "wf-1", Name: "build", ProjectName: "proj-a"},
					{Id: "wf-2", Name: "deploy", ProjectName: "proj-b"},
				},
			},
			expectedScope: "global",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := pbWorkflowContractItemToAction(tc.input)

			assert.Equal(t, tc.input.GetName(), result.Name)
			assert.Equal(t, tc.input.GetId(), result.ID)
			assert.Equal(t, int(tc.input.GetLatestRevision()), result.LatestRevision)
			assert.Equal(t, tc.expectedScope, result.Scope)

			if tc.input.ScopedEntity != nil {
				assert.NotNil(t, result.ScopedEntity)
				assert.Equal(t, tc.input.ScopedEntity.Type, result.ScopedEntity.Type)
				assert.Equal(t, tc.input.ScopedEntity.Id, result.ScopedEntity.ID)
				assert.Equal(t, tc.input.ScopedEntity.Name, result.ScopedEntity.Name)
			} else {
				assert.Nil(t, result.ScopedEntity)
			}

			assert.Len(t, result.WorkflowRefs, len(tc.input.WorkflowRefs))
		})
	}
}
