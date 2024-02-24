//
// Copyright 2023 The Chainloop Authors.
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

package biz_test

import (
	"context"
	"testing"
	"time"

	"github.com/chainloop-dev/chainloop/app/controlplane/internal/biz"
	"github.com/chainloop-dev/chainloop/app/controlplane/internal/biz/testhelpers"
	"github.com/docker/distribution/uuid"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

func (s *workflowIntegrationTestSuite) TestUpdate() {
	ctx := context.Background()
	const (
		name        = "test workflow"
		team        = "test team"
		project     = "test project"
		description = "test description"
	)

	org2, err := s.Organization.CreateWithRandomName(context.Background())
	require.NoError(s.T(), err)
	workflow, err := s.Workflow.Create(ctx, &biz.WorkflowCreateOpts{Name: name, OrgID: s.org.ID})
	require.NoError(s.T(), err)

	s.Run("by default the workflow is private", func() {
		s.False(workflow.Public)
	})

	s.Run("can't update a workflow in another org", func() {
		got, err := s.Workflow.Update(ctx, org2.ID, workflow.ID.String(), nil)
		s.True(biz.IsNotFound(err))
		s.Error(err)
		s.Nil(got)
	})

	testCases := []struct {
		name string
		// if not set, it will use the workflow we create on each run
		id      string
		updates *biz.WorkflowUpdateOpts
		want    *biz.Workflow
		wantErr bool
	}{
		{
			name:    "non existing workflow",
			id:      uuid.Generate().String(),
			updates: &biz.WorkflowUpdateOpts{Name: toPtrS("new name")},
			wantErr: true,
		},
		{
			name:    "invalid uuid",
			id:      "deadbeef",
			updates: &biz.WorkflowUpdateOpts{Name: toPtrS("new name")},
			wantErr: true,
		},
		{
			name: "no updates",
			want: &biz.Workflow{Name: name, Team: team, Project: project, Public: false, Description: description},
		},
		{
			name:    "update name",
			updates: &biz.WorkflowUpdateOpts{Name: toPtrS("new name")},
			want:    &biz.Workflow{Name: "new name", Description: description, Team: team, Project: project, Public: false},
		},
		{
			name:    "update description",
			updates: &biz.WorkflowUpdateOpts{Description: toPtrS("new description")},
			want:    &biz.Workflow{Name: name, Description: "new description", Team: team, Project: project, Public: false},
		},
		{
			name:    "update visibility",
			updates: &biz.WorkflowUpdateOpts{Public: toPtrBool(true)},
			want:    &biz.Workflow{Name: name, Description: description, Team: team, Project: project, Public: true},
		},
		{
			name:    "update all options",
			updates: &biz.WorkflowUpdateOpts{Name: toPtrS("new name"), Project: toPtrS("new project"), Team: toPtrS("new team"), Public: toPtrBool(true)},
			want:    &biz.Workflow{Name: "new name", Description: description, Team: "new team", Project: "new project", Public: true},
		},
		{
			name:    "name can't be emptied",
			updates: &biz.WorkflowUpdateOpts{Name: toPtrS("")},
			want:    &biz.Workflow{Name: name, Team: team, Project: project, Description: description},
		},
		{
			name:    "but other opts can",
			updates: &biz.WorkflowUpdateOpts{Team: toPtrS(""), Project: toPtrS(""), Description: toPtrS("")},
			want:    &biz.Workflow{Name: name, Team: "", Project: "", Description: ""},
		},
	}

	for _, tc := range testCases {
		s.Run(tc.name, func() {
			workflow, err := s.Workflow.Create(ctx, &biz.WorkflowCreateOpts{Description: description, Name: name, Team: team, Project: project, OrgID: s.org.ID})
			require.NoError(s.T(), err)

			workflowID := tc.id
			if workflowID == "" {
				workflowID = workflow.ID.String()
			}

			got, err := s.Workflow.Update(ctx, s.org.ID, workflowID, tc.updates)
			if tc.wantErr {
				s.Error(err)
				return
			}
			s.NoError(err)

			if diff := cmp.Diff(tc.want, got,
				cmpopts.IgnoreFields(biz.Workflow{}, "CreatedAt", "ID", "OrgID", "ContractID", "LatestContractRevision"),
			); diff != "" {
				s.Failf("mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

// Run the tests
func TestWorkflowUseCase(t *testing.T) {
	suite.Run(t, new(workflowIntegrationTestSuite))
}

// Utility struct to hold the test suite
type workflowIntegrationTestSuite struct {
	testhelpers.UseCasesEachTestSuite
	org *biz.Organization
}

func (s *workflowIntegrationTestSuite) SetupTest() {
	var err error
	assert := assert.New(s.T())
	s.TestingUseCases = testhelpers.NewTestingUseCases(s.T())

	ctx := context.Background()
	s.org, err = s.Organization.CreateWithRandomName(ctx)
	assert.NoError(err)
}

func toPtrS(s string) *string {
	return &s
}

func toPtrBool(b bool) *bool {
	return &b
}

func toPtrDuration(d time.Duration) *time.Duration {
	return &d
}
