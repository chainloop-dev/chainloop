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
	"github.com/google/uuid"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

const receiverEmail = "sarah@cyberdyne.io"

func (s *OrgInvitationIntegrationTestSuite) TestList() {
	ctx := context.Background()
	inviteOrg1A, err := s.OrgInvitation.Create(ctx, s.org1.ID, s.user.ID, receiverEmail)
	s.NoError(err)
	// same org but another user
	inviteOrg1B, err := s.OrgInvitation.Create(ctx, s.org1.ID, s.user2.ID, "another-email@cyberdyne.io")
	s.NoError(err)
	inviteOrg2A, err := s.OrgInvitation.Create(ctx, s.org2.ID, s.user.ID, receiverEmail)
	s.NoError(err)

	testCases := []struct {
		name     string
		orgID    string
		expected []*biz.OrgInvitation
	}{
		{
			name:     "org1",
			orgID:    s.org1.ID,
			expected: []*biz.OrgInvitation{inviteOrg1A, inviteOrg1B},
		},
		{
			name:     "org2",
			orgID:    s.org2.ID,
			expected: []*biz.OrgInvitation{inviteOrg2A},
		},
		{
			name:     "org3",
			orgID:    s.org3.ID,
			expected: []*biz.OrgInvitation{},
		},
	}

	for _, tc := range testCases {
		s.T().Run(tc.name, func(t *testing.T) {
			invites, err := s.OrgInvitation.ListByOrg(ctx, tc.orgID)
			s.NoError(err)
			s.Equal(tc.expected, invites)
		})
	}
}

func (s *OrgInvitationIntegrationTestSuite) TestCreate() {
	ctx := context.Background()
	s.T().Run("invalid org ID", func(t *testing.T) {
		invite, err := s.OrgInvitation.Create(ctx, "deadbeef", s.user.ID, receiverEmail)
		s.Error(err)
		s.True(biz.IsErrInvalidUUID(err))
		s.Nil(invite)
	})

	s.T().Run("invalid user ID", func(t *testing.T) {
		invite, err := s.OrgInvitation.Create(ctx, s.org1.ID, "deadbeef", receiverEmail)
		s.Error(err)
		s.True(biz.IsErrInvalidUUID(err))
		s.Nil(invite)
	})

	s.T().Run("missing receiver email", func(t *testing.T) {
		invite, err := s.OrgInvitation.Create(ctx, s.org1.ID, s.user.ID, "")
		s.Error(err)
		s.True(biz.IsErrValidation(err))
		s.Nil(invite)
	})

	s.T().Run("receiver email same than sender", func(t *testing.T) {
		invite, err := s.OrgInvitation.Create(ctx, s.org1.ID, s.user.ID, s.user.Email)
		s.Error(err)
		s.ErrorContains(err, "sender and receiver emails cannot be the same")
		s.True(biz.IsErrValidation(err))
		s.Nil(invite)
	})

	s.T().Run("receiver is already a member", func(t *testing.T) {
		invite, err := s.OrgInvitation.Create(ctx, s.org1.ID, s.user.ID, s.user2.Email)
		s.Error(err)
		s.ErrorContains(err, "user already exists in the org")
		s.True(biz.IsErrValidation(err))
		s.Nil(invite)
	})

	s.T().Run("org not found", func(t *testing.T) {
		invite, err := s.OrgInvitation.Create(ctx, s.org1.ID, uuid.NewString(), receiverEmail)
		s.Error(err)
		s.True(biz.IsNotFound(err))
		s.Nil(invite)
	})

	s.T().Run("sender is not member of that org", func(t *testing.T) {
		invite, err := s.OrgInvitation.Create(ctx, s.org3.ID, s.user.ID, receiverEmail)
		s.Error(err)
		s.ErrorContains(err, "user does not have permission to invite to this org")
		s.True(biz.IsNotFound(err))
		s.Nil(invite)
	})

	s.T().Run("sender is not member of that org but receiver is", func(t *testing.T) {
		invite, err := s.OrgInvitation.Create(ctx, s.org3.ID, s.user.ID, s.user2.Email)
		s.Error(err)
		s.ErrorContains(err, "user does not have permission to invite to this org")
		s.True(biz.IsNotFound(err))
		s.Nil(invite)
	})

	s.T().Run("can create invites for org1 and 2", func(t *testing.T) {
		for _, org := range []*biz.Organization{s.org1, s.org2} {
			invite, err := s.OrgInvitation.Create(ctx, org.ID, s.user.ID, receiverEmail)
			s.NoError(err)
			s.Equal(org, invite.Org)
			s.Equal(s.user, invite.Sender)
			s.Equal(receiverEmail, invite.ReceiverEmail)
			s.Equal(biz.OrgInvitationStatusPending, invite.Status)
			s.NotNil(invite.CreatedAt)
		}
	})

	s.T().Run("but can't create if there is one pending", func(t *testing.T) {
		invite, err := s.OrgInvitation.Create(ctx, s.org1.ID, s.user.ID, receiverEmail)
		s.Error(err)
		s.ErrorContains(err, "already exists")
		s.True(biz.IsErrValidation(err))
		s.Nil(invite)
	})

	s.T().Run("but it can if it's another email", func(t *testing.T) {
		invite, err := s.OrgInvitation.Create(ctx, s.org1.ID, s.user.ID, "anotheremail@cyberdyne.io")
		s.Equal("anotheremail@cyberdyne.io", invite.ReceiverEmail)
		s.Equal(s.org1, invite.Org)
		s.NoError(err)
	})

	s.T().Run("the default role is viewer", func(t *testing.T) {
		invite, err := s.OrgInvitation.Create(ctx, s.org1.ID, s.user.ID, "viewer@cyberdyne.io")
		s.NoError(err)
		s.Equal(authz.RoleViewer, invite.Role)
	})

	s.T().Run("but can have other roles", func(t *testing.T) {
		for _, r := range []authz.Role{authz.RoleOwner, authz.RoleAdmin, authz.RoleViewer} {
			invite, err := s.OrgInvitation.Create(ctx, s.org1.ID, s.user.ID, fmt.Sprintf("%s@cyberdyne.io", r), biz.WithInvitationRole(r))
			s.NoError(err)
			s.Equal(r, invite.Role)
		}
	})

	s.Run("and the email address is downcased", func() {
		invite, err := s.OrgInvitation.Create(ctx, s.org1.ID, s.user.ID, "WasCamelCase@cyberdyne.io")
		s.NoError(err)
		s.Equal("wascamelcase@cyberdyne.io", invite.ReceiverEmail)
	})
}

