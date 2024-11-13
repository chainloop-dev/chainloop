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

	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/pagination"

	v1 "github.com/chainloop-dev/chainloop/app/controlplane/api/workflowcontract/v1"
	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/biz"
	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/biz/testhelpers"
	"github.com/chainloop-dev/chainloop/pkg/credentials"
	creds "github.com/chainloop-dev/chainloop/pkg/credentials/mocks"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
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

		c := &v1.CraftingSchema{
			SchemaVersion: "v1",
			Runner:        &v1.CraftingSchema_Runner{Type: v1.CraftingSchema_Runner_CIRCLECI_BUILD},
		}
		rawSchema, err := biz.SchemaToRawContract(c)
		require.NoError(s.T(), err)

		_, err = s.WorkflowContract.Update(ctx, s.org.ID, contract.Name, &biz.WorkflowContractUpdateOpts{RawSchema: rawSchema.Raw})
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
		wf, err := s.Workflow.FindByNameInOrg(context.TODO(), s.org.ID, s.wf.Project, s.wf.Name)
		s.NoError(err)
		s.Equal(s.wf.ID, wf.ID)
	})

	s.Run("fails if workflow belongs to a different org", func() {
		org2, err := s.Organization.CreateWithRandomName(context.Background())
		require.NoError(s.T(), err)

		_, err = s.Workflow.FindByNameInOrg(context.TODO(), org2.ID, s.wf.Project, s.wf.Name)
		s.Error(err)
	})
}

func (s *workflowIntegrationTestSuite) TestCreateDuplicatedName() {
	ctx := context.Background()

	const workflowName = "name"
	existingWorkflow, err := s.Workflow.Create(ctx, &biz.WorkflowCreateOpts{OrgID: s.org.ID, Name: workflowName, Project: "project"})
	require.NoError(s.T(), err)

	s.Run("can't create a workflow with the same name in the same project", func() {
		_, err = s.Workflow.Create(ctx, &biz.WorkflowCreateOpts{OrgID: s.org.ID, Name: workflowName, Project: "project"})
		s.ErrorContains(err, "already exists")
	})

	s.Run("but can do it in another project", func() {
		_, err = s.Workflow.Create(ctx, &biz.WorkflowCreateOpts{OrgID: s.org.ID, Name: workflowName, Project: "another-project"})
		s.NoError(err)
	})

	s.Run("but if we delete it we can", func() {
		err = s.Workflow.Delete(ctx, s.org.ID, existingWorkflow.ID.String())
		require.NoError(s.T(), err)

		_, err = s.Workflow.Create(ctx, &biz.WorkflowCreateOpts{OrgID: s.org.ID, Name: workflowName, Project: "project"})
		require.NoError(s.T(), err)
	})
}

func (s *workflowIntegrationTestSuite) TestCreate() {
	ctx := context.Background()
	const project = "project"
	testCases := []struct {
		name       string
		opts       *biz.WorkflowCreateOpts
		wantErrMsg string
	}{
		{
			name:       "org missing",
			opts:       &biz.WorkflowCreateOpts{Name: "name", Project: project},
			wantErrMsg: "required",
		},
		{
			name:       "name missing",
			opts:       &biz.WorkflowCreateOpts{OrgID: s.org.ID, Project: project},
			wantErrMsg: "required",
		},
		{
			name:       "project missing",
			opts:       &biz.WorkflowCreateOpts{OrgID: s.org.ID, Name: "name"},
			wantErrMsg: "required",
		},
		{
			name:       "invalid name",
			opts:       &biz.WorkflowCreateOpts{OrgID: s.org.ID, Name: "this/not/valid", Project: project},
			wantErrMsg: "RFC 1123",
		},
		{
			name:       "another invalid name",
			opts:       &biz.WorkflowCreateOpts{OrgID: s.org.ID, Name: "this-not Valid", Project: project},
			wantErrMsg: "RFC 1123",
		},
		{
			name:       "invalid project name",
			opts:       &biz.WorkflowCreateOpts{OrgID: s.org.ID, Name: "valid", Project: "this-not Valid"},
			wantErrMsg: "RFC 1123",
		},
		{
			name: "non-existing contract will create it",
			opts: &biz.WorkflowCreateOpts{OrgID: s.org.ID, Name: "name", ContractName: uuid.NewString(), Project: project},
		},
		{
			name: "can create it with just the name, the project and the org",
			opts: &biz.WorkflowCreateOpts{OrgID: s.org.ID, Name: "name2", Project: project},
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
			s.NotEmpty(got.ContractID)
			s.NotEmpty(got.ContractName)
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
	workflow, err := s.Workflow.Create(ctx, &biz.WorkflowCreateOpts{Name: name, OrgID: s.org.ID, Project: project})
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
			id:      uuid.NewString(),
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
			updates: &biz.WorkflowUpdateOpts{Team: toPtrS("new team"), Public: toPtrBool(true)},
			want:    &biz.Workflow{Description: description, Team: "new team", Project: "test-project", Public: true},
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
			updates: &biz.WorkflowUpdateOpts{Team: toPtrS(""), Description: toPtrS("")},
			want:    &biz.Workflow{Team: "", Project: "test-project", Description: ""},
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
				cmpopts.IgnoreFields(biz.Workflow{}, "Name", "CreatedAt", "ID", "OrgID", "ContractName", "ContractID", "ContractRevisionLatest", "ProjectID"),
			); diff != "" {
				s.Failf("mismatch (-want +got):\n%s", diff)
			}

			s.NotEqual(uuid.Nil, got.ProjectID)

			if tc.want.Name != "" {
				s.Equal(tc.want.Name, got.Name)
			} else {
				s.Equal(wfName, got.Name)
			}
		})
	}
}

