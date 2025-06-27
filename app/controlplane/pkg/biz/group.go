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
	// FindGroupMembershipByGroupAndID finds a group membership by group ID and user ID.
	FindGroupMembershipByGroupAndID(ctx context.Context, groupID uuid.UUID, userID uuid.UUID) (*GroupMembership, error)
	// SoftDelete soft-deletes a group by marking it as deleted.
	SoftDelete(ctx context.Context, orgID uuid.UUID, groupID uuid.UUID) error
	// ListMembers retrieves a list of members in a group, optionally filtered by maintainer status.
	ListMembers(ctx context.Context, orgID uuid.UUID, groupID uuid.UUID, opts *ListMembersOpts, paginationOpts *pagination.OffsetPaginationOpts) ([]*GroupMembership, int, error)
	// AddMemberToGroup adds a user to a group, optionally specifying if they are a maintainer.
	AddMemberToGroup(ctx context.Context, orgID uuid.UUID, groupID uuid.UUID, userID uuid.UUID, maintainer bool) (*GroupMembership, error)
	// RemoveMemberFromGroup removes a user from a group.
	RemoveMemberFromGroup(ctx context.Context, orgID uuid.UUID, groupID uuid.UUID, userID uuid.UUID) error
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

// UpdateGroupOpts defines options for updating a group.
type UpdateGroupOpts struct {
	// NewDescription is the new description of the group.
	NewDescription *string
	// NewName is the new name of the group.
	NewName *string
}

// ListGroupOpts defines options for listing groups.
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
	*IdentityReference
	// Maintainers indicate whether to filter the members by their maintainer status.
	Maintainers *bool
	// MemberEmail is the email of the member to filter by.
	MemberEmail *string
}

// AddMemberToGroupOpts defines options for adding a member to a group.
type AddMemberToGroupOpts struct {
	*IdentityReference
	// UserEmail is the email of the user to add to the group.
	UserEmail string
	// RequesterID is the ID of the user who is requesting to add the member. Must be a maintainer.
	RequesterID uuid.UUID
	// Maintainer indicates if the new member should be a maintainer.
	Maintainer bool
}