func (s *OrgInvitationIntegrationTestSuite) TestAcceptPendingInvitations() {
	ctx := context.Background()
	receiver, err := s.User.UpsertByEmail(ctx, receiverEmail, nil)
	require.NoError(s.T(), err)

	s.T().Run("user doesn't exist", func(t *testing.T) {
		err := s.OrgInvitation.AcceptPendingInvitations(ctx, "non-existant@cyberdyne.io")
		s.ErrorContains(err, "not found")
	})

	s.T().Run("no invites for user", func(t *testing.T) {
		err = s.OrgInvitation.AcceptPendingInvitations(ctx, receiverEmail)
		s.NoError(err)

		memberships, err := s.Membership.ByUser(ctx, receiver.ID)
		s.NoError(err)
		s.Len(memberships, 0)
	})

	s.T().Run("user is invited to org 1 as viewer", func(t *testing.T) {
		invite, err := s.OrgInvitation.Create(ctx, s.org1.ID, s.user.ID, receiverEmail)
		require.NoError(s.T(), err)
		err = s.OrgInvitation.AcceptPendingInvitations(ctx, receiverEmail)
		s.NoError(err)

		memberships, err := s.Membership.ByUser(ctx, receiver.ID)
		s.NoError(err)
		s.Len(memberships, 1)
		assert.Equal(s.T(), s.org1.ID, memberships[0].OrganizationID.String())
		// It should be a viewer
		assert.Equal(s.T(), authz.RoleViewer, memberships[0].Role)

		// the invite is now accepted
		invite, err = s.OrgInvitation.FindByID(ctx, invite.ID.String())
		s.NoError(err)
		s.Equal(biz.OrgInvitationStatusAccepted, invite.Status)
	})

	s.T().Run("or take any other role", func(t *testing.T) {
		for i, r := range []authz.Role{authz.RoleOwner, authz.RoleAdmin, authz.RoleViewer} {
			// Create user and invite it with different roles
			receiverEmail := fmt.Sprintf("user%d@cyberdyne.io", i)
			receiver, err := s.User.UpsertByEmail(ctx, receiverEmail, nil)
			s.NoError(err)
			invite, err := s.OrgInvitation.Create(ctx, s.org1.ID, s.user.ID, receiverEmail, biz.WithInvitationRole(r))
			s.NoError(err)
			s.Equal(r, invite.Role)
			// accept the invite and make sure the new membership has the role
			err = s.OrgInvitation.AcceptPendingInvitations(ctx, receiverEmail)
			s.NoError(err)

			memberships, err := s.Membership.ByUser(ctx, receiver.ID)
			s.NoError(err)
			s.Len(memberships, 1)
			assert.Equal(s.T(), r, memberships[0].Role)
		}
	})
}

