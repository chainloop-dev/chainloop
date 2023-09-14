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

	"github.com/chainloop-dev/chainloop/app/controlplane/internal/biz"
	"github.com/chainloop-dev/chainloop/app/controlplane/internal/biz/testhelpers"
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

func (s *workflowRunIntegrationTestSuite) TestSaveAttestation() {
	assert := assert.New(s.T())
	ctx := context.Background()

	validEnvelope := &dsse.Envelope{}

	s.T().Run("non existing workflowRun", func(t *testing.T) {
		err := s.WorkflowRun.SaveAttestation(ctx, uuid.NewString(), validEnvelope, validDigest)
		assert.Error(err)
		assert.True(biz.IsNotFound(err))
	})

	s.T().Run("valid workflowRun", func(t *testing.T) {
		run, err := s.WorkflowRun.Create(ctx, &biz.WorkflowRunCreateOpts{
			WorkflowID: s.workflow.ID.String(), RobotaccountID: s.robotAccount.ID.String(), ContractRevisionUUID: s.contractVersion.Version.ID, CASBackendID: s.casBackend.ID,
		})
		assert.NoError(err)

		err = s.WorkflowRun.SaveAttestation(ctx, run.ID.String(), validEnvelope, validDigest)
		assert.NoError(err)

		// Retrieve attestation ref from storage and compare
		r, err := s.WorkflowRun.GetByID(ctx, s.org.ID, run.ID.String())
		assert.NoError(err)
		assert.Equal(r.Attestation, &biz.Attestation{Envelope: validEnvelope, Digest: validDigest})
	})

	s.T().Run("valid workflowRun attestation not stored in CAS", func(t *testing.T) {
		run, err := s.WorkflowRun.Create(ctx, &biz.WorkflowRunCreateOpts{
			WorkflowID: s.workflow.ID.String(), RobotaccountID: s.robotAccount.ID.String(), ContractRevisionUUID: s.contractVersion.Version.ID, CASBackendID: s.casBackend.ID,
		})
		assert.NoError(err)

		err = s.WorkflowRun.SaveAttestation(ctx, run.ID.String(), validEnvelope, "")
		assert.NoError(err)

		// Retrieve attestation ref from storage and compare
		r, err := s.WorkflowRun.GetByID(ctx, s.org.ID, run.ID.String())
		assert.NoError(err)
		assert.Equal(r.Attestation, &biz.Attestation{Envelope: validEnvelope, Digest: ""})
	})
}

func (s *workflowRunIntegrationTestSuite) TestCreate() {
	assert := assert.New(s.T())
	ctx := context.Background()

	s.T().Run("valid workflowRun", func(t *testing.T) {
		run, err := s.WorkflowRun.Create(ctx, &biz.WorkflowRunCreateOpts{
			WorkflowID: s.workflow.ID.String(), RobotaccountID: s.robotAccount.ID.String(), ContractRevisionUUID: s.contractVersion.Version.ID, CASBackendID: s.casBackend.ID,
			RunnerType: "runnerType", RunnerRunURL: "runURL",
		})
		assert.NoError(err)
		if diff := cmp.Diff(&biz.WorkflowRun{
			RunnerType: "runnerType", RunURL: "runURL", State: string(biz.WorkflowRunInitialized), ContractVersionID: s.contractVersion.Version.ID,
			Workflow:    s.workflow,
			CASBackends: []*biz.CASBackend{s.casBackend},
		}, run,
			cmpopts.IgnoreFields(biz.WorkflowRun{}, "CreatedAt", "ID", "Workflow"),
			cmpopts.IgnoreFields(biz.CASBackend{}, "CreatedAt", "ValidatedAt", "OrganizationID"),
		); diff != "" {
			assert.Failf("mismatch (-want +got):\n%s", diff)
		}
	})
}

// Run the tests
func TestWorkflowRunUseCase(t *testing.T) {
	suite.Run(t, new(workflowRunIntegrationTestSuite))
}

// Utility struct to hold the test suite
type workflowRunIntegrationTestSuite struct {
	testhelpers.UseCasesEachTestSuite
	org             *biz.Organization
	casBackend      *biz.CASBackend
	workflow        *biz.Workflow
	robotAccount    *biz.RobotAccount
	contractVersion *biz.WorkflowContractWithVersion
}

func (s *workflowRunIntegrationTestSuite) SetupTest() {
	var err error
	assert := assert.New(s.T())
	ctx := context.Background()
	// OCI repository credentials
	credsWriter := creds.NewReaderWriter(s.T())
	credsWriter.On(
		"SaveCredentials", ctx, mock.Anything, &credentials.OCIKeypair{Repo: "repo", Username: "username", Password: "pass"},
	).Return("stored-OCI-secret", nil)

	s.TestingUseCases = testhelpers.NewTestingUseCases(s.T(), testhelpers.WithCredsReaderWriter(credsWriter))

	s.org, err = s.Organization.Create(ctx, "testing org")
	assert.NoError(err)

	// Workflow
	s.workflow, err = s.Workflow.Create(ctx, &biz.CreateOpts{Name: "test workflow", OrgID: s.org.ID})
	assert.NoError(err)

	// Robot account
	s.robotAccount, err = s.RobotAccount.Create(ctx, "name", s.org.ID, s.workflow.ID.String())
	assert.NoError(err)

	// Find contract revision
	s.contractVersion, err = s.WorkflowContract.Describe(ctx, s.org.ID, s.workflow.ContractID.String(), 0)
	assert.NoError(err)

	s.casBackend, err = s.CASBackend.CreateOrUpdate(ctx, s.org.ID, "repo", "username", "pass", biz.CASBackendOCI, true)
	assert.NoError(err)
}
