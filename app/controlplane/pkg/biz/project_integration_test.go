//
// Copyright 2025 The Chainloop Authors.
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
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

// Run the tests
func TestProjectUseCase(t *testing.T) {
	suite.Run(t, new(projectMembersIntegrationTestSuite))
	suite.Run(t, new(projectGroupMembersIntegrationTestSuite))
	suite.Run(t, new(projectAdminPermissionsTestSuite))
	suite.Run(t, new(projectPermissionsTestSuite))
}

// Utility struct for project members tests
type projectMembersIntegrationTestSuite struct {
	testhelpers.UseCasesEachTestSuite
	org     *biz.Organization
	user    *biz.User
	project *biz.Project
}

func (s *projectMembersIntegrationTestSuite) SetupTest() {
	var err error
	assert := assert.New(s.T())
	s.TestingUseCases = testhelpers.NewTestingUseCases(s.T())

	ctx := context.Background()
	s.org, err = s.Organization.CreateWithRandomName(ctx)
	assert.NoError(err)

	// Create a user for membership tests - this user will be an org admin by default
	s.user, err = s.User.UpsertByEmail(ctx, fmt.Sprintf("test-user-%s@example.com", uuid.New().String()), nil)
	assert.NoError(err)

	// Add user to organization as an admin
	_, err = s.Membership.Create(ctx, s.org.ID, s.user.ID, biz.WithMembershipRole(authz.RoleAdmin), biz.WithCurrentMembership())
	assert.NoError(err)

	// Create a project for membership tests
	s.project, err = s.Project.Create(ctx, s.org.ID, "test-members-project")
	assert.NoError(err)
}

// TearDownTest cleans up resources after each test has completed
func (s *projectMembersIntegrationTestSuite) TearDownTest() {
	ctx := context.Background()
	// Clean up database tables to avoid test interference
	_, _ = s.Data.DB.Membership.Delete().Exec(ctx)
	_, _ = s.Data.DB.Project.Delete().Exec(ctx)
}

// Test listing project members
func (s *projectMembersIntegrationTestSuite) TestListMembers() {
	ctx := context.Background()

	// Create additional users
	user2, err := s.User.UpsertByEmail(ctx, "user2@example.com", nil)
	require.NoError(s.T(), err)

	user3, err := s.User.UpsertByEmail(ctx, "user3@example.com", nil)
	require.NoError(s.T(), err)

	// Add users to organization
	_, err = s.Membership.Create(ctx, s.org.ID, user2.ID)
	require.NoError(s.T(), err)
	_, err = s.Membership.Create(ctx, s.org.ID, user3.ID)
	require.NoError(s.T(), err)

	// Add users to the project
	projectID := s.project.ID
	projectRef := &biz.IdentityReference{
		ID: &projectID,
	}

	opts1 := &biz.AddMemberToProjectOpts{
		ProjectReference: projectRef,
		UserEmail:        "user2@example.com",
		RequesterID:      uuid.MustParse(s.user.ID),
		Role:             authz.RoleProjectViewer,
	}
	_, err = s.Project.AddMemberToProject(ctx, uuid.MustParse(s.org.ID), opts1)
	require.NoError(s.T(), err)

	opts2 := &biz.AddMemberToProjectOpts{
		ProjectReference: projectRef,
		UserEmail:        "user3@example.com",
		RequesterID:      uuid.MustParse(s.user.ID),
		Role:             authz.RoleProjectAdmin,
	}
	_, err = s.Project.AddMemberToProject(ctx, uuid.MustParse(s.org.ID), opts2)
	require.NoError(s.T(), err)

	s.Run("list all members", func() {
		members, count, err := s.Project.ListMembers(ctx, uuid.MustParse(s.org.ID), projectRef, nil)
		s.NoError(err)
		s.Equal(2, len(members))
		s.Equal(2, count)

		// Verify we have both a regular member and an admin
		adminCount := 0
		for _, member := range members {
			if member.Role == authz.RoleProjectAdmin {
				adminCount++
			}
		}
		s.Equal(1, adminCount, "Should have exactly one admin member")
	})

	s.Run("list members with pagination", func() {
		paginationOpts, err := pagination.NewOffsetPaginationOpts(1, 1)
		require.NoError(s.T(), err)

		members, count, err := s.Project.ListMembers(ctx, uuid.MustParse(s.org.ID), projectRef, paginationOpts)
		s.NoError(err)
		s.Equal(1, len(members))
		s.Equal(2, count) // Total count should be 2

		// Get the next page
		paginationOpts, err = pagination.NewOffsetPaginationOpts(2, 1)
		require.NoError(s.T(), err)

		secondPageMembers, secondCount, err := s.Project.ListMembers(ctx, uuid.MustParse(s.org.ID), projectRef, paginationOpts)
		s.NoError(err)
		s.Equal(1, len(secondPageMembers))
		s.Equal(2, secondCount)

		// Verify the two pages contain different members
		s.NotEqual(members[0].User.ID, secondPageMembers[0].User.ID)
	})

	s.Run("list members with project name", func() {
		projectName := s.project.Name
		nameRef := &biz.IdentityReference{
			Name: &projectName,
		}
		members, count, err := s.Project.ListMembers(ctx, uuid.MustParse(s.org.ID), nameRef, nil)
		s.NoError(err)
		s.Equal(2, len(members))
		s.Equal(2, count)
	})

	s.Run("list members with non-existent project", func() {
		nonExistentID := uuid.New()
		invalidRef := &biz.IdentityReference{
			ID: &nonExistentID,
		}
		_, _, err := s.Project.ListMembers(ctx, uuid.MustParse(s.org.ID), invalidRef, nil)
		s.Error(err)
		s.True(biz.IsNotFound(err))
	})

	s.Run("list members with wrong organization", func() {
		org2, err := s.Organization.CreateWithRandomName(ctx)
		require.NoError(s.T(), err)

		_, _, err = s.Project.ListMembers(ctx, uuid.MustParse(org2.ID), projectRef, nil)
		s.Error(err)
		s.True(biz.IsNotFound(err))
	})
}

