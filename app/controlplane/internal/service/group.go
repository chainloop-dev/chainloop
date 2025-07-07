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

	pb "github.com/chainloop-dev/chainloop/app/controlplane/api/controlplane/v1"
	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/biz"

	"github.com/go-kratos/kratos/v2/errors"
	"github.com/google/uuid"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type GroupService struct {
	pb.UnimplementedGroupServiceServer
	*service
	// Use Cases
	groupUseCase *biz.GroupUseCase
}

func NewGroupService(groupUseCase *biz.GroupUseCase, opts ...NewOpt) *GroupService {
	return &GroupService{
		service:      newService(opts...),
		groupUseCase: groupUseCase,
	}
}

// Create creates a new group in the organization.
func (g *GroupService) Create(ctx context.Context, req *pb.GroupServiceCreateRequest) (*pb.GroupServiceCreateResponse, error) {
	currentOrg, err := requireCurrentOrg(ctx)
	if err != nil {
		return nil, err
	}

	currentUser, err := requireCurrentUser(ctx)
	if err != nil {
		return nil, err
	}

	// Parse userUUID (current user)
	userUUID, err := uuid.Parse(currentUser.ID)
	if err != nil {
		return nil, errors.BadRequest("invalid", "invalid user ID")
	}

	// Parse orgID
	orgUUID, err := uuid.Parse(currentOrg.ID)
	if err != nil {
		return nil, errors.BadRequest("invalid", "invalid organization ID")
	}

	gr, err := g.groupUseCase.Create(ctx, orgUUID, req.Name, req.Description, userUUID)
	if err != nil {
		return nil, handleUseCaseErr(err, g.log)
	}

	return &pb.GroupServiceCreateResponse{
		Group: bizGroupToPb(gr),
	}, nil
}

// Get retrieves a group by its ID within the current organization.
func (g *GroupService) Get(ctx context.Context, req *pb.GroupServiceGetRequest) (*pb.GroupServiceGetResponse, error) {
	currentOrg, err := requireCurrentOrg(ctx)
	if err != nil {
		return nil, err
	}

	// Parse orgID
	orgUUID, err := uuid.Parse(currentOrg.ID)
	if err != nil {
		return nil, errors.BadRequest("invalid", "invalid organization ID")
	}

	// Parse groupID and groupName from the request
	id, name, err := req.GetGroupReference().Parse()
	if err != nil {
		return nil, errors.BadRequest("invalid", fmt.Sprintf("invalid group reference: %s", err.Error()))
	}

	// Initialize the options for getting the group
	opts := &biz.IdentityReference{
		ID:   id,
		Name: name,
	}

	gr, err := g.groupUseCase.Get(ctx, orgUUID, opts)
	if err != nil {
		return nil, handleUseCaseErr(err, g.log)
	}

	return &pb.GroupServiceGetResponse{
		Group: bizGroupToPb(gr),
	}, nil
}

// List retrieves a list of groups within the current organization, with optional filters and pagination.
func (g *GroupService) List(ctx context.Context, req *pb.GroupServiceListRequest) (*pb.GroupServiceListResponse, error) {
	currentOrg, err := requireCurrentOrg(ctx)
	if err != nil {
		return nil, err
	}

	// Parse orgID
	orgUUID, err := uuid.Parse(currentOrg.ID)
	if err != nil {
		return nil, errors.BadRequest("invalid", "invalid organization ID")
	}

	// Initialize the pagination options, with default values
	paginationOpts, err := initializePaginationOpts(req.GetPagination())
	if err != nil {
		return nil, handleUseCaseErr(err, g.log)
	}

	// Initialize the filters
	filters := &biz.ListGroupOpts{}

	if req.GetName() != "" {
		filters.Name = req.GetName()
	}

	if req.GetDescription() != "" {
		filters.Description = req.GetDescription()
	}

	if req.GetMemberEmail() != "" {
		filters.MemberEmail = req.GetMemberEmail()
	}

	grs, count, err := g.groupUseCase.List(ctx, orgUUID, filters, paginationOpts)
	if err != nil {
		return nil, handleUseCaseErr(err, g.log)
	}

	// Convert the groups to protobuf messages
	result := make([]*pb.Group, 0, len(grs))
	for _, gr := range grs {
		result = append(result, bizGroupToPb(gr))
	}
	return &pb.GroupServiceListResponse{
		Groups:     result,
		Pagination: paginationToPb(count, paginationOpts.Offset(), paginationOpts.Limit()),
	}, nil
}

