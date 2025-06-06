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

	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/authz"
	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/biz"
	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/biz/testhelpers"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
)

func (s *membershipIntegrationTestSuite) TestByOrg() {
	ctx := context.Background()
	user, err := s.User.UpsertByEmail(ctx, "foo@test.com", nil)
	s.NoError(err)
	user2, err := s.User.UpsertByEmail(ctx, "foo-2@test.com", nil)
	s.NoError(err)
	userOrg, err := s.Organization.CreateWithRandomName(ctx)
	s.NoError(err)
	sharedOrg, err := s.Organization.CreateWithRandomName(ctx)
	s.NoError(err)

	_, err = s.Membership.Create(ctx, userOrg.ID, user.ID, biz.WithCurrentMembership())
	s.NoError(err)
	_, err = s.Membership.Create(ctx, sharedOrg.ID, user.ID, biz.WithCurrentMembership())
	s.NoError(err)
	_, err = s.Membership.Create(ctx, sharedOrg.ID, user2.ID, biz.WithCurrentMembership())
	s.NoError(err)

	s.Run("org 1", func() {
		memberships, err := s.Membership.ByOrg(ctx, userOrg.ID)
		s.NoError(err)
		s.Len(memberships, 1)
		s.Equal(memberships[0].OrganizationID.String(), userOrg.ID)
		s.Equal(memberships[0].User.Email, user.Email)
		s.Equal(memberships[0].Role, authz.RoleViewer)
	})

	s.Run("shared org", func() {
		memberships, err := s.Membership.ByOrg(ctx, sharedOrg.ID)
		s.NoError(err)
		s.Len(memberships, 2)
	})

	s.T().Run("non existing org", func(t *testing.T) {
		memberships, err := s.Membership.ByOrg(ctx, uuid.NewString())
		s.NoError(err)
		s.Len(memberships, 0)
	})
}

func (s *membershipIntegrationTestSuite) TestDeleteWithOrg() {
	ctx := context.Background()

	user, err := s.User.UpsertByEmail(ctx, "foo@test.com", nil)
	s.NoError(err)
	user2, err := s.User.UpsertByEmail(ctx, "foo-2@test.com", nil)
	s.NoError(err)
	userOrg, err := s.Organization.CreateWithRandomName(ctx)
	s.NoError(err)
	sharedOrg, err := s.Organization.CreateWithRandomName(ctx)
	s.NoError(err)

	mUser, err := s.Membership.Create(ctx, userOrg.ID, user.ID, biz.WithCurrentMembership())
	s.NoError(err)
	mUserSharedOrg, err := s.Membership.Create(ctx, sharedOrg.ID, user.ID, biz.WithCurrentMembership())
	s.NoError(err)
	mUser2SharedOrg, err := s.Membership.Create(ctx, sharedOrg.ID, user2.ID, biz.WithCurrentMembership())
	s.NoError(err)

	s.T().Run("invalid userID", func(t *testing.T) {
		err := s.Membership.LeaveAndDeleteOrg(ctx, "invalid", mUser.ID.String())
		s.True(biz.IsErrInvalidUUID(err))
	})

	s.T().Run("invalid orgID", func(t *testing.T) {
		err := s.Membership.LeaveAndDeleteOrg(ctx, user.ID, "invalid")
		s.True(biz.IsErrInvalidUUID(err))
	})

	s.T().Run("membership ID from another user", func(t *testing.T) {
		err := s.Membership.LeaveAndDeleteOrg(ctx, user.ID, mUser2SharedOrg.ID.String())
		s.True(biz.IsNotFound(err))
	})

	s.T().Run("delete the membership when the only member", func(t *testing.T) {
		err := s.Membership.LeaveAndDeleteOrg(ctx, user.ID, mUser.ID.String())
		s.NoError(err)
		// The org should also be deleted
		_, err = s.Organization.FindByID(ctx, userOrg.ID)
		s.True(biz.IsNotFound(err))
	})

	s.T().Run("delete the membership when there are more than 1 member", func(t *testing.T) {
		err := s.Membership.LeaveAndDeleteOrg(ctx, user.ID, mUserSharedOrg.ID.String())
		s.NoError(err)
		// The org should not be deleted
		got, err := s.Organization.FindByID(ctx, sharedOrg.ID)
		s.NoError(err)

		// User 2 is still a member
		members, err := s.Membership.ByOrg(ctx, got.ID)
		s.NoError(err)
		s.Len(members, 1)
		s.Equal(user2.ID, members[0].User.ID)
	})

	s.T().Run("we can remove the latest member", func(t *testing.T) {
		err := s.Membership.LeaveAndDeleteOrg(ctx, user2.ID, mUser2SharedOrg.ID.String())
		s.NoError(err)
		_, err = s.Organization.FindByID(ctx, sharedOrg.ID)
		s.True(biz.IsNotFound(err))
	})
}

