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

package biz

import (
	"context"
	"fmt"
	"time"

	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/auditor/events"
	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/authz"
	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/pagination"
	"github.com/chainloop-dev/chainloop/pkg/servicelogger"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/google/uuid"
)

// ProjectsRepo is a repository for projects
type ProjectsRepo interface {
	FindProjectByOrgIDAndName(ctx context.Context, orgID uuid.UUID, projectName string) (*Project, error)
	FindProjectByOrgIDAndID(ctx context.Context, orgID uuid.UUID, projectID uuid.UUID) (*Project, error)
	Create(ctx context.Context, orgID uuid.UUID, name string) (*Project, error)
	ListProjectsByOrgID(ctx context.Context, orgID uuid.UUID) ([]*Project, error)
	// ListMembers retrieves a list of members in a project, optionally filtered by admin status.
	ListMembers(ctx context.Context, orgID uuid.UUID, projectID uuid.UUID, paginationOpts *pagination.OffsetPaginationOpts) ([]*ProjectMembership, int, error)
	// AddMemberToProject adds a user or group to a project with a specific role.
	AddMemberToProject(ctx context.Context, orgID uuid.UUID, projectID uuid.UUID, memberID uuid.UUID, membershipType authz.MembershipType, role authz.Role) (*ProjectMembership, error)
	// RemoveMemberFromProject removes a user or group from a project.
	RemoveMemberFromProject(ctx context.Context, orgID uuid.UUID, projectID uuid.UUID, memberID uuid.UUID, membershipType authz.MembershipType) error
	// UpdateMemberRoleInProject updates the role of a user or group in a project.
	UpdateMemberRoleInProject(ctx context.Context, orgID uuid.UUID, projectID uuid.UUID, memberID uuid.UUID, membershipType authz.MembershipType, newRole authz.Role) (*ProjectMembership, error)
	// FindProjectMembershipByProjectAndID finds a project membership by project ID and member ID (user or group).
	FindProjectMembershipByProjectAndID(ctx context.Context, orgID uuid.UUID, projectID uuid.UUID, memberID uuid.UUID, membershipType authz.MembershipType) (*ProjectMembership, error)
	// ListPendingInvitationsByProject retrieves a list of pending invitations for a project.
	ListPendingInvitationsByProject(ctx context.Context, orgID uuid.UUID, projectID uuid.UUID, paginationOpts *pagination.OffsetPaginationOpts) ([]*OrgInvitation, int, error)
}

// ProjectUseCase is a use case for projects
type ProjectUseCase struct {
	logger   *log.Helper
	enforcer *authz.Enforcer
	// Use Cases
	auditorUC       *AuditorUseCase
	groupUC         *GroupUseCase
	membershipUC    *MembershipUseCase
	orgInvitationUC *OrgInvitationUseCase
	// Repositories
	projectsRepository   ProjectsRepo
	membershipRepository MembershipRepo
	orgInvitationRepo    OrgInvitationRepo
}

// Project is a project in the organization
type Project struct {
	// ID is the unique identifier of the project
	ID uuid.UUID
	// Name is the name of the project
	Name string
	// OrgID is the organization that this project belongs to
	OrgID uuid.UUID
	// CreatedAt is the time when the project was created
	CreatedAt *time.Time
	// UpdatedAt is the time when the project was last updated
	UpdatedAt *time.Time
}

// ProjectMembership represents a membership of a user or group in a project.
type ProjectMembership struct {
	// User is the user who is a member of the project (nil for group memberships).
	User *User
	// Group is the group that is a member of the project (nil for user memberships).
	Group *Group
	// MembershipType indicates if this is a user or group membership.
	MembershipType authz.MembershipType
	// Role represents the role of the user/group in the project (admin or viewer).
	Role authz.Role
	// LatestProjectVersionID is the ID of the latest project version this membership is associated with.
	LatestProjectVersionID *uuid.UUID
	// CreatedAt is the timestamp when the user/group was added to the project.
	CreatedAt *time.Time
	// UpdatedAt is the timestamp when the membership was last updated.
	UpdatedAt *time.Time
}