// Test adding members to projects
func (s *projectMembersIntegrationTestSuite) TestAddMemberToProject() {
	ctx := context.Background()

	// Create additional users
	user2, err := s.User.UpsertByEmail(ctx, "add-user2@example.com", nil)
	require.NoError(s.T(), err)

	user3, err := s.User.UpsertByEmail(ctx, "add-user3@example.com", nil)
	require.NoError(s.T(), err)

	// Add users to organization
	_, err = s.Membership.Create(ctx, s.org.ID, user2.ID)
	require.NoError(s.T(), err)
	_, err = s.Membership.Create(ctx, s.org.ID, user3.ID)
	require.NoError(s.T(), err)

	projectID := s.project.ID
	projectRef := &biz.IdentityReference{
		ID: &projectID,
	}

	s.Run("add member using project ID", func() {
		// Add user2 as a viewer
		opts := &biz.AddMemberToProjectOpts{
			ProjectReference: projectRef,
			UserEmail:        "add-user2@example.com",
			RequesterID:      uuid.MustParse(s.user.ID),
			Role:             authz.RoleProjectViewer,
		}

		membership, err := s.Project.AddMemberToProject(ctx, uuid.MustParse(s.org.ID), opts)
		s.NoError(err)
		s.NotNil(membership)
		s.Equal(user2.ID, membership.Membership.User.ID)
		s.Equal(authz.RoleProjectViewer, membership.Membership.Role)

		// Verify the member was added by listing members
		members, count, err := s.Project.ListMembers(ctx, uuid.MustParse(s.org.ID), projectRef, nil)
		s.NoError(err)
		s.Equal(1, len(members))
		s.Equal(1, count)
	})

	s.Run("add member using project name", func() {
		// Add user3 as an admin
		projectName := s.project.Name
		nameRef := &biz.IdentityReference{
			Name: &projectName,
		}
		opts := &biz.AddMemberToProjectOpts{
			ProjectReference: nameRef,
			UserEmail:        "add-user3@example.com",
			RequesterID:      uuid.MustParse(s.user.ID),
			Role:             authz.RoleProjectAdmin,
		}

		membership, err := s.Project.AddMemberToProject(ctx, uuid.MustParse(s.org.ID), opts)
		s.NoError(err)
		s.NotNil(membership)
		s.Equal(user3.ID, membership.Membership.User.ID)
		s.Equal(authz.RoleProjectAdmin, membership.Membership.Role)

		// Verify the member was added by listing members
		members, count, err := s.Project.ListMembers(ctx, uuid.MustParse(s.org.ID), projectRef, nil)
		s.NoError(err)
		s.Equal(2, len(members))
		s.Equal(2, count)
	})

	s.Run("add member to project in wrong organization", func() {
		// Create a new organization
		org2, err := s.Organization.CreateWithRandomName(ctx)
		require.NoError(s.T(), err)

		// Attempt to add user2 to a project in the wrong organization
		opts := &biz.AddMemberToProjectOpts{
			ProjectReference: projectRef,
			UserEmail:        "add-user2@example.com",
			RequesterID:      uuid.MustParse(s.user.ID),
			Role:             authz.RoleProjectViewer,
		}

		_, err = s.Project.AddMemberToProject(ctx, uuid.MustParse(org2.ID), opts)
		s.Error(err)
		s.True(biz.IsNotFound(err))
	})

	s.Run("add member to non-existent project", func() {
		nonExistentProjectID := uuid.New()
		invalidRef := &biz.IdentityReference{
			ID: &nonExistentProjectID,
		}
		opts := &biz.AddMemberToProjectOpts{
			ProjectReference: invalidRef,
			UserEmail:        "add-user2@example.com",
			RequesterID:      uuid.MustParse(s.user.ID),
			Role:             authz.RoleProjectViewer,
		}

		_, err := s.Project.AddMemberToProject(ctx, uuid.MustParse(s.org.ID), opts)
		s.Error(err)
		s.True(biz.IsNotFound(err))
	})

	s.Run("add member who is not in the organization", func() {
		// Create user who is not in the organization
		nonExistingEmail := "not-in-org@example.com"
		_, err := s.User.UpsertByEmail(ctx, nonExistingEmail, nil)
		require.NoError(s.T(), err)
		// Note: not adding this user to the organization

		opts := &biz.AddMemberToProjectOpts{
			ProjectReference: projectRef,
			UserEmail:        nonExistingEmail,
			RequesterID:      uuid.MustParse(s.user.ID),
			Role:             authz.RoleProjectViewer,
		}

		result, err := s.Project.AddMemberToProject(ctx, uuid.MustParse(s.org.ID), opts)
		s.NoError(err)
		s.NotNil(result)
		s.True(result.InvitationSent, "An invitation should be sent for users not in the organization when requester is an org admin")
		s.Nil(result.Membership, "No membership should be created directly")

		// Verify an invitation was created
		invitations, err := s.OrgInvitation.ListByOrg(ctx, s.org.ID)
		s.NoError(err)
		s.GreaterOrEqual(len(invitations), 1)

		// Find the invitation for this email
		var foundInvitation *biz.OrgInvitation
		for _, inv := range invitations {
			if inv.ReceiverEmail == nonExistingEmail {
				foundInvitation = inv
				break
			}
		}

		s.NotNil(foundInvitation, "Should find an invitation for the email")

		// Verify the invitation has project context
		s.NotNil(foundInvitation.Context, "Invitation should have context")
		s.Equal(projectID, *foundInvitation.Context.ProjectIDToJoin)
		s.Equal(authz.RoleProjectViewer, foundInvitation.Context.ProjectRole)
	})

	s.Run("project admin (non-org admin) should not be able to invite new users", func() {
		// Create a project admin who is not an org admin
		projectAdminUser, err := s.User.UpsertByEmail(ctx, "project-admin@example.com", nil)
		require.NoError(s.T(), err)

		// Add user to organization as a regular member (not org admin)
		_, err = s.Membership.Create(ctx, s.org.ID, projectAdminUser.ID, biz.WithMembershipRole(authz.RoleOrgMember), biz.WithCurrentMembership())
		require.NoError(s.T(), err)

		// Add user to project as a project admin
		projectAdminOpts := &biz.AddMemberToProjectOpts{
			ProjectReference: projectRef,
			UserEmail:        "project-admin@example.com",
			RequesterID:      uuid.MustParse(s.user.ID), // Using the org admin to add them
			Role:             authz.RoleProjectAdmin,
		}
		_, err = s.Project.AddMemberToProject(ctx, uuid.MustParse(s.org.ID), projectAdminOpts)
		require.NoError(s.T(), err)

		// Create a new user who is not in the organization
		nonExistingEmail := "not-in-org-2@example.com"
		_, err = s.User.UpsertByEmail(ctx, nonExistingEmail, nil)
		require.NoError(s.T(), err)

		// Try to add the non-existing user using the project admin
		opts := &biz.AddMemberToProjectOpts{
			ProjectReference: projectRef,
			UserEmail:        nonExistingEmail,
			RequesterID:      uuid.MustParse(projectAdminUser.ID),
			Role:             authz.RoleProjectViewer,
		}

		// The invitation should be rejected
		_, err = s.Project.AddMemberToProject(ctx, uuid.MustParse(s.org.ID), opts)
		s.Error(err)
		s.True(biz.IsErrValidation(err))
		s.Contains(err.Error(), "only organization admins or owners can invite new users")
	})

	s.Run("org owner should be able to invite new users", func() {
		// Create an org owner
		orgOwnerUser, err := s.User.UpsertByEmail(ctx, "org-owner@example.com", nil)
		require.NoError(s.T(), err)

		// Add user to organization as an owner
		_, err = s.Membership.Create(ctx, s.org.ID, orgOwnerUser.ID, biz.WithMembershipRole(authz.RoleOwner), biz.WithCurrentMembership())
		require.NoError(s.T(), err)

		// Create a new user who is not in the organization
		nonExistingEmail := "not-in-org-3@example.com"
		_, err = s.User.UpsertByEmail(ctx, nonExistingEmail, nil)
		require.NoError(s.T(), err)

		// Try to add the non-existing user using the org owner
		opts := &biz.AddMemberToProjectOpts{
			ProjectReference: projectRef,
			UserEmail:        nonExistingEmail,
			RequesterID:      uuid.MustParse(orgOwnerUser.ID),
			Role:             authz.RoleProjectViewer,
		}

		// The invitation should be sent successfully
		result, err := s.Project.AddMemberToProject(ctx, uuid.MustParse(s.org.ID), opts)
		s.NoError(err)
		s.NotNil(result)
		s.True(result.InvitationSent, "An invitation should be sent for users not in the organization when requester is an org owner")
		s.Nil(result.Membership, "No membership should be created directly")
	})
}