// Update updates an existing group in the organization.
func (g *GroupService) Update(ctx context.Context, req *pb.GroupServiceUpdateRequest) (*pb.GroupServiceUpdateResponse, error) {
	currentOrg, err := requireCurrentOrg(ctx)
	if err != nil {
		return nil, err
	}

	// Parse orgID
	orgUUID, err := uuid.Parse(currentOrg.ID)
	if err != nil {
		return nil, errors.BadRequest("invalid", "invalid organization ID")
	}

	// Parse groupID and groupName from the request
	id, name, err := req.GetGroupReference().Parse()
	if err != nil {
		return nil, errors.BadRequest("invalid", fmt.Sprintf("invalid project reference: %s", err.Error()))
	}

	// Update the group with the provided options
	gr, err := g.groupUseCase.Update(ctx, orgUUID, &biz.IdentityReference{
		Name: name,
		ID:   id,
	}, &biz.UpdateGroupOpts{
		NewDescription: req.NewDescription,
		NewName:        req.NewName,
	})
	if err != nil {
		return nil, handleUseCaseErr(err, g.log)
	}

	return &pb.GroupServiceUpdateResponse{
		Group: bizGroupToPb(gr),
	}, nil
}

// Delete soft-deletes a group by its ID within the current organization.
func (g *GroupService) Delete(ctx context.Context, req *pb.GroupServiceDeleteRequest) (*pb.GroupServiceDeleteResponse, error) {
	currentOrg, err := requireCurrentOrg(ctx)
	if err != nil {
		return nil, err
	}

	// Parse orgID
	orgUUID, err := uuid.Parse(currentOrg.ID)
	if err != nil {
		return nil, errors.BadRequest("invalid", "invalid organization ID")
	}

	// Initialize the options for deleting the group
	idReference := &biz.IdentityReference{}

	// Parse groupID and groupName from the request
	idReference.ID, idReference.Name, err = req.GetGroupReference().Parse()
	if err != nil {
		return nil, errors.BadRequest("invalid", fmt.Sprintf("invalid project reference: %s", err.Error()))
	}

	err = g.groupUseCase.Delete(ctx, orgUUID, idReference)
	if err != nil {
		return nil, handleUseCaseErr(err, g.log)
	}

	return &pb.GroupServiceDeleteResponse{}, nil
}

// ListMembers retrieves a list of members in a group within the current organization, with optional filters and pagination.
func (g *GroupService) ListMembers(ctx context.Context, req *pb.GroupServiceListMembersRequest) (*pb.GroupServiceListMembersResponse, error) {
	currentOrg, err := requireCurrentOrg(ctx)
	if err != nil {
		return nil, err
	}

	// Parse orgID
	orgUUID, err := uuid.Parse(currentOrg.ID)
	if err != nil {
		return nil, errors.BadRequest("invalid", "invalid organization ID")
	}

	// Initialize the options for listing members
	opts := &biz.ListMembersOpts{
		IdentityReference: &biz.IdentityReference{},
		Maintainers:       req.Maintainers,
		MemberEmail:       req.MemberEmail,
	}

	// Parse groupID and groupName from the request
	opts.ID, opts.Name, err = req.GetGroupReference().Parse()
	if err != nil {
		return nil, errors.BadRequest("invalid", fmt.Sprintf("invalid project reference: %s", err.Error()))
	}

	// Initialize the pagination options, with default values
	paginationOpts, err := initializePaginationOpts(req.GetPagination())
	if err != nil {
		return nil, handleUseCaseErr(err, g.log)
	}

	grs, count, err := g.groupUseCase.ListMembers(ctx, orgUUID, opts, paginationOpts)
	if err != nil {
		return nil, handleUseCaseErr(err, g.log)
	}

	// Convert the group members to protobuf messages
	result := make([]*pb.GroupMember, 0, len(grs))
	for _, gr := range grs {
		result = append(result, bizGroupMemberToPb(gr))
	}
	return &pb.GroupServiceListMembersResponse{
		Members:    result,
		Pagination: paginationToPb(count, paginationOpts.Offset(), paginationOpts.Limit()),
	}, nil
}