func (s *OrgInvitationIntegrationTestSuite) TestRevoke() {
	ctx := context.Background()
	s.T().Run("invalid ID", func(t *testing.T) {
		err := s.OrgInvitation.Revoke(ctx, s.org1.ID, "deadbeef")
		s.Error(err)
		s.True(biz.IsErrInvalidUUID(err))
	})

	s.T().Run("invitation not found", func(t *testing.T) {
		err := s.OrgInvitation.Revoke(ctx, s.org1.ID, uuid.NewString())
		s.Error(err)
		s.True(biz.IsNotFound(err))
	})

	s.T().Run("invitation in another org", func(t *testing.T) {
		_, err := s.OrgInvitation.Create(ctx, s.org2.ID, s.user.ID, receiverEmail)
		s.NoError(err)
		err = s.OrgInvitation.Revoke(ctx, s.org1.ID, uuid.NewString())
		s.Error(err)
		s.True(biz.IsNotFound(err))
	})

	s.T().Run("invitation not in pending state", func(t *testing.T) {
		invite, err := s.OrgInvitation.Create(ctx, s.org1.ID, s.user.ID, receiverEmail)
		require.NoError(s.T(), err)
		err = s.OrgInvitation.AcceptInvitation(ctx, invite.ID.String())
		require.NoError(s.T(), err)

		// It's in accepted state now so it can not be revoked
		err = s.OrgInvitation.Revoke(ctx, s.org1.ID, invite.ID.String())
		s.Error(err)
		s.ErrorContains(err, "not in pending state")
		s.True(biz.IsErrValidation(err))
	})

	s.T().Run("happy path", func(t *testing.T) {
		invite, err := s.OrgInvitation.Create(ctx, s.org1.ID, s.user.ID, receiverEmail)
		require.NoError(s.T(), err)
		err = s.OrgInvitation.Revoke(ctx, s.org1.ID, invite.ID.String())
		s.NoError(err)
		err = s.OrgInvitation.Revoke(ctx, s.org1.ID, invite.ID.String())
		s.Error(err)
		s.True(biz.IsNotFound(err))
	})
}

// Run the tests
func TestOrgInvitationUseCase(t *testing.T) {
	suite.Run(t, new(OrgInvitationIntegrationTestSuite))
}