func (s *workflowListIntegrationTestSuite) TestList() {
	ctx := context.Background()
	const project = "project"
	const team = "team"
	const description = "description"

	s.Run("no workflows", func() {
		workflows, _, err := s.Workflow.List(ctx, s.org.ID, nil, nil)
		s.NoError(err)
		s.Empty(workflows)
	})

	s.Run("list workflows without filters and pagination", func() {
		_, err := s.Workflow.Create(ctx, &biz.WorkflowCreateOpts{OrgID: s.org.ID, Name: "name1", Project: project, Team: team, Description: description})
		require.NoError(s.T(), err)
		_, err = s.Workflow.Create(ctx, &biz.WorkflowCreateOpts{OrgID: s.org.ID, Name: "name2", Project: project, Team: team, Description: description})
		require.NoError(s.T(), err)

		workflows, _, err := s.Workflow.List(ctx, s.org.ID, nil, nil)
		s.NoError(err)
		s.Len(workflows, 2)
	})

	s.Run("list workflows with workflow name filter", func() {
		_, err := s.Workflow.Create(ctx, &biz.WorkflowCreateOpts{OrgID: s.org.ID, Name: "name1", Project: project, Team: team, Description: description})
		require.NoError(s.T(), err)
		_, err = s.Workflow.Create(ctx, &biz.WorkflowCreateOpts{OrgID: s.org.ID, Name: "name2", Project: project, Team: team, Description: description})
		require.NoError(s.T(), err)

		workflows, _, err := s.Workflow.List(ctx, s.org.ID, &biz.WorkflowListOpts{WorkflowName: "name1"}, nil)
		s.NoError(err)
		s.Len(workflows, 1)
	})

	s.Run("list workflows with workflow team filter", func() {
		_, err := s.Workflow.Create(ctx, &biz.WorkflowCreateOpts{OrgID: s.org.ID, Name: "name1", Project: project, Team: team, Description: description})
		require.NoError(s.T(), err)
		_, err = s.Workflow.Create(ctx, &biz.WorkflowCreateOpts{OrgID: s.org.ID, Name: "name2", Project: project, Team: "other-team", Description: description})
		require.NoError(s.T(), err)

		workflows, _, err := s.Workflow.List(ctx, s.org.ID, &biz.WorkflowListOpts{WorkflowTeam: team}, nil)
		s.NoError(err)
		s.Len(workflows, 2)
	})

	s.Run("list workflows with workflow project name filter", func() {
		_, err := s.Workflow.Create(ctx, &biz.WorkflowCreateOpts{OrgID: s.org.ID, Name: "name1", Project: project, Team: team, Description: description})
		require.NoError(s.T(), err)
		_, err = s.Workflow.Create(ctx, &biz.WorkflowCreateOpts{OrgID: s.org.ID, Name: "name2", Project: "other-project", Team: team, Description: description})
		require.NoError(s.T(), err)

		workflows, _, err := s.Workflow.List(ctx, s.org.ID, &biz.WorkflowListOpts{WorkflowProjectNames: []string{"other-project"}}, nil)
		s.NoError(err)
		s.Len(workflows, 1)
	})

	s.Run("list workflows with workflow public filter", func() {
		_, err := s.Workflow.Create(ctx, &biz.WorkflowCreateOpts{OrgID: s.org.ID, Name: "name1", Project: project, Team: team, Description: description, Public: true})
		require.NoError(s.T(), err)
		_, err = s.Workflow.Create(ctx, &biz.WorkflowCreateOpts{OrgID: s.org.ID, Name: "name2", Project: project, Team: team, Description: description, Public: false})
		require.NoError(s.T(), err)

		workflows, _, err := s.Workflow.List(ctx, s.org.ID, &biz.WorkflowListOpts{WorkflowPublic: toPtrBool(true)}, nil)
		s.NoError(err)
		s.Len(workflows, 1)
	})

	s.Run("list workflows with workflow run runner type filter", func() {
		_, err := s.Workflow.Create(ctx, &biz.WorkflowCreateOpts{OrgID: s.org.ID, Name: "name1", Project: project, Team: team, Description: description})
		require.NoError(s.T(), err)
		w, err := s.Workflow.Create(ctx, &biz.WorkflowCreateOpts{OrgID: s.org.ID, Name: "name2", Project: project, Team: team, Description: description})
		require.NoError(s.T(), err)

		// Find contract revision
		contractVersion, err := s.TestingUseCases.WorkflowContract.Describe(ctx, s.org.ID, w.ContractID.String(), 0)
		assert.NoError(s.T(), err)
		assert.NotNil(s.T(), contractVersion)

		// Create a CAS backend
		casBackend, err := s.TestingUseCases.CASBackend.CreateOrUpdate(ctx, s.org.ID, "repo", "username", "pass", backendType, true)
		assert.NoError(s.T(), err)
		assert.NotNil(s.T(), casBackend)

		// Create the workflow run
		wfRun, err := s.TestingUseCases.WorkflowRun.Create(ctx,
			&biz.WorkflowRunCreateOpts{
				WorkflowID: w.ID.String(), ContractRevision: contractVersion, CASBackendID: casBackend.ID,
				ProjectVersion: version1, RunnerType: "GITHUB_ACTION",
			})
		assert.NoError(s.T(), err)
		assert.NotNil(s.T(), wfRun)

		// Update the workflow run
		err = s.TestingUseCases.WorkflowRun.MarkAsFinished(ctx, wfRun.ID.String(), biz.WorkflowRunSuccess, "the-logs")
		assert.NoError(s.T(), err)

		workflows, _, err := s.Workflow.List(ctx, s.org.ID, &biz.WorkflowListOpts{WorkflowRunRunnerType: "GITHUB_ACTION"}, nil)
		s.NoError(err)
		s.Len(workflows, 1)
	})

	s.Run("list workflow with pagination", func() {
		_, err := s.Workflow.Create(ctx, &biz.WorkflowCreateOpts{OrgID: s.org.ID, Name: "name1", Project: project, Team: team, Description: description})
		require.NoError(s.T(), err)
		_, err = s.Workflow.Create(ctx, &biz.WorkflowCreateOpts{OrgID: s.org.ID, Name: "name2", Project: project, Team: team, Description: description})
		require.NoError(s.T(), err)

		paginationOpts, err := pagination.NewOffsetPaginationOpts(0, 1)
		require.NoError(s.T(), err)

		workflows, count, err := s.Workflow.List(ctx, s.org.ID, nil, paginationOpts)
		s.NoError(err)
		s.Len(workflows, 1)
		s.Equal(2, count)
	})
}