func (s *membershipIntegrationTestSuite) TestDeleteOther() {
	ctx := context.Background()

	user, err := s.User.UpsertByEmail(ctx, "foo@test.com", nil)
	s.NoError(err)
	user2, err := s.User.UpsertByEmail(ctx, "foo-2@test.com", nil)
	s.NoError(err)
	otherOrg, err := s.Organization.CreateWithRandomName(ctx)
	s.NoError(err)
	sharedOrg, err := s.Organization.CreateWithRandomName(ctx)
	s.NoError(err)

	mUser2SharedOrg, err := s.Membership.Create(ctx, sharedOrg.ID, user2.ID, biz.WithCurrentMembership())
	s.NoError(err)

	s.T().Run("I can not delete my own membership", func(t *testing.T) {
		err := s.Membership.DeleteOther(ctx, sharedOrg.ID, user2.ID, mUser2SharedOrg.ID.String())
		s.ErrorContains(err, "cannot delete yourself from the org")
	})

	s.T().Run("I can't find the membership", func(t *testing.T) {
		err := s.Membership.DeleteOther(ctx, otherOrg.ID, user2.ID, mUser2SharedOrg.ID.String())
		s.True(biz.IsNotFound(err))
	})

	s.T().Run("I can delete other user membership", func(t *testing.T) {
		memberships, err := s.Membership.ByOrg(ctx, sharedOrg.ID)
		s.NoError(err)
		s.Len(memberships, 1)

		err = s.Membership.DeleteOther(ctx, sharedOrg.ID, user.ID, mUser2SharedOrg.ID.String())
		s.NoError(err)

		memberships, err = s.Membership.ByOrg(ctx, sharedOrg.ID)
		s.NoError(err)
		s.Len(memberships, 0)
	})
}

func (s *membershipIntegrationTestSuite) TestUpdateRole() {
	ctx := context.Background()

	user, err := s.User.UpsertByEmail(ctx, "foo@test.com", nil)
	s.NoError(err)
	user2, err := s.User.UpsertByEmail(ctx, "foo-2@test.com", nil)
	s.NoError(err)
	otherOrg, err := s.Organization.CreateWithRandomName(ctx)
	s.NoError(err)
	sharedOrg, err := s.Organization.CreateWithRandomName(ctx)
	s.NoError(err)
	mUser2SharedOrg, err := s.Membership.Create(ctx, sharedOrg.ID, user2.ID, biz.WithCurrentMembership())
	s.NoError(err)

	// empty role
	s.T().Run("a role is required", func(t *testing.T) {
		_, err := s.Membership.UpdateRole(ctx, sharedOrg.ID, user.ID, mUser2SharedOrg.ID.String(), "")
		s.ErrorContains(err, "role is required")
	})

	s.T().Run("I can not update my own membership", func(t *testing.T) {
		_, err := s.Membership.UpdateRole(ctx, sharedOrg.ID, user2.ID, mUser2SharedOrg.ID.String(), authz.RoleAdmin)
		s.ErrorContains(err, "cannot update yourself")
	})

	s.T().Run("I can't find the membership in another org", func(t *testing.T) {
		_, err := s.Membership.UpdateRole(ctx, otherOrg.ID, user.ID, mUser2SharedOrg.ID.String(), authz.RoleAdmin)
		s.True(biz.IsNotFound(err))
	})

	s.T().Run("I can update other roles", func(t *testing.T) {
		memberships, err := s.Membership.ByOrg(ctx, sharedOrg.ID)
		s.NoError(err)
		s.Len(memberships, 1)
		s.Equal(authz.RoleViewer, memberships[0].Role)

		got, err := s.Membership.UpdateRole(ctx, sharedOrg.ID, user.ID, mUser2SharedOrg.ID.String(), authz.RoleAdmin)
		s.NoError(err)
		s.Equal(authz.RoleAdmin, got.Role)
	})
}

func (s *membershipIntegrationTestSuite) TestCreateMembership() {
	assert := assert.New(s.T())
	ctx := context.Background()

	// Create User
	user, err := s.User.UpsertByEmail(ctx, "foo@test.com", nil)
	assert.NoError(err)

	s.T().Run("Create current", func(t *testing.T) {
		org, err := s.Organization.CreateWithRandomName(ctx)
		assert.NoError(err)

		m, err := s.Membership.Create(ctx, org.ID, user.ID, biz.WithCurrentMembership())
		assert.NoError(err)
		assert.Equal(true, m.Current, "Membership should be current")

		assert.Equal(user.ID, m.User.ID, "User ID")
		assert.Equal(org.ID, m.OrganizationID.String(), "Organization ID")
		assert.EqualValues(org, m.Org, "Embedded organization")
	})

	s.T().Run("Non current", func(t *testing.T) {
		org, err := s.Organization.CreateWithRandomName(ctx)
		assert.NoError(err)

		m, err := s.Membership.Create(ctx, org.ID, user.ID)
		assert.NoError(err)
		assert.Equal(false, m.Current, "Membership should not be current")
	})

	s.T().Run("current override", func(t *testing.T) {
		org, err := s.Organization.CreateWithRandomName(ctx)
		assert.NoError(err)
		org2, err := s.Organization.CreateWithRandomName(ctx)
		assert.NoError(err)

		m, err := s.Membership.Create(ctx, org.ID, user.ID, biz.WithCurrentMembership())
		assert.NoError(err)
		s.True(m.Current)
		// Creating a new one will override the current status of the previous one
		m, err = s.Membership.Create(ctx, org2.ID, user.ID, biz.WithCurrentMembership())
		assert.NoError(err)
		s.True(m.Current)

		m, err = s.Membership.FindByOrgAndUser(ctx, org.ID, user.ID)
		assert.NoError(err)
		s.False(m.Current)
	})

	s.T().Run("Invalid ORG", func(t *testing.T) {
		m, err := s.Membership.Create(ctx, uuid.NewString(), user.ID)
		assert.Error(err)
		assert.Nil(m)
	})

	s.T().Run("Invalid User", func(t *testing.T) {
		org, err := s.Organization.CreateWithRandomName(ctx)
		assert.NoError(err)
		m, err := s.Membership.Create(ctx, org.ID, uuid.NewString())
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
