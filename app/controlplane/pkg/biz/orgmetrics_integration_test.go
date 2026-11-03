//
// Copyright 2024-2026 The Chainloop Authors.
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

	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/biz"
	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/biz/testhelpers"
	"github.com/chainloop-dev/chainloop/pkg/credentials"
	creds "github.com/chainloop-dev/chainloop/pkg/credentials/mocks"
	"github.com/google/uuid"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

// OCI credentials used to create the CAS backend required by workflow runs
const (
	ociTestRepo     = "repo"
	ociTestUsername = "username"
	ociTestPassword = "pass"
)

var ociTestKeypair = &credentials.OCIKeypair{Repo: ociTestRepo, Username: ociTestUsername, Password: ociTestPassword}

func TestOrgMetricsUseCase(t *testing.T) {
	suite.Run(t, new(orgMetricsGetLastWorkflowStatusByRunTestSuite))
	suite.Run(t, new(orgMetricsTopWorkflowsTestSuite))
}

type orgMetricsTopWorkflowsTestSuite struct {
	testhelpers.UseCasesEachTestSuite
	org, otherOrg *biz.Organization
	p1, p2        uuid.UUID
}

// SetupTest creates the following dataset
// org:
//   - wf-a (project p1): 2 success, 1 error runs
//   - wf-b (project p2): 2 success runs
//   - wf-c (project p1): 1 initialized run
//   - wf-deleted (project p1): 5 success runs, soft-deleted afterwards
//
// otherOrg:
//   - wf-other (project p1): 4 success runs
func (s *orgMetricsTopWorkflowsTestSuite) SetupTest() {
	t := s.T()
	ctx := context.Background()

	credsWriter := creds.NewReaderWriter(t)
	credsWriter.On("SaveCredentials", mock.Anything, mock.Anything, ociTestKeypair).Return("stored-OCI-secret", nil)
	s.TestingUseCases = testhelpers.NewTestingUseCases(t, testhelpers.WithCredsReaderWriter(credsWriter))

	var err error
	s.org, err = s.Organization.CreateWithRandomName(ctx)
	require.NoError(t, err)
	s.otherOrg, err = s.Organization.CreateWithRandomName(ctx)
	require.NoError(t, err)

	wfA := s.createWorkflowWithRuns(s.org, "wf-a", "p1", biz.WorkflowRunSuccess, biz.WorkflowRunSuccess, biz.WorkflowRunError)
	wfB := s.createWorkflowWithRuns(s.org, "wf-b", "p2", biz.WorkflowRunSuccess, biz.WorkflowRunSuccess)
	s.createWorkflowWithRuns(s.org, "wf-c", "p1", biz.WorkflowRunInitialized)
	wfDeleted := s.createWorkflowWithRuns(s.org, "wf-deleted", "p1",
		biz.WorkflowRunSuccess, biz.WorkflowRunSuccess, biz.WorkflowRunSuccess, biz.WorkflowRunSuccess, biz.WorkflowRunSuccess)
	s.createWorkflowWithRuns(s.otherOrg, "wf-other", "p1",
		biz.WorkflowRunSuccess, biz.WorkflowRunSuccess, biz.WorkflowRunSuccess, biz.WorkflowRunSuccess)

	require.NoError(t, s.Workflow.Delete(ctx, s.org.ID, wfDeleted.ID.String()))

	s.p1, s.p2 = wfA.ProjectID, wfB.ProjectID
}

// createWorkflowWithRuns creates a workflow in the given org and project, and one run per given status
func (s *orgMetricsTopWorkflowsTestSuite) createWorkflowWithRuns(org *biz.Organization, name, project string, statuses ...biz.WorkflowRunStatus) *biz.Workflow {
	t := s.T()
	ctx := context.Background()

	wf, err := s.Workflow.Create(ctx, &biz.WorkflowCreateOpts{Name: name, Project: project, OrgID: org.ID})
	require.NoError(t, err)

	contractVersion, err := s.WorkflowContract.Describe(ctx, org.ID, wf.ContractID.String(), 0)
	require.NoError(t, err)

	casBackend, err := s.CASBackend.CreateOrUpdate(ctx, org.ID, ociTestRepo, ociTestUsername, ociTestPassword, backendType, true)
	require.NoError(t, err)

	for _, status := range statuses {
		run, err := s.WorkflowRun.Create(ctx, &biz.WorkflowRunCreateOpts{
			WorkflowID: wf.ID.String(), ContractRevision: contractVersion, CASBackendID: casBackend.ID, ProjectVersion: version1,
		})
		require.NoError(t, err)

		if status != biz.WorkflowRunInitialized {
			require.NoError(t, s.WorkflowRun.MarkAsFinished(ctx, run.ID.String(), status, ""))
		}
	}

	return wf
}