// AddMember adds a member to a group within the current organization.
func (g *GroupService) AddMember(ctx context.Context, req *pb.GroupServiceAddMemberRequest) (*pb.GroupServiceAddMemberResponse, error) {
	currentUser, err := requireCurrentUser(ctx)
	if err != nil {
		return nil, err
	}
	currentOrg, err := requireCurrentOrg(ctx)
	if err != nil {
		return nil, err
	}

	if err := g.userHasPermissionToAddGroupMember(ctx, currentOrg.ID, req.GetGroupReference()); err != nil {
		return nil, err
	}

	// Parse orgID
	orgUUID, err := uuid.Parse(currentOrg.ID)
	if err != nil {
		return nil, errors.BadRequest("invalid", "invalid organization ID")
	}

	// Parse requesterID (current user)
	requesterUUID, err := uuid.Parse(currentUser.ID)
	if err != nil {
		return nil, errors.BadRequest("invalid", "invalid user ID")
	}

	// Create options for adding the member
	addOpts := &biz.AddMemberToGroupOpts{
		IdentityReference: &biz.IdentityReference{},
		UserEmail:         req.GetUserEmail(),
		RequesterID:       requesterUUID,
		Maintainer:        req.GetIsMaintainer(),
	}

	// Parse groupID and groupName from the request
	addOpts.ID, addOpts.Name, err = req.GetGroupReference().Parse()
	if err != nil {
		return nil, errors.BadRequest("invalid", fmt.Sprintf("invalid project reference: %s", err.Error()))
	}

	// Call the business logic to add the member
	_, err = g.groupUseCase.AddMemberToGroup(ctx, orgUUID, addOpts)
	if err != nil {
		return nil, handleUseCaseErr(err, g.log)
	}

	return &pb.GroupServiceAddMemberResponse{}, nil
}

// RemoveMember removes a member from a group within the current organization.
func (g *GroupService) RemoveMember(ctx context.Context, req *pb.GroupServiceRemoveMemberRequest) (*pb.GroupServiceRemoveMemberResponse, error) {
	currentUser, err := requireCurrentUser(ctx)
	if err != nil {
		return nil, err
	}

	currentOrg, err := requireCurrentOrg(ctx)
	if err != nil {
		return nil, err
	}

	if err := g.userHasPermissionToRemoveGroupMember(ctx, currentOrg.ID, req.GetGroupReference()); err != nil {
		return nil, err
	}

	// Parse orgID
	orgUUID, err := uuid.Parse(currentOrg.ID)
	if err != nil {
		return nil, errors.BadRequest("invalid", "invalid organization ID")
	}

	// Parse requesterID (current user)
	requesterUUID, err := uuid.Parse(currentUser.ID)
	if err != nil {
		return nil, errors.BadRequest("invalid", "invalid user ID")
	}

	// Create options for removing the member
	removeOpts := &biz.RemoveMemberFromGroupOpts{
		IdentityReference: &biz.IdentityReference{},
		UserEmail:         req.GetUserEmail(),
		RequesterID:       requesterUUID,
	}

	// Parse groupID and groupName from the request
	removeOpts.ID, removeOpts.Name, err = req.GetGroupReference().Parse()
	if err != nil {
		return nil, errors.BadRequest("invalid", fmt.Sprintf("invalid project reference: %s", err.Error()))
	}

	// Call the business logic to remove the member
	err = g.groupUseCase.RemoveMemberFromGroup(ctx, orgUUID, removeOpts)
	if err != nil {
		return nil, handleUseCaseErr(err, g.log)
	}

	return &pb.GroupServiceRemoveMemberResponse{}, nil
}

// ListPendingInvitations retrieves a list of pending invitations for a group
func (g *GroupService) ListPendingInvitations(ctx context.Context, req *pb.GroupServiceListPendingInvitationsRequest) (*pb.GroupServiceListPendingInvitationsResponse, error) {
	currentOrg, err := requireCurrentOrg(ctx)
	if err != nil {
		return nil, err
	}

	// Parse orgID
	orgUUID, err := uuid.Parse(currentOrg.ID)
	if err != nil {
		return nil, errors.BadRequest("invalid", "invalid organization ID")
	}

	if err := g.userHasPermissionToListPendingGroupInvitations(ctx, currentOrg.ID, req.GetGroupReference()); err != nil {
		return nil, err
	}

	// Parse groupID and groupName from the request
	groupID, groupName, err := req.GetGroupReference().Parse()
	if err != nil {
		return nil, errors.BadRequest("invalid", fmt.Sprintf("invalid project reference: %s", err.Error()))
	}

	// Initialize the pagination options, with default values
	paginationOpts, err := initializePaginationOpts(req.GetPagination())
	if err != nil {
		return nil, handleUseCaseErr(err, g.log)
	}

	// Call the business logic to list pending invitations
	invitations, count, err := g.groupUseCase.ListPendingInvitations(ctx, orgUUID, groupID, groupName, paginationOpts)
	if err != nil {
		return nil, handleUseCaseErr(err, g.log)
	}

	// Convert business objects to protobuf messages
	pbInvitations := make([]*pb.PendingGroupInvitation, 0, len(invitations))
	for _, invitation := range invitations {
		pbInvitations = append(pbInvitations, bizOrgInvitationToPendingGroupInvitationPb(invitation))
	}

	return &pb.GroupServiceListPendingInvitationsResponse{
		Invitations: pbInvitations,
		Pagination:  paginationToPb(count, paginationOpts.Offset(), paginationOpts.Limit()),
	}, nil
}

