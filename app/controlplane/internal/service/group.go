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

	pb "github.com/chainloop-dev/chainloop/app/controlplane/api/controlplane/v1"
	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/biz"
	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/pagination"

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
func (g GroupService) Create(ctx context.Context, req *pb.GroupServiceCreateRequest) (*pb.GroupServiceCreateResponse, error) {
	currentUser, err := requireCurrentUser(ctx)
	if err != nil {
		return nil, err
	}
	currentOrg, err := requireCurrentOrg(ctx)
	if err != nil {
		return nil, err
	}

	// Parse orgID
	orgUUID, err := uuid.Parse(currentOrg.ID)
	if err != nil {
		return nil, errors.BadRequest("invalid", "invalid organization ID")
	}

	// Parse userID
	userUUID, err := uuid.Parse(currentUser.ID)
	if err != nil {
		return nil, errors.BadRequest("invalid", "invalid user ID")
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
func (g GroupService) Get(ctx context.Context, req *pb.GroupServiceGetRequest) (*pb.GroupServiceGetResponse, error) {
	currentOrg, err := requireCurrentOrg(ctx)
	if err != nil {
		return nil, err
	}

	// Parse orgID
	orgUUID, err := uuid.Parse(currentOrg.ID)
	if err != nil {
		return nil, errors.BadRequest("invalid", "invalid organization ID")
	}

	// Parse groupID
	groupUUID, err := uuid.Parse(req.GetGroupId())
	if err != nil {
		return nil, errors.BadRequest("invalid", "invalid group ID")
	}

	gr, err := g.groupUseCase.FindByOrgAndID(ctx, orgUUID, groupUUID)
	if err != nil {
		return nil, handleUseCaseErr(err, g.log)
	}

	return &pb.GroupServiceGetResponse{
		Group: bizGroupToPb(gr),
	}, nil
}

// List retrieves a list of groups within the current organization, with optional filters and pagination.
func (g GroupService) List(ctx context.Context, req *pb.GroupServiceListRequest) (*pb.GroupServiceListResponse, error) {
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
	paginationOpts := pagination.NewDefaultOffsetPaginationOpts()

	// Override the pagination options if they are provided
	if req.GetPagination() != nil {
		paginationOpts, err = pagination.NewOffsetPaginationOpts(
			int(req.GetPagination().GetPage()),
			int(req.GetPagination().GetPageSize()),
		)
		if err != nil {
			return nil, handleUseCaseErr(err, g.log)
		}
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
func (g GroupService) Update(ctx context.Context, req *pb.GroupServiceUpdateRequest) (*pb.GroupServiceUpdateResponse, error) {
	currentOrg, err := requireCurrentOrg(ctx)
	if err != nil {
		return nil, err
	}

	// Parse orgID
	orgUUID, err := uuid.Parse(currentOrg.ID)
	if err != nil {
		return nil, errors.BadRequest("invalid", "invalid organization ID")
	}

	// Parse groupID
	groupUUID, err := uuid.Parse(req.GetGroupId())
	if err != nil {
		return nil, errors.BadRequest("invalid", "invalid group ID")
	}

	// Update the group with the provided options
	gr, err := g.groupUseCase.Update(ctx, orgUUID, groupUUID, req.Description, req.Name)
	if err != nil {
		return nil, handleUseCaseErr(err, g.log)
	}

	return &pb.GroupServiceUpdateResponse{
		Group: bizGroupToPb(gr),
	}, nil
}

// Delete soft-deletes a group by its ID within the current organization.
func (g GroupService) Delete(ctx context.Context, req *pb.GroupServiceDeleteRequest) (*pb.GroupServiceDeleteResponse, error) {
	currentOrg, err := requireCurrentOrg(ctx)
	if err != nil {
		return nil, err
	}

	// Parse orgID
	orgUUID, err := uuid.Parse(currentOrg.ID)
	if err != nil {
		return nil, errors.BadRequest("invalid", "invalid organization ID")
	}

	// Parse groupID
	groupUUID, err := uuid.Parse(req.Id)
	if err != nil {
		return nil, errors.BadRequest("invalid", "invalid group ID")
	}

	err = g.groupUseCase.SoftDelete(ctx, orgUUID, groupUUID)
	if err != nil {
		return nil, handleUseCaseErr(err, g.log)
	}

	return &pb.GroupServiceDeleteResponse{}, nil
}

// ListMembers retrieves a list of members in a group within the current organization, with optional filters and pagination.
func (g GroupService) ListMembers(ctx context.Context, req *pb.GroupServiceListMembersRequest) (*pb.GroupServiceListMembersResponse, error) {
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
	paginationOpts := pagination.NewDefaultOffsetPaginationOpts()

	// Override the pagination options if they are provided
	if req.GetPagination() != nil {
		paginationOpts, err = pagination.NewOffsetPaginationOpts(
			int(req.GetPagination().GetPage()),
			int(req.GetPagination().GetPageSize()),
		)
		if err != nil {
			return nil, handleUseCaseErr(err, g.log)
		}
	}

	// Parse groupID
	groupUUID, err := uuid.Parse(req.GetGroupId())
	if err != nil {
		return nil, errors.BadRequest("invalid", "invalid group ID")
	}

	grs, count, err := g.groupUseCase.ListMembers(ctx, orgUUID, groupUUID, req.Maintainers, req.MemberEmail, paginationOpts)
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

// bizGroupToPb converts a biz.Group to a pb.Group protobuf message.
func bizGroupToPb(gr *biz.Group) *pb.Group {
	base := &pb.Group{
		Id:          gr.ID.String(),
		Name:        gr.Name,
		Description: gr.Description,
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