// Test removing members from projects
func (s *projectMembersIntegrationTestSuite) TestRemoveMemberFromProject() {
	ctx := context.Background()

	// Create additional users
	user2, err := s.User.UpsertByEmail(ctx, "remove-user2@example.com", nil)
	require.NoError(s.T(), err)

	user3, err := s.User.UpsertByEmail(ctx, "remove-user3@example.com", nil)
	require.NoError(s.T(), err)

	user4, err := s.User.UpsertByEmail(ctx, "remove-user4@example.com", nil)
	require.NoError(s.T(), err)

	// Add users to organization
	_, err = s.Membership.Create(ctx, s.org.ID, user2.ID)
	require.NoError(s.T(), err)
	_, err = s.Membership.Create(ctx, s.org.ID, user3.ID)
	require.NoError(s.T(), err)
	_, err = s.Membership.Create(ctx, s.org.ID, user4.ID)
	require.NoError(s.T(), err)

	projectID := s.project.ID
	projectRef := &biz.IdentityReference{
		ID: &projectID,
	}

	// Add users to the project
	opts1 := &biz.AddMemberToProjectOpts{
		ProjectReference: projectRef,
		UserEmail:        "remove-user2@example.com",
		RequesterID:      uuid.MustParse(s.user.ID),
		Role:             authz.RoleProjectViewer,
	}
	_, err = s.Project.AddMemberToProject(ctx, uuid.MustParse(s.org.ID), opts1)
	require.NoError(s.T(), err)

	opts2 := &biz.AddMemberToProjectOpts{
		ProjectReference: projectRef,
		UserEmail:        "remove-user3@example.com",
		RequesterID:      uuid.MustParse(s.user.ID),
		Role:             authz.RoleProjectAdmin,
	}
	_, err = s.Project.AddMemberToProject(ctx, uuid.MustParse(s.org.ID), opts2)
	require.NoError(s.T(), err)

	opts3 := &biz.AddMemberToProjectOpts{
		ProjectReference: projectRef,
		UserEmail:        "remove-user4@example.com",
		RequesterID:      uuid.MustParse(s.user.ID),
		Role:             authz.RoleProjectViewer,
	}
	_, err = s.Project.AddMemberToProject(ctx, uuid.MustParse(s.org.ID), opts3)
	require.NoError(s.T(), err)

	// Verify initial member count
	members, count, err := s.Project.ListMembers(ctx, uuid.MustParse(s.org.ID), projectRef, nil)
	s.NoError(err)
	s.Equal(3, len(members))
	s.Equal(3, count)

	s.Run("remove a regular member from project", func() {
		// Remove user2 (regular member)
		removeOpts := &biz.RemoveMemberFromProjectOpts{
			ProjectReference: projectRef,
			UserEmail:        "remove-user2@example.com",
			RequesterID:      uuid.MustParse(s.user.ID),
		}

		err := s.Project.RemoveMemberFromProject(ctx, uuid.MustParse(s.org.ID), removeOpts)
		s.NoError(err)

		// Verify member was removed
		members, count, err := s.Project.ListMembers(ctx, uuid.MustParse(s.org.ID), projectRef, nil)
		s.NoError(err)
		s.Equal(2, len(members))
		s.Equal(2, count)

		// Verify the removed user is not in the list
		for _, member := range members {
			s.NotEqual(user2.ID, member.User.ID)
		}
	})

	s.Run("remove an admin member from project", func() {
		// Remove user3 (admin)
		removeOpts := &biz.RemoveMemberFromProjectOpts{
			ProjectReference: projectRef,
			UserEmail:        "remove-user3@example.com",
			RequesterID:      uuid.MustParse(s.user.ID),
		}

		err := s.Project.RemoveMemberFromProject(ctx, uuid.MustParse(s.org.ID), removeOpts)
		s.NoError(err)

		// Verify member was removed
		members, count, err := s.Project.ListMembers(ctx, uuid.MustParse(s.org.ID), projectRef, nil)
		s.NoError(err)
		s.Equal(1, len(members))
		s.Equal(1, count)

		// Check remaining members - user3 should not be present
		for _, member := range members {
			s.NotEqual(user3.ID, member.User.ID)
		}
	})

	s.Run("try to remove non-existent member", func() {
		// Create a user who's not in the project
		nonMemberUser, err := s.User.UpsertByEmail(ctx, "non-member@example.com", nil)
		require.NoError(s.T(), err)
		_, err = s.Membership.Create(ctx, s.org.ID, nonMemberUser.ID)
		require.NoError(s.T(), err)

		// Try to remove a user who's not in the project
		removeOpts := &biz.RemoveMemberFromProjectOpts{
			ProjectReference: projectRef,
			UserEmail:        "non-member@example.com",
			RequesterID:      uuid.MustParse(s.user.ID),
		}

		err = s.Project.RemoveMemberFromProject(ctx, uuid.MustParse(s.org.ID), removeOpts)
		s.Error(err)
		s.True(biz.IsErrValidation(err))

		// Member count should remain unchanged
		_, count, err := s.Project.ListMembers(ctx, uuid.MustParse(s.org.ID), projectRef, nil)
		s.NoError(err)
		s.Equal(1, count)
	})

	s.Run("remove member from wrong organization", func() {
		// Create a new organization
		org2, err := s.Organization.CreateWithRandomName(ctx)
		require.NoError(s.T(), err)

		// Try to remove user4 using the wrong org ID
		removeOpts := &biz.RemoveMemberFromProjectOpts{
			ProjectReference: projectRef,
			UserEmail:        "remove-user4@example.com",
			RequesterID:      uuid.MustParse(s.user.ID),
		}

		err = s.Project.RemoveMemberFromProject(ctx, uuid.MustParse(org2.ID), removeOpts)
		s.Error(err)
		s.True(biz.IsNotFound(err))

		// Member count should remain unchanged
		_, count, err := s.Project.ListMembers(ctx, uuid.MustParse(s.org.ID), projectRef, nil)
		s.NoError(err)
		s.Equal(1, count)
	})

	s.Run("remove member from non-existent project", func() {
		nonExistentProjectID := uuid.New()
		invalidRef := &biz.IdentityReference{
			ID: &nonExistentProjectID,
		}
		removeOpts := &biz.RemoveMemberFromProjectOpts{
			ProjectReference: invalidRef,
			UserEmail:        "remove-user4@example.com",
			RequesterID:      uuid.MustParse(s.user.ID),
		}

		err = s.Project.RemoveMemberFromProject(ctx, uuid.MustParse(s.org.ID), removeOpts)
		s.Error(err)
		s.True(biz.IsNotFound(err))
	})

	s.Run("requester not part of organization", func() {
		// Create a user who is not in any organization
		externalUser, err := s.User.UpsertByEmail(ctx, "external-user@example.com", nil)
		require.NoError(s.T(), err)

		// Try to remove a member with an external user as requester
		removeOpts := &biz.RemoveMemberFromProjectOpts{
			ProjectReference: projectRef,
			UserEmail:        "remove-user4@example.com",
			RequesterID:      uuid.MustParse(externalUser.ID),
		}

		err = s.Project.RemoveMemberFromProject(ctx, uuid.MustParse(s.org.ID), removeOpts)
		s.Error(err)
		s.Contains(err.Error(), "requester is not a member of the organization")
	})

	s.Run("non-existent user email", func() {
		// Try to remove a non-existent user
		removeOpts := &biz.RemoveMemberFromProjectOpts{
			ProjectReference: projectRef,
			UserEmail:        "non-existent-user@example.com",
			RequesterID:      uuid.MustParse(s.user.ID),
		}

		err = s.Project.RemoveMemberFromProject(ctx, uuid.MustParse(s.org.ID), removeOpts)
		s.Error(err)
		s.Contains(err.Error(), "not a member of the organization")
	})
}

// projectAdminPermissionsTestSuite tests the permissions of project admins
type projectAdminPermissionsTestSuite struct {
	testhelpers.UseCasesEachTestSuite
	org     *biz.Organization
	user    *biz.User
	project *biz.Project
}

func (s *projectAdminPermissionsTestSuite) SetupTest() {
	var err error
	assert := assert.New(s.T())
	s.TestingUseCases = testhelpers.NewTestingUseCases(s.T())

	ctx := context.Background()
	s.org, err = s.Organization.CreateWithRandomName(ctx)
	assert.NoError(err)

	// Create a user for admin tests - this user will be an org admin by default
	s.user, err = s.User.UpsertByEmail(ctx, fmt.Sprintf("admin-user-%s@example.com", uuid.New().String()), nil)
	assert.NoError(err)

	// Add user to organization as an admin
	_, err = s.Membership.Create(ctx, s.org.ID, s.user.ID, biz.WithMembershipRole(authz.RoleAdmin), biz.WithCurrentMembership())
	assert.NoError(err)

	// Create a project for admin tests
	s.project, err = s.Project.Create(ctx, s.org.ID, "test-admin-project")
	assert.NoError(err)
}

// TearDownTest cleans up resources after each test has completed
func (s *projectAdminPermissionsTestSuite) TearDownTest() {
	ctx := context.Background()
	// Clean up database tables to avoid test interference
	_, _ = s.Data.DB.Membership.Delete().Exec(ctx)
	_, _ = s.Data.DB.Project.Delete().Exec(ctx)
}
func (s *projectAdminPermissionsTestSuite) TestAdminPermissions() {
	ctx := context.Background()

	// Create a regular user
	user2, err := s.User.UpsertByEmail(ctx, "regular-user@example.com", nil)
	require.NoError(s.T(), err)

	// Add the user to the organization
	_, err = s.Membership.Create(ctx, s.org.ID, user2.ID, biz.WithCurrentMembership())
	require.NoError(s.T(), err)

	// Grant project admin role to the user
	projectID := s.project.ID
	projectRef := &biz.IdentityReference{
		ID: &projectID,
	}

	opts := &biz.AddMemberToProjectOpts{
		ProjectReference: projectRef,
		UserEmail:        "regular-user@example.com",
		RequesterID:      uuid.MustParse(s.user.ID),
		Role:             authz.RoleProjectAdmin,
	}
	_, err = s.Project.AddMemberToProject(ctx, uuid.MustParse(s.org.ID), opts)
	require.NoError(s.T(), err)

	s.Run("admin can add member to project", func() {
		// Create a new user and add them to the organization first
		newUserEmail := "new-member@example.com"
		newUser, err := s.User.UpsertByEmail(ctx, newUserEmail, nil)
		require.NoError(s.T(), err)

		// Add the new user to the organization
		_, err = s.Membership.Create(ctx, s.org.ID, newUser.ID)
		require.NoError(s.T(), err)

		addOpts := &biz.AddMemberToProjectOpts{
			ProjectReference: projectRef,
			UserEmail:        newUserEmail,
			RequesterID:      uuid.MustParse(user2.ID),
			Role:             authz.RoleProjectViewer,
		}

		membership, err := s.Project.AddMemberToProject(ctx, uuid.MustParse(s.org.ID), addOpts)
		s.NoError(err)
		s.NotNil(membership)
		s.Equal(newUser.ID, membership.Membership.User.ID)
		s.Equal(authz.RoleProjectViewer, membership.Membership.Role)

		// Verify the member was added
		members, count, err := s.Project.ListMembers(ctx, uuid.MustParse(s.org.ID), projectRef, nil)
		s.NoError(err)
		s.Equal(2, len(members))
		s.Equal(2, count)
	})

	s.Run("admin can remove member from project", func() {
		// Admin user removes a member from the project
		removeOpts := &biz.RemoveMemberFromProjectOpts{
			ProjectReference: projectRef,
			UserEmail:        "regular-user@example.com",
			RequesterID:      uuid.MustParse(user2.ID),
		}

		err := s.Project.RemoveMemberFromProject(ctx, uuid.MustParse(s.org.ID), removeOpts)
		s.NoError(err)

		// Verify the member was removed
		members, count, err := s.Project.ListMembers(ctx, uuid.MustParse(s.org.ID), projectRef, nil)
		s.NoError(err)
		s.Equal(1, len(members))
		s.Equal(1, count)
	})

	s.Run("admin can add themselves as a member", func() {
		addOpts := &biz.AddMemberToProjectOpts{
			ProjectReference: projectRef,
			UserEmail:        s.user.Email,
			RequesterID:      uuid.MustParse(s.user.ID),
			Role:             authz.RoleProjectViewer,
		}

		_, err := s.Project.AddMemberToProject(ctx, uuid.MustParse(s.org.ID), addOpts)
		s.NoError(err)
	})

	s.Run("admin can remove themselves from the project", func() {
		removeOpts := &biz.RemoveMemberFromProjectOpts{
			ProjectReference: projectRef,
			UserEmail:        s.user.Email,
			RequesterID:      uuid.MustParse(s.user.ID),
		}

		err := s.Project.RemoveMemberFromProject(ctx, uuid.MustParse(s.org.ID), removeOpts)
		s.NoError(err)
	})
}

