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
	"fmt"
	"testing"

	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/authz"
	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/biz"
	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/biz/testhelpers"
	"github.com/google/uuid"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
)

const receiverEmail = "sarah@cyberdyne.io"

func (s *OrgInvitationIntegrationTestSuite) TestList() {
	ctx := context.Background()
	inviteOrg1A, err := s.OrgInvitation.Create(ctx, s.org1.ID, receiverEmail, authz.RoleOwner, biz.WithSender(uuid.MustParse(s.user.ID)))
	s.NoError(err)
	// same org but another user
	inviteOrg1B, err := s.OrgInvitation.Create(ctx, s.org1.ID, "another-email@cyberdyne.io", authz.RoleOwner, biz.WithSender(uuid.MustParse(s.user2.ID)))
	s.NoError(err)
	inviteOrg2A, err := s.OrgInvitation.Create(ctx, s.org2.ID, receiverEmail, authz.RoleOwner, biz.WithSender(uuid.MustParse(s.user.ID)))
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
		s.Run(tc.name, func() {
			invites, err := s.OrgInvitation.ListByOrg(ctx, tc.orgID)
			s.NoError(err)
			s.Equal(tc.expected, invites)
		})
	}
}

func (s *OrgInvitationIntegrationTestSuite) TestCreate() {
	ctx := context.Background()
	s.Run("invalid org ID", func() {
		invite, err := s.OrgInvitation.Create(ctx, "deadbeef", receiverEmail, authz.RoleOwner, biz.WithSender(uuid.MustParse(s.user.ID)))
		s.Error(err)
		s.True(biz.IsErrInvalidUUID(err))
		s.Nil(invite)
	})

	s.Run("missing receiver email", func() {
		invite, err := s.OrgInvitation.Create(ctx, s.org1.ID, "", authz.RoleOwner, biz.WithSender(uuid.MustParse(s.user.ID)))
		s.Error(err)
		s.True(biz.IsErrValidation(err))
		s.Nil(invite)
	})

	s.Run("receiver email same than sender", func() {
		invite, err := s.OrgInvitation.Create(ctx, s.org1.ID, s.user.Email, authz.RoleOwner, biz.WithSender(uuid.MustParse(s.user.ID)))
		s.Error(err)
		s.ErrorContains(err, "sender and receiver emails cannot be the same")
		s.True(biz.IsErrValidation(err))
		s.Nil(invite)
	})

	s.Run("receiver is already a member", func() {
		invite, err := s.OrgInvitation.Create(ctx, s.org1.ID, s.user2.Email, authz.RoleOwner, biz.WithSender(uuid.MustParse(s.user.ID)))
		s.Error(err)
		s.ErrorContains(err, "user already exists in the org")
		s.True(biz.IsErrValidation(err))
		s.Nil(invite)
	})

	s.Run("org not found", func() {
		invite, err := s.OrgInvitation.Create(ctx, s.org1.ID, receiverEmail, authz.RoleOwner, biz.WithSender(uuid.Nil))
		s.Error(err)
		s.True(biz.IsNotFound(err))
		s.Nil(invite)
	})

	s.Run("can create invites for org1 and 2", func() {
		for _, org := range []*biz.Organization{s.org1, s.org2} {
			invite, err := s.OrgInvitation.Create(ctx, org.ID, receiverEmail, authz.RoleOwner, biz.WithSender(uuid.MustParse(s.user.ID)))
			s.NoError(err)
			s.Equal(org, invite.Org)
			s.Equal(s.user, invite.Sender)
			s.Equal(receiverEmail, invite.ReceiverEmail)
			s.Equal(biz.OrgInvitationStatusPending, invite.Status)
			s.NotNil(invite.CreatedAt)
		}
	})

	s.Run("but can't create if there is one pending", func() {
		invite, err := s.OrgInvitation.Create(ctx, s.org1.ID, receiverEmail, authz.RoleOwner, biz.WithSender(uuid.MustParse(s.user.ID)))
		s.Error(err)
		s.ErrorContains(err, "already exists")
		s.True(biz.IsErrValidation(err))
		s.Nil(invite)
	})

	s.Run("but it can if it's another email", func() {
		invite, err := s.OrgInvitation.Create(ctx, s.org1.ID, "anotheremail@cyberdyne.io", authz.RoleOwner, biz.WithSender(uuid.MustParse(s.user.ID)))
		s.Equal("anotheremail@cyberdyne.io", invite.ReceiverEmail)
		s.Equal(s.org1, invite.Org)
		s.NoError(err)
	})

	s.Run("the default role is viewer", func() {
		invite, err := s.OrgInvitation.Create(ctx, s.org1.ID, "viewer@cyberdyne.io", authz.RoleOwner, biz.WithSender(uuid.MustParse(s.user.ID)))
		s.NoError(err)
		s.Equal(authz.RoleViewer, invite.Role)
	})

	s.Run("but can have other roles", func() {
		for _, r := range []authz.Role{authz.RoleOwner, authz.RoleAdmin, authz.RoleViewer} {
			invite, err := s.OrgInvitation.Create(ctx, s.org1.ID, fmt.Sprintf("%s@cyberdyne.io", r), authz.RoleOwner, biz.WithInvitationRole(r), biz.WithSender(uuid.MustParse(s.user.ID)))
			s.NoError(err)
			s.Equal(r, invite.Role)
		}
	})

	s.Run("and the email address is downcased", func() {
		invite, err := s.OrgInvitation.Create(ctx, s.org1.ID, "WasCamelCase@cyberdyne.io", authz.RoleOwner, biz.WithSender(uuid.MustParse(s.user.ID)))
		s.NoError(err)
		s.Equal("wascamelcase@cyberdyne.io", invite.ReceiverEmail)
	})
}

