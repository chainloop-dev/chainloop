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
	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/authz"
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
	// FindByOrgAndName finds a group by its organization ID and group name.
	FindByOrgAndName(ctx context.Context, orgID uuid.UUID, name string) (*Group, error)
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
	// UpdateMemberMaintainerStatus updates the maintainer status of a group member.
	UpdateMemberMaintainerStatus(ctx context.Context, orgID uuid.UUID, groupID uuid.UUID, userID uuid.UUID, isMaintainer bool) error
	// ListPendingInvitationsByGroup retrieves a list of pending invitations for a group
	ListPendingInvitationsByGroup(ctx context.Context, orgID uuid.UUID, groupID uuid.UUID, paginationOpts *pagination.OffsetPaginationOpts) ([]*OrgInvitation, int, error)
	// ListProjectsByGroup retrieves a list of projects that a group is a member of with pagination.
	ListProjectsByGroup(ctx context.Context, orgID uuid.UUID, groupID uuid.UUID, visibleProjectIDs []uuid.UUID, paginationOpts *pagination.OffsetPaginationOpts) ([]*GroupProjectInfo, int, error)
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
	// MemberCount is the total number of members in the group.
	MemberCount int
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
	UserID *uuid.UUID
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
	// RequesterID is the ID of the user who is requesting to add the member. Optional.
	// If provided, the requester must be a maintainer or admin.
	RequesterID uuid.UUID
	// Maintainer indicates if the new member should be a maintainer.
	Maintainer bool
}

// RemoveMemberFromGroupOpts defines options for removing a member from a group.
type RemoveMemberFromGroupOpts struct {
	*IdentityReference
	// UserEmail is the email of the user to remove from the group.
	UserEmail string
	// RequesterID is the ID of the user who is requesting to remove the member. Optional.
	// If provided, the requester must be a maintainer or admin.
	RequesterID uuid.UUID
}

// AddMemberToGroupResult represents the result of adding a member to a group.
type AddMemberToGroupResult struct {
	// Membership is the membership that was created or found.
	Membership *GroupMembership
	// InvitationSent indicates if an invitation was sent instead of creating a membership directly.
	InvitationSent bool
}

// UpdateMemberMaintainerStatusOpts defines options for updating a member's maintainer status in a group.
type UpdateMemberMaintainerStatusOpts struct {
	// Group reference
	*IdentityReference
	// UserReference is used to identify the user whose maintainer status is to be updated
	UserReference *IdentityReference
	// RequesterID is the ID of the user who is requesting to update the maintainer status. Optional.
	// If provided, the requester must be a maintainer or admin.
	RequesterID uuid.UUID
	// IsMaintainer is the new maintainer status for the user.
	IsMaintainer bool
}

// ListProjectsByGroupOpts defines options for listing projects by group.
type ListProjectsByGroupOpts struct {
	// Group reference
	*IdentityReference
	// VisibleProjectsIDs is a list of project IDs that the requester can see.
	VisibleProjectsIDs []uuid.UUID
}

type GroupUseCase struct {
	// logger is used to log messages.
	logger   *log.Helper
	enforcer *authz.Enforcer
	// Repositories
	groupRepo         GroupRepo
	membershipRepo    MembershipRepo
	userRepo          UserRepo
	orgInvitationRepo OrgInvitationRepo
	// Use Cases
	orgInvitationUC *OrgInvitationUseCase
	auditorUC       *AuditorUseCase
	membershipUC    *MembershipUseCase
}