// GroupProjectInfo represents detailed information about a project that a group is a member of
type GroupProjectInfo struct {
	// ID is the unique identifier of the project
	ID uuid.UUID
	// Name is the name of the project
	Name string
	// Description is the description of the project
	Description string
	// Role represents the role of the group in the project (admin or viewer)
	Role authz.Role
	// LatestVersionID is the ID of the latest version of the project, if available
	LatestVersionID *uuid.UUID
	// Group is the group that is a member of this project
	Group *Group
	// CreatedAt is the timestamp when the project was created
	CreatedAt *time.Time
}

// AddMemberToProjectOpts defines options for adding a member to a project.
type AddMemberToProjectOpts struct {
	// ProjectReference is the reference to the project.
	ProjectReference *IdentityReference
	// UserEmail is the email of the user to add to the project.
	UserEmail string
	// GroupReference is the reference to the group to add to the project.
	GroupReference *IdentityReference
	// RequesterID is the ID of the user who is requesting to add the member.
	RequesterID uuid.UUID
	// Role represents the role to assign to the user in the project.
	Role authz.Role
}

// RemoveMemberFromProjectOpts defines options for removing a member from a project.
type RemoveMemberFromProjectOpts struct {
	// ProjectReference is the reference to the project.
	ProjectReference *IdentityReference
	// UserEmail is the email of the user to remove from the project.
	UserEmail string
	// GroupReference is the reference to the group to remove from the project.
	GroupReference *IdentityReference
	// RequesterID is the ID of the user who is requesting to remove the member.
	RequesterID uuid.UUID
}

// UpdateMemberRoleOpts defines options for updating a member's role in a project.
type UpdateMemberRoleOpts struct {
	// ProjectReference is the reference to the project.
	ProjectReference *IdentityReference
	// UserEmail is the email of the user whose role to update.
	UserEmail string
	// GroupReference is the reference to the group whose role to update.
	GroupReference *IdentityReference
	// RequesterID is the ID of the user who is requesting to update the role.
	RequesterID uuid.UUID
	// NewRole represents the new role to assign to the member in the project.
	NewRole authz.Role
}

// AddMemberToProjectResult represents the result of adding a member to a project.
type AddMemberToProjectResult struct {
	// Membership is the membership that was created or found.
	Membership *ProjectMembership
	// InvitationSent indicates if an invitation was sent instead of creating a membership directly.
	InvitationSent bool
}

func NewProjectsUseCase(logger log.Logger, projectsRepository ProjectsRepo, membershipRepository MembershipRepo, auditorUC *AuditorUseCase, groupUC *GroupUseCase, membershipUC *MembershipUseCase, orgInvitationUC *OrgInvitationUseCase, orgInvitationRepo OrgInvitationRepo, enforcer *authz.Enforcer) *ProjectUseCase {
	return &ProjectUseCase{
		logger:               servicelogger.ScopedHelper(logger, "biz/project"),
		projectsRepository:   projectsRepository,
		membershipRepository: membershipRepository,
		auditorUC:            auditorUC,
		groupUC:              groupUC,
		membershipUC:         membershipUC,
		orgInvitationUC:      orgInvitationUC,
		orgInvitationRepo:    orgInvitationRepo,
		enforcer:             enforcer,
	}
}

// FindProjectByReference finds a project by reference, which can be either a project name or a project ID.
func (uc *ProjectUseCase) FindProjectByReference(ctx context.Context, orgID string, reference *IdentityReference) (*Project, error) {
	if reference == nil || orgID == "" {
		return nil, NewErrValidationStr("orgID or project reference are empty")
	}
	orgUUID, err := uuid.Parse(orgID)
	if err != nil {
		return nil, NewErrInvalidUUID(err)
	}

	switch {
	case reference.Name != nil && *reference.Name != "":
		return uc.projectsRepository.FindProjectByOrgIDAndName(ctx, orgUUID, *reference.Name)
	case reference.ID != nil && *reference.ID != uuid.Nil:
		return uc.projectsRepository.FindProjectByOrgIDAndID(ctx, orgUUID, *reference.ID)
	default:
		return nil, NewErrValidationStr("project reference is empty")
	}
}

