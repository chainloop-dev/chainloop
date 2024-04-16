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

	schemav1 "github.com/chainloop-dev/chainloop/app/controlplane/api/workflowcontract/v1"
	"github.com/chainloop-dev/chainloop/app/controlplane/internal/biz"
	"github.com/chainloop-dev/chainloop/app/controlplane/internal/biz/testhelpers"
	"github.com/chainloop-dev/chainloop/app/controlplane/internal/pagination"
	"github.com/chainloop-dev/chainloop/internal/credentials"
	creds "github.com/chainloop-dev/chainloop/internal/credentials/mocks"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/google/uuid"
	"github.com/secure-systems-lab/go-securesystemslib/dsse"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
)

func (s *workflowRunIntegrationTestSuite) TestList() {
	// Create a finished run
	finishedRun, err := s.WorkflowRun.Create(context.Background(),
		&biz.WorkflowRunCreateOpts{
			WorkflowID: s.workflowOrg2.ID.String(), RobotaccountID: s.robotAccount.ID.String(), ContractRevision: s.contractVersion, CASBackendID: s.casBackend.ID,
		})
	s.NoError(err)
	err = s.WorkflowRun.MarkAsFinished(context.Background(), finishedRun.ID.String(), biz.WorkflowRunSuccess, "")
	s.NoError(err)

	testCases := []struct {
		name    string
		filters *biz.RunListFilters
		want    []*biz.WorkflowRun
		wantErr bool
	}{
		{
			name:    "no filters",
			filters: &biz.RunListFilters{},
			want:    []*biz.WorkflowRun{s.runOrg2, s.runOrg2Public, finishedRun},
		},
		{
			name:    "filter by workflow",
			filters: &biz.RunListFilters{WorkflowID: s.workflowOrg2.ID},
			want:    []*biz.WorkflowRun{s.runOrg2, finishedRun},
		},
		{
			name:    "filter by status, no result",
			filters: &biz.RunListFilters{Status: biz.WorkflowRunCancelled},
			want:    []*biz.WorkflowRun{},
		},
		{
			name:    "filter by status, 2 results",
			filters: &biz.RunListFilters{Status: biz.WorkflowRunInitialized},
			want:    []*biz.WorkflowRun{s.runOrg2, s.runOrg2Public},
		},
		{
			name:    "filter by finished state and workflow with results",
			filters: &biz.RunListFilters{Status: biz.WorkflowRunSuccess, WorkflowID: s.workflowOrg2.ID},
			want:    []*biz.WorkflowRun{finishedRun},
		},
	}

	for _, tc := range testCases {
		s.T().Run(tc.name, func(t *testing.T) {
			got, _, err := s.WorkflowRun.List(context.Background(), s.org2.ID, tc.filters, &pagination.CursorOptions{Limit: 10})
			if tc.wantErr {
				s.Error(err)
				return
			}

			s.NoError(err)
			s.Len(got, len(tc.want))
			gotIDs := make([]uuid.UUID, len(got))
			for _, g := range got {
				gotIDs = append(gotIDs, g.ID)
			}

			wantIDs := make([]uuid.UUID, len(tc.want))
			for _, w := range tc.want {
				wantIDs = append(wantIDs, w.ID)
			}

			s.ElementsMatch(wantIDs, gotIDs)
		})
	}
}