func NewGroupUseCase(logger log.Logger, groupRepo GroupRepo, membershipRepo MembershipRepo, userRepo UserRepo, orgInvitationUC *OrgInvitationUseCase, auditorUC *AuditorUseCase, invitationRepo OrgInvitationRepo, enforcer *authz.Enforcer, membershipUseCase *MembershipUseCase) *GroupUseCase {
	return &GroupUseCase{
		logger:            log.NewHelper(log.With(logger, "component", "biz/group")),
		groupRepo:         groupRepo,
		membershipRepo:    membershipRepo,
		userRepo:          userRepo,
		orgInvitationUC:   orgInvitationUC,
		auditorUC:         auditorUC,
		orgInvitationRepo: invitationRepo,
		enforcer:          enforcer,
		membershipUC:      membershipUseCase,
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
func (uc *GroupUseCase) Create(ctx context.Context, orgID uuid.UUID, name string, description string, userID *uuid.UUID) (*Group, error) {
	if name == "" {
		return nil, NewErrValidationStr("name cannot be empty")
	}

	if orgID == uuid.Nil {
		return nil, NewErrValidationStr("organization ID")
	}

	// Only check if the user is a member of the organization if userID is provided
	if userID != nil {
		// Check if the user is a member of the organization
		m, err := uc.membershipRepo.FindByOrgAndUser(ctx, orgID, *userID)
		if err != nil {
			return nil, fmt.Errorf("failed to find membership: %w", err)
		} else if m == nil {
			return nil, NewErrNotFound("membership")
		}
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

// ListPendingInvitations retrieves a list of pending invitations for a group.
func (uc *GroupUseCase) ListPendingInvitations(ctx context.Context, orgID uuid.UUID, groupID *uuid.UUID, groupName *string, paginationOpts *pagination.OffsetPaginationOpts) ([]*OrgInvitation, int, error) {
	if groupID == nil && groupName == nil {
		return nil, 0, NewErrValidationStr("either group ID or group name must be provided")
	}

	if orgID == uuid.Nil {
		return nil, 0, NewErrValidationStr("organization ID cannot be empty")
	}

	resolvedGroupID, err := uc.ValidateGroupIdentifier(ctx, orgID, groupID, groupName)
	if err != nil {
		return nil, 0, err
	}

	pgOpts := pagination.NewDefaultOffsetPaginationOpts()
	if paginationOpts != nil {
		pgOpts = paginationOpts
	}

	return uc.groupRepo.ListPendingInvitationsByGroup(ctx, orgID, resolvedGroupID, pgOpts)
}

// AddMemberToGroup adds a user to a group.
// If RequesterID is provided, the requester must be either a maintainer of the group or have RoleOwner/RoleAdmin in the organization.
// Returns AddMemberToGroupResult which indicates whether a membership was created or an invitation was sent.
func (uc *GroupUseCase) AddMemberToGroup(ctx context.Context, orgID uuid.UUID, opts *AddMemberToGroupOpts) (*AddMemberToGroupResult, error) {
	// Validate input parameters
	if opts == nil {
		return nil, NewErrValidationStr("options cannot be nil")
	}

	if orgID == uuid.Nil || opts.UserEmail == "" {
		return nil, NewErrValidationStr("organization ID and user email cannot be empty")
	}

	// Resolve group ID and check that the group exists
	resolvedGroupID, existingGroup, err := uc.resolveAndValidateGroup(ctx, orgID, opts)
	if err != nil {
		return nil, err
	}

	// Validate requester permissions only if RequesterID is provided
	if opts.RequesterID != uuid.Nil {
		if err := uc.validateRequesterPermissions(ctx, orgID, opts.RequesterID, resolvedGroupID); err != nil {
			return nil, fmt.Errorf("failed to validate requester permissions: %w", err)
		}
	}

	// Find the user in the organization
	userMembership, err := uc.membershipRepo.FindByOrgIDAndUserEmail(ctx, orgID, opts.UserEmail)
	if err != nil && !IsNotFound(err) {
		return nil, fmt.Errorf("failed to find user by email: %w", err)
	}

	// If the user is not found in the organization, send an invitation
	if userMembership == nil {
		// We need a requester for creating invitations
		if opts.RequesterID == uuid.Nil {
			return nil, NewErrValidationStr("requester ID is required for inviting new users")
		}
		return uc.handleNonExistingUser(ctx, orgID, resolvedGroupID, opts)
	}

	// Process existing user
	return uc.addExistingUserToGroup(ctx, orgID, resolvedGroupID, existingGroup, userMembership, opts)
}

// resolveAndValidateGroup resolves the group ID and verifies the group exists
func (uc *GroupUseCase) resolveAndValidateGroup(ctx context.Context, orgID uuid.UUID, opts *AddMemberToGroupOpts) (uuid.UUID, *Group, error) {
	resolvedGroupID, err := uc.ValidateGroupIdentifier(ctx, orgID, opts.ID, opts.Name)
	if err != nil {
		return uuid.Nil, nil, err
	}

	// Check the group exists
	existingGroup, err := uc.groupRepo.FindByOrgAndID(ctx, orgID, resolvedGroupID)
	if err != nil {
		return uuid.Nil, nil, fmt.Errorf("failed to find group: %w", err)
	}

	if existingGroup == nil {
		return uuid.Nil, nil, NewErrNotFound("group")
	}

	return resolvedGroupID, existingGroup, nil
}

// validateRequesterPermissions checks if the requester has sufficient permissions
func (uc *GroupUseCase) validateRequesterPermissions(ctx context.Context, orgID, requesterID, groupID uuid.UUID) error {
	// Check if the requester is part of the organization
	requesterMembership, err := uc.membershipRepo.FindByOrgAndUser(ctx, orgID, requesterID)
	if err != nil && !IsNotFound(err) {
		return NewErrValidationStr("failed to check existing membership")
	}

	if requesterMembership == nil {
		return NewErrValidationStr("requester is not a member of the organization")
	}

	// Allow if the requester is an org owner or admin
	isAdminOrOwner := requesterMembership.Role == authz.RoleOwner || requesterMembership.Role == authz.RoleAdmin
	if isAdminOrOwner {
		return nil
	}

	// If not an admin/owner, check if the requester is a maintainer of this group
	requesterGroupMembership, err := uc.membershipRepo.FindByUserAndResourceID(ctx, requesterID, groupID)
	if err != nil && !IsNotFound(err) {
		return fmt.Errorf("failed to check requester's group membership: %w", err)
	}

	// If not a maintainer of this group, deny access
	if requesterGroupMembership == nil || requesterGroupMembership.Role != authz.RoleGroupMaintainer {
		return NewErrValidationStr("requester does not have permission to add members to this group")
	}

	return nil
}

// handleNonExistingUser creates an invitation for a user not yet in the organization
func (uc *GroupUseCase) handleNonExistingUser(ctx context.Context, orgID, groupID uuid.UUID, opts *AddMemberToGroupOpts) (*AddMemberToGroupResult, error) {
	// Check if the user email is already invited to the organization
	invitation, err := uc.orgInvitationRepo.PendingInvitation(ctx, orgID, opts.UserEmail)
	if err != nil && !IsNotFound(err) {
		return nil, fmt.Errorf("failed to check existing invitation: %w", err)
	}

	// If an invitation already exists, return an error
	if invitation != nil {
		return nil, NewErrAlreadyExistsStr("user is already invited to the organization")
	}

	// Check if the requester is an admin or owner of the organization
	requesterMembership, err := uc.membershipRepo.FindByOrgAndUser(ctx, orgID, opts.RequesterID)
	if err != nil {
		return nil, fmt.Errorf("failed to check requester's role: %w", err)
	}

	pass, err := uc.enforcer.Enforce(string(requesterMembership.Role), authz.PolicyOrganizationInvitationsCreate)
	if err != nil {
		return nil, fmt.Errorf("failed to check requester's role: %w", err)
	}

	if !pass {
		return nil, NewErrValidationStr("only organization admins or owners can invite new users")
	}

	// Create an organization invitation with group context
	invitationContext := &OrgInvitationContext{
		GroupIDToJoin:   groupID,
		GroupMaintainer: opts.Maintainer,
	}

	// Create an invitation for the user to join the organization
	if _, err := uc.orgInvitationUC.Create(ctx, orgID.String(), opts.RequesterID.String(), opts.UserEmail, WithInvitationRole(authz.RoleOrgMember), WithInvitationContext(invitationContext)); err != nil {
		return nil, fmt.Errorf("failed to create invitation: %w", err)
	}

	// Return a result indicating an invitation was sent
	return &AddMemberToGroupResult{
		InvitationSent: true,
	}, nil
}

// addExistingUserToGroup adds an existing user to a group
func (uc *GroupUseCase) addExistingUserToGroup(ctx context.Context, orgID, groupID uuid.UUID, group *Group, userMembership *Membership, opts *AddMemberToGroupOpts) (*AddMemberToGroupResult, error) {
	userUUID, err := uuid.Parse(userMembership.User.ID)
	if err != nil {
		return nil, NewErrInvalidUUID(err)
	}

	// Check if the user is already a member of the group
	existingGroupMembership, err := uc.groupRepo.FindGroupMembershipByGroupAndID(ctx, groupID, userUUID)
	if err != nil && !IsNotFound(err) {
		return nil, fmt.Errorf("failed to check existing membership: %w", err)
	}
	if existingGroupMembership != nil {
		return nil, NewErrAlreadyExistsStr("user is already a member of this group")
	}

	// If trying to make the user a maintainer, verify they don't have the org viewer role
	if opts.Maintainer && userMembership.Role == authz.RoleViewer {
		return nil, NewErrValidationStr("users with organization viewer role cannot be group maintainers")
	}

	// Add the user to the group
	membership, err := uc.groupRepo.AddMemberToGroup(ctx, orgID, groupID, userUUID, opts.Maintainer)
	if err != nil {
		return nil, fmt.Errorf("failed to add member to group: %w", err)
	}

	// Dispatch event to the audit log for group membership addition
	uc.auditorUC.Dispatch(ctx, &events.GroupMemberAdded{
		GroupBase: &events.GroupBase{
			GroupID:   &group.ID,
			GroupName: group.Name,
		},
		UserID:     &userUUID,
		UserEmail:  opts.UserEmail,
		Maintainer: opts.Maintainer,
	}, &orgID)

	// Return a result indicating a direct membership was created
	return &AddMemberToGroupResult{
		Membership: membership,
	}, nil
}

// RemoveMemberFromGroup removes a user from a group.
// The requester must be either a maintainer of the group or have RoleOwner/RoleAdmin in the organization.
func (uc *GroupUseCase) RemoveMemberFromGroup(ctx context.Context, orgID uuid.UUID, opts *RemoveMemberFromGroupOpts) error {
	if opts == nil {
		return NewErrValidationStr("options cannot be nil")
	}

	if orgID == uuid.Nil || opts.UserEmail == "" {
		return NewErrValidationStr("organization ID and user email cannot be empty")
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

	if opts.RequesterID != uuid.Nil {
		// Check if the requester is part of the organization
		requesterMembership, err := uc.membershipRepo.FindByOrgAndUser(ctx, orgID, opts.RequesterID)
		if err != nil && !IsNotFound(err) {
			return NewErrValidationStr("failed to check existing membership")
		}

		if requesterMembership == nil {
			return NewErrValidationStr("requester is not a member of the organization")
		}

		// Check if the requester has sufficient permissions
		// Allow if the requester is an org owner or admin
		isAdminOrOwner := requesterMembership.Role == authz.RoleOwner || requesterMembership.Role == authz.RoleAdmin

		// If not an admin/owner, check if the requester is a maintainer of this group
		if !isAdminOrOwner {
			// Check if the requester is a maintainer of this group
			requesterGroupMembership, err := uc.membershipRepo.FindByUserAndResourceID(ctx, opts.RequesterID, resolvedGroupID)
			if err != nil && !IsNotFound(err) {
				return fmt.Errorf("failed to check requester's group membership: %w", err)
			}

			// If not a maintainer of this group, deny access
			if requesterGroupMembership == nil || requesterGroupMembership.Role != authz.RoleGroupMaintainer {
				return NewErrValidationStr("requester does not have permission to add members to this group")
			}
		}
	}

	// Find the user by email in the organization
	userMembership, err := uc.membershipRepo.FindByOrgIDAndUserEmail(ctx, orgID, opts.UserEmail)
	if err != nil && !IsNotFound(err) {
		return fmt.Errorf("failed to find user by email: %w", err)
	}
	if userMembership == nil {
		return NewErrValidationStr("user with the provided email is not a member of the organization")
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

// UpdateMemberMaintainerStatus updates the maintainer status of a group member.
// The requester must be either a maintainer of the group or have RoleOwner/RoleAdmin in the organization.
// nolint: gocyclo
func (uc *GroupUseCase) UpdateMemberMaintainerStatus(ctx context.Context, orgID uuid.UUID, opts *UpdateMemberMaintainerStatusOpts) error {
	if opts == nil {
		return NewErrValidationStr("options cannot be nil")
	}

	if orgID == uuid.Nil {
		return NewErrValidationStr("organization ID cannot be empty")
	}

	// Ensure we have either a UserReference or UserEmail
	if opts.UserReference == nil || (opts.UserReference.ID == nil && opts.UserReference.Name == nil) {
		return NewErrValidationStr("either user reference or user email must be provided")
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

	// Check if the requester is part of the organization
	if opts.RequesterID != uuid.Nil {
		requesterMembership, err := uc.membershipRepo.FindByOrgAndUser(ctx, orgID, opts.RequesterID)
		if err != nil && !IsNotFound(err) {
			return NewErrValidationStr("failed to check existing membership")
		}

		if requesterMembership == nil {
			return NewErrValidationStr("requester is not a member of the organization")
		}

		// Check if the requester has sufficient permissions
		// Allow if the requester is an org owner or admin
		isAdminOrOwner := requesterMembership.Role == authz.RoleOwner || requesterMembership.Role == authz.RoleAdmin

		// If not an admin/owner, check if the requester is a maintainer of this group
		if !isAdminOrOwner {
			// Check if the requester is a maintainer of this group
			requesterGroupMembership, err := uc.membershipRepo.FindByUserAndResourceID(ctx, opts.RequesterID, resolvedGroupID)
			if err != nil && !IsNotFound(err) {
				return fmt.Errorf("failed to check requester's group membership: %w", err)
			}

			// If not a maintainer of this group, deny access
			if requesterGroupMembership == nil || requesterGroupMembership.Role != authz.RoleGroupMaintainer {
				return NewErrValidationStr("requester does not have permission to update maintainer status in this group")
			}
		}
	}

	// Find the user by reference or email
	var userUUID uuid.UUID
	var userEmail string

	// If UserReference is provided, use it to resolve the user ID
	if opts.UserReference != nil && (opts.UserReference.ID != nil || opts.UserReference.Name != nil) {
		// If ID is provided directly, use it
		if opts.UserReference.ID != nil {
			userUUID = *opts.UserReference.ID
			// Look up the user to verify they exist and get their email
			user, err := uc.membershipRepo.FindByOrgAndUser(ctx, orgID, userUUID)
			if err != nil {
				return fmt.Errorf("failed to find user by ID: %w", err)
			}
			if user == nil {
				return NewErrNotFound("user")
			}
			userEmail = user.User.Email
		} else if opts.UserReference.Name != nil {
			// If name (email) is provided, look up the user
			userMembership, err := uc.membershipRepo.FindByOrgIDAndUserEmail(ctx, orgID, *opts.UserReference.Name)
			if err != nil && !IsNotFound(err) {
				return fmt.Errorf("failed to find user by email: %w", err)
			}
			if userMembership == nil {
				return NewErrValidationStr("user with the provided email is not a member of the organization")
			}
			userUUID = uuid.MustParse(userMembership.User.ID)
			userEmail = *opts.UserReference.Name
		}
	} else {
		// Fall back to using UserEmail
		userMembership, err := uc.membershipRepo.FindByOrgIDAndUserEmail(ctx, orgID, *opts.UserReference.Name)
		if err != nil && !IsNotFound(err) {
			return fmt.Errorf("failed to find user by email: %w", err)
		}
		if userMembership == nil {
			return NewErrValidationStr("user with the provided email is not a member of the organization")
		}
		userUUID = uuid.MustParse(userMembership.User.ID)
		userEmail = *opts.UserReference.Name
	}

	// Check if the user is a member of the group
	existingMembership, err := uc.groupRepo.FindGroupMembershipByGroupAndID(ctx, resolvedGroupID, userUUID)
	if err != nil && !IsNotFound(err) {
		return fmt.Errorf("failed to check existing membership: %w", err)
	}
	if existingMembership == nil {
		return NewErrValidationStr("user is not a member of this group")
	}

	// If trying to make the user a maintainer, verify they don't have the org viewer role
	if opts.IsMaintainer {
		// Check the user's org role
		userOrgMembership, err := uc.membershipRepo.FindByOrgAndUser(ctx, orgID, userUUID)
		if err != nil {
			return fmt.Errorf("failed to check user's organization role: %w", err)
		}

		// Prevent org viewers from becoming maintainers
		if userOrgMembership.Role == authz.RoleViewer {
			return NewErrValidationStr("users with organization viewer role cannot be group maintainers")
		}
	}

	// Update the member's maintainer status
	if err := uc.groupRepo.UpdateMemberMaintainerStatus(ctx, orgID, resolvedGroupID, userUUID, opts.IsMaintainer); err != nil {
		return fmt.Errorf("failed to update member maintainer status: %w", err)
	}

	// Dispatch event to the audit log for group membership update
	uc.auditorUC.Dispatch(ctx, &events.GroupMemberUpdated{
		GroupBase: &events.GroupBase{
			GroupID:   &existingGroup.ID,
			GroupName: existingGroup.Name,
		},
		UserID:              &userUUID,
		UserEmail:           userEmail,
		NewMaintainerStatus: opts.IsMaintainer,
		OldMaintainerStatus: existingMembership.Maintainer,
	}, &orgID)

	return nil
}

// ValidateGroupIdentifier validates and resolves the group ID or name to a group ID.
// Returns an error if both are nil or if the resolved group does not exist.
// TODO: change to return the group since this is very inefficient in some cases
func (uc *GroupUseCase) ValidateGroupIdentifier(ctx context.Context, orgID uuid.UUID, groupID *uuid.UUID, groupName *string) (uuid.UUID, error) {
	if groupID == nil && groupName == nil {
		return uuid.Nil, NewErrValidationStr("either group ID or group name must be provided")
	}

	if groupID != nil {
		return *groupID, nil
	}

	// If group ID is not provided, try to find the group by name
	group, err := uc.groupRepo.FindByOrgAndName(ctx, orgID, *groupName)
	if err != nil {
		return uuid.Nil, fmt.Errorf("failed to find group: %w", err)
	}

	return group.ID, nil
}

// ListProjectsByGroup retrieves a list of projects that a group is a member of with pagination.
func (uc *GroupUseCase) ListProjectsByGroup(ctx context.Context, orgID uuid.UUID, opts *ListProjectsByGroupOpts, paginationOpts *pagination.OffsetPaginationOpts) ([]*GroupProjectInfo, int, error) {
	if opts == nil {
		return nil, 0, NewErrValidationStr("options cannot be nil")
	}

	if orgID == uuid.Nil {
		return nil, 0, NewErrValidationStr("organization ID cannot be empty")
	}

	resolvedGroupID, err := uc.ValidateGroupIdentifier(ctx, orgID, opts.ID, opts.Name)
	if err != nil {
		return nil, 0, err
	}

	// Check the group exists
	existingGroup, err := uc.groupRepo.FindByOrgAndID(ctx, orgID, resolvedGroupID)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to find group: %w", err)
	}

	if existingGroup == nil {
		return nil, 0, NewErrNotFound("group")
	}

	pgOpts := pagination.NewDefaultOffsetPaginationOpts()
	if paginationOpts != nil {
		pgOpts = paginationOpts
	}

	return uc.groupRepo.ListProjectsByGroup(ctx, orgID, resolvedGroupID, opts.VisibleProjectsIDs, pgOpts)
}
