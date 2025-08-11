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
	"fmt"
	"testing"

	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/authz"
	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/biz"
	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/biz/testhelpers"
	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/pagination"

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
		memberships, count, err := s.Membership.ByOrg(ctx, userOrg.ID, &biz.ListByOrgOpts{}, nil)
		s.NoError(err)
		s.Len(memberships, 1)
		s.Equal(1, count)
		s.Equal(memberships[0].OrganizationID.String(), userOrg.ID)
		s.Equal(memberships[0].User.Email, user.Email)
		s.Equal(memberships[0].Role, authz.RoleViewer)
	})

	s.Run("shared org", func() {
		memberships, count, err := s.Membership.ByOrg(ctx, sharedOrg.ID, &biz.ListByOrgOpts{}, nil)
		s.NoError(err)
		s.Len(memberships, 2)
		s.Equal(2, count)
	})

	s.T().Run("non existing org", func(t *testing.T) {
		memberships, count, err := s.Membership.ByOrg(ctx, uuid.NewString(), &biz.ListByOrgOpts{}, nil)
		s.NoError(err)
		s.Len(memberships, 0)
		s.Equal(0, count)
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

	mUser, err := s.Membership.Create(ctx, userOrg.ID, user.ID, biz.WithMembershipRole(authz.RoleOwner), biz.WithCurrentMembership())
	s.NoError(err)
	mUserSharedOrg, err := s.Membership.Create(ctx, sharedOrg.ID, user.ID, biz.WithMembershipRole(authz.RoleOwner), biz.WithCurrentMembership())
	s.NoError(err)
	mUser2SharedOrg, err := s.Membership.Create(ctx, sharedOrg.ID, user2.ID, biz.WithMembershipRole(authz.RoleOwner), biz.WithCurrentMembership())
	s.NoError(err)

	s.T().Run("invalid userID", func(t *testing.T) {
		err := s.Membership.Leave(ctx, "invalid", mUser.ID.String())
		s.True(biz.IsErrInvalidUUID(err))
	})

	s.T().Run("invalid orgID", func(t *testing.T) {
		err := s.Membership.Leave(ctx, user.ID, "invalid")
		s.True(biz.IsErrInvalidUUID(err))
	})

	s.T().Run("membership ID from another user", func(t *testing.T) {
		err := s.Membership.Leave(ctx, user.ID, mUser2SharedOrg.ID.String())
		s.True(biz.IsNotFound(err))
	})

	s.T().Run("cannot leave when the only member (sole owner)", func(t *testing.T) {
		err := s.Membership.Leave(ctx, user.ID, mUser.ID.String())
		s.Require().Error(err)
		s.True(biz.IsErrValidation(err))
		s.Contains(err.Error(), "sole owner")
		// The org should still exist
		_, err = s.Organization.FindByID(ctx, userOrg.ID)
		s.NoError(err)
	})

	s.T().Run("can leave when there are more than 1 member", func(t *testing.T) {
		err := s.Membership.Leave(ctx, user.ID, mUserSharedOrg.ID.String())
		s.NoError(err)
		// The org should not be deleted
		got, err := s.Organization.FindByID(ctx, sharedOrg.ID)
		s.NoError(err)

		// User 2 is still a member
		members, count, err := s.Membership.ByOrg(ctx, got.ID, &biz.ListByOrgOpts{}, nil)
		s.NoError(err)
		s.Len(members, 1)
		s.Equal(1, count)
		s.Equal(user2.ID, members[0].User.ID)
	})

	s.T().Run("cannot leave when would become sole owner", func(t *testing.T) {
		err := s.Membership.Leave(ctx, user2.ID, mUser2SharedOrg.ID.String())
		s.Error(err)
		s.True(biz.IsErrValidation(err))
		s.Contains(err.Error(), "sole owner")
		// The org should still exist
		_, err = s.Organization.FindByID(ctx, sharedOrg.ID)
		s.NoError(err)
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
		memberships, count, err := s.Membership.ByOrg(ctx, sharedOrg.ID, &biz.ListByOrgOpts{}, nil)
		s.NoError(err)
		s.Len(memberships, 1)
		s.Equal(1, count)

		err = s.Membership.DeleteOther(ctx, sharedOrg.ID, user.ID, mUser2SharedOrg.ID.String())
		s.NoError(err)

		memberships, count, err = s.Membership.ByOrg(ctx, sharedOrg.ID, &biz.ListByOrgOpts{}, nil)
		s.NoError(err)
		s.Len(memberships, 0)
		s.Equal(0, count)
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
		memberships, count, err := s.Membership.ByOrg(ctx, sharedOrg.ID, &biz.ListByOrgOpts{}, nil)
		s.NoError(err)
		s.Len(memberships, 1)
		s.Equal(1, count)
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

func (s *membershipIntegrationTestSuite) TestDeleteCleanup() {
	ctx := context.Background()

	// Create users
	user, err := s.User.UpsertByEmail(ctx, "cleanup-test@example.com", nil)
	s.NoError(err)
	adminUser, err := s.User.UpsertByEmail(ctx, "admin-user@example.com", nil)
	s.NoError(err)

	// Create an organization
	org, err := s.Organization.CreateWithRandomName(ctx)
	s.NoError(err)

	// Add users to organization with different roles
	membershipUser, err := s.Membership.Create(ctx, org.ID, user.ID, biz.WithCurrentMembership())
	s.NoError(err)
	_, err = s.Membership.Create(ctx, org.ID, adminUser.ID, biz.WithMembershipRole(authz.RoleAdmin), biz.WithCurrentMembership())
	s.NoError(err)

	// Create a project in the organization
	project, err := s.Project.Create(ctx, org.ID, "test-cleanup-project")
	s.NoError(err)

	// Add user to the project
	projectRef := &biz.IdentityReference{ID: &project.ID}
	projectOpts := &biz.AddMemberToProjectOpts{
		ProjectReference: projectRef,
		UserEmail:        "cleanup-test@example.com",
		RequesterID:      uuid.MustParse(adminUser.ID),
		Role:             authz.RoleProjectViewer,
	}
	_, err = s.Project.AddMemberToProject(ctx, uuid.MustParse(org.ID), projectOpts)
	s.NoError(err)

	// Create a group in the organization
	userUUID := uuid.MustParse(user.ID)
	group, err := s.Group.Create(ctx, uuid.MustParse(org.ID), "test-cleanup-group", "Group for cleanup testing", &userUUID)
	s.NoError(err)

	// Create another user to add to the group
	otherUser, err := s.User.UpsertByEmail(ctx, "other-member@example.com", nil)
	s.NoError(err)
	_, err = s.Membership.Create(ctx, org.ID, otherUser.ID)
	s.NoError(err)

	// Add other user to the group
	groupRef := &biz.IdentityReference{ID: &group.ID}
	groupOpts := &biz.AddMemberToGroupOpts{
		IdentityReference: groupRef,
		UserEmail:         "other-member@example.com",
		RequesterID:       uuid.MustParse(user.ID),
		Maintainer:        false,
	}
	_, err = s.Group.AddMemberToGroup(ctx, uuid.MustParse(org.ID), groupOpts)
	s.NoError(err)

	// Verify initial state
	s.Run("verify initial state", func() {
		// Verify user is in the project
		members, count, err := s.Project.ListMembers(ctx, uuid.MustParse(org.ID), projectRef, nil)
		s.NoError(err)
		s.Equal(1, count)
		s.Equal(1, len(members))
		s.Equal(user.ID, members[0].User.ID)

		// Verify user is in the group as maintainer
		groupMembers, groupCount, err := s.Group.ListMembers(ctx, uuid.MustParse(org.ID), &biz.ListMembersOpts{
			IdentityReference: groupRef,
		}, nil)
		s.NoError(err)
		s.Equal(2, groupCount) // User + otherUser
		userFound := false
		for _, member := range groupMembers {
			if member.User.ID == user.ID {
				s.True(member.Maintainer)
				userFound = true
				break
			}
		}
		s.True(userFound, "User should be found in the group as a maintainer")
	})

	// Delete the user's membership
	s.Run("delete user membership", func() {
		err := s.Membership.Leave(ctx, user.ID, membershipUser.ID.String())
		s.NoError(err)

		// Check that the organization still exists (since there's still admin user)
		_, err = s.Organization.FindByID(ctx, org.ID)
		s.NoError(err)

		// Verify user is removed from project
		projectMembers, projectCount, err := s.Project.ListMembers(ctx, uuid.MustParse(org.ID), projectRef, nil)
		s.NoError(err)
		s.Equal(0, projectCount)
		s.Equal(0, len(projectMembers))

		// Verify user is removed from group but other member remains
		groupMembers, groupCount, err := s.Group.ListMembers(ctx, uuid.MustParse(org.ID), &biz.ListMembersOpts{
			IdentityReference: groupRef,
		}, nil)
		s.NoError(err)
		s.Equal(1, groupCount) // Only otherUser should remain
		s.Equal(1, len(groupMembers))
		s.Equal(otherUser.ID, groupMembers[0].User.ID)
		s.False(groupMembers[0].Maintainer)

		// Verify group membership has been decremented
		updatedGroup, err := s.Group.Get(ctx, uuid.MustParse(org.ID), &biz.IdentityReference{ID: &group.ID})
		s.NoError(err)
		s.Equal(1, updatedGroup.MemberCount)
	})
}

func (s *membershipIntegrationTestSuite) TestDeleteWithGroups() {
	ctx := context.Background()

	// Create a user
	user, err := s.User.UpsertByEmail(ctx, "groups-test@example.com", nil)
	s.NoError(err)
	userUUID := uuid.MustParse(user.ID)

	// Create an organization
	org, err := s.Organization.CreateWithRandomName(ctx)
	s.NoError(err)
	orgUUID := uuid.MustParse(org.ID)

	// Add user to organization
	membershipUser, err := s.Membership.Create(ctx, org.ID, user.ID, biz.WithMembershipRole(authz.RoleOwner), biz.WithCurrentMembership())
	s.NoError(err)

	// Create multiple groups with the user as maintainer
	group1, err := s.Group.Create(ctx, orgUUID, "group-1", "First group", &userUUID)
	s.NoError(err)
	group2, err := s.Group.Create(ctx, orgUUID, "group-2", "Second group", &userUUID)
	s.NoError(err)

	// Add the groups to a project
	pr, err := s.Project.Create(ctx, org.ID, "test-groups-project")
	s.NoError(err)
	projectRef := &biz.IdentityReference{ID: &pr.ID}

	// Add group1 to project
	groupProjectOpts := &biz.AddMemberToProjectOpts{
		ProjectReference: projectRef,
		GroupReference:   &biz.IdentityReference{ID: &group1.ID},
		RequesterID:      userUUID,
		Role:             authz.RoleProjectAdmin,
	}
	_, err = s.Project.AddMemberToProject(ctx, orgUUID, groupProjectOpts)
	s.NoError(err)

	// Verify initial state
	s.Run("verify initial state with groups", func() {
		// Check user is a maintainer in both groups
		group1Members, _, err := s.Group.ListMembers(ctx, orgUUID, &biz.ListMembersOpts{
			IdentityReference: &biz.IdentityReference{ID: &group1.ID},
		}, nil)
		s.NoError(err)
		s.Equal(1, len(group1Members))
		s.Equal(user.ID, group1Members[0].User.ID)
		s.True(group1Members[0].Maintainer)

		group2Members, _, err := s.Group.ListMembers(ctx, orgUUID, &biz.ListMembersOpts{
			IdentityReference: &biz.IdentityReference{ID: &group2.ID},
		}, nil)
		s.NoError(err)
		s.Equal(1, len(group2Members))
		s.Equal(user.ID, group2Members[0].User.ID)
		s.True(group2Members[0].Maintainer)

		// Check group1 is in the project
		projectMembers, _, err := s.Project.ListMembers(ctx, orgUUID, projectRef, nil)
		s.NoError(err)
		s.Equal(1, len(projectMembers))
		s.Equal(group1.ID, projectMembers[0].Group.ID)
	})

	// Delete the user's membership
	s.Run("cannot delete user membership when sole owner", func() {
		err := s.Membership.Leave(ctx, user.ID, membershipUser.ID.String())
		s.Error(err)
		s.True(biz.IsErrValidation(err))
		s.Contains(err.Error(), "sole owner")

		// The organization should still exist since sole owners cannot leave
		_, err = s.Organization.FindByID(ctx, org.ID)
		s.NoError(err)

		// Groups should still exist since user couldn't leave (no cleanup happened)
		_, err = s.Group.Get(ctx, orgUUID, &biz.IdentityReference{ID: &group1.ID})
		s.NoError(err, "Group 1 should still exist")

		_, err = s.Group.Get(ctx, orgUUID, &biz.IdentityReference{ID: &group2.ID})
		s.NoError(err, "Group 2 should still exist since user couldn't leave")

		// The project should still exist since no cleanup happened
		_, err = s.Project.FindProjectByReference(ctx, org.ID, &biz.IdentityReference{ID: projectRef.ID})
		s.NoError(err, "Project should still exist since user couldn't leave")

		// Verify group memberships still exist since user couldn't leave
		group1Mem, group1Err := s.Repos.GroupRepo.FindGroupMembershipByGroupAndID(ctx, group1.ID, userUUID)
		s.NoError(group1Err)
		s.NotNil(group1Mem)

		group2Mem, group2Err := s.Repos.GroupRepo.FindGroupMembershipByGroupAndID(ctx, group2.ID, userUUID)
		s.NoError(group2Err)
		s.NotNil(group2Mem)
	})
}

// Run the tests
func TestMembershipUseCase(t *testing.T) {
	suite.Run(t, new(membershipIntegrationTestSuite))
	suite.Run(t, new(membershipFilteringPaginationTestSuite))
}

// Utility struct to hold the test suite
type membershipIntegrationTestSuite struct {
	testhelpers.UseCasesEachTestSuite
}

type membershipFilteringPaginationTestSuite struct {
	testhelpers.UseCasesEachTestSuite
	org  *biz.Organization
	user *biz.User
}

func (s *membershipFilteringPaginationTestSuite) SetupTest() {
	var err error
	assert := assert.New(s.T())
	s.TestingUseCases = testhelpers.NewTestingUseCases(s.T())

	ctx := context.Background()
	s.org, err = s.Organization.CreateWithRandomName(ctx)
	assert.NoError(err)

	// Create a user for membership tests
	s.user, err = s.User.UpsertByEmail(ctx, fmt.Sprintf("test-user-%s@example.com", uuid.New().String()), nil)
	assert.NoError(err)

	// Add user to organization
	_, err = s.Membership.Create(ctx, s.org.ID, s.user.ID)
	assert.NoError(err)
}

// Test comprehensive filtering and pagination functionality
func (s *membershipFilteringPaginationTestSuite) TestByOrgWithFiltersAndPagination() {
	ctx := context.Background()

	// Create an organization
	org, err := s.Organization.CreateWithRandomName(ctx)
	s.NoError(err)

	// Create users with different names and emails for filtering tests
	johnSmith, err := s.User.UpsertByEmail(ctx, "john.smith@example.com", &biz.UpsertByEmailOpts{
		FirstName: toPtrS("John"),
		LastName:  toPtrS("Smith"),
	})
	s.NoError(err)

	janeSmith, err := s.User.UpsertByEmail(ctx, "jane.smith@example.com", &biz.UpsertByEmailOpts{
		FirstName: toPtrS("Jane"),
		LastName:  toPtrS("Smith"),
	})
	s.NoError(err)

	bobJohnson, err := s.User.UpsertByEmail(ctx, "bob.johnson@company.com", &biz.UpsertByEmailOpts{
		FirstName: toPtrS("Bob"),
		LastName:  toPtrS("Johnson"),
	})
	s.NoError(err)

	aliceWilson, err := s.User.UpsertByEmail(ctx, "alice.wilson@test.org", &biz.UpsertByEmailOpts{
		FirstName: toPtrS("Alice"),
		LastName:  toPtrS("Wilson"),
	})
	s.NoError(err)

	// Add all users to the organization
	_, err = s.Membership.Create(ctx, org.ID, johnSmith.ID)
	s.NoError(err)
	_, err = s.Membership.Create(ctx, org.ID, janeSmith.ID)
	s.NoError(err)
	_, err = s.Membership.Create(ctx, org.ID, bobJohnson.ID)
	s.NoError(err)
	_, err = s.Membership.Create(ctx, org.ID, aliceWilson.ID)
	s.NoError(err)

	s.Run("filter by first name", func() {
		nameFilter := "Bob"
		opts := &biz.ListByOrgOpts{Name: &nameFilter}

		memberships, count, err := s.Membership.ByOrg(ctx, org.ID, opts, nil)
		s.NoError(err)
		s.Equal(1, count)
		s.Len(memberships, 1)
		s.Equal("Bob", memberships[0].User.FirstName)
		s.Equal("bob.johnson@company.com", memberships[0].User.Email)
	})

	s.Run("filter by last name", func() {
		nameFilter := "Smith"
		opts := &biz.ListByOrgOpts{Name: &nameFilter}

		memberships, count, err := s.Membership.ByOrg(ctx, org.ID, opts, nil)
		s.NoError(err)
		s.Equal(2, count)
		s.Len(memberships, 2)

		// Should find both John Smith and Jane Smith
		emails := []string{memberships[0].User.Email, memberships[1].User.Email}
		s.Contains(emails, "john.smith@example.com")
		s.Contains(emails, "jane.smith@example.com")
	})

	s.Run("filter by partial name", func() {
		nameFilter := "ob" // Should match "Bob"
		opts := &biz.ListByOrgOpts{Name: &nameFilter}

		memberships, count, err := s.Membership.ByOrg(ctx, org.ID, opts, nil)
		s.NoError(err)
		s.Equal(1, count)
		s.Len(memberships, 1)
		s.Equal("bob.johnson@company.com", memberships[0].User.Email)
	})

	s.Run("filter by email domain", func() {
		emailFilter := "@example.com"
		opts := &biz.ListByOrgOpts{Email: &emailFilter}

		memberships, count, err := s.Membership.ByOrg(ctx, org.ID, opts, nil)
		s.NoError(err)
		s.Equal(2, count)
		s.Len(memberships, 2)

		// Should find both John Smith and Jane Smith
		emails := []string{memberships[0].User.Email, memberships[1].User.Email}
		s.Contains(emails, "john.smith@example.com")
		s.Contains(emails, "jane.smith@example.com")
	})

	s.Run("filter by specific email", func() {
		emailFilter := "bob.johnson"
		opts := &biz.ListByOrgOpts{Email: &emailFilter}

		memberships, count, err := s.Membership.ByOrg(ctx, org.ID, opts, nil)
		s.NoError(err)
		s.Equal(1, count)
		s.Len(memberships, 1)
		s.Equal("bob.johnson@company.com", memberships[0].User.Email)
	})

	s.Run("filter with no matches", func() {
		nameFilter := "NonExistentName"
		opts := &biz.ListByOrgOpts{Name: &nameFilter}

		memberships, count, err := s.Membership.ByOrg(ctx, org.ID, opts, nil)
		s.NoError(err)
		s.Equal(0, count)
		s.Len(memberships, 0)
	})

	s.Run("pagination with limit", func() {
		opts := &biz.ListByOrgOpts{}
		paginationOpts, err := pagination.NewOffsetPaginationOpts(0, 2)
		s.NoError(err)

		memberships, count, err := s.Membership.ByOrg(ctx, org.ID, opts, paginationOpts)
		s.NoError(err)
		s.Equal(4, count)     // Total count should be 4
		s.Len(memberships, 2) // But only 2 results returned due to limit
	})

	s.Run("pagination with offset", func() {
		opts := &biz.ListByOrgOpts{}
		paginationOpts, err := pagination.NewOffsetPaginationOpts(2, 2)
		s.NoError(err)

		memberships, count, err := s.Membership.ByOrg(ctx, org.ID, opts, paginationOpts)
		s.NoError(err)
		s.Equal(4, count)     // Total count should still be 4
		s.Len(memberships, 2) // Should return remaining 2 results
	})

	s.Run("pagination beyond available results", func() {
		opts := &biz.ListByOrgOpts{}
		paginationOpts, err := pagination.NewOffsetPaginationOpts(10, 5)
		s.NoError(err)

		memberships, count, err := s.Membership.ByOrg(ctx, org.ID, opts, paginationOpts)
		s.NoError(err)
		s.Equal(4, count)     // Total count should still be 4
		s.Len(memberships, 0) // No results due to high offset
	})

	s.Run("pagination with filtering", func() {
		nameFilter := "Smith"
		opts := &biz.ListByOrgOpts{Name: &nameFilter}
		paginationOpts, err := pagination.NewOffsetPaginationOpts(0, 1)
		s.NoError(err)

		memberships, count, err := s.Membership.ByOrg(ctx, org.ID, opts, paginationOpts)
		s.NoError(err)
		s.Equal(2, count)     // Total filtered count should be 2 (both Smiths)
		s.Len(memberships, 1) // But only 1 result returned due to limit
		s.Contains(memberships[0].User.Email, "smith@example.com")
	})

	s.Run("empty filters should return all results", func() {
		emptyName := ""
		emptyEmail := ""
		opts := &biz.ListByOrgOpts{
			Name:  &emptyName,
			Email: &emptyEmail,
		}

		memberships, count, err := s.Membership.ByOrg(ctx, org.ID, opts, nil)
		s.NoError(err)
		s.Equal(4, count)
		s.Len(memberships, 4)
	})

	s.Run("nil filter options should return all results", func() {
		memberships, count, err := s.Membership.ByOrg(ctx, org.ID, nil, nil)
		s.NoError(err)
		s.Equal(4, count)
		s.Len(memberships, 4)
	})

	s.Run("case insensitive filtering", func() {
		nameFilter := "SMITH" // uppercase
		opts := &biz.ListByOrgOpts{Name: &nameFilter}

		memberships, count, err := s.Membership.ByOrg(ctx, org.ID, opts, nil)
		s.NoError(err)
		s.Equal(2, count) // Should still find both Smiths
		s.Len(memberships, 2)
	})
}

// Test that verifies the ordering of results
func (s *membershipFilteringPaginationTestSuite) TestByOrgOrdering() {
	ctx := context.Background()

	// Create an organization
	org, err := s.Organization.CreateWithRandomName(ctx)
	s.NoError(err)

	// Create users and add them with some delay to test ordering
	user1, err := s.User.UpsertByEmail(ctx, "first@example.com", nil)
	s.NoError(err)
	_, err = s.Membership.Create(ctx, org.ID, user1.ID)
	s.NoError(err)

	// Small delay to ensure different creation times
	user2, err := s.User.UpsertByEmail(ctx, "second@example.com", nil)
	s.NoError(err)
	_, err = s.Membership.Create(ctx, org.ID, user2.ID)
	s.NoError(err)

	user3, err := s.User.UpsertByEmail(ctx, "third@example.com", nil)
	s.NoError(err)
	_, err = s.Membership.Create(ctx, org.ID, user3.ID)
	s.NoError(err)

	s.Run("results should be ordered by creation date descending", func() {
		opts := &biz.ListByOrgOpts{}

		memberships, count, err := s.Membership.ByOrg(ctx, org.ID, opts, nil)
		s.NoError(err)
		s.Equal(3, count)
		s.Len(memberships, 3)

		// Most recent should be first
		s.Equal("third@example.com", memberships[0].User.Email)
		s.Equal("second@example.com", memberships[1].User.Email)
		s.Equal("first@example.com", memberships[2].User.Email)

		// Verify timestamps are in descending order
		s.True(memberships[0].CreatedAt.After(*memberships[1].CreatedAt))
		s.True(memberships[1].CreatedAt.After(*memberships[2].CreatedAt))
	})
}

// Test edge cases and error scenarios
func (s *membershipFilteringPaginationTestSuite) TestByOrgEdgeCases() {
	ctx := context.Background()

	// Create an organization
	org, err := s.Organization.CreateWithRandomName(ctx)
	s.NoError(err)

	s.Run("organization with no members", func() {
		opts := &biz.ListByOrgOpts{}

		memberships, count, err := s.Membership.ByOrg(ctx, org.ID, opts, nil)
		s.NoError(err)
		s.Equal(0, count)
		s.Len(memberships, 0)
	})

	s.Run("non-existent organization", func() {
		nonExistentOrgID := uuid.NewString()
		opts := &biz.ListByOrgOpts{}

		memberships, count, err := s.Membership.ByOrg(ctx, nonExistentOrgID, opts, nil)
		s.NoError(err)
		s.Equal(0, count)
		s.Len(memberships, 0)
	})

	s.Run("invalid UUID", func() {
		invalidOrgID := "invalid-uuid"
		opts := &biz.ListByOrgOpts{}

		_, _, err := s.Membership.ByOrg(ctx, invalidOrgID, opts, nil)
		s.Error(err)
		s.True(biz.IsErrInvalidUUID(err))
	})

	// Add a user to test special characters in names/emails
	user, err := s.User.UpsertByEmail(ctx, "test.user+special@example.com", &biz.UpsertByEmailOpts{
		FirstName: toPtrS("Test-User"),
		LastName:  toPtrS("O'Connor"),
	})
	s.NoError(err)
	_, err = s.Membership.Create(ctx, org.ID, user.ID)
	s.NoError(err)

	s.Run("special characters in filters", func() {
		nameFilter := "O'Connor"
		opts := &biz.ListByOrgOpts{Name: &nameFilter}

		memberships, count, err := s.Membership.ByOrg(ctx, org.ID, opts, nil)
		s.NoError(err)
		s.Equal(1, count)
		s.Len(memberships, 1)
		s.Equal("Test-User", memberships[0].User.FirstName)
		s.Equal("O'Connor", memberships[0].User.LastName)
	})

	s.Run("special characters in email filter", func() {
		emailFilter := "test.user+special"
		opts := &biz.ListByOrgOpts{Email: &emailFilter}

		memberships, count, err := s.Membership.ByOrg(ctx, org.ID, opts, nil)
		s.NoError(err)
		s.Equal(1, count)
		s.Len(memberships, 1)
		s.Equal("test.user+special@example.com", memberships[0].User.Email)
	})
}