func (uc *ProjectUseCase) Create(ctx context.Context, orgID, name string) (*Project, error) {
	if name == "" || orgID == "" {
		return nil, NewErrValidationStr("orgID or project name are empty")
	}

	orgUUID, err := uuid.Parse(orgID)
	if err != nil {
		return nil, NewErrInvalidUUID(err)
	}

	return uc.projectsRepository.Create(ctx, orgUUID, name)
}

// ListMembers lists the members of a project with pagination.
func (uc *ProjectUseCase) ListMembers(ctx context.Context, orgID uuid.UUID, projectRef *IdentityReference, paginationOpts *pagination.OffsetPaginationOpts) ([]*ProjectMembership, int, error) {
	if orgID == uuid.Nil {
		return nil, 0, NewErrValidationStr("organization ID cannot be empty")
	}

	// Validate and resolve the project reference
	resolvedProjectID, _, err := uc.validateAndResolveProject(ctx, orgID, projectRef)
	if err != nil {
		return nil, 0, err
	}

	// Use default pagination options if none provided
	pgOpts := pagination.NewDefaultOffsetPaginationOpts()
	if paginationOpts != nil {
		pgOpts = paginationOpts
	}

	return uc.projectsRepository.ListMembers(ctx, orgID, resolvedProjectID, pgOpts)
}

// ListPendingInvitations retrieves a list of pending invitations for a project.
func (uc *ProjectUseCase) ListPendingInvitations(ctx context.Context, orgID uuid.UUID, projectRef *IdentityReference, paginationOpts *pagination.OffsetPaginationOpts) ([]*OrgInvitation, int, error) {
	if projectRef == nil {
		return nil, 0, NewErrValidationStr("project reference cannot be nil")
	}

	if orgID == uuid.Nil {
		return nil, 0, NewErrValidationStr("organization ID cannot be empty")
	}

	// Validate and resolve the project reference to a project ID
	resolvedProjectID, err := uc.ValidateProjectIdentifier(ctx, orgID, projectRef)
	if err != nil {
		return nil, 0, err
	}

	// Use default pagination options if none provided
	pgOpts := pagination.NewDefaultOffsetPaginationOpts()
	if paginationOpts != nil {
		pgOpts = paginationOpts
	}

	// Call the repository method to get the pending invitations
	return uc.projectsRepository.ListPendingInvitationsByProject(ctx, orgID, resolvedProjectID, pgOpts)
}

// AddMemberToProject adds a user or group to a project.
// Returns AddMemberToProjectResult which indicates whether a membership was created or an invitation was sent.
func (uc *ProjectUseCase) AddMemberToProject(ctx context.Context, orgID uuid.UUID, opts *AddMemberToProjectOpts) (*AddMemberToProjectResult, error) {
	if opts == nil {
		return nil, NewErrValidationStr("options cannot be nil")
	}

	if orgID == uuid.Nil {
		return nil, NewErrValidationStr("organization ID cannot be empty")
	}

	// Ensure only one of UserEmail or GroupReference is provided
	if (opts.UserEmail != "" && opts.GroupReference != nil) || (opts.UserEmail == "" && opts.GroupReference == nil) {
		return nil, NewErrValidationStr("exactly one of user email or group reference must be provided")
	}

	// Validate the role
	if opts.Role != authz.RoleProjectAdmin && opts.Role != authz.RoleProjectViewer {
		return nil, NewErrValidationStr("role must be either 'admin' or 'viewer'")
	}

	// Validate and resolve the project reference
	resolvedProjectID, existingProject, err := uc.validateAndResolveProject(ctx, orgID, opts.ProjectReference)
	if err != nil {
		return nil, err
	}

	// Verify the requester has permissions to add members to the project (if a requester is provided)
	if opts.RequesterID != uuid.Nil {
		if err := uc.verifyRequesterHasPermissions(ctx, orgID, resolvedProjectID, opts.RequesterID); err != nil {
			return nil, fmt.Errorf("requester does not have permission to add members to this project: %w", err)
		}
	}

	var result *AddMemberToProjectResult
	var resultErr error

	// Process based on whether we're adding a user or a group
	if opts.UserEmail != "" {
		result, resultErr = uc.addUserToProject(ctx, orgID, resolvedProjectID, existingProject, opts)
	} else {
		// For groups, we don't support invitations yet, so just create a membership
		membership, err := uc.addGroupToProject(ctx, orgID, resolvedProjectID, existingProject, opts)
		if err != nil {
			resultErr = err
		} else {
			result = &AddMemberToProjectResult{
				Membership: membership,
			}
		}
	}

	return result, resultErr
}