// RemoveMemberFromGroupOpts defines options for removing a member from a group.
type RemoveMemberFromGroupOpts struct {
	*IdentityReference
	// UserEmail is the email of the user to remove from the group.
	UserEmail string
	// RequesterID is the ID of the user who is requesting to remove the member. Must be a maintainer.
	RequesterID uuid.UUID
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

// ListMembers retrieves a list of members in a group, optionally filtered by maintainer status and email.
func (uc *GroupUseCase) ListMembers(ctx context.Context, orgID uuid.UUID, opts *ListMembersOpts, paginationOpts *pagination.OffsetPaginationOpts) ([]*GroupMembership, int, error) {
	if opts == nil {
		return nil, 0, NewErrValidationStr("options cannot be nil")
	}

	resolvedGroupID, err := uc.ValidateGroupIdentifier(ctx, orgID, opts.ID, opts.Name)
	if err != nil {
		return nil, 0, err
	}

	pgOpts := pagination.NewDefaultOffsetPaginationOpts()
	if paginationOpts != nil {
		pgOpts = paginationOpts
	}

	return uc.groupRepo.ListMembers(ctx, orgID, resolvedGroupID, opts, pgOpts)
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

// Get retrieves a group by its organization ID and either group ID or group name.
func (uc *GroupUseCase) Get(ctx context.Context, orgID uuid.UUID, opts *IdentityReference) (*Group, error) {
	if opts == nil {
		return nil, NewErrValidationStr("options cannot be nil")
	}

	if orgID == uuid.Nil {
		return nil, NewErrValidationStr("organization ID cannot be empty")
	}

	resolvedGroupID, err := uc.ValidateGroupIdentifier(ctx, orgID, opts.ID, opts.Name)
	if err != nil {
		return nil, err
	}

	group, err := uc.groupRepo.FindByOrgAndID(ctx, orgID, resolvedGroupID)
	if err != nil {
		return nil, fmt.Errorf("failed to find group: %w", err)
	} else if group == nil {
		return nil, NewErrNotFound("group")
	}

	return group, nil
}

// Update updates an existing group in the organization using the provided options.
func (uc *GroupUseCase) Update(ctx context.Context, orgID uuid.UUID, idReference *IdentityReference, opts *UpdateGroupOpts) (*Group, error) {
	if opts == nil {
		return nil, NewErrValidationStr("options cannot be nil")
	}

	if orgID == uuid.Nil {
		return nil, NewErrValidationStr("organization ID cannot be empty")
	}

	resolvedGroupID, err := uc.ValidateGroupIdentifier(ctx, orgID, idReference.ID, idReference.Name)
	if err != nil {
		return nil, err
	}

	// Check the group exists
	existingGroup, err := uc.groupRepo.FindByOrgAndID(ctx, orgID, resolvedGroupID)
	if err != nil {
		return nil, fmt.Errorf("failed to find group: %w", err)
	}

	if existingGroup == nil {
		return nil, NewErrNotFound("group")
	}

	updatedGroup, err := uc.groupRepo.Update(ctx, orgID, resolvedGroupID, &UpdateGroupOpts{
		NewDescription: opts.NewDescription,
		NewName:        opts.NewName,
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
		NewDescription: opts.NewDescription,
	}

	// Add old and new name only if the name was changed
	if opts.NewName != nil && existingGroup.Name != *opts.NewName {
		event.OldName = &existingGroup.Name
		event.NewName = opts.NewName
	}

	uc.auditorUC.Dispatch(ctx, event, &orgID)

	return updatedGroup, nil
}

// Delete soft-deletes a group by marking it as deleted using the provided options.
func (uc *GroupUseCase) Delete(ctx context.Context, orgID uuid.UUID, opts *IdentityReference) error {
	if opts == nil {
		return NewErrValidationStr("options cannot be nil")
	}

	if orgID == uuid.Nil {
		return NewErrValidationStr("organization ID cannot be empty")
	}

	resolvedGroupID, err := uc.ValidateGroupIdentifier(ctx, orgID, opts.ID, opts.Name)
	if err != nil {
		return err
	}

	// Check the group exists
	existingGroup, err := uc.groupRepo.FindByOrgAndID(ctx, orgID, resolvedGroupID)
	if err != nil {
		return fmt.Errorf("failed to find group: %w", err)
	}

	if existingGroup == nil {
		return NewErrNotFound("group")
	}

	if err := uc.groupRepo.SoftDelete(ctx, orgID, resolvedGroupID); err != nil {
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

// AddMemberToGroup adds a user to a group.
// The requester must be either a maintainer of the group or have RoleOwner/RoleAdmin in the organization.
func (uc *GroupUseCase) AddMemberToGroup(ctx context.Context, orgID uuid.UUID, opts *AddMemberToGroupOpts) (*GroupMembership, error) {
	if opts == nil {
		return nil, NewErrValidationStr("options cannot be nil")
	}

	if orgID == uuid.Nil || opts.UserEmail == "" || opts.RequesterID == uuid.Nil {
		return nil, NewErrValidationStr("organization ID, user email, and requester ID cannot be empty")
	}

	resolvedGroupID, err := uc.ValidateGroupIdentifier(ctx, orgID, opts.ID, opts.Name)
	if err != nil {
		return nil, err
	}

	// Check the group exists
	existingGroup, err := uc.groupRepo.FindByOrgAndID(ctx, orgID, resolvedGroupID)
	if err != nil {
		return nil, fmt.Errorf("failed to find group: %w", err)
	}

	if existingGroup == nil {
		return nil, NewErrNotFound("group")
	}

	// Find the user by email in the organization
	userMembership, err := uc.membershipRepo.FindByOrgIDAndUserEmail(ctx, orgID, opts.UserEmail)
	if err != nil && !IsNotFound(err) {
		return nil, fmt.Errorf("failed to find user by email: %w", err)
	}
	if userMembership == nil {
		return nil, NewErrValidationStr("user with the provided email is not a member of the organization")
	}

	userUUID := uuid.MustParse(userMembership.User.ID)

	// Check if the user is already a member of the group
	existingMembership, err := uc.groupRepo.FindGroupMembershipByGroupAndID(ctx, resolvedGroupID, userUUID)
	if err != nil && !IsNotFound(err) {
		return nil, fmt.Errorf("failed to check existing membership: %w", err)
	}
	if existingMembership != nil {
		return nil, NewErrAlreadyExistsStr("user is already a member of this group")
	}

	// Add the user to the group
	membership, err := uc.groupRepo.AddMemberToGroup(ctx, orgID, resolvedGroupID, userUUID, opts.Maintainer)
	if err != nil {
		return nil, fmt.Errorf("failed to add member to group: %w", err)
	}

	// Dispatch event to the audit log for group membership addition
	uc.auditorUC.Dispatch(ctx, &events.GroupMemberAdded{
		GroupBase: &events.GroupBase{
			GroupID:   &existingGroup.ID,
			GroupName: existingGroup.Name,
		},
		UserID:     &userUUID,
		Maintainer: opts.Maintainer,
	}, &orgID)

	return membership, nil
}

// RemoveMemberFromGroup removes a user from a group.
// The requester must be either a maintainer of the group or have RoleOwner/RoleAdmin in the organization.
func (uc *GroupUseCase) RemoveMemberFromGroup(ctx context.Context, orgID uuid.UUID, opts *RemoveMemberFromGroupOpts) error {
	if opts == nil {
		return NewErrValidationStr("options cannot be nil")
	}

	if orgID == uuid.Nil || opts.UserEmail == "" || opts.RequesterID == uuid.Nil {
		return NewErrValidationStr("organization ID, user email, and requester ID cannot be empty")
	}

	resolvedGroupID, err := uc.ValidateGroupIdentifier(ctx, orgID, opts.ID, opts.Name)
	if err != nil {
		return err
	}

	// Check the group exists
	existingGroup, err := uc.groupRepo.FindByOrgAndID(ctx, orgID, resolvedGroupID)
	if err != nil {
		return fmt.Errorf("failed to find group: %w", err)
	}

	if existingGroup == nil {
		return NewErrNotFound("group")
	}

	// Find the user by email in the organization
	userMembership, err := uc.membershipRepo.FindByOrgIDAndUserEmail(ctx, orgID, opts.UserEmail)
	if err != nil && !IsNotFound(err) {
		return fmt.Errorf("failed to find user by email: %w", err)
	}
	if userMembership == nil {
		return NewErrValidationStr("user with the provided email is not a member of the organization")
	}

	// Check if the requester is part of the organization
	membership, err := uc.groupRepo.FindGroupMembershipByGroupAndID(ctx, resolvedGroupID, opts.RequesterID)
	if err != nil && !IsNotFound(err) {
		return NewErrValidationStr("failed to check existing membership")
	}

	if membership == nil {
		return NewErrValidationStr("requester is not a member of the group")
	}

	userUUID := uuid.MustParse(userMembership.User.ID)
	// Check if the user is a member of the group
	existingMembership, err := uc.groupRepo.FindGroupMembershipByGroupAndID(ctx, resolvedGroupID, userUUID)
	if err != nil && !IsNotFound(err) {
		return fmt.Errorf("failed to check existing membership: %w", err)
	}
	if existingMembership == nil {
		return NewErrValidationStr("user is not a member of this group")
	}

	// Remove the user from the group
	if err := uc.groupRepo.RemoveMemberFromGroup(ctx, orgID, resolvedGroupID, userUUID); err != nil {
		return fmt.Errorf("failed to remove member from group: %w", err)
	}

	// Dispatch event to the audit log for group membership removal
	uc.auditorUC.Dispatch(ctx, &events.GroupMemberRemoved{
		GroupBase: &events.GroupBase{
			GroupID:   &existingGroup.ID,
			GroupName: existingGroup.Name,
		},
		UserID: &userUUID,
	}, &orgID)

	return nil
}

// IsUserGroupMaintainer checks if a user is a maintainer of a group.
func (uc *GroupUseCase) IsUserGroupMaintainer(ctx context.Context, orgID uuid.UUID, groupID uuid.UUID, userID uuid.UUID) (bool, error) {
	if orgID == uuid.Nil || groupID == uuid.Nil || userID == uuid.Nil {
		return false, NewErrValidationStr("organization ID, group ID, and user ID cannot be empty")
	}

	membership, err := uc.groupRepo.FindGroupMembershipByGroupAndID(ctx, groupID, userID)
	if err != nil && !IsNotFound(err) {
		return false, fmt.Errorf("failed to check group membership: %w", err)
	}

	if membership == nil {
		return false, nil // User is not a member of the group
	}

	return membership.Maintainer, nil
}

// ValidateGroupIdentifier validates and resolves the group ID or name to a group ID.
// Returns an error if both are nil or if the resolved group does not exist.
func (uc *GroupUseCase) ValidateGroupIdentifier(ctx context.Context, orgID uuid.UUID, groupID *uuid.UUID, groupName *string) (uuid.UUID, error) {
	if groupID == nil && groupName == nil {
		return uuid.Nil, NewErrValidationStr("either group ID or group name must be provided")
	}

	if groupID != nil {
		return *groupID, nil
	}

	// If group ID is not provided, try to find the group by name
	groups, _, err := uc.groupRepo.List(ctx, orgID, &ListGroupOpts{Name: *groupName}, pagination.NewDefaultOffsetPaginationOpts())
	if err != nil {
		return uuid.Nil, fmt.Errorf("failed to list groups: %w", err)
	}

	if len(groups) == 0 {
		return uuid.Nil, NewErrNotFound("group")
	}

	// If the group name is not unique, return an error
	if len(groups) > 1 {
		return uuid.Nil, NewErrValidationStr("group name is not unique")
	}

	return groups[0].ID, nil
}
