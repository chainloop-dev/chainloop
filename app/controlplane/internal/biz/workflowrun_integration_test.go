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

	"github.com/chainloop-dev/bedrock/app/controlplane/internal/biz"
	"github.com/chainloop-dev/bedrock/app/controlplane/internal/biz/testhelpers"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
)

func (s *workflowRunIntegrationTestSuite) TestAssociateAttestation() {
	assert := assert.New(s.T())
	ctx := context.Background()
	validRef := &biz.AttestationRef{Sha256: "deadbeef", SecretRef: "secret-ref"}

	s.T().Run("non existing workflowRun", func(t *testing.T) {
		err := s.WorkflowRun.AssociateAttestation(ctx, uuid.NewString(), validRef)
		assert.Error(err)
		assert.True(biz.IsNotFound(err))
	})

	s.T().Run("empty attestation ref", func(t *testing.T) {
		err := s.WorkflowRun.AssociateAttestation(ctx, uuid.NewString(), nil)
		assert.Error(err)
		assert.True(biz.IsErrValidation(err))
	})

	s.T().Run("valid workflowrun", func(t *testing.T) {
		org, err := s.Organization.Create(ctx, "testing org")
		assert.NoError(err)

		// Workflow
		wf, err := s.Workflow.Create(ctx, &biz.CreateOpts{Name: "test workflow", OrgID: org.ID})
		assert.NoError(err)

		// Robot account
		ra, err := s.RobotAccount.Create(ctx, "name", org.ID, wf.ID.String())
		assert.NoError(err)

		// Find contract revision
		contractVersion, err := s.WorkflowContract.Describe(ctx, org.ID, wf.ContractID.String(), 0)
		assert.NoError(err)

		run, err := s.WorkflowRun.Create(ctx, &biz.WorkflowRunCreateOpts{
			WorkflowID: wf.ID.String(), RobotaccountID: ra.ID.String(), ContractRevisionUUID: contractVersion.Version.ID,
		})
		assert.NoError(err)

		err = s.WorkflowRun.AssociateAttestation(ctx, run.ID.String(), validRef)
		assert.NoError(err)

		// Retrieve attestation ref from storage and compare
		r, err := s.WorkflowRun.View(ctx, org.ID, run.ID.String())
		assert.NoError(err)
		assert.Equal(r.AttestationRef, validRef)
	})
}

// Run the tests
func TestWorkflowRunUseCase(t *testing.T) {
	suite.Run(t, new(workflowRunIntegrationTestSuite))
}

// Utility struct to hold the test suite
type workflowRunIntegrationTestSuite struct {
	testhelpers.UseCasesEachTestSuite
}