// Run the tests
func TestWorkflowUseCase(t *testing.T) {
	suite.Run(t, new(workflowIntegrationTestSuite))
	suite.Run(t, new(workflowListIntegrationTestSuite))
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

// Utility struct to hold the test suite
type workflowListIntegrationTestSuite struct {
	testhelpers.UseCasesEachTestSuite
	org *biz.Organization
}

func (s *workflowListIntegrationTestSuite) SetupTest() {
	var err error
	assert := assert.New(s.T())

	credsWriter := creds.NewReaderWriter(s.T())
	credsWriter.On("SaveCredentials", context.Background(), mock.Anything, &credentials.OCIKeypair{Repo: "repo", Username: "username", Password: "pass"}).Return("stored-OCI-secret", nil)

	s.TestingUseCases = testhelpers.NewTestingUseCases(s.T(), testhelpers.WithCredsReaderWriter(credsWriter))

	ctx := context.Background()
	s.org, err = s.Organization.CreateWithRandomName(ctx)
	assert.NoError(err)
}

func (s *workflowListIntegrationTestSuite) TearDownSubTest() {
	_, _ = s.Data.DB.Workflow.Delete().Exec(context.Background())
	_, _ = s.Data.DB.Project.Delete().Exec(context.Background())
	_, _ = s.Data.DB.WorkflowRun.Delete().Exec(context.Background())
	_, _ = s.Data.DB.CASBackend.Delete().Exec(context.Background())
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
