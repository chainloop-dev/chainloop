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

package service

import (
	"context"
	"fmt"
	"time"

	pb "github.com/chainloop-dev/chainloop/app/controlplane/api/controlplane/v1"
	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/authz"
	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/biz"

	"github.com/go-kratos/kratos/v2/errors"
	"github.com/google/uuid"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type ProjectService struct {
	pb.UnimplementedProjectServiceServer
	*service

	// Use Cases
	apiTokenUseCase *biz.APITokenUseCase
}

func NewProjectService(apiTokenUseCase *biz.APITokenUseCase, opts ...NewOpt) *ProjectService {
	return &ProjectService{
		service:         newService(opts...),
		apiTokenUseCase: apiTokenUseCase,
	}
}

func (s *ProjectService) APITokenCreate(ctx context.Context, req *pb.ProjectServiceAPITokenCreateRequest) (*pb.ProjectServiceAPITokenCreateResponse, error) {
	currentOrg, err := requireCurrentOrg(ctx)
	if err != nil {
		return nil, err
	}

	// Make sure the provided project exists and the user has permission to create tokens in it
	project, err := s.userHasPermissionOnProject(ctx, currentOrg.ID, &pb.IdentityReference{Name: &req.ProjectName}, authz.PolicyProjectAPITokenCreate)
	if err != nil {
		return nil, err
	}

	var expiresIn *time.Duration
	if req.ExpiresIn != nil {
		expiresIn = new(time.Duration)
		*expiresIn = req.ExpiresIn.AsDuration()
	}

	token, err := s.apiTokenUseCase.Create(ctx, req.Name, req.Description, expiresIn, currentOrg.ID, biz.APITokenWithProject(project))
	if err != nil {
		return nil, handleUseCaseErr(err, s.log)
	}

	return &pb.ProjectServiceAPITokenCreateResponse{
		Result: &pb.ProjectServiceAPITokenCreateResponse_APITokenFull{
			Item: apiTokenBizToPb(token),
			Jwt:  token.JWT,
		},
	}, nil
}

func (s *ProjectService) APITokenList(ctx context.Context, req *pb.ProjectServiceAPITokenListRequest) (*pb.ProjectServiceAPITokenListResponse, error) {
	currentOrg, err := requireCurrentOrg(ctx)
	if err != nil {
		return nil, err
	}

	// Make sure the provided project exists and the user has permission to create tokens in it
	project, err := s.userHasPermissionOnProject(ctx, currentOrg.ID, req.GetProjectReference(), authz.PolicyProjectAPITokenList)
	if err != nil {
		return nil, err
	}

	tokens, err := s.apiTokenUseCase.List(ctx, currentOrg.ID, req.IncludeRevoked, biz.APITokenWithProject(project))
	if err != nil {
		return nil, handleUseCaseErr(err, s.log)
	}

	result := make([]*pb.APITokenItem, 0, len(tokens))
	for _, p := range tokens {
		result = append(result, apiTokenBizToPb(p))
	}

	return &pb.ProjectServiceAPITokenListResponse{Result: result}, nil
}

func (s *ProjectService) APITokenRevoke(ctx context.Context, req *pb.ProjectServiceAPITokenRevokeRequest) (*pb.ProjectServiceAPITokenRevokeResponse, error) {
	currentOrg, err := requireCurrentOrg(ctx)
	if err != nil {
		return nil, err
	}

	// Make sure the provided project exists and the user has permission to create tokens in it
	project, err := s.userHasPermissionOnProject(ctx, currentOrg.ID, req.GetProjectReference(), authz.PolicyProjectAPITokenRevoke)
	if err != nil {
		return nil, err
	}

	t, err := s.apiTokenUseCase.FindByNameInOrg(ctx, currentOrg.ID, req.Name, biz.APITokenWithProject(project))
	if err != nil {
		return nil, handleUseCaseErr(err, s.log)
	}

	if err := s.apiTokenUseCase.Revoke(ctx, currentOrg.ID, t.ID.String()); err != nil {
		return nil, handleUseCaseErr(err, s.log)
	}

	return &pb.ProjectServiceAPITokenRevokeResponse{}, nil
}

// ListMembers lists the members of a project.
func (s *ProjectService) ListMembers(ctx context.Context, req *pb.ProjectServiceListMembersRequest) (*pb.ProjectServiceListMembersResponse, error) {
	currentOrg, err := requireCurrentOrg(ctx)
	if err != nil {
		return nil, err
	}

	// Make sure the user has permission to list members of the project
	_, err = s.userHasPermissionOnProject(ctx, currentOrg.ID, req.ProjectReference, authz.PolicyProjectListMemberships)
	if err != nil {
		return nil, err
	}

	// Convert organization ID from string to UUID
	orgUUID, err := uuid.Parse(currentOrg.ID)
	if err != nil {
		return nil, handleUseCaseErr(err, s.log)
	}

	// Create the identity reference for the project
	identityRef := &biz.IdentityReference{}

	// Parse projectID and projectName from the request
	identityRef.ID, identityRef.Name, err = req.GetProjectReference().Parse()
	if err != nil {
		return nil, errors.BadRequest("invalid", fmt.Sprintf("invalid project reference: %s", err.Error()))
	}

	// Initialize the pagination options, with default values
	paginationOpts, err := initializePaginationOpts(req.GetPagination())
	if err != nil {
		return nil, handleUseCaseErr(err, s.log)
	}

	// Call the business logic to list members
	members, total, err := s.projectUseCase.ListMembers(ctx, orgUUID, identityRef, paginationOpts)
	if err != nil {
		return nil, handleUseCaseErr(err, s.log)
	}

	// Convert the project members to protobuf messages
	result := make([]*pb.ProjectMember, 0, len(members))
	for _, mem := range members {
		result = append(result, bizProjectMembershipToPb(mem))
	}

	return &pb.ProjectServiceListMembersResponse{
		Members:    result,
		Pagination: paginationToPb(total, paginationOpts.Offset(), paginationOpts.Limit()),
	}, nil
}

// AddMember adds a member to a project.
func (s *ProjectService) AddMember(ctx context.Context, req *pb.ProjectServiceAddMemberRequest) (*pb.ProjectServiceAddMemberResponse, error) {
	currentOrg, err := requireCurrentOrg(ctx)
	if err != nil {
		return nil, err
	}

	// Make sure the user has permission to add members to the project
	_, err = s.userHasPermissionOnProject(ctx, currentOrg.ID, req.ProjectReference, authz.PolicyProjectAddMemberships)
	if err != nil {
		return nil, err
	}

	// Get current user ID from context
	currentUser, err := requireCurrentUser(ctx)
	if err != nil {
		return nil, err
	}

	requesterUUID, err := uuid.Parse(currentUser.ID)
	if err != nil {
		return nil, handleUseCaseErr(err, s.log)
	}

	orgUUID, err := uuid.Parse(currentOrg.ID)
	if err != nil {
		return nil, handleUseCaseErr(err, s.log)
	}

	// Create the identity reference for the project
	identityRef := &biz.IdentityReference{}

	// Parse projectID and projectName from the request
	identityRef.ID, identityRef.Name, err = req.GetProjectReference().Parse()
	if err != nil {
		return nil, errors.BadRequest("invalid", fmt.Sprintf("invalid project reference: %s", err.Error()))
	}

	// Extract the user email and group reference from the membership reference field
	userEmail, groupReference, err := s.extractMembershipReference(req.GetMemberReference())
	if err != nil {
		return nil, handleUseCaseErr(err, s.log)
	}

	// Convert from protobuf role to internal authorization role
	role := mapProjectMemberRoleToAuthzRole(req.Role)

	// Prepare options for adding a member
	opts := &biz.AddMemberToProjectOpts{
		ProjectReference: identityRef,
		UserEmail:        userEmail,
		GroupReference:   groupReference,
		RequesterID:      requesterUUID,
		Role:             role,
	}

	// Call the business logic to add the member
	_, err = s.projectUseCase.AddMemberToProject(ctx, orgUUID, opts)
	if err != nil {
		return nil, handleUseCaseErr(err, s.log)
	}

	return &pb.ProjectServiceAddMemberResponse{}, nil
}

// RemoveMember removes a member from a project.
func (s *ProjectService) RemoveMember(ctx context.Context, req *pb.ProjectServiceRemoveMemberRequest) (*pb.ProjectServiceRemoveMemberResponse, error) {
	currentOrg, err := requireCurrentOrg(ctx)
	if err != nil {
		return nil, err
	}

	// Make sure the user has permission to remove members from the project
	_, err = s.userHasPermissionOnProject(ctx, currentOrg.ID, req.ProjectReference, authz.PolicyProjectRemoveMemberships)
	if err != nil {
		return nil, err
	}

	// Get current user ID from context
	currentUser, err := requireCurrentUser(ctx)
	if err != nil {
		return nil, err
	}

	requesterUUID, err := uuid.Parse(currentUser.ID)
	if err != nil {
		return nil, handleUseCaseErr(err, s.log)
	}

	orgUUID, err := uuid.Parse(currentOrg.ID)
	if err != nil {
		return nil, handleUseCaseErr(err, s.log)
	}

	// Create the identity reference for the project
	identityRef := &biz.IdentityReference{}

	// Parse projectID and projectName from the request
	identityRef.ID, identityRef.Name, err = req.GetProjectReference().Parse()
	if err != nil {
		return nil, errors.BadRequest("invalid", fmt.Sprintf("invalid project reference: %s", err.Error()))
	}

	// Extract the user email and group reference from the membership reference field
	userEmail, groupReference, err := s.extractMembershipReference(req.GetMemberReference())
	if err != nil {
		return nil, handleUseCaseErr(err, s.log)
	}

	// Prepare options for removing a member
	opts := &biz.RemoveMemberFromProjectOpts{
		ProjectReference: identityRef,
		UserEmail:        userEmail,
		GroupReference:   groupReference,
		RequesterID:      requesterUUID,
	}

	// Call the business logic to remove the member
	err = s.projectUseCase.RemoveMemberFromProject(ctx, orgUUID, opts)
	if err != nil {
		return nil, handleUseCaseErr(err, s.log)
	}

	return &pb.ProjectServiceRemoveMemberResponse{}, nil
}

// UpdateMemberRole updates the role of a user or group in a project.
func (s *ProjectService) UpdateMemberRole(ctx context.Context, req *pb.ProjectServiceUpdateMemberRoleRequest) (*pb.ProjectServiceUpdateMemberRoleResponse, error) {
	currentOrg, err := requireCurrentOrg(ctx)
	if err != nil {
		return nil, err
	}

	// Make sure the provided project exists and the user has permission to update member roles in it
	_, err = s.userHasPermissionOnProject(ctx, currentOrg.ID, req.GetProjectReference(), authz.PolicyProjectUpdateMemberships)
	if err != nil {
		return nil, err
	}

	// Get current user ID from context
	currentUser, err := requireCurrentUser(ctx)
	if err != nil {
		return nil, err
	}

	requesterUUID, err := uuid.Parse(currentUser.ID)
	if err != nil {
		return nil, handleUseCaseErr(err, s.log)
	}

	orgUUID, err := uuid.Parse(currentOrg.ID)
	if err != nil {
		return nil, handleUseCaseErr(err, s.log)
	}

	// Extract the user email and group reference from the membership reference field
	userEmail, groupReference, err := s.extractMembershipReference(req.GetMemberReference())
	if err != nil {
		return nil, handleUseCaseErr(err, s.log)
	}

	// Convert from protobuf role to internal authorization role
	newRole := mapProjectMemberRoleToAuthzRole(req.NewRole)

	// Prepare options for updating a member's role
	opts := &biz.UpdateMemberRoleOpts{
		ProjectReference: &biz.IdentityReference{},
		UserEmail:        userEmail,
		GroupReference:   groupReference,
		RequesterID:      requesterUUID,
		NewRole:          newRole,
	}

	// Parse projectID and projectName from the request
	opts.ProjectReference.ID, opts.ProjectReference.Name, err = req.GetProjectReference().Parse()
	if err != nil {
		return nil, errors.BadRequest("invalid", fmt.Sprintf("invalid project reference: %s", err.Error()))
	}

	// Call the business logic to update the member's role
	if err := s.projectUseCase.UpdateMemberRole(ctx, orgUUID, opts); err != nil {
		return nil, handleUseCaseErr(err, s.log)
	}

	return &pb.ProjectServiceUpdateMemberRoleResponse{}, nil
}

// ListPendingInvitations retrieves a list of pending invitations for a project
func (s *ProjectService) ListPendingInvitations(ctx context.Context, req *pb.ProjectServiceListPendingInvitationsRequest) (*pb.ProjectServiceListPendingInvitationsResponse, error) {
	currentOrg, err := requireCurrentOrg(ctx)
	if err != nil {
		return nil, err
	}

	// Parse orgID
	orgUUID, err := uuid.Parse(currentOrg.ID)
	if err != nil {
		return nil, errors.BadRequest("invalid", "invalid organization ID")
	}

	// Make sure the user has permission to list members of the project
	_, err = s.userHasPermissionOnProject(ctx, currentOrg.ID, req.ProjectReference, authz.PolicyProjectListMemberships)
	if err != nil {
		return nil, err
	}

	// Parse groupID and projectName from the request
	projectID, projectName, err := req.GetProjectReference().Parse()
	if err != nil {
		return nil, errors.BadRequest("invalid", fmt.Sprintf("invalid project reference: %s", err.Error()))
	}

	// Initialize the pagination options, with default values
	paginationOpts, err := initializePaginationOpts(req.GetPagination())
	if err != nil {
		return nil, handleUseCaseErr(err, s.log)
	}

	// Call the business logic to list pending invitations
	invitations, count, err := s.projectUseCase.ListPendingInvitations(ctx, orgUUID, &biz.IdentityReference{
		ID:   projectID,
		Name: projectName,
	}, paginationOpts)
	if err != nil {
		return nil, handleUseCaseErr(err, s.log)
	}

	// Convert business objects to protobuf messages
	pbInvitations := make([]*pb.PendingProjectInvitation, 0, len(invitations))
	for _, invitation := range invitations {
		pbInvitations = append(pbInvitations, bizOrgInvitationToPendingProjectInvitationPb(invitation))
	}

	return &pb.ProjectServiceListPendingInvitationsResponse{
		Invitations: pbInvitations,
		Pagination:  paginationToPb(count, paginationOpts.Offset(), paginationOpts.Limit()),
	}, nil
}

// extractMembershipReference extracts either a user email or a group reference from a membership reference
// If both or neither are provided, returns an error
func (s *ProjectService) extractMembershipReference(membershipRef *pb.ProjectMembershipReference) (string, *biz.IdentityReference, error) {
	if membershipRef == nil {
		return "", nil, biz.NewErrValidationStr("membership reference is required")
	}

	// Check if the membershipRef has a user email
	userEmail := membershipRef.GetUserEmail()
	groupRef := membershipRef.GetGroupReference()

	// Validate that exactly one of user email or group reference is provided
	if (userEmail == "" && groupRef == nil) || (userEmail != "" && groupRef != nil) {
		return "", nil, biz.NewErrValidationStr("exactly one of user email or group reference must be provided")
	}

	// If we have a user email, return it and nil for group reference
	if userEmail != "" {
		return userEmail, nil, nil
	}

	// Otherwise, create a new IdentityReference from the group reference
	identityRef := &biz.IdentityReference{}
	var err error

	identityRef.ID, identityRef.Name, err = groupRef.Parse()
	if err != nil {
		return "", nil, errors.BadRequest("invalid_group_reference", fmt.Sprintf("invalid group reference: %s", err.Error()))
	}

	if identityRef.ID == nil && identityRef.Name == nil {
		return "", nil, biz.NewErrValidationStr("either group ID or name must be provided")
	}

	return "", identityRef, nil
}

// MapProjectMemberRoleToAuthzRole maps a ProjectMemberRole from protobuf to an authz.Role
func mapProjectMemberRoleToAuthzRole(role pb.ProjectMemberRole) authz.Role {
	switch role {
	case pb.ProjectMemberRole_PROJECT_MEMBER_ROLE_ADMIN:
		return authz.RoleProjectAdmin
	case pb.ProjectMemberRole_PROJECT_MEMBER_ROLE_VIEWER:
		return authz.RoleProjectViewer
	default:
		// Default to viewer role for safety
		return authz.RoleProjectViewer
	}
}

// bizProjectMembershipToPb converts a biz.ProjectMembership to a pb.ProjectMember
func bizProjectMembershipToPb(m *biz.ProjectMembership) *pb.ProjectMember {
	var role pb.ProjectMemberRole

	// Map the role string back to a protobuf enum
	switch m.Role {
	case authz.RoleProjectAdmin:
		role = pb.ProjectMemberRole_PROJECT_MEMBER_ROLE_ADMIN
	case authz.RoleProjectViewer:
		role = pb.ProjectMemberRole_PROJECT_MEMBER_ROLE_VIEWER
	default:
		role = pb.ProjectMemberRole_PROJECT_MEMBER_ROLE_UNSPECIFIED
	}

	pbMember := &pb.ProjectMember{
		Role: role,
	}

	if m.User != nil {
		pbMember.Subject = &pb.ProjectMember_User{
			User: bizUserToPb(m.User),
		}
	}

	if m.Group != nil {
		pbMember.Subject = &pb.ProjectMember_Group{
			Group: bizGroupToPb(m.Group),
		}
	}

	if m.CreatedAt != nil {
		pbMember.CreatedAt = timestamppb.New(*m.CreatedAt)
	}
	if m.UpdatedAt != nil {
		pbMember.UpdatedAt = timestamppb.New(*m.UpdatedAt)
	}

	return pbMember
}

// bizOrgInvitationToPendingProjectInvitationPb converts a biz.OrgInvitation to a pb.PendingProjectInvitation protobuf message.
func bizOrgInvitationToPendingProjectInvitationPb(inv *biz.OrgInvitation) *pb.PendingProjectInvitation {
	base := &pb.PendingProjectInvitation{
		InvitationId: inv.ID.String(),
		UserEmail:    inv.ReceiverEmail,
		CreatedAt:    timestamppb.New(*inv.CreatedAt),
	}

	// Include the sender if available
	if inv.Sender != nil {
		base.InvitedBy = bizUserToPb(inv.Sender)
	}

	return base
}
