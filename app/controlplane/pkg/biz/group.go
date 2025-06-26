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

package biz

import (
	"context"
	"fmt"
	"time"

	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/auditor/events"
	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/pagination"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/google/uuid"
)

type GroupRepo interface {
	// List retrieves a list of groups in the organization, optionally filtered by name, description, and owner.
	List(ctx context.Context, orgID uuid.UUID, filterOpts *ListGroupOpts, paginationOpts *pagination.OffsetPaginationOpts) ([]*Group, int, error)
	// Create creates a new group.
	Create(ctx context.Context, orgID uuid.UUID, opts *CreateGroupOpts) (*Group, error)
	// Update updates an existing group.
	Update(ctx context.Context, orgID uuid.UUID, groupID uuid.UUID, opts *UpdateGroupOpts) (*Group, error)
	// FindByOrgAndID finds a group by its organization ID and group ID.
	FindByOrgAndID(ctx context.Context, orgID uuid.UUID, groupID uuid.UUID) (*Group, error)
	// SoftDelete soft-deletes a group by marking it as deleted.
	SoftDelete(ctx context.Context, orgID uuid.UUID, groupID uuid.UUID) error
	// ListMembers retrieves a list of members in a group, optionally filtered by maintainer status.
	ListMembers(ctx context.Context, orgID uuid.UUID, groupID uuid.UUID, opts *ListMembersOpts, paginationOpts *pagination.OffsetPaginationOpts) ([]*GroupMembership, int, error)
}

// GroupMembership represents a membership of a user in a group.
type GroupMembership struct {
	// User is the user who is a member of the group.
	User *User
	// Maintainer indicates if the user is a maintainer of the group.
	Maintainer bool
	// CreatedAt is the timestamp when the user was added to the group.
	CreatedAt *time.Time
	// UpdatedAt is the timestamp when the membership was last updated.
	UpdatedAt *time.Time
	// DeletedAt is the timestamp when the membership was deleted, if applicable.
	DeletedAt *time.Time
}

type Group struct {
	// ID is the unique identifier for the group.
	ID uuid.UUID
	// Name is the name of the group.
	Name string
	// The Description is a brief description of the group.
	Description string
	// Members is a list of group memberships, which includes the users who are members of the group.
	Members []*GroupMembership
	// Organization is the organization to which the group belongs.
	Organization *Organization
	// CreatedAt is the timestamp when the group was created.
	CreatedAt *time.Time
	// UpdatedAt is the timestamp when the group was last updated.
	UpdatedAt *time.Time
	// DeletedAt is the timestamp when the group was deleted, if applicable.
	DeletedAt *time.Time
}

type CreateGroupOpts struct {
	// Name is the name of the group.
	Name string
	// The description is a brief description of the group.
	Description string
	// UserID is the ID of the user who owns the group.
	UserID uuid.UUID
}

type UpdateGroupOpts struct {
	// Description is the new description of the group.
	Description *string
	// Name is the new name of the group.
	Name *string
}

type ListGroupOpts struct {
	// Name is the name of the group to filter by.
	Name string
	// Description is the description of the group to filter by.
	Description string
	// MemberEmail is the email of the member to filter by.
	MemberEmail string
}

// ListMembersOpts defines options for listing members of a group.
type ListMembersOpts struct {
	// Maintainers indicate whether to filter the members by their maintainer status.
	Maintainers *bool
	// MemberEmail is the email of the member to filter by.
	MemberEmail *string
}

type GroupUseCase struct {
	// logger is used to log messages.
	logger *log.Helper
	// Repositories
	groupRepo      GroupRepo
	membershipRepo MembershipRepo
	// Auditor use case for logging events
	auditorUC *AuditorUseCase
}

func NewGroupUseCase(logger log.Logger, groupRepo GroupRepo, membershipRepo MembershipRepo, auditorUC *AuditorUseCase) *GroupUseCase {
	return &GroupUseCase{
		logger:         log.NewHelper(log.With(logger, "component", "biz/group")),
		groupRepo:      groupRepo,
		membershipRepo: membershipRepo,
		auditorUC:      auditorUC,
	}
}

func (uc *GroupUseCase) List(ctx context.Context, orgID uuid.UUID, filterOpts *ListGroupOpts, paginationOpts *pagination.OffsetPaginationOpts) ([]*Group, int, error) {
	pgOpts := pagination.NewDefaultOffsetPaginationOpts()
	if paginationOpts != nil {
		pgOpts = paginationOpts
	}

	return uc.groupRepo.List(ctx, orgID, filterOpts, pgOpts)
}