// addUserToProject adds a user to a project and logs the action.
// If the user doesn't exist in the organization, an invitation is sent instead.
func (uc *ProjectUseCase) addUserToProject(ctx context.Context, orgID uuid.UUID, projectID uuid.UUID, project *Project, opts *AddMemberToProjectOpts) (*AddMemberToProjectResult, error) {
	// Find the user by email in the organization
	userMembership, err := uc.membershipRepository.FindByOrgIDAndUserEmail(ctx, orgID, opts.UserEmail)
	if err != nil && !IsNotFound(err) {
		return nil, fmt.Errorf("failed to find user by email: %w", err)
	}

	// If the user is not in the organization, handle invitation flow
	if userMembership == nil {
		return uc.handleNonExistingUser(ctx, orgID, projectID, opts)
	}

	userUUID := uuid.MustParse(userMembership.User.ID)

	// Check if the user is already a member of the project
	existingMembership, err := uc.projectsRepository.FindProjectMembershipByProjectAndID(ctx, orgID, projectID, userUUID, authz.MembershipTypeUser)
	if err != nil && !IsNotFound(err) {
		return nil, fmt.Errorf("failed to check existing membership: %w", err)
	}
	if existingMembership != nil {
		return nil, NewErrAlreadyExistsStr("user is already a member of this project")
	}

	// Add the user to the project
	membership, err := uc.projectsRepository.AddMemberToProject(ctx, orgID, projectID, userUUID, authz.MembershipTypeUser, opts.Role)
	if err != nil {
		return nil, fmt.Errorf("failed to add member to project: %w", err)
	}

	// Dispatch event to the audit log for project membership addition using the unified event
	uc.auditorUC.Dispatch(ctx, &events.ProjectMembershipAdded{
		ProjectBase: &events.ProjectBase{
			ProjectID:   &projectID,
			ProjectName: project.Name,
		},
		UserID:    &userUUID,
		UserEmail: opts.UserEmail,
		Role:      string(opts.Role),
	}, &orgID)

	return &AddMemberToProjectResult{
		Membership:     membership,
		InvitationSent: false,
	}, nil
}

// handleNonExistingUser creates an invitation for a user not yet in the organization with project context
// Only sends an invitation if the requester is an admin or owner of the organization
func (uc *ProjectUseCase) handleNonExistingUser(ctx context.Context, orgID, projectID uuid.UUID, opts *AddMemberToProjectOpts) (*AddMemberToProjectResult, error) {
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
	requesterMembership, err := uc.membershipUC.FindByOrgAndUser(ctx, orgID.String(), opts.RequesterID.String())
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

	// Create an organization invitation with project context
	invitationContext := &OrgInvitationContext{
		ProjectIDToJoin: projectID,
		ProjectRole:     opts.Role,
	}

	// Create an invitation for the user to join the organization with project context
	if _, err := uc.orgInvitationUC.Create(ctx, orgID.String(), opts.RequesterID.String(), opts.UserEmail, WithInvitationRole(authz.RoleOrgMember), WithInvitationContext(invitationContext)); err != nil {
		return nil, fmt.Errorf("failed to create invitation: %w", err)
	}

	// Return a result indicating an invitation was sent
	return &AddMemberToProjectResult{
		InvitationSent: true,
	}, nil
}

