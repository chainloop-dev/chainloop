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
	"github.com/google/uuid"
	"github.com/stretchr/testify/suite"
)

func (s *robotAccountTestSuite) TestFindByID() {
	ctx := context.Background()

	s.Run("returns an error if the ID format is not valid", func() {
		_, err := s.RobotAccount.FindByID(ctx, "not_valid_uuid")
		s.True(biz.IsErrInvalidUUID(err))
	})

	s.Run("returns nil if the robot account cannot be found", func() {
		ra, err := s.RobotAccount.FindByID(ctx, uuid.NewString())
		s.NoError(err)
		s.Nil(ra)
	})

	s.Run("finds an active robot account", func() {
		ra, err := s.RobotAccount.FindByID(ctx, s.raID.String())
		s.NoError(err)
		s.NotNil(ra)
		s.Equal(s.raID, ra.ID)
		s.Equal(s.workflowID, ra.WorkflowID)
		s.Nil(ra.RevokedAt)
	})

	s.Run("finds a revoked robot account with its revocation timestamp", func() {
		ra, err := s.RobotAccount.FindByID(ctx, s.revokedRaID.String())
		s.NoError(err)
		s.NotNil(ra)
		s.NotNil(ra.RevokedAt)
	})
}

// Utility struct to hold the test suite
type robotAccountTestSuite struct {
	testhelpers.UseCasesEachTestSuite
	org               *biz.Organization
	workflowID        uuid.UUID
	raID, revokedRaID uuid.UUID
}

// Run the tests
func TestRobotAccountUseCase(t *testing.T) {
	suite.Run(t, new(robotAccountTestSuite))
}

func (s *robotAccountTestSuite) SetupTest() {
	var err error
	ctx := context.Background()

	s.TestingUseCases = testhelpers.NewTestingUseCases(s.T())
	s.org, err = s.Organization.CreateWithRandomName(ctx)
	s.Require().NoError(err)

	wf, err := s.Workflow.Create(ctx, &biz.WorkflowCreateOpts{
		Name:    "myworkflow",
		Project: "myproject",
		OrgID:   s.org.ID,
	})
	s.Require().NoError(err)
	s.workflowID = wf.ID

	// Seeded via ent: management is no longer exposed, but existing accounts must remain
	// resolvable for attestation-time authentication.
	ra, err := s.Data.DB.RobotAccount.Create().SetName("myRobotAccount").SetWorkflowID(wf.ID).Save(ctx)
	s.Require().NoError(err)
	s.raID = ra.ID

	revoked, err := s.Data.DB.RobotAccount.Create().SetName("myRevokedRobotAccount").
		SetWorkflowID(wf.ID).SetRevokedAt(time.Now()).Save(ctx)
	s.Require().NoError(err)
	s.revokedRaID = revoked.ID
}