// TestCreateOwnerGuard covers audit finding CP-1: only organization owners may
// send owner-role invitations. Admins (and lower roles) must not be able to
// bootstrap a fresh Owner via the invitation path, which would bypass
// PolicyOrganizationManageOwners (the Owner-only manage-owners gate).
func (s *OrgInvitationIntegrationTestSuite) TestCreateOwnerGuard() {
	ctx := context.Background()

	org, err := s.Organization.CreateWithRandomName(ctx)
	s.Require().NoError(err)

	testCases := []struct {
		name       string
		callerRole authz.Role
		inviteRole authz.Role
		wantDeny   bool
	}{
		{name: "owner can invite owner", callerRole: authz.RoleOwner, inviteRole: authz.RoleOwner},
		{name: "admin cannot invite owner", callerRole: authz.RoleAdmin, inviteRole: authz.RoleOwner, wantDeny: true},
		{name: "viewer cannot invite owner", callerRole: authz.RoleViewer, inviteRole: authz.RoleOwner, wantDeny: true},
		// Instance admins hold PolicyOrganizationInvitationsCreate but
		// intentionally not PolicyOrganizationManageOwners: they must not be
		// able to seed owners into organizations they administer.
		{name: "instance admin cannot invite owner", callerRole: authz.RoleInstanceAdmin, inviteRole: authz.RoleOwner, wantDeny: true},
		{name: "empty callerRole cannot invite owner", callerRole: "", inviteRole: authz.RoleOwner, wantDeny: true},
		// RBAC-enabled org roles (Member / Contributor) are denied owner
		// invitations at the Casbin middleware before reaching the biz layer,
		// but the biz guard must also fail-closed if they ever did reach it.
		{name: "org member cannot invite owner", callerRole: authz.RoleOrgMember, inviteRole: authz.RoleOwner, wantDeny: true},
		{name: "org contributor cannot invite owner", callerRole: authz.RoleOrgContributor, inviteRole: authz.RoleOwner, wantDeny: true},
		{name: "admin can invite admin", callerRole: authz.RoleAdmin, inviteRole: authz.RoleAdmin},
		{name: "admin can invite viewer", callerRole: authz.RoleAdmin, inviteRole: authz.RoleViewer},
		{name: "viewer can invite viewer (guard only applies to owner role)", callerRole: authz.RoleViewer, inviteRole: authz.RoleViewer},
	}

	for i, tc := range testCases {
		s.Run(tc.name, func() {
			receiver := fmt.Sprintf("owner-guard-%d@cyberdyne.io", i)
			invite, err := s.OrgInvitation.Create(ctx, org.ID, receiver, tc.callerRole, biz.WithInvitationRole(tc.inviteRole))
			if tc.wantDeny {
				s.Error(err)
				s.True(biz.IsErrUnauthorized(err))
				s.Contains(err.Error(), "only organization owners can invite owners")
				return
			}

			s.NoError(err)
			s.Equal(tc.inviteRole, invite.Role)
		})
	}

	// API-token caller path. AuthzUseCase.Enforce resolves the token's stored
	// DB policies when the subject is "api-token:<id>". A default API token
	// does NOT carry PolicyOrganizationManageOwners, so it must be rejected;
	// a token explicitly granted that policy must be admitted.
	tokenCases := []struct {
		name     string
		policies []*authz.Policy
		wantDeny bool
	}{
		{name: "default API token cannot invite owner", wantDeny: true},
		{name: "API token with manage-owners policy can invite owner", policies: []*authz.Policy{authz.PolicyOrganizationManageOwners}},
	}

	for i, tc := range tokenCases {
		s.Run(tc.name, func() {
			var opts []biz.APITokenCreateOpt
			if tc.policies != nil {
				opts = append(opts, biz.APITokenWithPolicies(tc.policies))
			}
			token, err := s.APIToken.Create(ctx, fmt.Sprintf("guard-token-%d", i), nil, nil, &org.ID, opts...)
			s.Require().NoError(err)

			subject := authz.Role(fmt.Sprintf("api-token:%s", token.ID))
			receiver := fmt.Sprintf("token-guard-%d@cyberdyne.io", i)
			invite, err := s.OrgInvitation.Create(ctx, org.ID, receiver, subject, biz.WithInvitationRole(authz.RoleOwner))
			if tc.wantDeny {
				s.Error(err)
				s.True(biz.IsErrUnauthorized(err))
				return
			}

			s.NoError(err)
			s.Equal(authz.RoleOwner, invite.Role)
		})
	}
}