type projectPermissionsTestSuite struct {
	testhelpers.UseCasesEachTestSuite
	org              *biz.Organization
	adminUser        *biz.User
	projectAdminUser *biz.User
	regularUser      *biz.User
	project          *biz.Project
}

func (s *projectPermissionsTestSuite) SetupTest() {
	var err error
	assert := assert.New(s.T())
	s.TestingUseCases = testhelpers.NewTestingUseCases(s.T())

	ctx := context.Background()
	s.org, err = s.Organization.CreateWithRandomName(ctx)
	assert.NoError(err)

	// Create an org admin user
	s.adminUser, err = s.User.UpsertByEmail(ctx, fmt.Sprintf("admin-user-%s@example.com", uuid.New().String()), nil)
	assert.NoError(err)

	// Add admin user to organization as an admin
	_, err = s.Membership.Create(ctx, s.org.ID, s.adminUser.ID, biz.WithMembershipRole(authz.RoleAdmin), biz.WithCurrentMembership())
	assert.NoError(err)

	// Create a project admin user
	s.projectAdminUser, err = s.User.UpsertByEmail(ctx, fmt.Sprintf("project-admin-%s@example.com", uuid.New().String()), nil)
	assert.NoError(err)

	// Add project admin user to organization as a regular member
	_, err = s.Membership.Create(ctx, s.org.ID, s.projectAdminUser.ID, biz.WithCurrentMembership())
	assert.NoError(err)

	// Create a regular user
	s.regularUser, err = s.User.UpsertByEmail(ctx, fmt.Sprintf("regular-user-%s@example.com", uuid.New().String()), nil)
	assert.NoError(err)

	// Add regular user to organization as a regular member
	_, err = s.Membership.Create(ctx, s.org.ID, s.regularUser.ID)
	assert.NoError(err)

	// Create a project for tests
	s.project, err = s.Project.Create(ctx, s.org.ID, "test-permissions-project")
	assert.NoError(err)

	// Add project admin user to the project as admin
	projectID := s.project.ID
	projectRef := &biz.IdentityReference{
		ID: &projectID,
	}

	opts := &biz.AddMemberToProjectOpts{
		ProjectReference: projectRef,
		UserEmail:        s.projectAdminUser.Email,
		RequesterID:      uuid.MustParse(s.adminUser.ID),
		Role:             authz.RoleProjectAdmin,
	}
	_, err = s.Project.AddMemberToProject(ctx, uuid.MustParse(s.org.ID), opts)
	assert.NoError(err)

	// Add regular user to the project as regular member
	opts = &biz.AddMemberToProjectOpts{
		ProjectReference: projectRef,
		UserEmail:        s.regularUser.Email,
		RequesterID:      uuid.MustParse(s.adminUser.ID),
		Role:             authz.RoleProjectViewer,
	}
	_, err = s.Project.AddMemberToProject(ctx, uuid.MustParse(s.org.ID), opts)
	assert.NoError(err)
}

// TearDownTest cleans up resources after each test has completed
func (s *projectPermissionsTestSuite) TearDownTest() {
	ctx := context.Background()
	// Clean up database tables to avoid test interference
	_, _ = s.Data.DB.Membership.Delete().Exec(ctx)
	_, _ = s.Data.DB.Project.Delete().Exec(ctx)
	_, _ = s.Data.DB.Membership.Delete().Exec(ctx)
}

// TestRegularUserPermissions verifies that regular users can't modify project memberships
func (s *projectPermissionsTestSuite) TestRegularUserPermissions() {
	ctx := context.Background()
	projectID := s.project.ID
	projectRef := &biz.IdentityReference{
		ID: &projectID,
	}

	// Create a new user to try adding to the project
	newUser, err := s.User.UpsertByEmail(ctx, "new-user@example.com", nil)
	require.NoError(s.T(), err)

	// Add the user to the organization
	_, err = s.Membership.Create(ctx, s.org.ID, newUser.ID)
	require.NoError(s.T(), err)

	s.Run("regular user cannot add member to project", func() {
		// Regular user tries to add a new member
		addOpts := &biz.AddMemberToProjectOpts{
			ProjectReference: projectRef,
			UserEmail:        "new-user@example.com",
			RequesterID:      uuid.MustParse(s.regularUser.ID),
			Role:             authz.RoleProjectViewer,
		}

		_, err := s.Project.AddMemberToProject(ctx, uuid.MustParse(s.org.ID), addOpts)
		s.Error(err)
		s.Contains(err.Error(), "does not have permission")
	})

	s.Run("regular user cannot remove member from project", func() {
		// Regular user tries to remove a member
		removeOpts := &biz.RemoveMemberFromProjectOpts{
			ProjectReference: projectRef,
			UserEmail:        s.projectAdminUser.Email,
			RequesterID:      uuid.MustParse(s.regularUser.ID),
		}

		err := s.Project.RemoveMemberFromProject(ctx, uuid.MustParse(s.org.ID), removeOpts)
		s.Error(err)
		s.Contains(err.Error(), "does not have permission")
	})

	s.Run("regular user cannot grant admin to others", func() {
		// First, let admin add the new user to the project
		addOpts := &biz.AddMemberToProjectOpts{
			ProjectReference: projectRef,
			UserEmail:        "new-user@example.com",
			RequesterID:      uuid.MustParse(s.adminUser.ID),
			Role:             authz.RoleProjectViewer,
		}

		_, err := s.Project.AddMemberToProject(ctx, uuid.MustParse(s.org.ID), addOpts)
		s.NoError(err)

		// Now try to update the new user to admin with regular user permissions
		// Note: There's no direct "update" method, so we would need to remove and re-add
		// with admin permission, which would fail at the removal step
		removeOpts := &biz.RemoveMemberFromProjectOpts{
			ProjectReference: projectRef,
			UserEmail:        "new-user@example.com",
			RequesterID:      uuid.MustParse(s.regularUser.ID),
		}

		err = s.Project.RemoveMemberFromProject(ctx, uuid.MustParse(s.org.ID), removeOpts)
		s.Error(err)
		s.Contains(err.Error(), "does not have permission")
	})
}

// TestProjectAdminPermissions verifies that project admins can modify project memberships
func (s *projectPermissionsTestSuite) TestProjectAdminPermissions() {
	ctx := context.Background()
	projectID := s.project.ID
	projectRef := &biz.IdentityReference{
		ID: &projectID,
	}

	// Create a new user to add to the project
	newUser, err := s.User.UpsertByEmail(ctx, "new-user-2@example.com", nil)
	require.NoError(s.T(), err)

	// Add the user to the organization
	_, err = s.Membership.Create(ctx, s.org.ID, newUser.ID)
	require.NoError(s.T(), err)

	s.Run("project admin can add member to project", func() {
		// Project admin adds a new member
		addOpts := &biz.AddMemberToProjectOpts{
			ProjectReference: projectRef,
			UserEmail:        "new-user-2@example.com",
			RequesterID:      uuid.MustParse(s.projectAdminUser.ID),
			Role:             authz.RoleProjectViewer,
		}

		membership, err := s.Project.AddMemberToProject(ctx, uuid.MustParse(s.org.ID), addOpts)
		s.NoError(err)
		s.NotNil(membership)
		s.Equal(newUser.ID, membership.Membership.User.ID)
		s.Equal(authz.RoleProjectViewer, membership.Membership.Role)
	})

	s.Run("project admin can remove member from project", func() {
		// Project admin removes a member
		removeOpts := &biz.RemoveMemberFromProjectOpts{
			ProjectReference: projectRef,
			UserEmail:        s.regularUser.Email,
			RequesterID:      uuid.MustParse(s.projectAdminUser.ID),
		}

		err := s.Project.RemoveMemberFromProject(ctx, uuid.MustParse(s.org.ID), removeOpts)
		s.NoError(err)

		// Verify the member was removed
		members, count, err := s.Project.ListMembers(ctx, uuid.MustParse(s.org.ID), projectRef, nil)
		s.NoError(err)
		s.Equal(2, count) // Project admin + new user

		// Verify the regularUser is not in the list
		for _, member := range members {
			s.NotEqual(s.regularUser.ID, member.User.ID)
		}
	})

	s.Run("project admin can grant admin privileges to others", func() {
		// First, add the regular user back
		addOpts := &biz.AddMemberToProjectOpts{
			ProjectReference: projectRef,
			UserEmail:        s.regularUser.Email,
			RequesterID:      uuid.MustParse(s.projectAdminUser.ID),
			Role:             authz.RoleProjectAdmin, // Make them an admin this time
		}

		membership, err := s.Project.AddMemberToProject(ctx, uuid.MustParse(s.org.ID), addOpts)
		s.NoError(err)
		s.NotNil(membership)
		s.Equal(s.regularUser.ID, membership.Membership.User.ID)
		s.True(membership.Membership.Role == authz.RoleProjectAdmin)
	})
}