// addGroupToProject adds a group to a project and logs the action.
func (uc *ProjectUseCase) addGroupToProject(ctx context.Context, orgID uuid.UUID, projectID uuid.UUID, project *Project, opts *AddMemberToProjectOpts) (*ProjectMembership, error) {
	// Validate and resolve the group reference
	resolvedGroupID, err := uc.groupUC.ValidateGroupIdentifier(ctx, orgID, opts.GroupReference.ID, opts.GroupReference.Name)
	if err != nil {
		return nil, fmt.Errorf("failed to validate group reference: %w", err)
	}

	// Check if the group already has membership in the project
	existingMembership, err := uc.projectsRepository.FindProjectMembershipByProjectAndID(ctx, orgID, projectID, resolvedGroupID, authz.MembershipTypeGroup)
	if err != nil && !IsNotFound(err) {
		return nil, fmt.Errorf("failed to check existing group membership: %w", err)
	}
	if existingMembership != nil {
		return nil, NewErrAlreadyExistsStr("group is already a member of this project")
	}

	// Add the group to the project
	membership, err := uc.projectsRepository.AddMemberToProject(ctx, orgID, projectID, resolvedGroupID, authz.MembershipTypeGroup, opts.Role)
	if err != nil {
		return nil, fmt.Errorf("failed to add member to project: %w", err)
	}

	// Get group info for the audit log
	group, err := uc.groupUC.Get(ctx, orgID, &IdentityReference{ID: &resolvedGroupID})
	if err != nil {
		uc.logger.Warnf("failed to get group info for audit log: %v", err)
	}

	// Dispatch event to the audit log for group membership addition
	uc.auditorUC.Dispatch(ctx, &events.ProjectMembershipAdded{
		ProjectBase: &events.ProjectBase{
			ProjectID:   &projectID,
			ProjectName: project.Name,
		},
		GroupID:   &resolvedGroupID,
		GroupName: group.Name,
		Role:      string(opts.Role),
	}, &orgID)

	return membership, nil
}

// RemoveMemberFromProject removes a user or group from a project.
func (uc *ProjectUseCase) RemoveMemberFromProject(ctx context.Context, orgID uuid.UUID, opts *RemoveMemberFromProjectOpts) error {
	if opts == nil {
		return NewErrValidationStr("options cannot be nil")
	}

	if orgID == uuid.Nil {
		return NewErrValidationStr("organization ID cannot be empty")
	}

	// Ensure only one of UserEmail or GroupReference is provided
	if (opts.UserEmail != "" && opts.GroupReference != nil) || (opts.UserEmail == "" && opts.GroupReference == nil) {
		return NewErrValidationStr("exactly one of user email or group reference must be provided")
	}

	// Validate and resolve the project reference
	resolvedProjectID, existingProject, err := uc.validateAndResolveProject(ctx, orgID, opts.ProjectReference)
	if err != nil {
		return err
	}

	// Verify the requester has permissions to remove members from the project (if a requester is provided)
	if opts.RequesterID != uuid.Nil {
		if err := uc.verifyRequesterHasPermissions(ctx, orgID, resolvedProjectID, opts.RequesterID); err != nil {
			return fmt.Errorf("requester does not have permission to remove members from this project: %w", err)
		}
	}

	// Process based on whether we're removing a user or a group
	if opts.UserEmail != "" {
		return uc.removeUserFromProject(ctx, orgID, resolvedProjectID, existingProject, opts)
	}

	return uc.removeGroupFromProject(ctx, orgID, resolvedProjectID, existingProject, opts)
}

// removeUserFromProject removes a user from a project and logs the action.
func (uc *ProjectUseCase) removeUserFromProject(ctx context.Context, orgID uuid.UUID, projectID uuid.UUID, project *Project, opts *RemoveMemberFromProjectOpts) error {
	// Find the user by email in the organization
	userMembership, err := uc.membershipRepository.FindByOrgIDAndUserEmail(ctx, orgID, opts.UserEmail)
	if err != nil && !IsNotFound(err) {
		return fmt.Errorf("failed to find user by email: %w", err)
	}
	if userMembership == nil {
		return NewErrValidationStr("user with the provided email is not a member of the organization")
	}

	userUUID := uuid.MustParse(userMembership.User.ID)

	// Check if the user is a member of the project
	existingMembership, err := uc.projectsRepository.FindProjectMembershipByProjectAndID(ctx, orgID, projectID, userUUID, authz.MembershipTypeUser)
	if err != nil && !IsNotFound(err) {
		return fmt.Errorf("failed to check existing membership: %w", err)
	}
	if existingMembership == nil {
		return NewErrValidationStr("user is not a member of this project")
	}

	// Remove the user from the project
	if err := uc.projectsRepository.RemoveMemberFromProject(ctx, orgID, projectID, userUUID, authz.MembershipTypeUser); err != nil {
		return fmt.Errorf("failed to remove member from project: %w", err)
	}

	// Dispatch event to the audit log for project membership removal
	uc.auditorUC.Dispatch(ctx, &events.ProjectMembershipRemoved{
		ProjectBase: &events.ProjectBase{
			ProjectID:   &projectID,
			ProjectName: project.Name,
		},
		UserID:    &userUUID,
		UserEmail: opts.UserEmail,
	}, &orgID)

	return nil
}

