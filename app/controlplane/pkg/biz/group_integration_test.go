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
func TestGroupUseCase(t *testing.T) {
	suite.Run(t, new(groupIntegrationTestSuite))
	suite.Run(t, new(groupListIntegrationTestSuite))
	suite.Run(t, new(groupMembersIntegrationTestSuite))
}

// Utility struct to hold the base test suite
type groupIntegrationTestSuite struct {
	testhelpers.UseCasesEachTestSuite
	org  *biz.Organization
	user *biz.User
}

func (s *groupIntegrationTestSuite) SetupTest() {
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

// TearDown cleans up resources after all tests in the suite have completed
func (s *groupIntegrationTestSuite) TearDownTest() {
	ctx := context.Background()
	// Clean up any test groups created during testing
	_, _ = s.Data.DB.Group.Delete().Exec(ctx)
}

// Test creating groups
func (s *groupIntegrationTestSuite) TestCreate() {
	ctx := context.Background()
	localDescription := "A test group"
	testCases := []struct {
		name        string
		opts        *biz.CreateGroupOpts
		expectError bool
		errorMsg    string
	}{
		{
			name: "successful creation",
			opts: &biz.CreateGroupOpts{
				Name:        "test-group",
				Description: localDescription,
				UserID:      uuid.MustParse(s.user.ID),
			},
			expectError: false,
		},
		{
			name: "empty name",
			opts: &biz.CreateGroupOpts{
				Name:        "",
				Description: localDescription,
				UserID:      uuid.MustParse(s.user.ID),
			},
			expectError: true,
			errorMsg:    "name cannot be empty",
		},
		{
			name: "nil user ID",
			opts: &biz.CreateGroupOpts{
				Name:        "test-group",
				Description: localDescription,
				UserID:      uuid.Nil,
			},
			expectError: true,
			errorMsg:    "organization ID and user ID cannot be empty",
		},
	}

	for _, tc := range testCases {
		s.Run(tc.name, func() {
			group, err := s.Group.Create(ctx, uuid.MustParse(s.org.ID), tc.opts.Name, tc.opts.Description, tc.opts.UserID)

			if tc.expectError {
				s.Error(err)
				if tc.errorMsg != "" {
					s.Contains(err.Error(), tc.errorMsg)
				}
				return
			}

			s.NoError(err)
			s.NotNil(group)
			s.Equal(tc.opts.Name, group.Name)
			s.Equal(tc.opts.Description, group.Description)
			s.NotEmpty(group.ID)
			s.NotNil(group.CreatedAt)
			s.NotNil(group.Organization)
			s.Equal(s.org.ID, group.Organization.ID)
		})
	}
}

// Test creating duplicate groups
func (s *groupIntegrationTestSuite) TestCreateDuplicate() {
	ctx := context.Background()

	// Create initial group
	name := "duplicate-test-group"
	description := "This is a test group for duplicate tests"
	differentDescription := "Different description"

	group, err := s.Group.Create(ctx, uuid.MustParse(s.org.ID), name, description, uuid.MustParse(s.user.ID))
	s.NoError(err)
	s.NotNil(group)

	// Try to create another group with the same name
	_, err = s.Group.Create(ctx, uuid.MustParse(s.org.ID), name, differentDescription, uuid.MustParse(s.user.ID))
	s.Error(err)
	s.Contains(err.Error(), "duplicated")

	// Create a group with the same name in a different organization
	org2, err := s.Organization.CreateWithRandomName(ctx)
	require.NoError(s.T(), err)

	// Add user to second organization
	_, err = s.Membership.Create(ctx, org2.ID, s.user.ID)
	require.NoError(s.T(), err)

	// Should succeed because it's in a different organization
	group2, err := s.Group.Create(ctx, uuid.MustParse(org2.ID), name, description, uuid.MustParse(s.user.ID))
	s.NoError(err)
	s.NotNil(group2)
	s.Equal(name, group2.Name)
}

// Test finding groups by ID
func (s *groupIntegrationTestSuite) TestFindByID() {
	ctx := context.Background()

	// Create a group
	name := "test-find-group"
	groupDescription := "This is a test group for finding by ID"

	group, err := s.Group.Create(ctx, uuid.MustParse(s.org.ID), name, groupDescription, uuid.MustParse(s.user.ID))
	s.NoError(err)
	s.NotNil(group)

	// Test finding the group by ID
	s.Run("find existing group", func() {
		foundGroup, err := s.Group.Get(ctx, uuid.MustParse(s.org.ID), &biz.IdentityReference{
			ID: &group.ID,
		})
		s.NoError(err)
		s.NotNil(foundGroup)
		s.Equal(group.ID, foundGroup.ID)
		s.Equal(name, foundGroup.Name)
		s.Equal(groupDescription, foundGroup.Description)
	})

	s.Run("try to find in wrong organization", func() {
		org2, org2Err := s.Organization.CreateWithRandomName(ctx)
		require.NoError(s.T(), org2Err)

		_, expectedErr := s.Group.Get(ctx, uuid.MustParse(org2.ID), &biz.IdentityReference{
			ID: &group.ID,
		})
		s.Error(expectedErr)
		s.True(biz.IsNotFound(expectedErr))
	})

	s.Run("try to find non-existent group", func() {
		id := uuid.New() // Generate a new UUID for a non-existent group
		_, err := s.Group.Get(ctx, uuid.MustParse(s.org.ID), &biz.IdentityReference{
			ID: &id,
		})
		s.Error(err)
		s.True(biz.IsNotFound(err))
	})
}

// Test updating groups
func (s *groupIntegrationTestSuite) TestUpdate() {
	ctx := context.Background()

	// Create a group
	name := "test-update-group"
	description := "This is a test group for updating"

	group, err := s.Group.Create(ctx, uuid.MustParse(s.org.ID), name, description, uuid.MustParse(s.user.ID))
	s.NoError(err)
	s.NotNil(group)

	// Create a second group to test name uniqueness constraint
	secondGroupName := "existing-group-name"
	secondGroup, err := s.Group.Create(ctx, uuid.MustParse(s.org.ID), secondGroupName, "Second group description", uuid.MustParse(s.user.ID))
	s.NoError(err)
	s.NotNil(secondGroup)

	// Test updating the group
	s.Run("update description", func() {
		newDescription := "Updated description"

		updatedGroup, err := s.Group.Update(ctx, uuid.MustParse(s.org.ID), &biz.IdentityReference{
			ID: &group.ID,
		}, &biz.UpdateGroupOpts{
			NewDescription: &newDescription,
		})

		s.NoError(err)
		s.NotNil(updatedGroup)
		s.Equal(newDescription, updatedGroup.Description)
		s.Equal(name, updatedGroup.Name) // Name should not change
	})

	s.Run("try to update name to an existing group name", func() {
		// Try to update the first group's name to match the second group's name
		_, dupErr := s.Group.Update(ctx, uuid.MustParse(s.org.ID), &biz.IdentityReference{
			ID: &group.ID,
		}, &biz.UpdateGroupOpts{
			NewName: &secondGroupName,
		})

		s.Error(dupErr)
		s.True(biz.IsErrAlreadyExists(dupErr), "Expected an 'already exists' error")
		s.Contains(dupErr.Error(), "already exists", "Error should indicate name already exists")

		// Verify the group name wasn't changed
		unchangedGroup, err := s.Group.Get(ctx, uuid.MustParse(s.org.ID), &biz.IdentityReference{
			ID: &group.ID,
		})
		s.NoError(err)
		s.Equal(name, unchangedGroup.Name, "Group name should remain unchanged after failed update")
	})

	s.Run("try to update in wrong organization", func() {
		org2, err := s.Organization.CreateWithRandomName(ctx)
		require.NoError(s.T(), err)

		newDescription := "Updated description"

		_, err = s.Group.Update(ctx, uuid.MustParse(org2.ID), &biz.IdentityReference{
			ID: &group.ID,
		}, &biz.UpdateGroupOpts{
			NewDescription: &newDescription,
		})

		s.Error(err)
		s.True(biz.IsNotFound(err))
	})

	s.Run("try to update non-existent group", func() {
		nonExistentGroupID := uuid.New()
		newDescription := "Updated description for non-existent group"

		_, err := s.Group.Update(ctx, uuid.MustParse(s.org.ID), &biz.IdentityReference{
			ID: &nonExistentGroupID,
		}, &biz.UpdateGroupOpts{
			NewDescription: &newDescription,
		})

		s.Error(err)
		s.True(biz.IsNotFound(err))
	})
}

// Test soft deleting groups
func (s *groupIntegrationTestSuite) TestSoftDelete() {
	ctx := context.Background()

	// Create a group
	name := "test-delete-group"
	description := "This is a test group for deleting"

	group, err := s.Group.Create(ctx, uuid.MustParse(s.org.ID), name, description, uuid.MustParse(s.user.ID))
	s.NoError(err)
	s.NotNil(group)

	// Test deleting the group
	s.Run("delete existing group", func() {
		err := s.Group.Delete(ctx, uuid.MustParse(s.org.ID), &biz.IdentityReference{
			ID: &group.ID,
		})
		s.NoError(err)

		// Try to find it after deletion
		_, err = s.Group.Get(ctx, uuid.MustParse(s.org.ID), &biz.IdentityReference{
			ID: &group.ID,
		})
		s.Error(err)
		s.True(biz.IsNotFound(err))

		// We should be able to create a new group with the same name after deletion
		newGroup, err := s.Group.Create(ctx, uuid.MustParse(s.org.ID), name, description, uuid.MustParse(s.user.ID))
		s.NoError(err)
		s.NotNil(newGroup)
	})

	s.Run("try to delete in wrong organization", func() {
		org2, err := s.Organization.CreateWithRandomName(ctx)
		require.NoError(s.T(), err)

		group, err := s.Group.Create(ctx, uuid.MustParse(s.org.ID), "org-specific-group", description, uuid.MustParse(s.user.ID))
		s.NoError(err)

		err = s.Group.Delete(ctx, uuid.MustParse(org2.ID), &biz.IdentityReference{
			ID: &group.ID,
		})
		s.Error(err)
		s.True(biz.IsNotFound(err))
	})

	s.Run("try to delete non-existent group", func() {
		nonExistentGroupID := uuid.New()
		err := s.Group.Delete(ctx, uuid.MustParse(s.org.ID), &biz.IdentityReference{
			ID: &nonExistentGroupID,
		})
		s.Error(err)
		s.True(biz.IsNotFound(err))
	})
}

// Utility struct for listing tests
type groupListIntegrationTestSuite struct {
	testhelpers.UseCasesEachTestSuite
	org  *biz.Organization
	user *biz.User
}

func (s *groupListIntegrationTestSuite) SetupTest() {
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

// TearDown cleans up resources after all tests in the suite have completed
func (s *groupListIntegrationTestSuite) TearDownSubTest() {
	ctx := context.Background()
	// Clean up the database after each test
	_, _ = s.Data.DB.Group.Delete().Exec(ctx)
}

// Test listing groups with various filters
func (s *groupListIntegrationTestSuite) TestList() {
	ctx := context.Background()

	s.Run("no groups", func() {
		groups, count, err := s.Group.List(ctx, uuid.MustParse(s.org.ID), nil, nil)
		s.NoError(err)
		s.Empty(groups)
		s.Equal(0, count)
	})

	s.Run("list groups without filters", func() {
		// Create a few groups
		desc1 := "Description 1"
		_, err := s.Group.Create(ctx, uuid.MustParse(s.org.ID), "group-1", desc1, uuid.MustParse(s.user.ID))
		require.NoError(s.T(), err)

		desc2 := "Description 2"
		_, err = s.Group.Create(ctx, uuid.MustParse(s.org.ID), "group-2", desc2, uuid.MustParse(s.user.ID))
		require.NoError(s.T(), err)

		groups, count, err := s.Group.List(ctx, uuid.MustParse(s.org.ID), nil, nil)
		s.NoError(err)
		s.Equal(2, len(groups))
		s.Equal(2, count)
	})

	s.Run("list groups with name filter", func() {
		devDescription := "Development Team"
		opsDescription := "Operations Team"
		// Create groups with different names
		_, err := s.Group.Create(ctx, uuid.MustParse(s.org.ID), "dev-team", devDescription, uuid.MustParse(s.user.ID))
		require.NoError(s.T(), err)
		_, err = s.Group.Create(ctx, uuid.MustParse(s.org.ID), "ops-team", opsDescription, uuid.MustParse(s.user.ID))
		require.NoError(s.T(), err)

		// Filter by name
		filterOpts := &biz.ListGroupOpts{Name: "dev"}
		groups, count, err := s.Group.List(ctx, uuid.MustParse(s.org.ID), filterOpts, nil)
		s.NoError(err)
		s.Equal(1, len(groups))
		s.Equal(1, count)
		s.Contains(groups[0].Name, "dev")
	})

	s.Run("list groups with description filter", func() {
		teamADescription := "This is the A team"
		teamBDescription := "This is the B team"
		// Create groups with different descriptions
		_, err := s.Group.Create(ctx, uuid.MustParse(s.org.ID), "team-a", teamADescription, uuid.MustParse(s.user.ID))
		require.NoError(s.T(), err)
		_, err = s.Group.Create(ctx, uuid.MustParse(s.org.ID), "team-b", teamBDescription, uuid.MustParse(s.user.ID))
		require.NoError(s.T(), err)

		// Filter by description
		filterOpts := &biz.ListGroupOpts{Description: "A team"}
		groups, count, err := s.Group.List(ctx, uuid.MustParse(s.org.ID), filterOpts, nil)
		s.NoError(err)
		s.Equal(1, len(groups))
		s.Equal(1, count)
		s.Contains(groups[0].Description, "A team")
	})

	s.Run("list groups with member email filter", func() {
		// Create a second user
		user2, err := s.User.UpsertByEmail(ctx, "user2@example.com", nil)
		require.NoError(s.T(), err)

		// Add user2 to organization
		_, err = s.Membership.Create(ctx, s.org.ID, user2.ID)
		require.NoError(s.T(), err)

		// Create a group with user as maintainer
		groupWithUser1 := "Group with user 1"
		_, err = s.Group.Create(ctx, uuid.MustParse(s.org.ID), "group-with-user1", groupWithUser1, uuid.MustParse(s.user.ID))
		require.NoError(s.T(), err)

		// Create a group with user2 as maintainer
		groupWithUser2 := "Group with user 2"
		group2, err := s.Group.Create(ctx, uuid.MustParse(s.org.ID), "group-with-user2", groupWithUser2, uuid.MustParse(user2.ID))
		require.NoError(s.T(), err)

		// Filter by member email
		filterOpts := &biz.ListGroupOpts{MemberEmail: "user2@example.com"}
		groups, count, err := s.Group.List(ctx, uuid.MustParse(s.org.ID), filterOpts, nil)
		s.NoError(err)
		s.Equal(1, len(groups))
		s.Equal(1, count)
		s.Equal(group2.ID, groups[0].ID)
	})

	s.Run("list groups with pagination", func() {
		// Create several groups
		for i := 1; i <= 5; i++ {
			lDescription := fmt.Sprintf("Description %d", i)
			_, err := s.Group.Create(ctx, uuid.MustParse(s.org.ID), fmt.Sprintf("the-group-%d", i), lDescription, uuid.MustParse(s.user.ID))
			require.NoError(s.T(), err)
		}

		// Test with offset pagination
		paginationOpts, err := pagination.NewOffsetPaginationOpts(0, 2)
		require.NoError(s.T(), err)

		groups, count, err := s.Group.List(ctx, uuid.MustParse(s.org.ID), nil, paginationOpts)
		s.NoError(err)
		s.Equal(2, len(groups))
		s.Equal(5, count) // Total count should be 5

		// Get the next page
		paginationOpts, err = pagination.NewOffsetPaginationOpts(2, 2)
		require.NoError(s.T(), err)

		groups, count, err = s.Group.List(ctx, uuid.MustParse(s.org.ID), nil, paginationOpts)
		s.NoError(err)
		s.Equal(2, len(groups))
		s.Equal(5, count)
	})
}

// Utility struct for group members tests
type groupMembersIntegrationTestSuite struct {
	testhelpers.UseCasesEachTestSuite
	org   *biz.Organization
	user  *biz.User
	group *biz.Group
}

func (s *groupMembersIntegrationTestSuite) SetupTest() {
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

	// Create a group for membership tests
	membersDescription := "Group for testing members"
	s.group, err = s.Group.Create(ctx, uuid.MustParse(s.org.ID), "test-members-group", membersDescription, uuid.MustParse(s.user.ID))
	assert.NoError(err)
}

// Test group membership operations
func (s *groupMembersIntegrationTestSuite) TestListMembers() {
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

	s.Run("initial group has creator as maintainer", func() {
		groupID := &s.group.ID
		opts := &biz.ListMembersOpts{
			IdentityReference: &biz.IdentityReference{
				ID: groupID,
			},
		}
		members, count, err := s.Group.ListMembers(ctx, uuid.MustParse(s.org.ID), opts, nil)
		s.NoError(err)
		s.Equal(1, len(members))
		s.Equal(1, count)
		s.Equal(s.user.ID, members[0].User.ID)
		s.True(members[0].Maintainer)
	})

	s.Run("filter members by maintainer status", func() {
		groupID := &s.group.ID
		isTrue := true
		opts := &biz.ListMembersOpts{
			IdentityReference: &biz.IdentityReference{
				ID: groupID,
			},
			Maintainers: &isTrue,
		}
		members, count, err := s.Group.ListMembers(ctx, uuid.MustParse(s.org.ID), opts, nil)
		s.NoError(err)
		s.Equal(1, len(members))
		s.Equal(1, count)
		s.True(members[0].Maintainer)

		isFalse := false
		opts.Maintainers = &isFalse
		members, count, err = s.Group.ListMembers(ctx, uuid.MustParse(s.org.ID), opts, nil)
		s.NoError(err)
		s.Equal(0, len(members))
		s.Equal(0, count)
	})

	s.Run("filter members by email", func() {
		groupID := &s.group.ID
		email := s.user.Email
		opts := &biz.ListMembersOpts{
			IdentityReference: &biz.IdentityReference{
				ID: groupID,
			},
			MemberEmail: &email,
		}
		members, count, err := s.Group.ListMembers(ctx, uuid.MustParse(s.org.ID), opts, nil)
		s.NoError(err)
		s.Equal(1, len(members))
		s.Equal(1, count)
		s.Equal(s.user.Email, members[0].User.Email)

		nonExistentEmail := "nonexistent@example.com"
		opts.MemberEmail = &nonExistentEmail
		members, count, err = s.Group.ListMembers(ctx, uuid.MustParse(s.org.ID), opts, nil)
		s.NoError(err)
		s.Equal(0, len(members))
		s.Equal(0, count)
	})

	s.Run("list members with pagination", func() {
		// TODO: Add more members to the group once that functionality is implemented
		groupID := &s.group.ID
		opts := &biz.ListMembersOpts{
			IdentityReference: &biz.IdentityReference{
				ID: groupID,
			},
		}
		paginationOpts, err := pagination.NewOffsetPaginationOpts(0, 1)
		require.NoError(s.T(), err)

		members, count, err := s.Group.ListMembers(ctx, uuid.MustParse(s.org.ID), opts, paginationOpts)
		s.NoError(err)
		s.Equal(1, len(members))
		s.Equal(1, count)
	})

	s.Run("list members with group name", func() {
		groupName := s.group.Name
		opts := &biz.ListMembersOpts{
			IdentityReference: &biz.IdentityReference{
				Name: &groupName,
			},
		}
		members, count, err := s.Group.ListMembers(ctx, uuid.MustParse(s.org.ID), opts, nil)
		s.NoError(err)
		s.Equal(1, len(members))
		s.Equal(1, count)
		s.Equal(s.user.ID, members[0].User.ID)
	})
}

// Test adding members to groups
func (s *groupMembersIntegrationTestSuite) TestAddMemberToGroup() {
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

	s.Run("add member using group ID", func() {
		// Add user2 as a regular member
		// Create options for adding member
		opts := &biz.AddMemberToGroupOpts{
			IdentityReference: &biz.IdentityReference{
				ID: &s.group.ID,
			},
			UserEmail:   "add-user2@example.com",
			RequesterID: uuid.MustParse(s.user.ID), // The creator is a maintainer
			Maintainer:  false,
		}

		membership, err := s.Group.AddMemberToGroup(ctx, uuid.MustParse(s.org.ID), opts)
		s.NoError(err)
		s.NotNil(membership)
		s.Equal(user2.ID, membership.Membership.User.ID)
		s.False(membership.Membership.Maintainer)

		// Verify the member was added by listing members
		members, count, err := s.Group.ListMembers(ctx, uuid.MustParse(s.org.ID), &biz.ListMembersOpts{
			IdentityReference: &biz.IdentityReference{
				ID: &s.group.ID,
			},
		}, nil)
		s.NoError(err)
		s.Equal(2, len(members))
		s.Equal(2, count)
	})

	s.Run("add member using group name", func() {
		// Add user3 as a maintainer
		groupName := s.group.Name
		opts := &biz.AddMemberToGroupOpts{
			IdentityReference: &biz.IdentityReference{
				Name: &groupName,
			},
			UserEmail:   "add-user3@example.com",
			RequesterID: uuid.MustParse(s.user.ID), // The creator is a maintainer
			Maintainer:  true,
		}

		membership, err := s.Group.AddMemberToGroup(ctx, uuid.MustParse(s.org.ID), opts)
		s.NoError(err)
		s.NotNil(membership)
		s.Equal(user3.ID, membership.Membership.User.ID)
		s.True(membership.Membership.Maintainer)

		// Verify the member was added by listing members
		members, count, err := s.Group.ListMembers(ctx, uuid.MustParse(s.org.ID), &biz.ListMembersOpts{
			IdentityReference: &biz.IdentityReference{
				ID: &s.group.ID,
			},
		}, nil)
		s.NoError(err)
		s.Equal(3, len(members))
		s.Equal(3, count)
	})

	s.Run("add member to group in wrong organization", func() {
		// Create a new organization
		org2, err := s.Organization.CreateWithRandomName(ctx)
		require.NoError(s.T(), err)

		// Attempt to add user2 to a group in the wrong organization
		opts := &biz.AddMemberToGroupOpts{
			IdentityReference: &biz.IdentityReference{
				ID: &s.group.ID,
			},
			UserEmail:   "add-user2@example.com",
			RequesterID: uuid.MustParse(s.user.ID),
			Maintainer:  false,
		}

		_, err = s.Group.AddMemberToGroup(ctx, uuid.MustParse(org2.ID), opts)
		s.Error(err)
		s.True(biz.IsNotFound(err))
	})

	s.Run("add member to non-existent group", func() {
		nonExistentGroupID := uuid.New()
		opts := &biz.AddMemberToGroupOpts{
			IdentityReference: &biz.IdentityReference{
				ID: &nonExistentGroupID,
			},
			UserEmail:   "add-user2@example.com",
			RequesterID: uuid.MustParse(s.user.ID),
			Maintainer:  false,
		}

		_, err := s.Group.AddMemberToGroup(ctx, uuid.MustParse(s.org.ID), opts)
		s.Error(err)
		s.True(biz.IsNotFound(err))
	})

	s.Run("add member who is not in the organization", func() {
		// Create user who is not in the organization
		_, err := s.User.UpsertByEmail(ctx, "not-in-org@example.com", nil)
		require.NoError(s.T(), err)
		// Note: not adding this user to the organization

		opts := &biz.AddMemberToGroupOpts{
			IdentityReference: &biz.IdentityReference{
				ID: &s.group.ID,
			},
			UserEmail:   "not-in-org@example.com",
			RequesterID: uuid.MustParse(s.user.ID),
			Maintainer:  false,
		}

		result, err := s.Group.AddMemberToGroup(ctx, uuid.MustParse(s.org.ID), opts)
		s.NoError(err)
		s.NotNil(result)
		s.True(result.InvitationSent, "Expected an invitation to be sent")
		s.Nil(result.Membership, "No membership should be created")

		// Verify that an invitation was created with the proper context
		invitations, err := s.OrgInvitation.ListByOrg(ctx, s.org.ID)
		s.NoError(err)
		s.GreaterOrEqual(len(invitations), 1, "Expected at least one invitation")

		// Find the invitation for our user
		var found bool
		for _, inv := range invitations {
			if inv.ReceiverEmail == "not-in-org@example.com" {
				found = true
				s.Equal(biz.OrgInvitationStatusPending, inv.Status)
				s.Equal(string(authz.RoleOrgMember), string(inv.Role))

				// Verify the invitation context
				s.NotNil(inv.Context, "Invitation context should not be nil")
				s.Equal(s.group.ID.String(), inv.Context.GroupIDToJoin.String(), "Group ID should match")
				s.Equal(opts.Maintainer, inv.Context.GroupMaintainer, "Maintainer status should match")
				break
			}
		}
		s.True(found, "Expected to find invitation for not-in-org@example.com")
	})

	s.Run("add member who is already invited", func() {
		// Create a user that will only be invited but not added to the organization
		userEmail := "already-invited@example.com"
		_, err := s.User.UpsertByEmail(ctx, userEmail, nil)
		require.NoError(s.T(), err)
		// Note: not adding this user to the organization

		// First invitation
		opts := &biz.AddMemberToGroupOpts{
			IdentityReference: &biz.IdentityReference{
				ID: &s.group.ID,
			},
			UserEmail:   userEmail,
			RequesterID: uuid.MustParse(s.user.ID),
			Maintainer:  false,
		}

		// First invitation should succeed
		result, err := s.Group.AddMemberToGroup(ctx, uuid.MustParse(s.org.ID), opts)
		s.NoError(err)
		s.NotNil(result)
		s.True(result.InvitationSent, "Expected an invitation to be sent")
		s.Nil(result.Membership, "No membership should be created")

		// Get initial invitation count
		invitations, err := s.OrgInvitation.ListByOrg(ctx, s.org.ID)
		s.NoError(err)
		initialInvitationCount := len(invitations)

		// Count invitations for this specific email
		emailInvitationCount := 0
		for _, inv := range invitations {
			if inv.ReceiverEmail == userEmail {
				emailInvitationCount++
			}
		}
		s.Equal(1, emailInvitationCount, "Should have exactly one invitation for the user initially")

		// Attempt to invite the same user again
		result2, err := s.Group.AddMemberToGroup(ctx, uuid.MustParse(s.org.ID), opts)
		s.Error(err)
		s.Nil(result2)

		// Verify that no additional invitation was created
		invitationsAfter, err := s.OrgInvitation.ListByOrg(ctx, s.org.ID)
		s.NoError(err)
		s.Equal(initialInvitationCount, len(invitationsAfter), "Should not create another invitation")

		// Verify that we still only have one invitation for this email
		emailInvitationCountAfter := 0
		for _, inv := range invitationsAfter {
			if inv.ReceiverEmail == userEmail {
				emailInvitationCountAfter++
			}
		}
		s.Equal(1, emailInvitationCountAfter, "Should still have exactly one invitation for the user")
	})

	s.Run("add member who is already in the group", func() {
		// Try to add user2 again (who we added in the first test)
		opts := &biz.AddMemberToGroupOpts{
			IdentityReference: &biz.IdentityReference{
				ID: &s.group.ID,
			},
			UserEmail:   "add-user2@example.com",
			RequesterID: uuid.MustParse(s.user.ID),
			Maintainer:  true,
		}

		_, err := s.Group.AddMemberToGroup(ctx, uuid.MustParse(s.org.ID), opts)
		s.Error(err)
		s.True(biz.IsErrAlreadyExists(err))

		// Verify the number of members hasn't changed
		_, count, err := s.Group.ListMembers(ctx, uuid.MustParse(s.org.ID), &biz.ListMembersOpts{
			IdentityReference: &biz.IdentityReference{
				ID: &s.group.ID,
			},
		}, nil)
		s.NoError(err)
		s.Equal(3, count) // still the original 3 members
	})
}

// Test removing members from groups
func (s *groupMembersIntegrationTestSuite) TestRemoveMemberFromGroup() {
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

	// Add users to the group
	opts1 := &biz.AddMemberToGroupOpts{
		IdentityReference: &biz.IdentityReference{
			ID: &s.group.ID,
		},
		UserEmail:   "remove-user2@example.com",
		RequesterID: uuid.MustParse(s.user.ID),
		Maintainer:  false,
	}
	_, err = s.Group.AddMemberToGroup(ctx, uuid.MustParse(s.org.ID), opts1)
	require.NoError(s.T(), err)

	opts2 := &biz.AddMemberToGroupOpts{
		IdentityReference: &biz.IdentityReference{
			ID: &s.group.ID,
		},
		UserEmail:   "remove-user3@example.com",
		RequesterID: uuid.MustParse(s.user.ID),
		Maintainer:  true,
	}
	_, err = s.Group.AddMemberToGroup(ctx, uuid.MustParse(s.org.ID), opts2)
	require.NoError(s.T(), err)

	opts3 := &biz.AddMemberToGroupOpts{
		IdentityReference: &biz.IdentityReference{
			ID: &s.group.ID,
		},
		UserEmail:   "remove-user4@example.com",
		RequesterID: uuid.MustParse(s.user.ID),
		Maintainer:  false,
	}
	_, err = s.Group.AddMemberToGroup(ctx, uuid.MustParse(s.org.ID), opts3)
	require.NoError(s.T(), err)

	// Verify initial member count
	members, count, err := s.Group.ListMembers(ctx, uuid.MustParse(s.org.ID), &biz.ListMembersOpts{
		IdentityReference: &biz.IdentityReference{
			ID: &s.group.ID,
		},
	}, nil)
	s.NoError(err)
	s.Equal(4, len(members)) // creator + 3 added users
	s.Equal(4, count)

	s.Run("remove a regular member from group", func() {
		// Remove user2 (regular member)
		removeOpts := &biz.RemoveMemberFromGroupOpts{
			IdentityReference: &biz.IdentityReference{
				ID: &s.group.ID,
			},
			UserEmail:   "remove-user2@example.com",
			RequesterID: uuid.MustParse(s.user.ID), // Creator is a maintainer
		}

		err := s.Group.RemoveMemberFromGroup(ctx, uuid.MustParse(s.org.ID), removeOpts)
		s.NoError(err)

		// Verify member was removed
		members, count, err := s.Group.ListMembers(ctx, uuid.MustParse(s.org.ID), &biz.ListMembersOpts{
			IdentityReference: &biz.IdentityReference{
				ID: &s.group.ID,
			},
		}, nil)
		s.NoError(err)
		s.Equal(3, len(members))
		s.Equal(3, count)

		// Verify the removed user is not in the list
		for _, member := range members {
			s.NotEqual(user2.ID, member.User.ID)
		}
	})

	s.Run("remove a maintainer from group", func() {
		// Remove user3 (maintainer)
		removeOpts := &biz.RemoveMemberFromGroupOpts{
			IdentityReference: &biz.IdentityReference{
				ID: &s.group.ID,
			},
			UserEmail:   "remove-user3@example.com",
			RequesterID: uuid.MustParse(s.user.ID), // Creator is a maintainer
		}

		err := s.Group.RemoveMemberFromGroup(ctx, uuid.MustParse(s.org.ID), removeOpts)
		s.NoError(err)

		// Verify member was removed
		members, count, err := s.Group.ListMembers(ctx, uuid.MustParse(s.org.ID), &biz.ListMembersOpts{
			IdentityReference: &biz.IdentityReference{
				ID: &s.group.ID,
			},
		}, nil)
		s.NoError(err)
		s.Equal(2, len(members))
		s.Equal(2, count)

		// Check remaining members - user3 should not be present
		for _, member := range members {
			s.NotEqual(user3.ID, member.User.ID)
		}

		// Verify we still have at least one maintainer (the original creator)
		foundMaintainer := false
		for _, member := range members {
			if member.Maintainer {
				foundMaintainer = true
				break
			}
		}
		s.True(foundMaintainer, "Group should still have at least one maintainer")
	})

	s.Run("try to remove non-existent member", func() {
		// Create a user who's not in the group
		nonMemberUser, err := s.User.UpsertByEmail(ctx, "non-member@example.com", nil)
		require.NoError(s.T(), err)
		_, err = s.Membership.Create(ctx, s.org.ID, nonMemberUser.ID)
		require.NoError(s.T(), err)

		// Try to remove a user who's not in the group
		removeOpts := &biz.RemoveMemberFromGroupOpts{
			IdentityReference: &biz.IdentityReference{
				ID: &s.group.ID,
			},
			UserEmail:   "non-member@example.com",
			RequesterID: uuid.MustParse(s.user.ID),
		}

		err = s.Group.RemoveMemberFromGroup(ctx, uuid.MustParse(s.org.ID), removeOpts)
		s.Error(err)
		s.True(biz.IsErrValidation(err))

		// Member count should remain unchanged
		_, count, err := s.Group.ListMembers(ctx, uuid.MustParse(s.org.ID), &biz.ListMembersOpts{
			IdentityReference: &biz.IdentityReference{
				ID: &s.group.ID,
			},
		}, nil)
		s.NoError(err)
		s.Equal(2, count)
	})

	s.Run("remove member from wrong organization", func() {
		// Create a new organization and group
		org2, err := s.Organization.CreateWithRandomName(ctx)
		require.NoError(s.T(), err)

		// Try to remove user4 using the wrong org ID
		removeOpts := &biz.RemoveMemberFromGroupOpts{
			IdentityReference: &biz.IdentityReference{
				ID: &s.group.ID,
			},
			UserEmail:   "remove-user4@example.com",
			RequesterID: uuid.MustParse(s.user.ID),
		}

		err = s.Group.RemoveMemberFromGroup(ctx, uuid.MustParse(org2.ID), removeOpts)
		s.Error(err)
		s.True(biz.IsNotFound(err))

		// Member count should remain unchanged
		_, count, err := s.Group.ListMembers(ctx, uuid.MustParse(s.org.ID), &biz.ListMembersOpts{
			IdentityReference: &biz.IdentityReference{
				ID: &s.group.ID,
			},
		}, nil)
		s.NoError(err)
		s.Equal(2, count)
	})

	s.Run("remove member from non-existent group", func() {
		nonExistentGroupID := uuid.New()
		removeOpts := &biz.RemoveMemberFromGroupOpts{
			IdentityReference: &biz.IdentityReference{
				ID: &nonExistentGroupID,
			},
			UserEmail:   "remove-user4@example.com",
			RequesterID: uuid.MustParse(s.user.ID),
		}

		err = s.Group.RemoveMemberFromGroup(ctx, uuid.MustParse(s.org.ID), removeOpts)
		s.Error(err)
		s.True(biz.IsNotFound(err))
	})

	s.Run("requester not part of organization", func() {
		// Create a user who is not in any organization
		externalUser, err := s.User.UpsertByEmail(ctx, "external-user@example.com", nil)
		require.NoError(s.T(), err)

		// Try to remove a member with an external user as requester
		removeOpts := &biz.RemoveMemberFromGroupOpts{
			IdentityReference: &biz.IdentityReference{
				ID: &s.group.ID,
			},
			UserEmail:   "remove-user4@example.com",
			RequesterID: uuid.MustParse(externalUser.ID),
		}

		err = s.Group.RemoveMemberFromGroup(ctx, uuid.MustParse(s.org.ID), removeOpts)
		s.Error(err)
		s.Contains(err.Error(), "requester is not a member of the organization")
	})

	s.Run("non-existent user email", func() {
		// Try to remove a non-existent user
		removeOpts := &biz.RemoveMemberFromGroupOpts{
			IdentityReference: &biz.IdentityReference{
				ID: &s.group.ID,
			},
			UserEmail:   "non-existent-user@example.com",
			RequesterID: uuid.MustParse(s.user.ID),
		}

		err = s.Group.RemoveMemberFromGroup(ctx, uuid.MustParse(s.org.ID), removeOpts)
		s.Error(err)
		s.Contains(err.Error(), "not a member of the organization")
	})
}

// Test checking group member count
func (s *groupMembersIntegrationTestSuite) TestGroupMemberCount() {
	ctx := context.Background()

	// Create additional users
	user2, err := s.User.UpsertByEmail(ctx, "count-user2@example.com", nil)
	require.NoError(s.T(), err)

	user3, err := s.User.UpsertByEmail(ctx, "count-user3@example.com", nil)
	require.NoError(s.T(), err)

	// Add users to organization
	_, err = s.Membership.Create(ctx, s.org.ID, user2.ID)
	require.NoError(s.T(), err)
	_, err = s.Membership.Create(ctx, s.org.ID, user3.ID)
	require.NoError(s.T(), err)

	// Check initial member count
	initialGroup, err := s.Group.Get(ctx, uuid.MustParse(s.org.ID), &biz.IdentityReference{ID: &s.group.ID})
	s.NoError(err)
	s.NotNil(initialGroup)
	// Initial count should be 1 (just the creator)
	s.Equal(1, initialGroup.MemberCount, "Initial group should have 1 member (creator)")

	// Add a member and check count increases
	s.Run("member count increases when adding members", func() {
		// Add user2 as a regular member
		opts := &biz.AddMemberToGroupOpts{
			IdentityReference: &biz.IdentityReference{
				ID: &s.group.ID,
			},
			UserEmail:   "count-user2@example.com",
			RequesterID: uuid.MustParse(s.user.ID),
			Maintainer:  false,
		}

		membership, err := s.Group.AddMemberToGroup(ctx, uuid.MustParse(s.org.ID), opts)
		s.NoError(err)
		s.NotNil(membership)

		// Check member count after adding one user
		groupAfterAdd1, err := s.Group.Get(ctx, uuid.MustParse(s.org.ID), &biz.IdentityReference{ID: &s.group.ID})
		s.NoError(err)
		s.Equal(2, groupAfterAdd1.MemberCount, "Group should have 2 members after adding one")

		// Add user3 as a maintainer
		opts = &biz.AddMemberToGroupOpts{
			IdentityReference: &biz.IdentityReference{
				ID: &s.group.ID,
			},
			UserEmail:   "count-user3@example.com",
			RequesterID: uuid.MustParse(s.user.ID),
			Maintainer:  true,
		}

		membership, err = s.Group.AddMemberToGroup(ctx, uuid.MustParse(s.org.ID), opts)
		s.NoError(err)
		s.NotNil(membership)

		// Check member count after adding another user
		groupAfterAdd2, err := s.Group.Get(ctx, uuid.MustParse(s.org.ID), &biz.IdentityReference{ID: &s.group.ID})
		s.NoError(err)
		s.Equal(3, groupAfterAdd2.MemberCount, "Group should have 3 members after adding two")
	})

	// Remove a member and check count decreases
	s.Run("member count decreases when removing members", func() {
		// Remove user2
		removeOpts := &biz.RemoveMemberFromGroupOpts{
			IdentityReference: &biz.IdentityReference{
				ID: &s.group.ID,
			},
			UserEmail:   "count-user2@example.com",
			RequesterID: uuid.MustParse(s.user.ID),
		}

		err := s.Group.RemoveMemberFromGroup(ctx, uuid.MustParse(s.org.ID), removeOpts)
		s.NoError(err)

		// Check member count after removing one user
		groupAfterRemove1, err := s.Group.Get(ctx, uuid.MustParse(s.org.ID), &biz.IdentityReference{ID: &s.group.ID})
		s.NoError(err)
		s.Equal(2, groupAfterRemove1.MemberCount, "Group should have 2 members after removing one")

		// Remove user3
		removeOpts = &biz.RemoveMemberFromGroupOpts{
			IdentityReference: &biz.IdentityReference{
				ID: &s.group.ID,
			},
			UserEmail:   "count-user3@example.com",
			RequesterID: uuid.MustParse(s.user.ID),
		}

		err = s.Group.RemoveMemberFromGroup(ctx, uuid.MustParse(s.org.ID), removeOpts)
		s.NoError(err)

		// Check member count after removing another user
		groupAfterRemove2, err := s.Group.Get(ctx, uuid.MustParse(s.org.ID), &biz.IdentityReference{ID: &s.group.ID})
		s.NoError(err)
		s.Equal(1, groupAfterRemove2.MemberCount, "Group should have 1 member after removing two")
	})
}
