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
		foundGroup, err := s.Group.FindByOrgAndID(ctx, uuid.MustParse(s.org.ID), group.ID)
		s.NoError(err)
		s.NotNil(foundGroup)
		s.Equal(group.ID, foundGroup.ID)
		s.Equal(name, foundGroup.Name)
		s.Equal(groupDescription, foundGroup.Description)
	})

	s.Run("try to find in wrong organization", func() {
		org2, org2Err := s.Organization.CreateWithRandomName(ctx)
		require.NoError(s.T(), org2Err)

		_, expectedErr := s.Group.FindByOrgAndID(ctx, uuid.MustParse(org2.ID), group.ID)
		s.Error(expectedErr)
		s.True(biz.IsNotFound(expectedErr))
	})

	s.Run("try to find non-existent group", func() {
		_, err := s.Group.FindByOrgAndID(ctx, uuid.MustParse(s.org.ID), uuid.New())
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

	// Test updating the group
	s.Run("update description", func() {
		newDescription := "Updated description"
		descPtr := &newDescription

		updatedGroup, err := s.Group.Update(ctx, uuid.MustParse(s.org.ID), group.ID, descPtr, nil)

		s.NoError(err)
		s.NotNil(updatedGroup)
		s.Equal(newDescription, updatedGroup.Description)
		s.Equal(name, updatedGroup.Name) // Name should not change
	})

	s.Run("try to update in wrong organization", func() {
		org2, err := s.Organization.CreateWithRandomName(ctx)
		require.NoError(s.T(), err)

		newDescription := "Updated description"
		descPtr := &newDescription

		_, err = s.Group.Update(ctx, uuid.MustParse(org2.ID), group.ID, descPtr, nil)

		s.Error(err)
		s.True(biz.IsNotFound(err))
	})

	s.Run("try to update non-existent group", func() {
		nonExistentGroupID := uuid.New()
		newDescription := "Updated description for non-existent group"
		descPtr := &newDescription

		_, err := s.Group.Update(ctx, uuid.MustParse(s.org.ID), nonExistentGroupID, descPtr, nil)

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
		err := s.Group.SoftDelete(ctx, uuid.MustParse(s.org.ID), group.ID)
		s.NoError(err)

		// Try to find it after deletion
		_, err = s.Group.FindByOrgAndID(ctx, uuid.MustParse(s.org.ID), group.ID)
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

		err = s.Group.SoftDelete(ctx, uuid.MustParse(org2.ID), group.ID)
		s.Error(err)
		s.True(biz.IsNotFound(err))
	})

	s.Run("try to delete non-existent group", func() {
		err := s.Group.SoftDelete(ctx, uuid.MustParse(s.org.ID), uuid.New())
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

func (s *groupMembersIntegrationTestSuite) TearDownTest() {
	ctx := context.Background()
	// Clean up the database after each test
	_, _ = s.Data.DB.Group.Delete().Exec(ctx)
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
		members, count, err := s.Group.ListMembers(ctx, uuid.MustParse(s.org.ID), s.group.ID, nil, nil, nil)
		s.NoError(err)
		s.Equal(1, len(members))
		s.Equal(1, count)
		s.Equal(s.user.ID, members[0].User.ID)
		s.True(members[0].Maintainer)
	})

	// TODO: Add tests for adding members to groups once that functionality is implemented

	s.Run("filter members by maintainer status", func() {
		isTrue := true
		members, count, err := s.Group.ListMembers(ctx, uuid.MustParse(s.org.ID), s.group.ID, &isTrue, nil, nil)
		s.NoError(err)
		s.Equal(1, len(members))
		s.Equal(1, count)
		s.True(members[0].Maintainer)

		isFalse := false
		members, count, err = s.Group.ListMembers(ctx, uuid.MustParse(s.org.ID), s.group.ID, &isFalse, nil, nil)
		s.NoError(err)
		s.Equal(0, len(members))
		s.Equal(0, count)
	})

	s.Run("filter members by email", func() {
		email := s.user.Email
		members, count, err := s.Group.ListMembers(ctx, uuid.MustParse(s.org.ID), s.group.ID, nil, &email, nil)
		s.NoError(err)
		s.Equal(1, len(members))
		s.Equal(1, count)
		s.Equal(s.user.Email, members[0].User.Email)

		nonExistentEmail := "nonexistent@example.com"
		members, count, err = s.Group.ListMembers(ctx, uuid.MustParse(s.org.ID), s.group.ID, nil, &nonExistentEmail, nil)
		s.NoError(err)
		s.Equal(0, len(members))
		s.Equal(0, count)
	})

	s.Run("list members with pagination", func() {
		// TODO: Add more members to the group once that functionality is implemented

		paginationOpts, err := pagination.NewOffsetPaginationOpts(0, 1)
		require.NoError(s.T(), err)

		members, count, err := s.Group.ListMembers(ctx, uuid.MustParse(s.org.ID), s.group.ID, nil, nil, paginationOpts)
		s.NoError(err)
		s.Equal(1, len(members))
		s.Equal(1, count)
	})
}