// removeGroupFromProject removes a group from a project and logs the action.
func (uc *ProjectUseCase) removeGroupFromProject(ctx context.Context, orgID uuid.UUID, projectID uuid.UUID, project *Project, opts *RemoveMemberFromProjectOpts) error {
	// Validate and resolve the group reference
	resolvedGroupID, err := uc.groupUC.ValidateGroupIdentifier(ctx, orgID, opts.GroupReference.ID, opts.GroupReference.Name)
	if err != nil {
		return fmt.Errorf("failed to validate group reference: %w", err)
	}

	// Check if the group has membership in the project
	existingMembership, err := uc.projectsRepository.FindProjectMembershipByProjectAndID(ctx, orgID, projectID, resolvedGroupID, authz.MembershipTypeGroup)
	if err != nil && !IsNotFound(err) {
		return fmt.Errorf("failed to check existing group membership: %w", err)
	}
	if existingMembership == nil {
		return NewErrValidationStr("group is not a member of this project")
	}

	// Delete the membership
	if err := uc.projectsRepository.RemoveMemberFromProject(ctx, orgID, projectID, resolvedGroupID, authz.MembershipTypeGroup); err != nil {
		return fmt.Errorf("failed to remove member from project: %w", err)
	}

	// Get group info for the audit log
	group, err := uc.groupUC.Get(ctx, orgID, &IdentityReference{ID: &resolvedGroupID})
	if err != nil {
		uc.logger.Warnf("failed to get group info for audit log: %v", err)
	}

	// Dispatch event to the audit log for group membership removal
	uc.auditorUC.Dispatch(ctx, &events.ProjectMembershipRemoved{
		ProjectBase: &events.ProjectBase{
			ProjectID:   &projectID,
			ProjectName: project.Name,
		},
		GroupID:   &resolvedGroupID,
		GroupName: group.Name,
	}, &orgID)

	return nil
}

// validateAndResolveProject is a helper method that validates and resolves a project reference.
// It checks if the project exists and returns the project ID and the project itself.
func (uc *ProjectUseCase) validateAndResolveProject(ctx context.Context, orgID uuid.UUID, projectRef *IdentityReference) (uuid.UUID, *Project, error) {
	if projectRef == nil {
		return uuid.Nil, nil, NewErrValidationStr("project reference cannot be nil")
	}

	// Validate and resolve the project reference to a project ID
	resolvedProjectID, err := uc.ValidateProjectIdentifier(ctx, orgID, projectRef)
	if err != nil {
		return uuid.Nil, nil, err
	}

	// Check the project exists
	existingProject, err := uc.projectsRepository.FindProjectByOrgIDAndID(ctx, orgID, resolvedProjectID)
	if err != nil {
		return uuid.Nil, nil, fmt.Errorf("failed to find project: %w", err)
	}

	if existingProject == nil {
		return uuid.Nil, nil, NewErrNotFound("project")
	}

	return resolvedProjectID, existingProject, nil
}

// ValidateProjectIdentifier validates and resolves the project reference to a project ID.
func (uc *ProjectUseCase) ValidateProjectIdentifier(ctx context.Context, orgID uuid.UUID, projectRef *IdentityReference) (uuid.UUID, error) {
	if projectRef == nil {
		return uuid.Nil, NewErrValidationStr("project reference cannot be nil")
	}

	if projectRef.ID == nil && projectRef.Name == nil {
		return uuid.Nil, NewErrValidationStr("either project ID or project name must be provided")
	}

	if projectRef.ID != nil {
		return *projectRef.ID, nil
	}

	// If project ID is not provided, try to find the project by name
	project, err := uc.projectsRepository.FindProjectByOrgIDAndName(ctx, orgID, *projectRef.Name)
	if err != nil {
		return uuid.Nil, fmt.Errorf("failed to find project by name: %w", err)
	}

	if project == nil {
		return uuid.Nil, NewErrNotFound("project")
	}

	return project.ID, nil
}