func (s *OrgInvitationIntegrationTestSuite) TestInvitationWithGroupContext() {
	ctx := context.Background()

	// Create a test group in org1
	groupName := "test-group-for-invitation"
	groupDescription := "A group for testing invitation with group context"
	userUUID := uuid.MustParse(s.user.ID)
	orgUUID := uuid.MustParse(s.org1.ID)

	group, err := s.Group.Create(ctx, orgUUID, groupName, groupDescription, userUUID)
	require.NoError(s.T(), err)
	require.NotNil(s.T(), group)

	// Create a new receiver that isn't a member of any org yet
	receiverForGroupEmail := "group-receiver@cyberdyne.io"
	receiver, err := s.User.UpsertByEmail(ctx, receiverForGroupEmail, nil)
	require.NoError(s.T(), err)

	s.T().Run("invitation with group context adds user to group when accepted", func(t *testing.T) {
		// Create invitation context with group information
		invitationContext := &biz.OrgInvitationContext{
			GroupIDToJoin:   group.ID,
			GroupMaintainer: true,
		}

		// Create invitation with group context
		invite, err := s.OrgInvitation.Create(
			ctx,
			s.org1.ID,
			s.user.ID,
			receiverForGroupEmail,
			biz.WithInvitationRole(authz.RoleViewer),
			biz.WithInvitationContext(invitationContext),
		)
		require.NoError(t, err)
		require.NotNil(t, invite)

		// Verify context was saved properly
		assert.NotNil(t, invite.Context)
		assert.Equal(t, group.ID, invite.Context.GroupIDToJoin)
		assert.Equal(t, true, invite.Context.GroupMaintainer)

		// Accept the invitation
		err = s.OrgInvitation.AcceptPendingInvitations(ctx, receiverForGroupEmail)
		require.NoError(t, err)

		// Verify user is now a member of the organization
		memberships, err := s.Membership.ByUser(ctx, receiver.ID)
		require.NoError(t, err)
		assert.Len(t, memberships, 1)
		assert.Equal(t, s.org1.ID, memberships[0].OrganizationID.String())
		assert.Equal(t, authz.RoleViewer, memberships[0].Role)

		// Verify user is now a member of the group
		groupMembers, count, err := s.Group.ListMembers(ctx, orgUUID, &biz.ListMembersOpts{
			IdentityReference: &biz.IdentityReference{
				ID: &group.ID,
			},
		}, nil)
		require.NoError(t, err)

		// Should be 2 members: the original creator and the new member
		assert.Equal(t, 2, count)

		// Find the new member in the list
		var foundMember bool
		var isMaintainer bool
		for _, member := range groupMembers {
			if member.User.ID == receiver.ID {
				foundMember = true
				isMaintainer = member.Maintainer
				break
			}
		}

		assert.True(t, foundMember, "The user should be a member of the group")
		assert.True(t, isMaintainer, "The user should be a maintainer of the group")
	})

	s.T().Run("invitation with non-maintainer group context works correctly", func(t *testing.T) {
		// Create another test receiver
		anotherReceiverEmail := "regular-group-member@cyberdyne.io"
		anotherReceiver, err := s.User.UpsertByEmail(ctx, anotherReceiverEmail, nil)
		require.NoError(t, err)

		// Create invitation context with group information, but not as maintainer
		invitationContext := &biz.OrgInvitationContext{
			GroupIDToJoin:   group.ID,
			GroupMaintainer: false,
		}

		// Create invitation with group context
		invite, err := s.OrgInvitation.Create(
			ctx,
			s.org1.ID,
			s.user.ID,
			anotherReceiverEmail,
			biz.WithInvitationRole(authz.RoleViewer),
			biz.WithInvitationContext(invitationContext),
		)
		require.NoError(t, err)
		require.NotNil(t, invite)

		// Accept the invitation
		err = s.OrgInvitation.AcceptPendingInvitations(ctx, anotherReceiverEmail)
		require.NoError(t, err)

		// Verify user is now a member of the group
		groupMembers, count, err := s.Group.ListMembers(ctx, orgUUID, &biz.ListMembersOpts{
			IdentityReference: &biz.IdentityReference{
				ID: &group.ID,
			},
		}, nil)
		require.NoError(t, err)

		// Should be 3 members now
		assert.Equal(t, 3, count)

		// Find the new member in the list
		var foundMember bool
		var isMaintainer bool
		for _, member := range groupMembers {
			if member.User.ID == anotherReceiver.ID {
				foundMember = true
				isMaintainer = member.Maintainer
				break
			}
		}

		assert.True(t, foundMember, "The user should be a member of the group")
		assert.False(t, isMaintainer, "The user should not be a maintainer of the group")
	})
}

