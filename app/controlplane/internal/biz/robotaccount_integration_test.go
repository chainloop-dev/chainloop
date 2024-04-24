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

	"github.com/chainloop-dev/chainloop/app/controlplane/internal/biz"
	"github.com/chainloop-dev/chainloop/app/controlplane/internal/biz/testhelpers"
	"github.com/google/uuid"
	"github.com/stretchr/testify/suite"
)

func (s *robotAccountTestSuite) TestRevoke() {
	ctx := context.Background()
	s.Run("returns an error if org ID format is not valid", func() {
		err := s.RobotAccount.Revoke(ctx, "not_valid_uuid", uuid.NewString())
		s.ErrorAs(err, &biz.ErrInvalidUUID{})
	})

	s.Run("returns an error if robot account ID format is not valid", func() {
		err := s.RobotAccount.Revoke(ctx, uuid.NewString(), "not_valid_uuid")
		s.ErrorAs(err, &biz.ErrInvalidUUID{})
	})

	s.Run("returns a Not Found if robot account cannot be found", func() {
		err := s.RobotAccount.Revoke(ctx, s.org.ID, uuid.NewString())
		s.ErrorAs(err, &biz.ErrNotFound{})
	})

	s.Run("revokes the robot account", func() {
		err := s.RobotAccount.Revoke(ctx, s.org.ID, s.ra.ID.String())
		s.NoError(err)

		// Reload the robot account
		ra, err := s.RobotAccount.FindByID(ctx, s.ra.ID.String())
		s.NoError(err)
		s.NotNil(ra.RevokedAt)
	})
}

// Utility struct to hold the test suite
type robotAccountTestSuite struct {
	testhelpers.UseCasesEachTestSuite
	org *biz.Organization
	ra  *biz.RobotAccount
}

// // Run the tests
func TestRobotAccountUseCase(t *testing.T) {
	suite.Run(t, new(robotAccountTestSuite))
}

func (s *robotAccountTestSuite) SetupTest() {
	var err error
	ctx := context.Background()

	s.TestingUseCases = testhelpers.NewTestingUseCases(s.T())
	s.org, err = s.Organization.CreateWithRandomName(ctx)
	s.NoError(err)

	wf, err := s.Workflow.Create(ctx, &biz.WorkflowCreateOpts{
		Name:  "myworkflow",
		OrgID: s.org.ID,
	})
	s.NoError(err)
	s.ra, err = s.RobotAccount.Create(ctx, "myRobotAccount", s.org.ID, wf.ID.String())
	s.NoError(err)
}