// TestProjectIsolation verifies that project admins can't modify other projects
func (s *projectPermissionsTestSuite) TestProjectIsolation() {
	ctx := context.Background()

	// Create a second project
	secondProject, err := s.Project.Create(ctx, s.org.ID, "test-other-project")
	require.NoError(s.T(), err)

	secondProjectID := secondProject.ID
	secondProjectRef := &biz.IdentityReference{
		ID: &secondProjectID,
	}

	// Create a new user to try to add to the second project
	newUser, err := s.User.UpsertByEmail(ctx, "new-user-3@example.com", nil)
	require.NoError(s.T(), err)

	// Add the user to the organization
	_, err = s.Membership.Create(ctx, s.org.ID, newUser.ID)
	require.NoError(s.T(), err)

	s.Run("project admin cannot add member to another project", func() {
		// Project admin tries to add a member to a different project
		addOpts := &biz.AddMemberToProjectOpts{
			ProjectReference: secondProjectRef,
			UserEmail:        "new-user-3@example.com",
			RequesterID:      uuid.MustParse(s.projectAdminUser.ID),
			Role:             authz.RoleProjectViewer,
		}

		_, err := s.Project.AddMemberToProject(ctx, uuid.MustParse(s.org.ID), addOpts)
		s.Error(err)
		s.Contains(err.Error(), "does not have permission")
	})

	// Add the new user to the second project with org admin permissions
	addOpts := &biz.AddMemberToProjectOpts{
		ProjectReference: secondProjectRef,
		UserEmail:        "new-user-3@example.com",
		RequesterID:      uuid.MustParse(s.adminUser.ID),
		Role:             authz.RoleProjectViewer,
	}
	_, err = s.Project.AddMemberToProject(ctx, uuid.MustParse(s.org.ID), addOpts)
	require.NoError(s.T(), err)

	s.Run("project admin cannot remove member from another project", func() {
		// Project admin tries to remove a member from a different project
		removeOpts := &biz.RemoveMemberFromProjectOpts{
			ProjectReference: secondProjectRef,
			UserEmail:        "new-user-3@example.com",
			RequesterID:      uuid.MustParse(s.projectAdminUser.ID),
		}

		err := s.Project.RemoveMemberFromProject(ctx, uuid.MustParse(s.org.ID), removeOpts)
		s.Error(err)
		s.Contains(err.Error(), "does not have permission")
	})
}

// TestOrgAdminPermissions verifies that organization admins can modify any project membership
func (s *projectPermissionsTestSuite) TestOrgAdminPermissions() {
	ctx := context.Background()

	// Create a second project
	secondProject, err := s.Project.Create(ctx, s.org.ID, "test-admin-other-project")
	require.NoError(s.T(), err)

	secondProjectID := secondProject.ID
	secondProjectRef := &biz.IdentityReference{
		ID: &secondProjectID,
	}

	// Create a new user to add to the second project
	newUser, err := s.User.UpsertByEmail(ctx, "new-user-4@example.com", nil)
	require.NoError(s.T(), err)

	// Add the user to the organization
	_, err = s.Membership.Create(ctx, s.org.ID, newUser.ID)
	require.NoError(s.T(), err)

	s.Run("organization admin can add member to any project", func() {
		// Org admin adds a member to a project
		addOpts := &biz.AddMemberToProjectOpts{
			ProjectReference: secondProjectRef,
			UserEmail:        "new-user-4@example.com",
			RequesterID:      uuid.MustParse(s.adminUser.ID),
			Role:             authz.RoleProjectViewer,
		}

		membership, err := s.Project.AddMemberToProject(ctx, uuid.MustParse(s.org.ID), addOpts)
		s.NoError(err)
		s.NotNil(membership)
		s.Equal(newUser.ID, membership.Membership.User.ID)
		s.Equal(authz.RoleProjectViewer, membership.Membership.Role)
	})

	s.Run("organization admin can remove member from any project", func() {
		// Org admin removes a member from a project
		removeOpts := &biz.RemoveMemberFromProjectOpts{
			ProjectReference: secondProjectRef,
			UserEmail:        "new-user-4@example.com",
			RequesterID:      uuid.MustParse(s.adminUser.ID),
		}

		err := s.Project.RemoveMemberFromProject(ctx, uuid.MustParse(s.org.ID), removeOpts)
		s.NoError(err)

		// Verify the member was removed
		members, count, err := s.Project.ListMembers(ctx, uuid.MustParse(s.org.ID), secondProjectRef, nil)
		s.NoError(err)
		s.Equal(0, count)
		s.Empty(members)
	})
}

// Utility struct for project group members tests
type projectGroupMembersIntegrationTestSuite struct {
	testhelpers.UseCasesEachTestSuite
	org      *biz.Organization
	user     *biz.User
	userUUID *uuid.UUID
	project  *biz.Project
	group    *biz.Group
}

func (s *projectGroupMembersIntegrationTestSuite) SetupTest() {
	var err error
	assert := assert.New(s.T())
	s.TestingUseCases = testhelpers.NewTestingUseCases(s.T())

	ctx := context.Background()
	s.org, err = s.Organization.CreateWithRandomName(ctx)
	assert.NoError(err)

	// Create a user for tests - this user will be an org admin by default
	s.user, err = s.User.UpsertByEmail(ctx, fmt.Sprintf("group-test-user-%s@example.com", uuid.New().String()), nil)
	assert.NoError(err)
	userUUID := uuid.MustParse(s.user.ID)
	s.userUUID = &userUUID

	// Add user to organization as an admin
	_, err = s.Membership.Create(ctx, s.org.ID, s.user.ID, biz.WithMembershipRole(authz.RoleAdmin), biz.WithCurrentMembership())
	assert.NoError(err)

	// Create a project for group membership tests
	s.project, err = s.Project.Create(ctx, s.org.ID, "test-group-members-project")
	assert.NoError(err)

	// Create a group for membership tests
	s.group, err = s.Group.Create(ctx, uuid.MustParse(s.org.ID), "test-group", "A test group for project membership", s.userUUID)
	assert.NoError(err)
}

// TearDownTest cleans up resources after each test has completed
func (s *projectGroupMembersIntegrationTestSuite) TearDownTest() {
	ctx := context.Background()
	// Clean up database tables to avoid test interference
	_, _ = s.Data.DB.Membership.Delete().Exec(ctx)
	_, _ = s.Data.DB.Project.Delete().Exec(ctx)
	_, _ = s.Data.DB.Group.Delete().Exec(ctx)
	_, _ = s.Data.DB.GroupMembership.Delete().Exec(ctx)
}

