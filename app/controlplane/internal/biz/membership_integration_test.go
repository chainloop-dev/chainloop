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

	"github.com/chainloop-dev/bedrock/app/controlplane/internal/biz/testhelpers"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
)

func (s *membershipIntegrationTestSuite) TestCreateMembership() {
	assert := assert.New(s.T())
	ctx := context.Background()

	// Create User
	user, err := s.User.FindOrCreateByEmail(ctx, "foo@test.com")
	assert.NoError(err)

	s.T().Run("Create default", func(t *testing.T) {
		org, err := s.Organization.Create(ctx, "foo")
		assert.NoError(err)

		m, err := s.Membership.Create(ctx, org.ID, user.ID, true)
		assert.NoError(err)
		assert.Equal(true, m.Current, "Membership should be current")

		wantUserID, err := uuid.Parse(user.ID)
		assert.NoError(err)
		assert.Equal(wantUserID, m.UserID, "User ID")

		wantORGID, err := uuid.Parse(org.ID)
		assert.NoError(err)
		assert.Equal(wantORGID, m.OrganizationID, "Organization ID")

		assert.EqualValues(org, m.Org, "Embedded organization")
	})

	s.T().Run("Non current", func(t *testing.T) {
		org, err := s.Organization.Create(ctx, "foo")
		assert.NoError(err)

		m, err := s.Membership.Create(ctx, org.ID, user.ID, false)
		assert.NoError(err)
		assert.Equal(false, m.Current, "Membership should be current")
	})

	s.T().Run("Invalid ORG", func(t *testing.T) {
		m, err := s.Membership.Create(ctx, uuid.NewString(), user.ID, false)
		assert.Error(err)
		assert.Nil(m)
	})

	s.T().Run("Invalid User", func(t *testing.T) {
		org, err := s.Organization.Create(ctx, "foo")
		assert.NoError(err)
		m, err := s.Membership.Create(ctx, org.ID, uuid.NewString(), false)
		assert.Error(err)
		assert.Nil(m)
	})
}

// Run the tests
func TestMembershipUseCase(t *testing.T) {
	suite.Run(t, new(membershipIntegrationTestSuite))
}

// Utility struct to hold the test suite
type membershipIntegrationTestSuite struct {
	testhelpers.UseCasesEachTestSuite
}
