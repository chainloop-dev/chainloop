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

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
)

/*
User mapping:
- userOne -> userOne org
- userOne, userTwo -> shared org
*/
func (s *userIntegrationTestSuite) TestDeleteUser() {
	assert := assert.New(s.T())
	ctx := context.Background()

	err := s.User.DeleteUser(ctx, s.userOne.ID)
	assert.NoError(err)

	// Organization where the user is the only member got deleted
	gotOrgOne, err := s.Organization.FindByID(ctx, s.userOneOrg.ID)
	assert.NoError(err)
	assert.Nil(gotOrgOne)

	// Organization that it's shared with another user is still present
	gotSharedOrg, err := s.Organization.FindByID(ctx, s.sharedOrg.ID)
	assert.NoError(err)
	assert.NotNil(gotSharedOrg)

	// user and associated memberships have been deleted
	gotUser, err := s.User.FindByID(ctx, s.userOne.ID)
	assert.NoError(err)
	assert.Nil(gotUser)

	gotMembership, err := s.Membership.ByUser(ctx, s.userOne.ID)
	assert.NoError(err)
	assert.Empty(gotMembership)
}

// Run the tests
func TestUserUseCase(t *testing.T) {
	suite.Run(t, new(userIntegrationTestSuite))
}

// Utility struct to hold the test suite
type userIntegrationTestSuite struct {
	testhelpers.UseCasesEachTestSuite
	userOne, userTwo      *biz.User
	userOneOrg, sharedOrg *biz.Organization
}

func (s *userIntegrationTestSuite) SetupTest() {
	t := s.T()
	var err error
	assert := assert.New(s.T())
	ctx := context.Background()

	s.TestingUseCases = testhelpers.NewTestingUseCases(t)

	s.userOneOrg, err = s.Organization.Create(ctx, "user-1-org")
	assert.NoError(err)
	s.sharedOrg, err = s.Organization.Create(ctx, "shared-org")
	assert.NoError(err)

	// Create User 1
	s.userOne, err = s.User.FindOrCreateByEmail(ctx, "user-1@test.com")
	assert.NoError(err)
	// Attach both orgs
	_, err = s.Membership.Create(ctx, s.userOneOrg.ID, s.userOne.ID, true)
	assert.NoError(err)
	_, err = s.Membership.Create(ctx, s.sharedOrg.ID, s.userOne.ID, true)
	assert.NoError(err)

	// Create User 2 and attach shared org
	s.userTwo, err = s.User.FindOrCreateByEmail(ctx, "user-2@test.com")
	assert.NoError(err)
	_, err = s.Membership.Create(ctx, s.sharedOrg.ID, s.userTwo.ID, true)
	assert.NoError(err)
}
