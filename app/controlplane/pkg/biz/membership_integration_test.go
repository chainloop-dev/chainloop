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
	"slices"
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
		err := s.Membership.LeaveAndDeleteOrg(ctx, user.ID, membershipUser.ID.String())
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
	membershipUser, err := s.Membership.Create(ctx, org.ID, user.ID, biz.WithMembershipRole(authz.RoleAdmin), biz.WithCurrentMembership())
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
	s.Run("delete user membership with groups", func() {
		err := s.Membership.LeaveAndDeleteOrg(ctx, user.ID, membershipUser.ID.String())
		s.NoError(err)

		// The organization should be deleted since this was the only user
		_, err = s.Organization.FindByID(ctx, org.ID)
		s.True(biz.IsNotFound(err), "Organization should be deleted")

		// All groups should be soft-deleted
		_, err = s.Group.Get(ctx, orgUUID, &biz.IdentityReference{ID: &group1.ID})
		s.True(biz.IsNotFound(err), "Group 1 should be deleted")

		_, err = s.Group.Get(ctx, orgUUID, &biz.IdentityReference{ID: &group2.ID})
		s.True(biz.IsNotFound(err), "Group 2 should be deleted")

		// The project should be deleted
		_, err = s.Project.FindProjectByReference(ctx, org.ID, &biz.IdentityReference{ID: projectRef.ID})
		s.True(biz.IsNotFound(err), "Project should be deleted")

		// Verify group memberships are marked as deleted
		group1Mem, group1Err := s.Repos.GroupRepo.FindGroupMembershipByGroupAndID(ctx, group1.ID, userUUID)
		s.True(biz.IsNotFound(group1Err))
		s.Nil(group1Mem)

		group2Mem, group2Err := s.Repos.GroupRepo.FindGroupMembershipByGroupAndID(ctx, group2.ID, userUUID)
		s.True(biz.IsNotFound(group2Err))
		s.Nil(group2Mem)
	})
}

func (s *membershipIntegrationTestSuite) TestListAllMemberships() {
	// test illegal combinations (viewer and project admin)

	ctx := context.Background()

	// Create a user
	user, err := s.User.UpsertByEmail(ctx, "user@example.com", nil)
	s.NoError(err)
	userUUID := uuid.MustParse(user.ID)

	// Create an organization
	org, err := s.Organization.CreateWithRandomName(ctx)
	s.NoError(err)
	orgUUID := uuid.MustParse(org.ID)

	// Add user to organization
	_, err = s.Membership.Create(ctx, org.ID, user.ID, biz.WithMembershipRole(authz.RoleViewer), biz.WithCurrentMembership())
	s.NoError(err)

	groupProjectAdmin, err := s.Group.Create(ctx, orgUUID, "group-admin", "Group project admin", nil)
	s.NoError(err)

	groupProjectViewer, err := s.Group.Create(ctx, orgUUID, "group-viewer", "Group project viewer", nil)
	s.NoError(err)

	// Add user to both groups
	_, err = s.Group.AddMemberToGroup(ctx, orgUUID, &biz.AddMemberToGroupOpts{
		IdentityReference: &biz.IdentityReference{ID: &groupProjectAdmin.ID},
		UserEmail:         user.Email,
	})
	s.NoError(err)

	_, err = s.Group.AddMemberToGroup(ctx, orgUUID, &biz.AddMemberToGroupOpts{
		IdentityReference: &biz.IdentityReference{ID: &groupProjectViewer.ID},
		UserEmail:         user.Email,
	})
	s.NoError(err)

	// Create a project
	pr, err := s.Project.Create(ctx, org.ID, "test-project")
	s.NoError(err)
	projectRef := &biz.IdentityReference{ID: &pr.ID}

	// try to add user to project as project admin
	_, err = s.Project.AddMemberToProject(ctx, orgUUID, &biz.AddMemberToProjectOpts{
		ProjectReference: projectRef,
		UserEmail:        user.Email,
		Role:             authz.RoleProjectAdmin,
	})
	// Expect error because of an illegal combination
	s.Error(err)

	// Add group admin to project as project admin
	_, err = s.Project.AddMemberToProject(ctx, orgUUID, &biz.AddMemberToProjectOpts{
		ProjectReference: projectRef,
		GroupReference:   &biz.IdentityReference{ID: &groupProjectAdmin.ID},
		Role:             authz.RoleProjectAdmin,
	})
	s.NoError(err)

	// User shouldn't acquire the project admin role
	mm, err := s.Membership.ListAllMembershipsForUser(ctx, userUUID)
	s.NoError(err)
	// Expect only org membership
	s.Equal(1, len(mm))
	s.Equal(authz.ResourceTypeOrganization, mm[0].ResourceType)

	// Add group viewer
	_, err = s.Project.AddMemberToProject(ctx, orgUUID, &biz.AddMemberToProjectOpts{
		ProjectReference: projectRef,
		GroupReference:   &biz.IdentityReference{ID: &groupProjectViewer.ID},
		Role:             authz.RoleProjectViewer,
	})
	s.NoError(err)

	// expect user to acquire the membership
	mm, err = s.Membership.ListAllMembershipsForUser(ctx, userUUID)
	s.NoError(err)
	s.Equal(2, len(mm))
	s.True(slices.ContainsFunc(mm, func(m *biz.Membership) bool {
		return m.ResourceType == authz.ResourceTypeProject && m.Role == authz.RoleProjectViewer && m.ResourceID == pr.ID
	}))
}

// Run the tests
func TestMembershipUseCase(t *testing.T) {
	suite.Run(t, new(membershipIntegrationTestSuite))
}

// Utility struct to hold the test suite
type membershipIntegrationTestSuite struct {
	testhelpers.UseCasesEachTestSuite
}