// verifyRequesterHasPermissions checks if the requester has the required permissions to perform an action on a project.
func (uc *ProjectUseCase) verifyRequesterHasPermissions(ctx context.Context, orgID uuid.UUID, projectID uuid.UUID, requesterID uuid.UUID) error {
	// Check if the requester is part of the organization
	requesterMemberships, err := uc.membershipUC.ListAllMembershipsForUser(ctx, requesterID)
	if err != nil {
		return NewErrValidationStr("failed to check existing membership")
	}

	if len(requesterMemberships) == 0 {
		return NewErrValidationStr("requester is not a member of the organization")
	}

	// First check: verify the requester has an active membership in the specified organization
	var currentOrgMembership *Membership
	for _, m := range requesterMemberships {
		if m.Current && m.ResourceType == authz.ResourceTypeOrganization && m.OrganizationID == orgID {
			currentOrgMembership = m
			break
		}
	}

	if currentOrgMembership == nil {
		return NewErrValidationStr("requester is not a member of the organization")
	}

	// Second check: if requester is an org owner or admin, they have all permissions
	if currentOrgMembership.Role == authz.RoleOwner || currentOrgMembership.Role == authz.RoleAdmin {
		return nil
	}

	// Third check: verify the requester has admin permissions for this specific project
	hasProjectAdminRole := false
	for _, m := range requesterMemberships {
		if m.ResourceType == authz.ResourceTypeProject &&
			m.ResourceID == projectID &&
			m.Role == authz.RoleProjectAdmin &&
			m.OrganizationID == orgID {
			hasProjectAdminRole = true
			break
		}
	}

	if !hasProjectAdminRole {
		return NewErrValidationStr("requester does not have permission to operate on this project")
	}

	return nil
}

// getProjectsWithMembership returns the list of project IDs in the org for which the user has a membership
func getProjectsWithMembershipInOrg(orgID uuid.UUID, memberships []*Membership) []uuid.UUID {
	ids := make([]uuid.UUID, 0)
	for _, m := range memberships {
		if m.ResourceType == authz.ResourceTypeProject && m.OrganizationID == orgID {
			ids = append(ids, m.ResourceID)
		}
	}

	return ids
}

// UpdateMemberRole updates the role of a user or group in a project.
func (uc *ProjectUseCase) UpdateMemberRole(ctx context.Context, orgID uuid.UUID, opts *UpdateMemberRoleOpts) error {
	if opts == nil {
		return NewErrValidationStr("options cannot be nil")
	}

	if orgID == uuid.Nil {
		return NewErrValidationStr("organization ID cannot be empty")
	}

	// Ensure only one of UserEmail or GroupReference is provided
	if (opts.UserEmail != "" && opts.GroupReference != nil) || (opts.UserEmail == "" && opts.GroupReference == nil) {
		return NewErrValidationStr("exactly one of user email or group reference must be provided")
	}

	// Validate the role
	if opts.NewRole != authz.RoleProjectAdmin && opts.NewRole != authz.RoleProjectViewer {
		return NewErrValidationStr("role must be either 'admin' or 'viewer'")
	}

	// Validate and resolve the project reference
	resolvedProjectID, existingProject, err := uc.validateAndResolveProject(ctx, orgID, opts.ProjectReference)
	if err != nil {
		return err
	}

	// Verify the requester has permissions to update member roles in the project (if a requester is provided)
	if opts.RequesterID != uuid.Nil {
		if err := uc.verifyRequesterHasPermissions(ctx, orgID, resolvedProjectID, opts.RequesterID); err != nil {
			return fmt.Errorf("requester does not have permission to update member roles in this project: %w", err)
		}
	}

	// Process based on whether we're updating a user or a group
	if opts.UserEmail != "" {
		return uc.updateUserRoleInProject(ctx, orgID, resolvedProjectID, existingProject, opts)
	}

	return uc.updateGroupRoleInProject(ctx, orgID, resolvedProjectID, existingProject, opts)
}