// UpdateMemberMaintainerStatus updates the maintainer status of a member in a group.
func (g *GroupService) UpdateMemberMaintainerStatus(ctx context.Context, req *pb.GroupServiceUpdateMemberMaintainerStatusRequest) (*pb.GroupServiceUpdateMemberMaintainerStatusResponse, error) {
	currentUser, err := requireCurrentUser(ctx)
	if err != nil {
		return nil, err
	}
	currentOrg, err := requireCurrentOrg(ctx)
	if err != nil {
		return nil, err
	}

	if err := g.userHasPermissionToUpdateMembership(ctx, currentOrg.ID, req.GetGroupReference()); err != nil {
		return nil, err
	}

	// Parse orgID
	orgUUID, err := uuid.Parse(currentOrg.ID)
	if err != nil {
		return nil, errors.BadRequest("invalid", "invalid organization ID")
	}

	// Parse requesterID (current user)
	requesterUUID, err := uuid.Parse(currentUser.ID)
	if err != nil {
		return nil, errors.BadRequest("invalid", "invalid user ID")
	}

	// Create options for updating the member's maintainer status
	updateOpts := &biz.UpdateMemberMaintainerStatusOpts{
		IdentityReference: &biz.IdentityReference{},
		UserReference:     &biz.IdentityReference{},
		RequesterID:       requesterUUID,
		IsMaintainer:      req.GetIsMaintainer(),
	}

	// Parse groupID and groupName from the request
	updateOpts.ID, updateOpts.Name, err = req.GetGroupReference().Parse()
	if err != nil {
		return nil, errors.BadRequest("invalid", fmt.Sprintf("invalid group reference: %s", err.Error()))
	}

	// Parse userID and userName from the request
	updateOpts.UserReference.ID, updateOpts.UserReference.Name, err = req.GetUserReference().Parse()
	if err != nil {
		return nil, errors.BadRequest("invalid", fmt.Sprintf("invalid user reference: %s", err.Error()))
	}

	// Call the business logic to update the member's maintainer status
	err = g.groupUseCase.UpdateMemberMaintainerStatus(ctx, orgUUID, updateOpts)
	if err != nil {
		return nil, handleUseCaseErr(err, g.log)
	}

	return &pb.GroupServiceUpdateMemberMaintainerStatusResponse{}, nil
}

// bizGroupToPb converts a biz.Group to a pb.Group protobuf message.
func bizGroupToPb(gr *biz.Group) *pb.Group {
	base := &pb.Group{
		Id:          gr.ID.String(),
		Name:        gr.Name,
		Description: gr.Description,
		MemberCount: int32(gr.MemberCount),
		CreatedAt:   timestamppb.New(*gr.CreatedAt),
		UpdatedAt:   timestamppb.New(*gr.UpdatedAt),
	}

	if gr.Organization != nil {
		base.OrganizationId = gr.Organization.ID
	}

	return base
}

// bizGroupMemberToPb converts a biz.GroupMembership to a pb.GroupMember protobuf message.
func bizGroupMemberToPb(m *biz.GroupMembership) *pb.GroupMember {
	return &pb.GroupMember{
		User:         bizUserToPb(m.User),
		IsMaintainer: m.Maintainer,
		CreatedAt:    timestamppb.New(*m.CreatedAt),
		UpdatedAt:    timestamppb.New(*m.UpdatedAt),
	}
}

// bizOrgInvitationToPendingGroupInvitationPb converts a biz.OrgInvitation to a pb.PendingGroupInvitation protobuf message.
func bizOrgInvitationToPendingGroupInvitationPb(inv *biz.OrgInvitation) *pb.PendingGroupInvitation {
	base := &pb.PendingGroupInvitation{
		UserEmail: inv.ReceiverEmail,
		CreatedAt: timestamppb.New(*inv.CreatedAt),
	}

	// Include the sender if available
	if inv.Sender != nil {
		base.InvitedBy = bizUserToPb(inv.Sender)
	}

	return base
}