func (s *OrgInvitationIntegrationTestSuite) TestAcceptPendingInvitations() {
	ctx := context.Background()
	receiver, err := s.User.UpsertByEmail(ctx, receiverEmail, nil)
	s.Require().NoError(err)

	s.Run("user doesn't exist", func() {
		err := s.OrgInvitation.AcceptPendingInvitations(ctx, "non-existant@cyberdyne.io")
		s.ErrorContains(err, "not found")
	})

	s.Run("no invites for user", func() {
		err = s.OrgInvitation.AcceptPendingInvitations(ctx, receiverEmail)
		s.NoError(err)

		memberships, err := s.Membership.ByUser(ctx, receiver.ID)
		s.NoError(err)
		s.Len(memberships, 0)
	})

	s.Run("user is invited to org 1 as viewer", func() {
		invite, err := s.OrgInvitation.Create(ctx, s.org1.ID, receiverEmail, authz.RoleOwner, biz.WithSender(uuid.MustParse(s.user.ID)))
		s.Require().NoError(err)
		err = s.OrgInvitation.AcceptPendingInvitations(ctx, receiverEmail)
		s.NoError(err)

		memberships, err := s.Membership.ByUser(ctx, receiver.ID)
		s.NoError(err)
		s.Len(memberships, 1)
		s.Equal(s.org1.ID, memberships[0].OrganizationID.String())
		// It should be a viewer
		s.Equal(authz.RoleViewer, memberships[0].Role)

		// the invite is now accepted
		invite, err = s.OrgInvitation.FindByID(ctx, invite.ID.String())
		s.NoError(err)
		s.Equal(biz.OrgInvitationStatusAccepted, invite.Status)
	})

	s.Run("or take any other role", func() {
		for i, r := range []authz.Role{authz.RoleOwner, authz.RoleAdmin, authz.RoleViewer} {
			// Create user and invite it with different roles
			receiverEmail := fmt.Sprintf("user%d@cyberdyne.io", i)
			receiver, err := s.User.UpsertByEmail(ctx, receiverEmail, nil)
			s.NoError(err)
			invite, err := s.OrgInvitation.Create(ctx, s.org1.ID, receiverEmail, authz.RoleOwner, biz.WithInvitationRole(r), biz.WithSender(uuid.MustParse(s.user.ID)))
			s.NoError(err)
			s.Equal(r, invite.Role)
			// accept the invite and make sure the new membership has the role
			err = s.OrgInvitation.AcceptPendingInvitations(ctx, receiverEmail)
			s.NoError(err)

			memberships, err := s.Membership.ByUser(ctx, receiver.ID)
			s.NoError(err)
			s.Len(memberships, 1)
			s.Equal(r, memberships[0].Role)
		}
	})
}

