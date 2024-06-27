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

package biz_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	v1 "github.com/chainloop-dev/chainloop/app/controlplane/api/workflowcontract/v1"
	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/biz"
	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/biz/testhelpers"
	"github.com/docker/distribution/uuid"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

func (s *workflowIntegrationTestSuite) TestContractLatestAvailable() {
	ctx := context.Background()
	var workflow *biz.Workflow
	var err error
	s.Run("by default is 1", func() {
		workflow, err = s.Workflow.Create(ctx, &biz.WorkflowCreateOpts{
			Description: description, Name: "name", Team: "team", Project: "project", OrgID: s.org.ID})
		require.NoError(s.T(), err)
		s.Equal(1, workflow.ContractRevisionLatest)
	})

	s.Run("it will increment if the contract is updated", func() {
		contract, err := s.WorkflowContract.FindByIDInOrg(ctx, s.org.ID, workflow.ContractID.String())
		require.NoError(s.T(), err)

		_, err = s.WorkflowContract.Update(ctx, s.org.ID, contract.Name,
			&biz.WorkflowContractUpdateOpts{Schema: &v1.CraftingSchema{
				Runner: &v1.CraftingSchema_Runner{Type: v1.CraftingSchema_Runner_CIRCLECI_BUILD},
			}})
		s.NoError(err)

		workflow, err := s.Workflow.FindByID(ctx, workflow.ID.String())
		s.NoError(err)
		s.Equal(2, workflow.ContractRevisionLatest)
	})
}

func (s *workflowIntegrationTestSuite) TestView() {
	s.Run("finds by id in org", func() {
		wf, err := s.Workflow.FindByIDInOrg(context.TODO(), s.org.ID, s.wf.ID.String())
		s.NoError(err)
		s.Equal(s.wf.ID, wf.ID)
	})

	s.Run("finds by name in org", func() {
		wf, err := s.Workflow.FindByNameInOrg(context.TODO(), s.org.ID, s.wf.Name)
		s.NoError(err)
		s.Equal(s.wf.ID, wf.ID)
	})

	s.Run("fails if workflow belongs to a different org", func() {
		org2, err := s.Organization.CreateWithRandomName(context.Background())
		require.NoError(s.T(), err)

		_, err = s.Workflow.FindByNameInOrg(context.TODO(), org2.ID, s.wf.Name)
		s.Error(err)
	})
}

func (s *workflowIntegrationTestSuite) TestCreateDuplicatedName() {
	ctx := context.Background()

	const workflowName = "name"
	existingWorkflow, err := s.Workflow.Create(ctx, &biz.WorkflowCreateOpts{OrgID: s.org.ID, Name: workflowName})
	require.NoError(s.T(), err)

	s.Run("can't create a workflow with the same name", func() {
		_, err = s.Workflow.Create(ctx, &biz.WorkflowCreateOpts{OrgID: s.org.ID, Name: workflowName})
		s.ErrorContains(err, "name already taken")
	})

	s.Run("but if we delete it we can", func() {
		err = s.Workflow.Delete(ctx, s.org.ID, existingWorkflow.ID.String())
		require.NoError(s.T(), err)

		_, err = s.Workflow.Create(ctx, &biz.WorkflowCreateOpts{OrgID: s.org.ID, Name: workflowName})
		require.NoError(s.T(), err)
	})
}

func (s *workflowIntegrationTestSuite) TestCreate() {
	ctx := context.Background()
	testCases := []struct {
		name       string
		opts       *biz.WorkflowCreateOpts
		wantErrMsg string
	}{
		{
			name:       "org missing",
			opts:       &biz.WorkflowCreateOpts{Name: "name"},
			wantErrMsg: "required",
		},
		{
			name:       "name missing",
			opts:       &biz.WorkflowCreateOpts{OrgID: s.org.ID},
			wantErrMsg: "required",
		},
		{
			name:       "invalid name",
			opts:       &biz.WorkflowCreateOpts{OrgID: s.org.ID, Name: "this/not/valid"},
			wantErrMsg: "RFC 1123",
		},
		{
			name:       "another invalid name",
			opts:       &biz.WorkflowCreateOpts{OrgID: s.org.ID, Name: "this-not Valid"},
			wantErrMsg: "RFC 1123",
		},
		{
			name:       "invalid project name",
			opts:       &biz.WorkflowCreateOpts{OrgID: s.org.ID, Name: "valid", Project: "this-not Valid"},
			wantErrMsg: "RFC 1123",
		},
		{
			name: "non-existing contract will create it",
			opts: &biz.WorkflowCreateOpts{OrgID: s.org.ID, Name: "name", ContractName: uuid.Generate().String()},
		},
		{
			name: "can create it with just the name and the org",
			opts: &biz.WorkflowCreateOpts{OrgID: s.org.ID, Name: "name2"},
		},
		{
			name: "with all items",
			opts: &biz.WorkflowCreateOpts{OrgID: s.org.ID, Name: "another-name", Project: "project", Team: "team", Description: "description"},
		},
	}

	for _, tc := range testCases {
		s.Run(tc.name, func() {
			got, err := s.Workflow.Create(ctx, tc.opts)
			if tc.wantErrMsg != "" {
				s.ErrorContains(err, tc.wantErrMsg)
				return
			}

			require.NoError(s.T(), err)
			s.NotEmpty(got.ID)
			s.NotEmpty(got.CreatedAt)
			s.Equal(tc.opts.Name, got.Name)
			s.Equal(tc.opts.Description, got.Description)
			s.Equal(tc.opts.Team, got.Team)
			s.Equal(tc.opts.Project, got.Project)
		})
	}
}