func (s *orgMetricsTopWorkflowsTestSuite) TestTopWorkflowsByRunsCount() {
	type want struct {
		name     string
		project  string
		total    int32
		byStatus map[string]int32
	}

	wantA := want{name: "wf-a", project: "p1", total: 3, byStatus: map[string]int32{"success": 2, "error": 1}}
	wantB := want{name: "wf-b", project: "p2", total: 2, byStatus: map[string]int32{"success": 2}}
	wantC := want{name: "wf-c", project: "p1", total: 1, byStatus: map[string]int32{"initialized": 1}}

	currentWindow := &biz.TimeWindow{From: time.Now().Add(-time.Hour), To: time.Now().Add(time.Hour)}

	testCases := []struct {
		name         string
		numWorkflows int
		timeWindow   *biz.TimeWindow
		projectIDs   func() []uuid.UUID
		want         []want
	}{
		{
			name:         "all visible workflows sorted by total, excluding deleted workflows and other orgs",
			numWorkflows: 10,
			want:         []want{wantA, wantB, wantC},
		},
		{
			name:         "limited to the top N",
			numWorkflows: 2,
			want:         []want{wantA, wantB},
		},
		{
			name:         "filtered by visible projects",
			numWorkflows: 10,
			projectIDs:   func() []uuid.UUID { return []uuid.UUID{s.p2} },
			want:         []want{wantB},
		},
		{
			name:         "no visible projects returns nothing",
			numWorkflows: 10,
			projectIDs:   func() []uuid.UUID { return []uuid.UUID{} },
			want:         []want{},
		},
		{
			name:         "time window without runs returns nothing",
			numWorkflows: 10,
			timeWindow:   &biz.TimeWindow{From: time.Now().Add(time.Hour), To: time.Now().Add(2 * time.Hour)},
			want:         []want{},
		},
	}

	for _, tc := range testCases {
		s.Run(tc.name, func() {
			tw := currentWindow
			if tc.timeWindow != nil {
				tw = tc.timeWindow
			}

			var projectIDs []uuid.UUID
			if tc.projectIDs != nil {
				projectIDs = tc.projectIDs()
			}

			got, err := s.OrgMetrics.TopWorkflowsByRunsCount(context.Background(), s.org.ID, tc.numWorkflows, tw, projectIDs)
			s.Require().NoError(err)
			s.Require().Len(got, len(tc.want))

			for i, w := range tc.want {
				s.Equal(w.name, got[i].Workflow.Name)
				s.Equal(w.project, got[i].Workflow.Project)
				s.Equal(w.total, got[i].Total)
				s.Equal(w.byStatus, got[i].ByStatus)
			}
		})
	}
}

type orgMetricsGetLastWorkflowStatusByRunTestSuite struct {
	testhelpers.UseCasesEachTestSuite
	org *biz.Organization
}

func (s *orgMetricsGetLastWorkflowStatusByRunTestSuite) TearDownSubTest() {
	_, _ = s.Data.DB.Project.Delete().Exec(context.Background())
	_, _ = s.Data.DB.Workflow.Delete().Exec(context.Background())
	_, _ = s.Data.DB.WorkflowRun.Delete().Exec(context.Background())
}

func (s *orgMetricsGetLastWorkflowStatusByRunTestSuite) SetupTest() {
	var err error
	assert := assert.New(s.T())

	credsWriter := creds.NewReaderWriter(s.T())
	credsWriter.On("SaveCredentials", mock.Anything, mock.Anything, ociTestKeypair).Return("stored-OCI-secret", nil)

	s.TestingUseCases = testhelpers.NewTestingUseCases(s.T(), testhelpers.WithCredsReaderWriter(credsWriter))

	ctx := context.Background()
	s.org, err = s.Organization.CreateWithRandomName(ctx)
	assert.NoError(err)
}

func (s *orgMetricsGetLastWorkflowStatusByRunTestSuite) TestGetLastWorkflowStatusByRun() {
	s.Run("no workflow runs", func() {
		ctx := context.Background()
		wfs, err := s.TestingUseCases.OrgMetrics.GetLastWorkflowStatusByRun(ctx, s.org.Name)
		assert.NoError(s.T(), err)
		assert.Len(s.T(), wfs, 0)
	})

	s.Run("one workflow with no runs", func() {
		ctx := context.Background()
		_, err := s.Workflow.Create(ctx, &biz.WorkflowCreateOpts{Description: description, Name: "the-name", Team: "the-team", Project: "the-project", OrgID: s.org.ID})
		assert.NoError(s.T(), err)

		_, err = s.TestingUseCases.OrgMetrics.GetLastWorkflowStatusByRun(ctx, s.org.Name)
		assert.NoError(s.T(), err)
	})

	s.Run("one workflow with one run", func() {
		ctx := context.Background()
		w, err := s.Workflow.Create(ctx, &biz.WorkflowCreateOpts{Description: description, Name: "the-name", Team: "the-team", Project: "the-project", OrgID: s.org.ID})
		assert.NoError(s.T(), err)

		// Find contract revision
		contractVersion, err := s.TestingUseCases.WorkflowContract.Describe(ctx, s.org.ID, w.ContractID.String(), 0)
		assert.NoError(s.T(), err)
		assert.NotNil(s.T(), contractVersion)

		// Create a CAS backend
		casBackend, err := s.CASBackend.CreateOrUpdate(ctx, s.org.ID, ociTestRepo, ociTestUsername, ociTestPassword, backendType, true)
		assert.NoError(s.T(), err)
		assert.NotNil(s.T(), casBackend)

		// Create the workflow run
		wfRun, err := s.TestingUseCases.WorkflowRun.Create(ctx,
			&biz.WorkflowRunCreateOpts{
				WorkflowID: w.ID.String(), ContractRevision: contractVersion, CASBackendID: casBackend.ID,
				ProjectVersion: version1,
			})
		assert.NoError(s.T(), err)
		assert.NotNil(s.T(), wfRun)

		// Update the workflow run
		err = s.TestingUseCases.WorkflowRun.MarkAsFinished(ctx, wfRun.ID.String(), biz.WorkflowRunSuccess, "the-logs")
		assert.NoError(s.T(), err)

		wfs, err := s.TestingUseCases.OrgMetrics.GetLastWorkflowStatusByRun(ctx, s.org.Name)
		assert.NoError(s.T(), err)
		assert.Len(s.T(), wfs, 1)
		assert.Equal(s.T(), "success", wfs[0].Status)
	})
}