func (s *workflowRunIntegrationTestSuite) TestSaveAttestation() {
	assert := assert.New(s.T())
	ctx := context.Background()

	validEnvelope := &dsse.Envelope{}

	s.T().Run("non existing workflowRun", func(t *testing.T) {
		_, err := s.WorkflowRun.SaveAttestation(ctx, uuid.NewString(), validEnvelope)
		assert.Error(err)
		assert.True(biz.IsNotFound(err))
	})

	s.T().Run("valid workflowRun", func(t *testing.T) {
		run, err := s.WorkflowRun.Create(ctx, &biz.WorkflowRunCreateOpts{
			WorkflowID: s.workflowOrg1.ID.String(), RobotaccountID: s.robotAccount.ID.String(), ContractRevision: s.contractVersion, CASBackendID: s.casBackend.ID,
		})
		assert.NoError(err)

		d, err := s.WorkflowRun.SaveAttestation(ctx, run.ID.String(), validEnvelope)
		assert.NoError(err)
		wantDigest := "sha256:f845058d865c3d4d491c9019f6afe9c543ad2cd11b31620cc512e341fb03d3d8"
		assert.Equal(wantDigest, d)

		// Retrieve attestation ref from storage and compare
		r, err := s.WorkflowRun.GetByIDInOrgOrPublic(ctx, s.org.ID, run.ID.String())
		assert.NoError(err)
		assert.Equal(r.Attestation, &biz.Attestation{Envelope: validEnvelope, Digest: wantDigest})
	})
}

func (s *workflowRunIntegrationTestSuite) TestGetByIDInOrgOrPublic() {
	assert := assert.New(s.T())
	ctx := context.Background()
	testCases := []struct {
		name    string
		orgID   string
		runID   string
		wantErr bool
	}{
		{
			name:    "non existing workflowRun",
			orgID:   s.org.ID,
			runID:   uuid.NewString(),
			wantErr: true,
		},
		{
			name:  "existing workflowRun in org1",
			orgID: s.org.ID,
			runID: s.runOrg1.ID.String(),
		},
		{
			name:    "can't access workflowRun from other org",
			orgID:   s.org.ID,
			runID:   s.runOrg2.ID.String(),
			wantErr: true,
		},
		{
			name:  "can access workflowRun from other org if public",
			orgID: s.org.ID,
			runID: s.runOrg2Public.ID.String(),
		},
	}

	for _, tc := range testCases {
		s.T().Run(tc.name, func(t *testing.T) {
			run, err := s.WorkflowRun.GetByIDInOrgOrPublic(ctx, tc.orgID, tc.runID)
			if tc.wantErr {
				assert.Error(err)
				assert.True(biz.IsNotFound(err))
			} else {
				assert.NoError(err)
				assert.Equal(tc.runID, run.ID.String())
			}
		})
	}
}

func (s *workflowRunIntegrationTestSuite) TestGetByDigestInOrgOrPublic() {
	assert := assert.New(s.T())
	ctx := context.Background()
	testCases := []struct {
		name           string
		orgID          string
		digest         string
		errTypeChecker func(err error) bool
	}{
		{
			name:           "non existing workflowRun",
			orgID:          s.org.ID,
			digest:         "sha256:b5bb9d8014a0f9b1d61e21e796d78dccdf1352f23cd32812f4850b878ae4944c",
			errTypeChecker: biz.IsNotFound,
		},
		{
			name:           "invalid digest",
			orgID:          s.org.ID,
			digest:         "b5bb9d8014a0f9b1d61e21e796d78dccdf1352f23cd32812f4850b878ae4944c",
			errTypeChecker: biz.IsErrValidation,
		},
		{
			name:   "existing workflowRun in org1",
			orgID:  s.org.ID,
			digest: s.digestAtt1,
		},
		{
			name:           "can't access workflowRun from other org",
			orgID:          s.org.ID,
			digest:         s.digestAttOrg2,
			errTypeChecker: biz.IsNotFound,
		},
		{
			name:   "can access workflowRun from other org if public",
			orgID:  s.org.ID,
			digest: s.digestAttPublic,
		},
	}

	for _, tc := range testCases {
		s.T().Run(tc.name, func(t *testing.T) {
			run, err := s.WorkflowRun.GetByDigestInOrgOrPublic(ctx, tc.orgID, tc.digest)
			if tc.errTypeChecker != nil {
				assert.Error(err)
				assert.True(tc.errTypeChecker(err))
			} else {
				assert.NoError(err)
				assert.Equal(tc.digest, run.Attestation.Digest)
			}
		})
	}
}