// updateUserRoleInProject updates the role of a user in a project and logs the action.
func (uc *ProjectUseCase) updateUserRoleInProject(ctx context.Context, orgID uuid.UUID, projectID uuid.UUID, project *Project, opts *UpdateMemberRoleOpts) error {
	// Find the user by email in the organization
	userMembership, err := uc.membershipRepository.FindByOrgIDAndUserEmail(ctx, orgID, opts.UserEmail)
	if err != nil && !IsNotFound(err) {
		return fmt.Errorf("failed to find user by email: %w", err)
	}
	if userMembership == nil {
		return NewErrValidationStr("user with the provided email is not a member of the organization")
	}

	userUUID := uuid.MustParse(userMembership.User.ID)

	// Check if the user is a member of the project
	existingMembership, err := uc.projectsRepository.FindProjectMembershipByProjectAndID(ctx, orgID, projectID, userUUID, authz.MembershipTypeUser)
	if err != nil && !IsNotFound(err) {
		return fmt.Errorf("failed to check existing membership: %w", err)
	}
	if existingMembership == nil {
		return NewErrValidationStr("user is not a member of this project")
	}

	// If the role is already the requested role, no need to update
	if existingMembership.Role == opts.NewRole {
		return nil
	}

	// Update the membership role
	if _, upErr := uc.projectsRepository.UpdateMemberRoleInProject(ctx, orgID, projectID, userUUID, authz.MembershipTypeUser, opts.NewRole); upErr != nil {
		return fmt.Errorf("failed to update membership with new role: %w", upErr)
	}

	// Dispatch event to the audit log for role update
	uc.auditorUC.Dispatch(ctx, &events.ProjectMemberRoleUpdated{
		ProjectBase: &events.ProjectBase{
			ProjectID:   &projectID,
			ProjectName: project.Name,
		},
		UserID:    &userUUID,
		UserEmail: opts.UserEmail,
		NewRole:   string(opts.NewRole),
		OldRole:   string(existingMembership.Role),
	}, &orgID)

	return nil
}

// updateGroupRoleInProject updates the role of a group in a project and logs the action.
func (uc *ProjectUseCase) updateGroupRoleInProject(ctx context.Context, orgID uuid.UUID, projectID uuid.UUID, project *Project, opts *UpdateMemberRoleOpts) error {
	// Validate and resolve the group reference
	resolvedGroupID, err := uc.groupUC.ValidateGroupIdentifier(ctx, orgID, opts.GroupReference.ID, opts.GroupReference.Name)
	if err != nil {
		return fmt.Errorf("failed to validate group reference: %w", err)
	}

	// Check if the group has membership in the project
	existingMembership, err := uc.projectsRepository.FindProjectMembershipByProjectAndID(ctx, orgID, projectID, resolvedGroupID, authz.MembershipTypeGroup)
	if err != nil && !IsNotFound(err) {
		return fmt.Errorf("failed to check existing group membership: %w", err)
	}
	if existingMembership == nil {
		return NewErrValidationStr("group is not a member of this project")
	}

	// If the role is already the requested role, no need to update
	if existingMembership.Role == opts.NewRole {
		return nil
	}

	// Update the membership role
	if _, upErr := uc.projectsRepository.UpdateMemberRoleInProject(ctx, orgID, projectID, resolvedGroupID, authz.MembershipTypeGroup, opts.NewRole); upErr != nil {
		return fmt.Errorf("failed to update membership with new role: %w", upErr)
	}

	// Get group info for the audit log
	group, err := uc.groupUC.Get(ctx, orgID, &IdentityReference{ID: &resolvedGroupID})
	if err != nil {
		uc.logger.Warnf("failed to get group info for audit log: %v", err)
	}

	// Dispatch event to the audit log for role update using the unified event
	uc.auditorUC.Dispatch(ctx, &events.ProjectMemberRoleUpdated{
		ProjectBase: &events.ProjectBase{
			ProjectID:   &projectID,
			ProjectName: project.Name,
		},
		GroupID:   &resolvedGroupID,
		GroupName: group.Name,
		NewRole:   string(opts.NewRole),
		OldRole:   string(existingMembership.Role),
	}, &orgID)

	return nil
}