func (s *OrgInvitationIntegrationTestSuite) TestRevoke() {
	ctx := context.Background()
	s.Run("invalid ID", func() {
		err := s.OrgInvitation.Revoke(ctx, s.org1.ID, "deadbeef")
		s.Error(err)
		s.True(biz.IsErrInvalidUUID(err))
	})

	s.Run("invitation not found", func() {
		err := s.OrgInvitation.Revoke(ctx, s.org1.ID, uuid.NewString())
		s.Error(err)
		s.True(biz.IsNotFound(err))
	})

	s.Run("invitation in another org", func() {
		_, err := s.OrgInvitation.Create(ctx, s.org2.ID, receiverEmail, authz.RoleOwner, biz.WithSender(uuid.MustParse(s.user.ID)))
		s.NoError(err)
		err = s.OrgInvitation.Revoke(ctx, s.org1.ID, uuid.NewString())
		s.Error(err)
		s.True(biz.IsNotFound(err))
	})

	s.Run("invitation not in pending state", func() {
		invite, err := s.OrgInvitation.Create(ctx, s.org1.ID, receiverEmail, authz.RoleOwner, biz.WithSender(uuid.MustParse(s.user.ID)))
		s.Require().NoError(err)
		err = s.OrgInvitation.AcceptInvitation(ctx, invite.ID.String())
		s.Require().NoError(err)

		// It's in accepted state now so it can not be revoked
		err = s.OrgInvitation.Revoke(ctx, s.org1.ID, invite.ID.String())
		s.Error(err)
		s.ErrorContains(err, "not in pending state")
		s.True(biz.IsErrValidation(err))
	})

	s.Run("happy path", func() {
		invite, err := s.OrgInvitation.Create(ctx, s.org1.ID, receiverEmail, authz.RoleOwner, biz.WithSender(uuid.MustParse(s.user.ID)))
		s.Require().NoError(err)
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

	group, err := s.Group.Create(ctx, orgUUID, groupName, groupDescription, &userUUID)
	s.Require().NoError(err)
	s.Require().NotNil(group)

	// Create a new receiver that isn't a member of any org yet
	receiverForGroupEmail := "group-receiver@cyberdyne.io"
	receiver, err := s.User.UpsertByEmail(ctx, receiverForGroupEmail, nil)
	s.Require().NoError(err)

	s.Run("invitation with group context adds user to group when accepted", func() {
		// Create invitation context with group information
		invitationContext := &biz.OrgInvitationContext{
			GroupIDToJoin:   &group.ID,
			GroupMaintainer: true,
		}

		// Create invitation with group context
		invite, err := s.OrgInvitation.Create(
			ctx,
			s.org1.ID,
			receiverForGroupEmail,
			authz.RoleOwner,
			biz.WithInvitationRole(authz.RoleViewer),
			biz.WithInvitationContext(invitationContext),
			biz.WithSender(uuid.MustParse(s.user.ID)),
		)
		s.Require().NoError(err)
		s.Require().NotNil(invite)

		// Verify context was saved properly
		s.NotNil(invite.Context)
		s.Equal(group.ID, *invite.Context.GroupIDToJoin)
		s.True(invite.Context.GroupMaintainer)

		// Accept the invitation
		err = s.OrgInvitation.AcceptPendingInvitations(ctx, receiverForGroupEmail)
		s.Require().NoError(err)

		// Verify user is now a member of the organization
		memberships, err := s.Membership.ByUser(ctx, receiver.ID)
		s.Require().NoError(err)
		s.Len(memberships, 1)
		s.Equal(s.org1.ID, memberships[0].OrganizationID.String())
		s.Equal(authz.RoleViewer, memberships[0].Role)

		// Verify user is now a member of the group
		groupMembers, count, err := s.Group.ListMembers(ctx, orgUUID, &biz.ListMembersOpts{
			IdentityReference: &biz.IdentityReference{
				ID: &group.ID,
			},
		}, nil)
		s.Require().NoError(err)

		// Should be 2 members: the original creator and the new member
		s.Equal(2, count)

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

		s.True(foundMember, "The user should be a member of the group")
		s.True(isMaintainer, "The user should be a maintainer of the group")
	})

	s.Run("invitation with non-maintainer group context works correctly", func() {
		// Create another test receiver
		anotherReceiverEmail := "regular-group-member@cyberdyne.io"
		anotherReceiver, err := s.User.UpsertByEmail(ctx, anotherReceiverEmail, nil)
		s.Require().NoError(err)

		// Create invitation context with group information, but not as maintainer
		invitationContext := &biz.OrgInvitationContext{
			GroupIDToJoin:   &group.ID,
			GroupMaintainer: false,
		}

		// Create invitation with group context
		invite, err := s.OrgInvitation.Create(
			ctx,
			s.org1.ID,
			anotherReceiverEmail,
			authz.RoleOwner,
			biz.WithInvitationRole(authz.RoleViewer),
			biz.WithInvitationContext(invitationContext),
			biz.WithSender(uuid.MustParse(s.user.ID)),
		)
		s.Require().NoError(err)
		s.Require().NotNil(invite)

		// Accept the invitation
		err = s.OrgInvitation.AcceptPendingInvitations(ctx, anotherReceiverEmail)
		s.Require().NoError(err)

		// Verify user is now a member of the group
		groupMembers, count, err := s.Group.ListMembers(ctx, orgUUID, &biz.ListMembersOpts{
			IdentityReference: &biz.IdentityReference{
				ID: &group.ID,
			},
		}, nil)
		s.Require().NoError(err)

		// Should be 3 members now
		s.Equal(3, count)

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

		s.True(foundMember, "The user should be a member of the group")
		s.False(isMaintainer, "The user should not be a maintainer of the group")
	})
}

func (s *OrgInvitationIntegrationTestSuite) TestInvitationWithProjectContext() {
	ctx := context.Background()

	// Create a test project in org1
	projectName := "test-project-for-invitation"
	userUUID := uuid.MustParse(s.user.ID)
	orgUUID := uuid.MustParse(s.org1.ID)

	project, err := s.Project.Create(ctx, s.org1.ID, projectName)
	s.Require().NoError(err)
	s.Require().NotNil(project)

	// Create a new receiver that isn't a member of any org yet
	receiverForProjectEmail := "project-receiver@cyberdyne.io"
	// Receiver shared by the combined-context and nil-UUID subtests below
	combinedReceiverEmail := "combined-receiver@cyberdyne.io"
	receiver, err := s.User.UpsertByEmail(ctx, receiverForProjectEmail, nil)
	s.Require().NoError(err)

	s.Run("invitation with project context adds user to project when accepted", func() {
		// Create invitation context with project information
		invitationContext := &biz.OrgInvitationContext{
			ProjectIDToJoin: &project.ID,
			ProjectRole:     authz.RoleProjectAdmin,
		}

		// Create invitation with project context
		invite, err := s.OrgInvitation.Create(
			ctx,
			s.org1.ID,
			receiverForProjectEmail,
			authz.RoleOwner,
			biz.WithInvitationRole(authz.RoleViewer),
			biz.WithSender(uuid.MustParse(s.user.ID)),
			biz.WithInvitationContext(invitationContext),
		)
		s.Require().NoError(err)
		s.Require().NotNil(invite)

		// Verify context was saved properly
		s.NotNil(invite.Context)
		s.Equal(project.ID, *invite.Context.ProjectIDToJoin)
		s.Equal(authz.RoleProjectAdmin, invite.Context.ProjectRole)

		// Accept the invitation
		err = s.OrgInvitation.AcceptPendingInvitations(ctx, receiverForProjectEmail)
		s.Require().NoError(err)

		// Verify user is now a member of the organization
		memberships, err := s.Membership.ByUser(ctx, receiver.ID)
		s.Require().NoError(err)
		s.Len(memberships, 1)
		s.Equal(s.org1.ID, memberships[0].OrganizationID.String())
		s.Equal(authz.RoleViewer, memberships[0].Role)

		// Verify user is now a member of the project
		projectMembers, count, err := s.Project.ListMembers(ctx, orgUUID, &biz.IdentityReference{
			ID: &project.ID,
		}, nil)
		s.Require().NoError(err)

		// The count should include the original project members plus the new member
		s.Greater(count, 0, "The project should have at least one member")

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

		s.True(foundMember, "The user should be a member of the project")
		s.Equal(authz.RoleProjectAdmin, memberRole, "The user should have the project admin role")
	})

	s.Run("invitation with different project role works correctly", func() {
		// Create another test receiver
		anotherReceiverEmail := "project-viewer@cyberdyne.io"
		anotherReceiver, err := s.User.UpsertByEmail(ctx, anotherReceiverEmail, nil)
		s.Require().NoError(err)

		// Create invitation context with project information, but with viewer role
		invitationContext := &biz.OrgInvitationContext{
			ProjectIDToJoin: &project.ID,
			ProjectRole:     authz.RoleProjectViewer,
		}

		// Create invitation with project context
		invite, err := s.OrgInvitation.Create(
			ctx,
			s.org1.ID,
			anotherReceiverEmail,
			authz.RoleOwner,
			biz.WithSender(uuid.MustParse(s.user.ID)),
			biz.WithInvitationRole(authz.RoleViewer),
			biz.WithInvitationContext(invitationContext),
		)
		s.Require().NoError(err)
		s.Require().NotNil(invite)

		// Accept the invitation
		err = s.OrgInvitation.AcceptPendingInvitations(ctx, anotherReceiverEmail)
		s.Require().NoError(err)

		// Verify user is now a member of the project
		projectMembers, count, err := s.Project.ListMembers(ctx, orgUUID, &biz.IdentityReference{
			ID: &project.ID,
		}, nil)
		s.Require().NoError(err)

		// The count should have increased
		s.Greater(count, 1, "The project should have multiple members")

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

		s.True(foundMember, "The user should be a member of the project")
		s.Equal(authz.RoleProjectViewer, memberRole, "The user should have the project viewer role")
	})

	s.Run("invitation with both group and project context works correctly", func() {
		// Create a test group
		groupName := "combined-test-group"
		groupDescription := "A group for testing combined invitation context"
		group, err := s.Group.Create(ctx, orgUUID, groupName, groupDescription, &userUUID)
		s.Require().NoError(err)
		s.Require().NotNil(group)

		// Create another test receiver
		combinedReceiver, err := s.User.UpsertByEmail(ctx, combinedReceiverEmail, nil)
		s.Require().NoError(err)

		// Create invitation context with both group and project information
		invitationContext := &biz.OrgInvitationContext{
			GroupIDToJoin:   &group.ID,
			GroupMaintainer: true,
			ProjectIDToJoin: &project.ID,
			ProjectRole:     authz.RoleProjectViewer,
		}

		// Create invitation with combined context
		invite, err := s.OrgInvitation.Create(
			ctx,
			s.org1.ID,
			combinedReceiverEmail,
			authz.RoleOwner,
			biz.WithSender(uuid.MustParse(s.user.ID)),
			biz.WithInvitationRole(authz.RoleViewer),
			biz.WithInvitationContext(invitationContext),
		)
		s.Require().NoError(err)
		s.Require().NotNil(invite)

		// Verify context was saved properly
		s.NotNil(invite.Context)
		s.Equal(group.ID, *invite.Context.GroupIDToJoin)
		s.True(invite.Context.GroupMaintainer)
		s.Equal(project.ID, *invite.Context.ProjectIDToJoin)
		s.Equal(authz.RoleProjectViewer, invite.Context.ProjectRole)

		// Accept the invitation
		err = s.OrgInvitation.AcceptPendingInvitations(ctx, combinedReceiverEmail)
		s.Require().NoError(err)

		// Verify user is now a member of the organization
		memberships, err := s.Membership.ByUser(ctx, combinedReceiver.ID)
		s.Require().NoError(err)
		s.Len(memberships, 1)
		s.Equal(s.org1.ID, memberships[0].OrganizationID.String())

		// Verify user is now a member of the group
		groupMembers, groupCount, err := s.Group.ListMembers(ctx, orgUUID, &biz.ListMembersOpts{
			IdentityReference: &biz.IdentityReference{
				ID: &group.ID,
			},
		}, nil)
		s.Require().NoError(err)
		s.Greater(groupCount, 0, "The group should have at least one member")

		var foundGroupMember bool
		var isMaintainer bool
		for _, member := range groupMembers {
			if member.User.ID == combinedReceiver.ID {
				foundGroupMember = true
				isMaintainer = member.Maintainer
				break
			}
		}
		s.True(foundGroupMember, "The user should be a member of the group")
		s.True(isMaintainer, "The user should be a maintainer of the group")

		// Verify user is now a member of the project
		projectMembers, projectCount, err := s.Project.ListMembers(ctx, orgUUID, &biz.IdentityReference{
			ID: &project.ID,
		}, nil)
		s.Require().NoError(err)
		s.Greater(projectCount, 0, "The project should have at least one member")

		var foundProjectMember bool
		var projectRole authz.Role
		for _, member := range projectMembers {
			if member.User != nil && member.User.ID == combinedReceiver.ID {
				foundProjectMember = true
				projectRole = member.Role
				break
			}
		}
		s.True(foundProjectMember, "The user should be a member of the project")
		s.Equal(authz.RoleProjectViewer, projectRole, "The user should have the project contributor role")
	})

	s.Run("invitation with nil UUID on project is rejected", func() {
		// Create a new receiver that isn't a member of any org yet
		newReceiver, err := s.User.UpsertByEmail(ctx, combinedReceiverEmail, nil)
		s.Require().NoError(err)
		s.Require().NotNil(newReceiver)

		// Create invitation context with nil project ID
		invitationContext := &biz.OrgInvitationContext{
			ProjectIDToJoin: &uuid.Nil,
		}

		// Create invitation with combined context
		invite, err := s.Repos.OrgInvitationRepo.Create(
			ctx,
			uuid.MustParse(s.org1.ID),
			biz.ToPtr(uuid.MustParse(s.user.ID)),
			combinedReceiverEmail,
			authz.RoleViewer,
			invitationContext,
		)
		s.Require().NoError(err)
		s.Require().NotNil(invite)

		// Accept the invitation and check that there is no error
		err = s.OrgInvitation.AcceptPendingInvitations(ctx, combinedReceiverEmail)
		s.Require().NoError(err, "Accepting invitation with nil project ID should not fail just skip the project context")
	})

	s.Run("invitation with nil UUID on group is rejected", func() {
		// Create a new receiver that isn't a member of any org yet
		newReceiver, err := s.User.UpsertByEmail(ctx, combinedReceiverEmail, nil)
		s.Require().NoError(err)
		s.Require().NotNil(newReceiver)

		// Create invitation context with nil group ID
		invitationContext := &biz.OrgInvitationContext{
			GroupIDToJoin: &uuid.Nil,
		}

		// Create invitation with combined context
		invite, err := s.Repos.OrgInvitationRepo.Create(
			ctx,
			uuid.MustParse(s.org1.ID),
			biz.ToPtr(uuid.MustParse(s.user.ID)),
			combinedReceiverEmail,
			authz.RoleViewer,
			invitationContext,
		)
		s.Require().NoError(err)
		s.Require().NotNil(invite)

		// Accept the invitation and check that there is no error
		err = s.OrgInvitation.AcceptPendingInvitations(ctx, combinedReceiverEmail)
		s.Require().NoError(err, "Accepting invitation with nil group ID should not fail just skip the project context")
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