func (s *workflowRunIntegrationTestSuite) TestCreate() {
	assert := assert.New(s.T())
	ctx := context.Background()

	s.T().Run("valid workflowRun", func(t *testing.T) {
		run, err := s.WorkflowRun.Create(ctx, &biz.WorkflowRunCreateOpts{
			WorkflowID: s.workflowOrg1.ID.String(), RobotaccountID: s.robotAccount.ID.String(), ContractRevision: s.contractVersion, CASBackendID: s.casBackend.ID,
			RunnerType: "runnerType", RunnerRunURL: "runURL",
		})
		assert.NoError(err)
		if diff := cmp.Diff(&biz.WorkflowRun{
			RunnerType: "runnerType", RunURL: "runURL", State: string(biz.WorkflowRunInitialized), ContractVersionID: s.contractVersion.Version.ID,
			Workflow:             s.workflowOrg1,
			CASBackends:          []*biz.CASBackend{s.casBackend},
			ContractRevisionUsed: 1, ContractRevisionLatest: 1,
		}, run,
			cmpopts.IgnoreFields(biz.WorkflowRun{}, "CreatedAt", "ID", "Workflow"),
			cmpopts.IgnoreFields(biz.CASBackend{}, "CreatedAt", "ValidatedAt", "OrganizationID"),
		); diff != "" {
			assert.Failf("mismatch (-want +got):\n%s", diff)
		}
	})
}

func (s *workflowRunIntegrationTestSuite) TestContractInformation() {
	ctx := context.Background()
	s.Run("if it's the first revision of the contract it matches", func() {
		run, err := s.WorkflowRun.Create(ctx, &biz.WorkflowRunCreateOpts{
			WorkflowID: s.workflowOrg1.ID.String(), RobotaccountID: s.robotAccount.ID.String(), ContractRevision: s.contractVersion, CASBackendID: s.casBackend.ID,
			RunnerType: "runnerType", RunnerRunURL: "runURL",
		})
		s.NoError(err)
		s.Equal(1, run.ContractRevisionUsed)
		s.Equal(1, run.ContractRevisionLatest)
	})

	s.Run("if the contract gets a new revision but it's not used, it shows spread", func() {
		updatedContractRevision, err := s.WorkflowContract.Update(ctx, s.org.ID, s.contractVersion.Contract.Name,
			&biz.WorkflowContractUpdateOpts{Schema: &schemav1.CraftingSchema{
				Runner: &schemav1.CraftingSchema_Runner{Type: schemav1.CraftingSchema_Runner_CIRCLECI_BUILD},
			}})
		s.NoError(err)
		// load the previous version of the contract
		updatedContractRevision.Version = s.contractVersion.Version

		run, err := s.WorkflowRun.Create(ctx, &biz.WorkflowRunCreateOpts{
			WorkflowID: s.workflowOrg1.ID.String(), RobotaccountID: s.robotAccount.ID.String(), ContractRevision: updatedContractRevision, CASBackendID: s.casBackend.ID,
			RunnerType: "runnerType", RunnerRunURL: "runURL",
		})
		s.NoError(err)
		// Shows that the latest available revision is 2, but the used one is 1
		s.Equal(1, run.ContractRevisionUsed)
		s.Equal(2, run.ContractRevisionLatest)
	})
}

// Run the tests
func TestWorkflowRunUseCase(t *testing.T) {
	suite.Run(t, new(workflowRunIntegrationTestSuite))
}

// Utility struct to hold the test suite
type workflowRunIntegrationTestSuite struct {
	testhelpers.UseCasesEachTestSuite
	*workflowRunTestData
}

type workflowRunTestData struct {
	org, org2                                      *biz.Organization
	casBackend                                     *biz.CASBackend
	workflowOrg1, workflowOrg2, workflowPublicOrg2 *biz.Workflow
	runOrg1, runOrg2, runOrg2Public                *biz.WorkflowRun
	robotAccount                                   *biz.RobotAccount
	contractVersion                                *biz.WorkflowContractWithVersion
	digestAtt1, digestAttOrg2, digestAttPublic     string
}