func (uc *GroupUseCase) ListMembers(ctx context.Context, orgID uuid.UUID, groupID uuid.UUID, maintainers *bool, memberEmail *string, paginationOpts *pagination.OffsetPaginationOpts) ([]*GroupMembership, int, error) {
	pgOpts := pagination.NewDefaultOffsetPaginationOpts()
	if paginationOpts != nil {
		pgOpts = paginationOpts
	}

	return uc.groupRepo.ListMembers(ctx, orgID, groupID, &ListMembersOpts{Maintainers: maintainers, MemberEmail: memberEmail}, pgOpts)
}

// Create creates a new group in the organization.
func (uc *GroupUseCase) Create(ctx context.Context, orgID uuid.UUID, name string, description string, userID uuid.UUID) (*Group, error) {
	if name == "" {
		return nil, NewErrValidationStr("name cannot be empty")
	}

	if orgID == uuid.Nil || userID == uuid.Nil {
		return nil, NewErrValidationStr("organization ID and user ID cannot be empty")
	}

	// Check if the user is a member of the organization
	m, err := uc.membershipRepo.FindByOrgAndUser(ctx, orgID, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to find membership: %w", err)
	} else if m == nil {
		return nil, NewErrNotFound("membership")
	}

	group, err := uc.groupRepo.Create(ctx, orgID, &CreateGroupOpts{
		Name:        name,
		Description: description,
		UserID:      userID,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create group: %w", err)
	}

	// Dispatch event to the audit log for group creation
	uc.auditorUC.Dispatch(ctx, &events.GroupCreated{
		GroupBase: &events.GroupBase{
			GroupID:   &group.ID,
			GroupName: group.Name,
		},
		GroupDescription: description,
	}, &orgID)

	return group, nil
}

// Update updates an existing group in the organization.
func (uc *GroupUseCase) Update(ctx context.Context, orgID uuid.UUID, groupID uuid.UUID, description *string, name *string) (*Group, error) {
	if orgID == uuid.Nil || groupID == uuid.Nil {
		return nil, NewErrValidationStr("organization ID and group ID cannot be empty")
	}

	// Check the group exists
	existingGroup, err := uc.groupRepo.FindByOrgAndID(ctx, orgID, groupID)
	if err != nil {
		return nil, fmt.Errorf("failed to find group: %w", err)
	}

	if existingGroup == nil {
		return nil, NewErrNotFound("group")
	}

	updatedGroup, err := uc.groupRepo.Update(ctx, orgID, groupID, &UpdateGroupOpts{
		Description: description,
		Name:        name,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to update group: %w", err)
	}

	// Dispatch event to the audit log for group update
	event := &events.GroupUpdated{
		GroupBase: &events.GroupBase{
			GroupID:   &updatedGroup.ID,
			GroupName: updatedGroup.Name,
		},
		NewDescription: description,
	}

	// Add old and new name only if the name was changed
	if name != nil && existingGroup.Name != *name {
		event.OldName = &existingGroup.Name
		event.NewName = name
	}

	uc.auditorUC.Dispatch(ctx, event, &orgID)

	return updatedGroup, nil
}

// FindByOrgAndID retrieves a group by its organization ID and group ID.
func (uc *GroupUseCase) FindByOrgAndID(ctx context.Context, orgID uuid.UUID, groupID uuid.UUID) (*Group, error) {
	if orgID == uuid.Nil || groupID == uuid.Nil {
		return nil, NewErrValidationStr("organization ID and group ID cannot be empty")
	}

	group, err := uc.groupRepo.FindByOrgAndID(ctx, orgID, groupID)
	if err != nil {
		return nil, fmt.Errorf("failed to find group: %w", err)
	} else if group == nil {
		return nil, NewErrNotFound("group")
	}

	return group, nil
}

// SoftDelete marks a group as deleted by setting the DeletedAt timestamp.
func (uc *GroupUseCase) SoftDelete(ctx context.Context, orgID uuid.UUID, groupID uuid.UUID) error {
	if orgID == uuid.Nil || groupID == uuid.Nil {
		return NewErrValidationStr("organization ID and group ID cannot be empty")
	}

	// Check the group exists
	existingGroup, err := uc.groupRepo.FindByOrgAndID(ctx, orgID, groupID)
	if err != nil {
		return fmt.Errorf("failed to find group: %w", err)
	}

	if existingGroup == nil {
		return NewErrNotFound("group")
	}

	if err := uc.groupRepo.SoftDelete(ctx, orgID, groupID); err != nil {
		return fmt.Errorf("failed to soft-delete group: %w", err)
	}

	// Dispatch event to the audit log for group deletion
	uc.auditorUC.Dispatch(ctx, &events.GroupDeleted{
		GroupBase: &events.GroupBase{
			GroupID:   &existingGroup.ID,
			GroupName: existingGroup.Name,
		},
	}, &orgID)

	return nil
}
