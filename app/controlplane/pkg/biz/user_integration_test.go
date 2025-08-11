//
// Copyright 2024-2025 The Chainloop Authors.
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
	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/authz"
	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/biz"
	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/biz/testhelpers"
	config "github.com/chainloop-dev/chainloop/app/controlplane/pkg/conf/controlplane/config/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

func (s *userIntegrationTestSuite) TestFindOrCreateByEmail() {
	// It will create a new user
	u, err := s.User.UpsertByEmail(context.Background(), "user1@user.com", nil)
	s.NoError(err)
	s.Equal("user1@user.com", u.Email)

	// running it again has the same ID
	u2, err := s.User.UpsertByEmail(context.Background(), "user1@user.com", nil)
	s.NoError(err)
	s.Equal(u2.ID, u.ID)

	// It will downcase the email
	// running it again has the same ID
	u3, err := s.User.UpsertByEmail(context.Background(), "WAS-UPPERCASE@user.com", nil)
	s.NoError(err)
	s.Equal("was-uppercase@user.com", u3.Email)

	// Include now the first and last name
	firstName := "First"
	lastName := "Last"
	u4, err := s.User.UpsertByEmail(context.Background(), "with-names@user.com", &biz.UpsertByEmailOpts{
		FirstName: &firstName,
		LastName:  &lastName,
	})
	s.NoError(err)
	s.Equal("with-names@user.com", u4.Email)
	s.Equal(firstName, u4.FirstName)
	s.Equal(lastName, u4.LastName)

	// Run it again with the same email, but different names to ensure it does update the names
	updatedFirstName := "UpdatedFirst"
	updatedLastName := "UpdatedLast"
	u5, err := s.User.UpsertByEmail(context.Background(), "with-names@user.com", &biz.UpsertByEmailOpts{
		FirstName: &updatedFirstName,
		LastName:  &updatedLastName,
	})
	s.NoError(err)
	s.Equal("with-names@user.com", u5.Email)
	s.Equal(updatedFirstName, u5.FirstName)
	s.Equal(updatedLastName, u5.LastName)
	s.Equal(u4.ID, u5.ID)
}

