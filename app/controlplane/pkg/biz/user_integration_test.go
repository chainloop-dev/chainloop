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

	v1 "github.com/chainloop-dev/chainloop/app/controlplane/api/controlplane/v1"
	"github.com/chainloop-dev/chainloop/app/controlplane/internal/authz"
	conf "github.com/chainloop-dev/chainloop/app/controlplane/internal/conf/controlplane/config/v1"
	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/biz"
	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/biz/testhelpers"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

/*
User mapping:
- userOne -> userOne org
- userOne, userTwo -> shared org
*/
func (s *userIntegrationTestSuite) TestDeleteUser() {
	ctx := context.Background()

	err := s.User.DeleteUser(ctx, s.userOne.ID)
	s.NoError(err)

	// Organization where the user is the only member got deleted
	gotOrgOne, err := s.Organization.FindByID(ctx, s.userOneOrg.ID)
	s.Error(err)
	s.True(biz.IsNotFound(err))
	s.Nil(gotOrgOne)

	// Organization that it's shared with another user is still present
	gotSharedOrg, err := s.Organization.FindByID(ctx, s.sharedOrg.ID)
	s.NoError(err)
	s.NotNil(gotSharedOrg)

	// user and associated memberships have been deleted
	gotUser, err := s.User.FindByID(ctx, s.userOne.ID)
	s.NoError(err)
	s.Nil(gotUser)

	gotMembership, err := s.Membership.ByUser(ctx, s.userOne.ID)
	s.NoError(err)
	s.Empty(gotMembership)
}

func (s *userIntegrationTestSuite) TestCurrentMembership() {
	ctx := context.Background()
	s.Run("if there is an associated, default org is returned", func() {
		// userOne has a default org
		m, err := s.Membership.FindByOrgAndUser(ctx, s.sharedOrg.ID, s.userOne.ID)
		s.NoError(err)
		s.True(m.Current)

		// and it's returned as currentOrg
		got, err := s.User.CurrentMembership(ctx, s.userOne.ID)
		s.NoError(err)
		s.Equal(s.sharedOrg, got.Org)

		// and it contains the default role
		s.Equal(authz.RoleViewer, got.Role)
	})

	s.Run("they have more orgs but none of them is the default, it will return the first one as default", func() {
		m, err := s.Membership.FindByOrgAndUser(ctx, s.sharedOrg.ID, s.userOne.ID)
		s.NoError(err)
		s.True(m.Current)
		// leave the current org
		err = s.Membership.LeaveAndDeleteOrg(ctx, s.userOne.ID, m.ID.String())
		s.NoError(err)

		// none of the orgs is marked as current
		mems, _ := s.Membership.ByUser(ctx, s.userOne.ID)
		s.Len(mems, 1)
		s.False(mems[0].Current)

		// asking for the current org will return the first one
		got, err := s.User.CurrentMembership(ctx, s.userOne.ID)
		s.NoError(err)
		s.Equal(s.userOneOrg, got.Org)

		// and now the membership will be set as current
		mems, _ = s.Membership.ByUser(ctx, s.userOne.ID)
		s.Len(mems, 1)
		s.True(mems[0].Current)
	})

	s.Run("it will fail if there are no memberships", func() {
		// none of the orgs is marked as current
		mems, _ := s.Membership.ByUser(ctx, s.userOne.ID)
		s.Len(mems, 1)
		// leave the current org
		err := s.Membership.LeaveAndDeleteOrg(ctx, s.userOne.ID, mems[0].ID.String())
		s.NoError(err)
		mems, _ = s.Membership.ByUser(ctx, s.userOne.ID)
		s.Len(mems, 0)

		_, err = s.User.CurrentMembership(ctx, s.userOne.ID)
		s.ErrorContains(err, "user does not have any organization associated")
	})
}

// Run the tests
func TestUserUseCase(t *testing.T) {
	suite.Run(t, new(userIntegrationTestSuite))
	suite.Run(t, new(userOnboardingTestSuite))
}

type userOnboardingTestSuite struct {
	testhelpers.UseCasesEachTestSuite
}

func (s *userOnboardingTestSuite) TestAutoOnboardOrganizationsNoConfiguration() {
	ctx := context.Background()
	// Create a user with no orgs
	user, err := s.User.FindOrCreateByEmail(ctx, "foo@bar.com", true)
	s.NoError(err)
	s.NotNil(user)
}

func (s *userOnboardingTestSuite) TestAutoOnboardOrganizationsWithConfiguration() {
	ctx := context.Background()
	const orgName = "existing-org"
	s.TestingUseCases = testhelpers.NewTestingUseCases(s.T(), testhelpers.WithOnboardingConfiguration([]*conf.OnboardingSpec{
		{
			Name: orgName,
			Role: v1.MembershipRole_MEMBERSHIP_ROLE_ORG_VIEWER,
		},
	}))

	// The user got onboarded in the existing org
	org, err := s.Organization.Create(ctx, orgName)
	require.NoError(s.T(), err)

	user, err := s.User.FindOrCreateByEmail(ctx, "foo@bar.com")
	s.NoError(err)
	s.NotNil(user)

	m, err := s.Membership.FindByOrgAndUser(ctx, org.ID, user.ID)
	s.NoError(err)
	s.NotNil(m)
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

	s.userOneOrg, err = s.Organization.CreateWithRandomName(ctx)
	assert.NoError(err)
	s.sharedOrg, err = s.Organization.CreateWithRandomName(ctx)
	assert.NoError(err)

	// Create User 1
	s.userOne, err = s.User.FindOrCreateByEmail(ctx, "user-1@test.com")
	assert.NoError(err)
	// Attach both orgs
	_, err = s.Membership.Create(ctx, s.userOneOrg.ID, s.userOne.ID)
	assert.NoError(err)
	_, err = s.Membership.Create(ctx, s.sharedOrg.ID, s.userOne.ID, biz.WithCurrentMembership())
	assert.NoError(err)

	// Create User 2 and attach shared org
	s.userTwo, err = s.User.FindOrCreateByEmail(ctx, "user-2@test.com")
	assert.NoError(err)
	_, err = s.Membership.Create(ctx, s.sharedOrg.ID, s.userTwo.ID, biz.WithCurrentMembership())
	assert.NoError(err)
}