// Test adding groups to projects
func (s *projectGroupMembersIntegrationTestSuite) TestAddGroupToProject() {
	ctx := context.Background()

	// Create additional users
	user2, err := s.User.UpsertByEmail(ctx, "group-user2@example.com", nil)
	require.NoError(s.T(), err)

	user3, err := s.User.UpsertByEmail(ctx, "group-user3@example.com", nil)
	require.NoError(s.T(), err)

	// Add users to organization
	_, err = s.Membership.Create(ctx, s.org.ID, user2.ID)
	require.NoError(s.T(), err)
	_, err = s.Membership.Create(ctx, s.org.ID, user3.ID)
	require.NoError(s.T(), err)

	projectID := s.project.ID
	projectRef := &biz.IdentityReference{
		ID: &projectID,
	}

	s.Run("add group to project", func() {
		// Add the group as a member to the project
		opts := &biz.AddMemberToProjectOpts{
			ProjectReference: projectRef,
			GroupReference:   &biz.IdentityReference{ID: &s.group.ID},
			RequesterID:      uuid.MustParse(s.user.ID),
			Role:             authz.RoleProjectViewer,
		}

		membership, err := s.Project.AddMemberToProject(ctx, uuid.MustParse(s.org.ID), opts)
		s.NoError(err)
		s.NotNil(membership)
		s.Equal(s.group.ID, membership.Membership.Group.ID)
		s.Equal(authz.RoleProjectViewer, membership.Membership.Role)

		// Verify the group was added by listing project members
		members, count, err := s.Project.ListMembers(ctx, uuid.MustParse(s.org.ID), projectRef, nil)
		s.NoError(err)
		s.Equal(1, len(members))
		s.Equal(1, count)
	})

	s.Run("add multiple groups to project", func() {
		// Create and add another group
		group2, err := s.Group.Create(ctx, uuid.MustParse(s.org.ID), "test-group-2", "Another test group", s.userUUID)
		require.NoError(s.T(), err)

		opts := &biz.AddMemberToProjectOpts{
			ProjectReference: projectRef,
			GroupReference:   &biz.IdentityReference{ID: &group2.ID},
			RequesterID:      uuid.MustParse(s.user.ID),
			Role:             authz.RoleProjectAdmin,
		}

		membership, err := s.Project.AddMemberToProject(ctx, uuid.MustParse(s.org.ID), opts)
		s.NoError(err)
		s.NotNil(membership)
		s.Equal(group2.ID, membership.Membership.Group.ID)
		s.Equal(authz.RoleProjectAdmin, membership.Membership.Role)

		// Verify both groups are members of the project
		members, count, err := s.Project.ListMembers(ctx, uuid.MustParse(s.org.ID), projectRef, nil)
		s.NoError(err)
		s.Equal(2, len(members))
		s.Equal(2, count)
	})

	s.Run("add group to project in wrong organization", func() {
		// Create a new organization
		org2, err := s.Organization.CreateWithRandomName(ctx)
		require.NoError(s.T(), err)

		// Attempt to add the group to a project in the wrong organization
		opts := &biz.AddMemberToProjectOpts{
			ProjectReference: projectRef,
			GroupReference:   &biz.IdentityReference{ID: &s.group.ID},
			RequesterID:      uuid.MustParse(s.user.ID),
			Role:             authz.RoleProjectViewer,
		}

		_, err = s.Project.AddMemberToProject(ctx, uuid.MustParse(org2.ID), opts)
		s.Error(err)
		s.True(biz.IsNotFound(err))
	})

	s.Run("add group to non-existent project", func() {
		nonExistentProjectID := uuid.New()
		invalidRef := &biz.IdentityReference{
			ID: &nonExistentProjectID,
		}
		opts := &biz.AddMemberToProjectOpts{
			ProjectReference: invalidRef,
			GroupReference:   &biz.IdentityReference{ID: &s.group.ID},
			RequesterID:      uuid.MustParse(s.user.ID),
			Role:             authz.RoleProjectViewer,
		}

		_, err := s.Project.AddMemberToProject(ctx, uuid.MustParse(s.org.ID), opts)
		s.Error(err)
		s.True(biz.IsNotFound(err))
	})
}

// Test removing groups from projects
func (s *projectGroupMembersIntegrationTestSuite) TestRemoveGroupFromProject() {
	ctx := context.Background()

	// Create additional users
	user2, err := s.User.UpsertByEmail(ctx, "remove-group-user2@example.com", nil)
	require.NoError(s.T(), err)

	user3, err := s.User.UpsertByEmail(ctx, "remove-group-user3@example.com", nil)
	require.NoError(s.T(), err)

	// Add users to organization
	_, err = s.Membership.Create(ctx, s.org.ID, user2.ID)
	require.NoError(s.T(), err)
	_, err = s.Membership.Create(ctx, s.org.ID, user3.ID)
	require.NoError(s.T(), err)

	projectID := s.project.ID
	projectRef := &biz.IdentityReference{
		ID: &projectID,
	}

	// Add the group to the project first
	_, err = s.Project.AddMemberToProject(ctx, uuid.MustParse(s.org.ID), &biz.AddMemberToProjectOpts{
		ProjectReference: projectRef,
		GroupReference:   &biz.IdentityReference{ID: &s.group.ID},
		RequesterID:      uuid.MustParse(s.user.ID),
		Role:             authz.RoleProjectViewer,
	})
	require.NoError(s.T(), err)

	s.Run("remove group from project", func() {
		// Remove the group from the project
		removeOpts := &biz.RemoveMemberFromProjectOpts{
			ProjectReference: projectRef,
			GroupReference:   &biz.IdentityReference{ID: &s.group.ID},
			RequesterID:      uuid.MustParse(s.user.ID),
		}

		err = s.Project.RemoveMemberFromProject(ctx, uuid.MustParse(s.org.ID), removeOpts)
		s.NoError(err)

		// Verify the group was removed
		members, count, err := s.Project.ListMembers(ctx, uuid.MustParse(s.org.ID), projectRef, nil)
		s.NoError(err)
		s.Equal(0, count)
		s.Empty(members)
	})

	s.Run("try to remove non-existent group", func() {
		// Try to remove a group that was already removed
		randomGroupID := uuid.New()
		removeOpts := &biz.RemoveMemberFromProjectOpts{
			ProjectReference: projectRef,
			GroupReference:   &biz.IdentityReference{ID: &randomGroupID},
			RequesterID:      uuid.MustParse(s.user.ID),
		}

		err := s.Project.RemoveMemberFromProject(ctx, uuid.MustParse(s.org.ID), removeOpts)
		s.Error(err)
		s.True(biz.IsErrValidation(err))
	})

	s.Run("remove group from wrong organization", func() {
		// Create a new organization
		org2, err := s.Organization.CreateWithRandomName(ctx)
		require.NoError(s.T(), err)

		// Try to remove the group using the wrong org ID
		removeOpts := &biz.RemoveMemberFromProjectOpts{
			ProjectReference: projectRef,
			GroupReference:   &biz.IdentityReference{ID: &s.group.ID},
			RequesterID:      uuid.MustParse(s.user.ID),
		}

		err = s.Project.RemoveMemberFromProject(ctx, uuid.MustParse(org2.ID), removeOpts)
		s.Error(err)
		s.True(biz.IsNotFound(err))
	})

	s.Run("remove group from non-existent project", func() {
		nonExistentProjectID := uuid.New()
		invalidRef := &biz.IdentityReference{
			ID: &nonExistentProjectID,
		}
		removeOpts := &biz.RemoveMemberFromProjectOpts{
			ProjectReference: invalidRef,
			GroupReference:   &biz.IdentityReference{ID: &s.group.ID},
			RequesterID:      uuid.MustParse(s.user.ID),
		}

		err = s.Project.RemoveMemberFromProject(ctx, uuid.MustParse(s.org.ID), removeOpts)
		s.Error(err)
		s.True(biz.IsNotFound(err))
	})
}