/*
User mapping:
- userOne -> userOne org
- userOne, userTwo -> shared org
*/
func (s *userIntegrationTestSuite) TestDeleteUser() {
	ctx := context.Background()

	s.Run("cannot delete user when sole owner", func() {
		// User deletion should be blocked because userOne is sole owner of userOneOrg
		err := s.User.DeleteUser(ctx, s.userOne.ID)
		s.Error(err)
		s.True(biz.IsErrValidation(err))
		s.Contains(err.Error(), "sole owner")

		// Both organizations should still exist since deletion was blocked
		gotOrgOne, err := s.Organization.FindByID(ctx, s.userOneOrg.ID)
		s.NoError(err)
		s.NotNil(gotOrgOne)

		gotSharedOrg, err := s.Organization.FindByID(ctx, s.sharedOrg.ID)
		s.NoError(err)
		s.NotNil(gotSharedOrg)

		// User should still exist since deletion was blocked
		gotUser, err := s.User.FindByID(ctx, s.userOne.ID)
		s.NoError(err)
		s.NotNil(gotUser)

		// Memberships should still exist since deletion was blocked
		gotMembership, err := s.Membership.ByUser(ctx, s.userOne.ID)
		s.NoError(err)
		s.NotEmpty(gotMembership)
	})

	s.Run("can delete user when not sole owner", func() {
		// userTwo is an owner but not sole owner (userOne is also owner of sharedOrg)
		err := s.User.DeleteUser(ctx, s.userTwo.ID)
		s.NoError(err)

		// sharedOrg should still exist since userOne is still an owner
		gotSharedOrg, err := s.Organization.FindByID(ctx, s.sharedOrg.ID)
		s.NoError(err)
		s.NotNil(gotSharedOrg)

		// userTwo should be deleted
		gotUser, err := s.User.FindByID(ctx, s.userTwo.ID)
		s.NoError(err)
		s.Nil(gotUser)

		// userTwo's memberships should be gone
		gotMembership, err := s.Membership.ByUser(ctx, s.userTwo.ID)
		s.NoError(err)
		s.Empty(gotMembership)
	})
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

		// and it contains the owner role (set in test setup)
		s.Equal(authz.RoleOwner, got.Role)
	})

	s.Run("they have more orgs but none of them is the default, it will return the first one as default", func() {
		m, err := s.Membership.FindByOrgAndUser(ctx, s.sharedOrg.ID, s.userOne.ID)
		s.NoError(err)
		s.True(m.Current)
		// leave the current org
		err = s.Membership.Leave(ctx, s.userOne.ID, m.ID.String())
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
		// Create a test user who is not an owner so they can leave
		testUser, err := s.User.UpsertByEmail(ctx, "test-no-membership@test.com", nil)
		s.NoError(err)
		
		// Create a new org and make both testUser and userOne owners
		testOrg, err := s.Organization.CreateWithRandomName(ctx)
		s.NoError(err)
		
		// Add testUser as owner
		_, err = s.Membership.Create(ctx, testOrg.ID, testUser.ID, biz.WithMembershipRole(authz.RoleOwner))
		s.NoError(err)
		
		// Add userOne as viewer (so they can leave)
		_, err = s.Membership.Create(ctx, testOrg.ID, s.userOne.ID, biz.WithMembershipRole(authz.RoleViewer))
		s.NoError(err)

		// Now userOne can leave because testUser is also an owner
		mems, _ := s.Membership.ByUser(ctx, s.userOne.ID)
		// Find the membership for testOrg
		var testOrgMembership *biz.Membership
		for _, m := range mems {
			if m.OrganizationID.String() == testOrg.ID {
				testOrgMembership = m
				break
			}
		}
		s.NotNil(testOrgMembership)
		
		// userOne leaves testOrg (allowed because testUser is still owner)
		err = s.Membership.Leave(ctx, s.userOne.ID, testOrgMembership.ID.String())
		s.NoError(err)
		
		// Now userOne leaves their original organizations by deleting them directly
		// since they are sole owners, they can delete the orgs instead of leaving
		err = s.Organization.Delete(ctx, s.userOneOrg.ID)
		s.NoError(err)
		err = s.Organization.Delete(ctx, s.sharedOrg.ID) 
		s.NoError(err)
		
		// Verify userOne has no memberships left
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
	user, err := s.User.UpsertByEmail(ctx, "foo@bar.com", &biz.UpsertByEmailOpts{DisableAutoOnboarding: toPtrBool(true)})
	s.NoError(err)
	s.NotNil(user)
}

func (s *userOnboardingTestSuite) TestAutoOnboardOrganizationsWithConfiguration() {
	ctx := context.Background()
	const orgName = "existing-org"
	s.TestingUseCases = testhelpers.NewTestingUseCases(s.T(), testhelpers.WithOnboardingConfiguration([]*config.OnboardingSpec{
		{
			Name: orgName,
			Role: v1.MembershipRole_MEMBERSHIP_ROLE_ORG_VIEWER,
		},
	}))

	// The user got onboarded in the existing org
	org, err := s.Organization.Create(ctx, orgName)
	require.NoError(s.T(), err)

	user, err := s.User.UpsertByEmail(ctx, "foo@bar.com", nil)
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
	s.userOne, err = s.User.UpsertByEmail(ctx, "user-1@test.com", nil)
	assert.NoError(err)
	// Attach both orgs - make user owner of userOneOrg and owner of sharedOrg
	_, err = s.Membership.Create(ctx, s.userOneOrg.ID, s.userOne.ID, biz.WithMembershipRole(authz.RoleOwner))
	assert.NoError(err)
	_, err = s.Membership.Create(ctx, s.sharedOrg.ID, s.userOne.ID, biz.WithMembershipRole(authz.RoleOwner), biz.WithCurrentMembership())
	assert.NoError(err)

	// Create User 2 and attach shared org as owner too
	s.userTwo, err = s.User.UpsertByEmail(ctx, "user-2@test.com", nil)
	assert.NoError(err)
	_, err = s.Membership.Create(ctx, s.sharedOrg.ID, s.userTwo.ID, biz.WithMembershipRole(authz.RoleOwner), biz.WithCurrentMembership())
	assert.NoError(err)
}