func (s *OrgInvitationIntegrationTestSuite) TestInvitationWithProjectContext() {
	ctx := context.Background()

	// Create a test project in org1
	projectName := "test-project-for-invitation"
	userUUID := uuid.MustParse(s.user.ID)
	orgUUID := uuid.MustParse(s.org1.ID)

	project, err := s.Project.Create(ctx, s.org1.ID, projectName)
	require.NoError(s.T(), err)
	require.NotNil(s.T(), project)

	// Create a new receiver that isn't a member of any org yet
	receiverForProjectEmail := "project-receiver@cyberdyne.io"
	receiver, err := s.User.UpsertByEmail(ctx, receiverForProjectEmail, nil)
	require.NoError(s.T(), err)

	s.T().Run("invitation with project context adds user to project when accepted", func(t *testing.T) {
		// Create invitation context with project information
		invitationContext := &biz.OrgInvitationContext{
			ProjectIDToJoin: project.ID,
			ProjectRole:     authz.RoleProjectAdmin,
		}

		// Create invitation with project context
		invite, err := s.OrgInvitation.Create(
			ctx,
			s.org1.ID,
			s.user.ID,
			receiverForProjectEmail,
			biz.WithInvitationRole(authz.RoleViewer),
			biz.WithInvitationContext(invitationContext),
		)
		require.NoError(t, err)
		require.NotNil(t, invite)

		// Verify context was saved properly
		assert.NotNil(t, invite.Context)
		assert.Equal(t, project.ID, invite.Context.ProjectIDToJoin)
		assert.Equal(t, authz.RoleProjectAdmin, invite.Context.ProjectRole)

		// Accept the invitation
		err = s.OrgInvitation.AcceptPendingInvitations(ctx, receiverForProjectEmail)
		require.NoError(t, err)

		// Verify user is now a member of the organization
		memberships, err := s.Membership.ByUser(ctx, receiver.ID)
		require.NoError(t, err)
		assert.Len(t, memberships, 1)
		assert.Equal(t, s.org1.ID, memberships[0].OrganizationID.String())
		assert.Equal(t, authz.RoleViewer, memberships[0].Role)

		// Verify user is now a member of the project
		projectMembers, count, err := s.Project.ListMembers(ctx, orgUUID, &biz.IdentityReference{
			ID: &project.ID,
		}, nil)
		require.NoError(t, err)

		// The count should include the original project members plus the new member
		assert.Greater(t, count, 0, "The project should have at least one member")

		// Find the new member in the list
		var foundMember bool
		var memberRole authz.Role
		for _, member := range projectMembers {
			if member.User != nil && member.User.ID == receiver.ID {
				foundMember = true
				memberRole = member.Role
				break
			}
		}

		assert.True(t, foundMember, "The user should be a member of the project")
		assert.Equal(t, authz.RoleProjectAdmin, memberRole, "The user should have the project admin role")
	})

	s.T().Run("invitation with different project role works correctly", func(t *testing.T) {
		// Create another test receiver
		anotherReceiverEmail := "project-viewer@cyberdyne.io"
		anotherReceiver, err := s.User.UpsertByEmail(ctx, anotherReceiverEmail, nil)
		require.NoError(t, err)

		// Create invitation context with project information, but with viewer role
		invitationContext := &biz.OrgInvitationContext{
			ProjectIDToJoin: project.ID,
			ProjectRole:     authz.RoleProjectViewer,
		}

		// Create invitation with project context
		invite, err := s.OrgInvitation.Create(
			ctx,
			s.org1.ID,
			s.user.ID,
			anotherReceiverEmail,
			biz.WithInvitationRole(authz.RoleViewer),
			biz.WithInvitationContext(invitationContext),
		)
		require.NoError(t, err)
		require.NotNil(t, invite)

		// Accept the invitation
		err = s.OrgInvitation.AcceptPendingInvitations(ctx, anotherReceiverEmail)
		require.NoError(t, err)

		// Verify user is now a member of the project
		projectMembers, count, err := s.Project.ListMembers(ctx, orgUUID, &biz.IdentityReference{
			ID: &project.ID,
		}, nil)
		require.NoError(t, err)

		// The count should have increased
		assert.Greater(t, count, 1, "The project should have multiple members")

		// Find the new member in the list
		var foundMember bool
		var memberRole authz.Role
		for _, member := range projectMembers {
			if member.User != nil && member.User.ID == anotherReceiver.ID {
				foundMember = true
				memberRole = member.Role
				break
			}
		}

		assert.True(t, foundMember, "The user should be a member of the project")
		assert.Equal(t, authz.RoleProjectViewer, memberRole, "The user should have the project viewer role")
	})

	s.T().Run("invitation with both group and project context works correctly", func(t *testing.T) {
		// Create a test group
		groupName := "combined-test-group"
		groupDescription := "A group for testing combined invitation context"
		group, err := s.Group.Create(ctx, orgUUID, groupName, groupDescription, userUUID)
		require.NoError(t, err)
		require.NotNil(t, group)

		// Create another test receiver
		combinedReceiverEmail := "combined-receiver@cyberdyne.io"
		combinedReceiver, err := s.User.UpsertByEmail(ctx, combinedReceiverEmail, nil)
		require.NoError(t, err)

		// Create invitation context with both group and project information
		invitationContext := &biz.OrgInvitationContext{
			GroupIDToJoin:   group.ID,
			GroupMaintainer: true,
			ProjectIDToJoin: project.ID,
			ProjectRole:     authz.RoleProjectViewer,
		}

		// Create invitation with combined context
		invite, err := s.OrgInvitation.Create(
			ctx,
			s.org1.ID,
			s.user.ID,
			combinedReceiverEmail,
			biz.WithInvitationRole(authz.RoleViewer),
			biz.WithInvitationContext(invitationContext),
		)
		require.NoError(t, err)
		require.NotNil(t, invite)

		// Verify context was saved properly
		assert.NotNil(t, invite.Context)
		assert.Equal(t, group.ID, invite.Context.GroupIDToJoin)
		assert.True(t, invite.Context.GroupMaintainer)
		assert.Equal(t, project.ID, invite.Context.ProjectIDToJoin)
		assert.Equal(t, authz.RoleProjectViewer, invite.Context.ProjectRole)

		// Accept the invitation
		err = s.OrgInvitation.AcceptPendingInvitations(ctx, combinedReceiverEmail)
		require.NoError(t, err)

		// Verify user is now a member of the organization
		memberships, err := s.Membership.ByUser(ctx, combinedReceiver.ID)
		require.NoError(t, err)
		assert.Len(t, memberships, 1)
		assert.Equal(t, s.org1.ID, memberships[0].OrganizationID.String())

		// Verify user is now a member of the group
		groupMembers, groupCount, err := s.Group.ListMembers(ctx, orgUUID, &biz.ListMembersOpts{
			IdentityReference: &biz.IdentityReference{
				ID: &group.ID,
			},
		}, nil)
		require.NoError(t, err)
		assert.Greater(t, groupCount, 0, "The group should have at least one member")

		var foundGroupMember bool
		var isMaintainer bool
		for _, member := range groupMembers {
			if member.User.ID == combinedReceiver.ID {
				foundGroupMember = true
				isMaintainer = member.Maintainer
				break
			}
		}
		assert.True(t, foundGroupMember, "The user should be a member of the group")
		assert.True(t, isMaintainer, "The user should be a maintainer of the group")

		// Verify user is now a member of the project
		projectMembers, projectCount, err := s.Project.ListMembers(ctx, orgUUID, &biz.IdentityReference{
			ID: &project.ID,
		}, nil)
		require.NoError(t, err)
		assert.Greater(t, projectCount, 0, "The project should have at least one member")

		var foundProjectMember bool
		var projectRole authz.Role
		for _, member := range projectMembers {
			if member.User != nil && member.User.ID == combinedReceiver.ID {
				foundProjectMember = true
				projectRole = member.Role
				break
			}
		}
		assert.True(t, foundProjectMember, "The user should be a member of the project")
		assert.Equal(t, authz.RoleProjectViewer, projectRole, "The user should have the project contributor role")
	})
}