// Test updating a user's role in a project
func (s *projectMembersIntegrationTestSuite) TestUpdateUserRoleInProject() {
	ctx := context.Background()

	// Create additional users
	user1, err := s.User.UpsertByEmail(ctx, "update-role-user1@example.com", nil)
	require.NoError(s.T(), err)

	user2, err := s.User.UpsertByEmail(ctx, "update-role-user2@example.com", nil)
	require.NoError(s.T(), err)

	// Add users to organization
	_, err = s.Membership.Create(ctx, s.org.ID, user1.ID)
	require.NoError(s.T(), err)
	_, err = s.Membership.Create(ctx, s.org.ID, user2.ID)
	require.NoError(s.T(), err)

	projectID := s.project.ID
	projectRef := &biz.IdentityReference{
		ID: &projectID,
	}

	// Add users to the project with initial roles
	opts1 := &biz.AddMemberToProjectOpts{
		ProjectReference: projectRef,
		UserEmail:        "update-role-user1@example.com",
		RequesterID:      uuid.MustParse(s.user.ID),
		Role:             authz.RoleProjectViewer,
	}
	_, err = s.Project.AddMemberToProject(ctx, uuid.MustParse(s.org.ID), opts1)
	require.NoError(s.T(), err)

	opts2 := &biz.AddMemberToProjectOpts{
		ProjectReference: projectRef,
		UserEmail:        "update-role-user2@example.com",
		RequesterID:      uuid.MustParse(s.user.ID),
		Role:             authz.RoleProjectAdmin,
	}
	_, err = s.Project.AddMemberToProject(ctx, uuid.MustParse(s.org.ID), opts2)
	require.NoError(s.T(), err)

	s.Run("update user role from viewer to admin", func() {
		// Update user1's role from viewer to admin
		updateOpts := &biz.UpdateMemberRoleOpts{
			ProjectReference: projectRef,
			UserEmail:        "update-role-user1@example.com",
			RequesterID:      uuid.MustParse(s.user.ID),
			NewRole:          authz.RoleProjectAdmin,
		}

		err := s.Project.UpdateMemberRole(ctx, uuid.MustParse(s.org.ID), updateOpts)
		s.NoError(err)

		// Verify the role was updated by listing members
		members, _, err := s.Project.ListMembers(ctx, uuid.MustParse(s.org.ID), projectRef, nil)
		s.NoError(err)

		// Find user1 in the members list and check their role
		var user1Member *biz.ProjectMembership
		for _, member := range members {
			if member.User.ID == user1.ID {
				user1Member = member
				break
			}
		}

		s.NotNil(user1Member, "User1 membership should exist")
		s.Equal(authz.RoleProjectAdmin, user1Member.Role)
	})

	s.Run("update user role from admin to viewer", func() {
		// Update user2's role from admin to viewer
		updateOpts := &biz.UpdateMemberRoleOpts{
			ProjectReference: projectRef,
			UserEmail:        "update-role-user2@example.com",
			RequesterID:      uuid.MustParse(s.user.ID),
			NewRole:          authz.RoleProjectViewer,
		}

		err := s.Project.UpdateMemberRole(ctx, uuid.MustParse(s.org.ID), updateOpts)
		s.NoError(err)

		// Verify the role was updated
		members, _, err := s.Project.ListMembers(ctx, uuid.MustParse(s.org.ID), projectRef, nil)
		s.NoError(err)

		// Find user2 in the members list and check their role
		var user2Member *biz.ProjectMembership
		for _, member := range members {
			if member.User.ID == user2.ID {
				user2Member = member
				break
			}
		}

		s.NotNil(user2Member, "User2 membership should exist")
		s.Equal(authz.RoleProjectViewer, user2Member.Role)
	})

	s.Run("update role with project name reference", func() {
		// Use project name instead of ID
		projectName := s.project.Name
		nameRef := &biz.IdentityReference{
			Name: &projectName,
		}

		updateOpts := &biz.UpdateMemberRoleOpts{
			ProjectReference: nameRef,
			UserEmail:        "update-role-user1@example.com",
			RequesterID:      uuid.MustParse(s.user.ID),
			NewRole:          authz.RoleProjectViewer, // Back to viewer
		}

		err := s.Project.UpdateMemberRole(ctx, uuid.MustParse(s.org.ID), updateOpts)
		s.NoError(err)

		// Verify the role was updated
		members, _, err := s.Project.ListMembers(ctx, uuid.MustParse(s.org.ID), projectRef, nil)
		s.NoError(err)

		// Find user1 in the members list and check their role
		var user1Member *biz.ProjectMembership
		for _, member := range members {
			if member.User.ID == user1.ID {
				user1Member = member
				break
			}
		}

		s.NotNil(user1Member, "User1 membership should exist")
		s.Equal(authz.RoleProjectViewer, user1Member.Role)
	})

	s.Run("no-op if role is already the requested role", func() {
		// Try to update to the same role
		updateOpts := &biz.UpdateMemberRoleOpts{
			ProjectReference: projectRef,
			UserEmail:        "update-role-user1@example.com",
			RequesterID:      uuid.MustParse(s.user.ID),
			NewRole:          authz.RoleProjectViewer, // Already a viewer from previous test
		}

		err := s.Project.UpdateMemberRole(ctx, uuid.MustParse(s.org.ID), updateOpts)
		s.NoError(err) // Should succeed but do nothing
	})

	s.Run("try to update role for non-existent user", func() {
		updateOpts := &biz.UpdateMemberRoleOpts{
			ProjectReference: projectRef,
			UserEmail:        "non-existent@example.com",
			RequesterID:      uuid.MustParse(s.user.ID),
			NewRole:          authz.RoleProjectAdmin,
		}

		err := s.Project.UpdateMemberRole(ctx, uuid.MustParse(s.org.ID), updateOpts)
		s.Error(err)
		s.Contains(err.Error(), "not a member of the organization")
	})

	s.Run("try to update role for user not in project", func() {
		// Create a user in the organization but not in the project
		nonMemberUser, err := s.User.UpsertByEmail(ctx, "non-project-member@example.com", nil)
		require.NoError(s.T(), err)
		_, err = s.Membership.Create(ctx, s.org.ID, nonMemberUser.ID)
		require.NoError(s.T(), err)

		updateOpts := &biz.UpdateMemberRoleOpts{
			ProjectReference: projectRef,
			UserEmail:        "non-project-member@example.com",
			RequesterID:      uuid.MustParse(s.user.ID),
			NewRole:          authz.RoleProjectAdmin,
		}

		err = s.Project.UpdateMemberRole(ctx, uuid.MustParse(s.org.ID), updateOpts)
		s.Error(err)
		s.Contains(err.Error(), "user is not a member of this project")
	})

	s.Run("try to update role with invalid role", func() {
		// This test is tricky since we're constrained by the type system,
		// but in a real API this would be a possible error case
		// We'll use a non-project role
		updateOpts := &biz.UpdateMemberRoleOpts{
			ProjectReference: projectRef,
			UserEmail:        "update-role-user1@example.com",
			RequesterID:      uuid.MustParse(s.user.ID),
			NewRole:          authz.Role("invalid-role"),
		}

		err := s.Project.UpdateMemberRole(ctx, uuid.MustParse(s.org.ID), updateOpts)
		s.Error(err)
		s.Contains(err.Error(), "role must be either")
	})

	s.Run("try to update role in non-existent project", func() {
		nonExistentProjectID := uuid.New()
		invalidRef := &biz.IdentityReference{
			ID: &nonExistentProjectID,
		}

		updateOpts := &biz.UpdateMemberRoleOpts{
			ProjectReference: invalidRef,
			UserEmail:        "update-role-user1@example.com",
			RequesterID:      uuid.MustParse(s.user.ID),
			NewRole:          authz.RoleProjectAdmin,
		}

		err := s.Project.UpdateMemberRole(ctx, uuid.MustParse(s.org.ID), updateOpts)
		s.Error(err)
		s.True(biz.IsNotFound(err))
	})
}