func (s *workflowIntegrationTestSuite) TestUpdate() {
	ctx := context.Background()
	const (
		name        = "test-workflow"
		team        = "test team"
		project     = "test-project"
		description = "test description"
	)

	org2, err := s.Organization.CreateWithRandomName(context.Background())
	require.NoError(s.T(), err)
	workflow, err := s.Workflow.Create(ctx, &biz.WorkflowCreateOpts{Name: name, OrgID: s.org.ID})
	require.NoError(s.T(), err)

	// Create two contracts in two different orgs
	contract1, err := s.WorkflowContract.Create(ctx, &biz.WorkflowContractCreateOpts{Name: "contract-1", OrgID: s.org.ID})
	require.NoError(s.T(), err)
	contract2, err := s.WorkflowContract.Create(ctx, &biz.WorkflowContractCreateOpts{Name: "contract-2", OrgID: org2.ID})
	require.NoError(s.T(), err)

	s.Run("by default the workflow is private", func() {
		s.False(workflow.Public)
	})

	s.Run("can't update if no changes are provided", func() {
		got, err := s.Workflow.Update(ctx, org2.ID, workflow.ID.String(), nil)
		s.True(biz.IsErrValidation(err))
		s.Error(err)
		s.Nil(got)
	})

	s.Run("can't update a workflow in another org", func() {
		got, err := s.Workflow.Update(ctx, org2.ID, workflow.ID.String(), &biz.WorkflowUpdateOpts{Description: toPtrS("new description")})
		s.True(biz.IsNotFound(err))
		s.Error(err)
		s.Nil(got)
	})

	testCases := []struct {
		name string
		// if not set, it will use the workflow we create on each run
		id         string
		updates    *biz.WorkflowUpdateOpts
		want       *biz.Workflow
		wantErr    bool
		wantErrMsg string
	}{
		{
			name:    "non existing workflow",
			id:      uuid.Generate().String(),
			updates: &biz.WorkflowUpdateOpts{Description: toPtrS("new description")},
			wantErr: true,
		},
		{
			name:    "invalid uuid",
			id:      "deadbeef",
			updates: &biz.WorkflowUpdateOpts{Description: toPtrS("new description")},
			wantErr: true,
		},
		{
			name:       "no updates",
			wantErr:    true,
			wantErrMsg: "no updates provided",
		},
		{
			name:       "invalid Project",
			wantErr:    true,
			wantErrMsg: "RFC 1123",
			updates:    &biz.WorkflowUpdateOpts{Project: toPtrS(" no no ")},
		},
		{
			name:    "update description",
			updates: &biz.WorkflowUpdateOpts{Description: toPtrS("new description")},
			want:    &biz.Workflow{Description: "new description", Team: team, Project: project, Public: false},
		},
		{
			name:    "update visibility",
			updates: &biz.WorkflowUpdateOpts{Public: toPtrBool(true)},
			want:    &biz.Workflow{Description: description, Team: team, Project: project, Public: true},
		},
		{
			name:    "update all options",
			updates: &biz.WorkflowUpdateOpts{Project: toPtrS("new-project"), Team: toPtrS("new team"), Public: toPtrBool(true)},
			want:    &biz.Workflow{Description: description, Team: "new team", Project: "new-project", Public: true},
		},
		{
			name:    "can update contract",
			updates: &biz.WorkflowUpdateOpts{ContractID: toPtrS(contract1.ID.String())},
			want:    &biz.Workflow{Description: description, Team: team, Project: project, ContractID: contract2.ID},
		},
		{
			name:    "can not update contract in another org",
			updates: &biz.WorkflowUpdateOpts{ContractID: toPtrS(contract2.ID.String())},
			wantErr: true,
		},
		{
			name:    "but other opts can",
			updates: &biz.WorkflowUpdateOpts{Team: toPtrS(""), Project: toPtrS(""), Description: toPtrS("")},
			want:    &biz.Workflow{Team: "", Project: "", Description: ""},
		},
	}

	for i, tc := range testCases {
		s.Run(tc.name, func() {
			wfName := fmt.Sprintf("%s-%d", name, i)
			workflow, err := s.Workflow.Create(ctx, &biz.WorkflowCreateOpts{Description: description, Name: wfName, Team: team, Project: project, OrgID: s.org.ID})
			require.NoError(s.T(), err)

			workflowID := tc.id
			if workflowID == "" {
				workflowID = workflow.ID.String()
			}

			got, err := s.Workflow.Update(ctx, s.org.ID, workflowID, tc.updates)
			if tc.wantErr {
				s.Error(err)
				if tc.wantErrMsg != "" {
					s.Contains(err.Error(), tc.wantErrMsg)
				}

				return
			}
			s.NoError(err)

			if diff := cmp.Diff(tc.want, got,
				cmpopts.IgnoreFields(biz.Workflow{}, "Name", "CreatedAt", "ID", "OrgID", "ContractID", "ContractRevisionLatest"),
			); diff != "" {
				s.Failf("mismatch (-want +got):\n%s", diff)
			}

			if tc.want.Name != "" {
				s.Equal(tc.want.Name, got.Name)
			} else {
				s.Equal(wfName, got.Name)
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
	wf  *biz.Workflow
}

func (s *workflowIntegrationTestSuite) SetupTest() {
	var err error
	assert := assert.New(s.T())
	s.TestingUseCases = testhelpers.NewTestingUseCases(s.T())

	ctx := context.Background()
	s.org, err = s.Organization.CreateWithRandomName(ctx)
	assert.NoError(err)

	s.wf, err = s.Workflow.Create(ctx, &biz.WorkflowCreateOpts{
		Name:    "my-workflow",
		Project: "my-project",
		OrgID:   s.org.ID,
	})
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