// extract this setup to a helper function so it can be used from other test suites
func setupWorkflowRunTestData(t *testing.T, suite *testhelpers.TestingUseCases, s *workflowRunTestData) {
	var err error
	assert := assert.New(t)
	ctx := context.Background()

	s.org, err = suite.Organization.Create(ctx, "testing-org")
	assert.NoError(err)
	s.org2, err = suite.Organization.Create(ctx, "second-org")
	assert.NoError(err)

	// Workflow
	s.workflowOrg1, err = suite.Workflow.Create(ctx, &biz.WorkflowCreateOpts{Name: "test-workflow", OrgID: s.org.ID})
	assert.NoError(err)
	s.workflowOrg2, err = suite.Workflow.Create(ctx, &biz.WorkflowCreateOpts{Name: "test-workflow", OrgID: s.org2.ID})
	assert.NoError(err)
	// Public workflow
	s.workflowPublicOrg2, err = suite.Workflow.Create(ctx, &biz.WorkflowCreateOpts{Name: "test-public-workflow", OrgID: s.org2.ID, Public: true})
	assert.NoError(err)

	// Robot account
	s.robotAccount, err = suite.RobotAccount.Create(ctx, "name", s.org.ID, s.workflowOrg1.ID.String())
	assert.NoError(err)

	// Find contract revision
	s.contractVersion, err = suite.WorkflowContract.Describe(ctx, s.org.ID, s.workflowOrg1.ContractID.String(), 0)
	assert.NoError(err)

	s.casBackend, err = suite.CASBackend.CreateOrUpdate(ctx, s.org.ID, "repo", "username", "pass", backendType, true)
	assert.NoError(err)

	// Let's create 3 runs, one in org1 and 2 in org2 (one public)
	s.runOrg1, err = suite.WorkflowRun.Create(ctx,
		&biz.WorkflowRunCreateOpts{
			WorkflowID: s.workflowOrg1.ID.String(), RobotaccountID: s.robotAccount.ID.String(), ContractRevision: s.contractVersion, CASBackendID: s.casBackend.ID,
		})
	assert.NoError(err)
	s.digestAtt1, err = suite.WorkflowRun.SaveAttestation(ctx, s.runOrg1.ID.String(), &dsse.Envelope{PayloadType: "test"})

	assert.NoError(err)

	s.runOrg2, err = suite.WorkflowRun.Create(ctx,
		&biz.WorkflowRunCreateOpts{
			WorkflowID: s.workflowOrg2.ID.String(), RobotaccountID: s.robotAccount.ID.String(), ContractRevision: s.contractVersion, CASBackendID: s.casBackend.ID,
		})
	assert.NoError(err)
	s.digestAttOrg2, err = suite.WorkflowRun.SaveAttestation(ctx, s.runOrg2.ID.String(), &dsse.Envelope{PayloadType: "test2"})
	assert.NoError(err)

	s.runOrg2Public, err = suite.WorkflowRun.Create(ctx,
		&biz.WorkflowRunCreateOpts{
			WorkflowID: s.workflowPublicOrg2.ID.String(), RobotaccountID: s.robotAccount.ID.String(), ContractRevision: s.contractVersion, CASBackendID: s.casBackend.ID,
		})
	assert.NoError(err)
	s.digestAttPublic, err = suite.WorkflowRun.SaveAttestation(ctx, s.runOrg2Public.ID.String(), &dsse.Envelope{PayloadType: "test3"})
	assert.NoError(err)
}

func (s *workflowRunIntegrationTestSuite) SetupTest() {
	credsWriter := creds.NewReaderWriter(s.T())
	credsWriter.On("SaveCredentials", context.Background(), mock.Anything, &credentials.OCIKeypair{Repo: "repo", Username: "username", Password: "pass"}).Return("stored-OCI-secret", nil)

	s.TestingUseCases = testhelpers.NewTestingUseCases(s.T(), testhelpers.WithCredsReaderWriter(credsWriter))

	s.workflowRunTestData = &workflowRunTestData{}
	setupWorkflowRunTestData(s.T(), s.TestingUseCases, s.workflowRunTestData)
}