// Utility struct to hold the test suite
type OrgInvitationIntegrationTestSuite struct {
	testhelpers.UseCasesEachTestSuite
	org1, org2, org3 *biz.Organization
	user, user2      *biz.User
}

// 3 orgs, user belongs to org1 and org2 but not org3
func (s *OrgInvitationIntegrationTestSuite) SetupTest() {
	t := s.T()
	var err error
	assert := assert.New(s.T())
	ctx := context.Background()

	s.TestingUseCases = testhelpers.NewTestingUseCases(t)
	s.org1, err = s.Organization.Create(ctx, "org1")
	assert.NoError(err)
	s.org2, err = s.Organization.Create(ctx, "org2")
	assert.NoError(err)
	s.org3, err = s.Organization.Create(ctx, "org3")
	assert.NoError(err)

	// Create User 1
	s.user, err = s.User.UpsertByEmail(ctx, "user-1@test.com", nil)
	assert.NoError(err)
	// Attach both orgs
	_, err = s.Membership.Create(ctx, s.org1.ID, s.user.ID, biz.WithCurrentMembership())
	assert.NoError(err)
	_, err = s.Membership.Create(ctx, s.org2.ID, s.user.ID, biz.WithCurrentMembership())
	assert.NoError(err)

	s.user2, err = s.User.UpsertByEmail(ctx, "user-2@test.com", nil)
	assert.NoError(err)
	_, err = s.Membership.Create(ctx, s.org1.ID, s.user2.ID)
	assert.NoError(err)
	_, err = s.Membership.Create(ctx, s.org3.ID, s.user2.ID, biz.WithCurrentMembership())
	assert.NoError(err)
}