// Test updating a group's role in a project
func (s *projectGroupMembersIntegrationTestSuite) TestUpdateGroupRoleInProject() {
	ctx := context.Background()

	projectID := s.project.ID
	projectRef := &biz.IdentityReference{
		ID: &projectID,
	}

	// Create additional groups
	group1, err := s.Group.Create(ctx, uuid.MustParse(s.org.ID), "update-role-group1", "Group 1 for role updates", s.userUUID)
	require.NoError(s.T(), err)

	group2, err := s.Group.Create(ctx, uuid.MustParse(s.org.ID), "update-role-group2", "Group 2 for role updates", s.userUUID)
	require.NoError(s.T(), err)

	// Add groups to the project with initial roles
	opts1 := &biz.AddMemberToProjectOpts{
		ProjectReference: projectRef,
		GroupReference:   &biz.IdentityReference{ID: &group1.ID},
		RequesterID:      uuid.MustParse(s.user.ID),
		Role:             authz.RoleProjectViewer,
	}
	_, err = s.Project.AddMemberToProject(ctx, uuid.MustParse(s.org.ID), opts1)
	require.NoError(s.T(), err)

	opts2 := &biz.AddMemberToProjectOpts{
		ProjectReference: projectRef,
		GroupReference:   &biz.IdentityReference{ID: &group2.ID},
		RequesterID:      uuid.MustParse(s.user.ID),
		Role:             authz.RoleProjectAdmin,
	}
	_, err = s.Project.AddMemberToProject(ctx, uuid.MustParse(s.org.ID), opts2)
	require.NoError(s.T(), err)

	s.Run("update group role from viewer to admin", func() {
		// Update group1's role from viewer to admin
		updateOpts := &biz.UpdateMemberRoleOpts{
			ProjectReference: projectRef,
			GroupReference:   &biz.IdentityReference{ID: &group1.ID},
			RequesterID:      uuid.MustParse(s.user.ID),
			NewRole:          authz.RoleProjectAdmin,
		}

		err := s.Project.UpdateMemberRole(ctx, uuid.MustParse(s.org.ID), updateOpts)
		s.NoError(err)

		// Verify the role was updated by listing members
		members, _, err := s.Project.ListMembers(ctx, uuid.MustParse(s.org.ID), projectRef, nil)
		s.NoError(err)

		// Find group1 in the members list and check its role
		var group1Member *biz.ProjectMembership
		for _, member := range members {
			if member.Group != nil && member.Group.ID == group1.ID {
				group1Member = member
				break
			}
		}

		s.NotNil(group1Member, "Group1 membership should exist")
		s.Equal(authz.RoleProjectAdmin, group1Member.Role)
	})

	s.Run("update group role from admin to viewer", func() {
		// Update group2's role from admin to viewer
		updateOpts := &biz.UpdateMemberRoleOpts{
			ProjectReference: projectRef,
			GroupReference:   &biz.IdentityReference{ID: &group2.ID},
			RequesterID:      uuid.MustParse(s.user.ID),
			NewRole:          authz.RoleProjectViewer,
		}

		err := s.Project.UpdateMemberRole(ctx, uuid.MustParse(s.org.ID), updateOpts)
		s.NoError(err)

		// Verify the role was updated by listing members
		members, _, err := s.Project.ListMembers(ctx, uuid.MustParse(s.org.ID), projectRef, nil)
		s.NoError(err)

		// Find group2 in the members list and check its role
		var group2Member *biz.ProjectMembership
		for _, member := range members {
			if member.Group != nil && member.Group.ID == group2.ID {
				group2Member = member
				break
			}
		}

		s.NotNil(group2Member, "Group2 membership should exist")
		s.Equal(authz.RoleProjectViewer, group2Member.Role)
	})

	s.Run("update role with group name reference", func() {
		// Use group name instead of ID for the reference
		groupNameRef := &biz.IdentityReference{
			Name: &group1.Name,
		}

		updateOpts := &biz.UpdateMemberRoleOpts{
			ProjectReference: projectRef,
			GroupReference:   groupNameRef,
			RequesterID:      uuid.MustParse(s.user.ID),
			NewRole:          authz.RoleProjectViewer, // Back to viewer
		}

		err := s.Project.UpdateMemberRole(ctx, uuid.MustParse(s.org.ID), updateOpts)
		s.NoError(err)

		// Verify the role was updated
		members, _, err := s.Project.ListMembers(ctx, uuid.MustParse(s.org.ID), projectRef, nil)
		s.NoError(err)

		// Find group1 in the members list and check its role
		var group1Member *biz.ProjectMembership
		for _, member := range members {
			if member.Group != nil && member.Group.ID == group1.ID {
				group1Member = member
				break
			}
		}

		s.NotNil(group1Member, "Group1 membership should exist")
		s.Equal(authz.RoleProjectViewer, group1Member.Role)
	})

	s.Run("no-op if role is already the requested role", func() {
		// Try to update to the same role
		updateOpts := &biz.UpdateMemberRoleOpts{
			ProjectReference: projectRef,
			GroupReference:   &biz.IdentityReference{ID: &group1.ID},
			RequesterID:      uuid.MustParse(s.user.ID),
			NewRole:          authz.RoleProjectViewer, // Already a viewer from previous test
		}

		err := s.Project.UpdateMemberRole(ctx, uuid.MustParse(s.org.ID), updateOpts)
		s.NoError(err) // Should succeed but do nothing
	})

	s.Run("try to update role for non-existent group", func() {
		nonExistentGroupID := uuid.New()
		updateOpts := &biz.UpdateMemberRoleOpts{
			ProjectReference: projectRef,
			GroupReference:   &biz.IdentityReference{ID: &nonExistentGroupID},
			RequesterID:      uuid.MustParse(s.user.ID),
			NewRole:          authz.RoleProjectAdmin,
		}

		err := s.Project.UpdateMemberRole(ctx, uuid.MustParse(s.org.ID), updateOpts)
		s.Error(err)
		s.Contains(err.Error(), "group is not a member of this project")
	})

	s.Run("try to update role for group not in project", func() {
		// Create a group in the organization but don't add it to the project
		nonMemberGroup, err := s.Group.Create(ctx, uuid.MustParse(s.org.ID), "non-project-group", "Group not in project", s.userUUID)
		require.NoError(s.T(), err)

		updateOpts := &biz.UpdateMemberRoleOpts{
			ProjectReference: projectRef,
			GroupReference:   &biz.IdentityReference{ID: &nonMemberGroup.ID},
			RequesterID:      uuid.MustParse(s.user.ID),
			NewRole:          authz.RoleProjectAdmin,
		}

		err = s.Project.UpdateMemberRole(ctx, uuid.MustParse(s.org.ID), updateOpts)
		s.Error(err)
		s.Contains(err.Error(), "group is not a member of this project")
	})

	s.Run("try to update role with invalid role", func() {
		// This test uses a non-project role
		updateOpts := &biz.UpdateMemberRoleOpts{
			ProjectReference: projectRef,
			GroupReference:   &biz.IdentityReference{ID: &group1.ID},
			RequesterID:      uuid.MustParse(s.user.ID),
			NewRole:          authz.Role("invalid-role"),
		}

		err := s.Project.UpdateMemberRole(ctx, uuid.MustParse(s.org.ID), updateOpts)
		s.Error(err)
		s.Contains(err.Error(), "role must be either")
	})

	s.Run("try to update role in non-existent project", func() {
		nonExistentProjectID := uuid.New()
		invalidRef := &biz.IdentityReference{
			ID: &nonExistentProjectID,
		}

		updateOpts := &biz.UpdateMemberRoleOpts{
			ProjectReference: invalidRef,
			GroupReference:   &biz.IdentityReference{ID: &group1.ID},
			RequesterID:      uuid.MustParse(s.user.ID),
			NewRole:          authz.RoleProjectAdmin,
		}

		err := s.Project.UpdateMemberRole(ctx, uuid.MustParse(s.org.ID), updateOpts)
		s.Error(err)
		s.True(biz.IsNotFound(err))
	})

	s.Run("requester not part of organization", func() {
		// Create a user who is not in any organization
		externalUser, err := s.User.UpsertByEmail(ctx, "external-group-update@example.com", nil)
		require.NoError(s.T(), err)

		// Try to update a group's role with an external user as requester
		updateOpts := &biz.UpdateMemberRoleOpts{
			ProjectReference: projectRef,
			GroupReference:   &biz.IdentityReference{ID: &group1.ID},
			RequesterID:      uuid.MustParse(externalUser.ID),
			NewRole:          authz.RoleProjectAdmin,
		}

		err = s.Project.UpdateMemberRole(ctx, uuid.MustParse(s.org.ID), updateOpts)
		s.Error(err)
		s.Contains(err.Error(), "requester is not a member of the organization")
	})
}

// Test adding non-existing users to projects through invitations
func (s *projectMembersIntegrationTestSuite) TestAddNonExistingMemberToProject() {
	ctx := context.Background()
	projectID := s.project.ID
	projectRef := &biz.IdentityReference{
		ID: &projectID,
	}

	s.Run("add non-existing member to project creates invitation", func() {
		nonExistingEmail := "non-existing-user@example.com"

		// Ensure user doesn't already exist
		_, err := s.Repos.Membership.FindByOrgIDAndUserEmail(ctx, uuid.MustParse(s.org.ID), nonExistingEmail)
		s.Error(err)
		s.True(biz.IsNotFound(err))

		opts := &biz.AddMemberToProjectOpts{
			ProjectReference: projectRef,
			UserEmail:        nonExistingEmail,
			RequesterID:      uuid.MustParse(s.user.ID),
			Role:             authz.RoleProjectViewer,
		}

		result, err := s.Project.AddMemberToProject(ctx, uuid.MustParse(s.org.ID), opts)
		s.NoError(err)
		s.NotNil(result)
		s.True(result.InvitationSent, "An invitation should be sent for non-existing users")
		s.Nil(result.Membership, "No membership should be created directly")

		// Verify an invitation was created
		invitations, err := s.OrgInvitation.ListByOrg(ctx, s.org.ID)
		s.NoError(err)
		s.GreaterOrEqual(len(invitations), 1)

		// Find the invitation for this email
		var foundInvitation *biz.OrgInvitation
		for _, inv := range invitations {
			if inv.ReceiverEmail == nonExistingEmail {
				foundInvitation = inv
				break
			}
		}

		s.NotNil(foundInvitation, "Should find an invitation for the email")

		// Verify the invitation has project context
		s.NotNil(foundInvitation.Context, "Invitation should have context")
		s.Equal(projectID, *foundInvitation.Context.ProjectIDToJoin)
		s.Equal(authz.RoleProjectViewer, foundInvitation.Context.ProjectRole)
		s.Equal(authz.RoleOrgMember, foundInvitation.Role)
	})

	s.Run("add already invited user returns error", func() {
		alreadyInvitedEmail := "already-invited@example.com"

		// First create an invitation
		_, err := s.OrgInvitation.Create(ctx, s.org.ID, s.user.ID, alreadyInvitedEmail)
		s.NoError(err)

		// Now try to add the same email to a project
		opts := &biz.AddMemberToProjectOpts{
			ProjectReference: projectRef,
			UserEmail:        alreadyInvitedEmail,
			RequesterID:      uuid.MustParse(s.user.ID),
			Role:             authz.RoleProjectAdmin,
		}

		_, err = s.Project.AddMemberToProject(ctx, uuid.MustParse(s.org.ID), opts)
		s.Error(err)
		s.True(biz.IsErrAlreadyExists(err), "Should get already exists error for invited users")
	})
}
