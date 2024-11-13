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
	"testing"

	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/biz"
	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/biz/testhelpers"
	"github.com/chainloop-dev/chainloop/pkg/credentials"
	creds "github.com/chainloop-dev/chainloop/pkg/credentials/mocks"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
)

func TestOrgMetricsUseCase(t *testing.T) {
	suite.Run(t, new(orgMetricsGetLastWorkflowStatusByRunTestSuite))
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
	credsWriter.On("SaveCredentials", context.Background(), mock.Anything, &credentials.OCIKeypair{Repo: "repo", Username: "username", Password: "pass"}).Return("stored-OCI-secret", nil)

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
		casBackend, err := s.TestingUseCases.CASBackend.CreateOrUpdate(ctx, s.org.ID, "repo", "username", "pass", backendType, true)
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
