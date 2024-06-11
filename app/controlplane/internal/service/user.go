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

package service

import (
	"context"

	"google.golang.org/protobuf/types/known/timestamppb"

	pb "github.com/chainloop-dev/chainloop/app/controlplane/api/controlplane/v1"
	"github.com/chainloop-dev/chainloop/app/controlplane/internal/authz"
	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/biz"
	errors "github.com/go-kratos/kratos/v2/errors"
)

type UserService struct {
	pb.UnimplementedUserServiceServer
	*service

	membershipUC *biz.MembershipUseCase
	orgUC        *biz.OrganizationUseCase
}

func NewUserService(muc *biz.MembershipUseCase, ouc *biz.OrganizationUseCase, opts ...NewOpt) *UserService {
	return &UserService{
		service:      newService(opts...),
		membershipUC: muc,
		orgUC:        ouc,
	}
}

func (s *UserService) ListMemberships(ctx context.Context, _ *pb.UserServiceListMembershipsRequest) (*pb.UserServiceListMembershipsResponse, error) {
	currentUser, err := requireCurrentUser(ctx)
	if err != nil {
		return nil, err
	}

	memberships, err := s.membershipUC.ByUser(ctx, currentUser.ID)
	if err != nil && biz.IsNotFound(err) {
		return nil, errors.NotFound("not found", err.Error())
	} else if err != nil {
		return nil, handleUseCaseErr(err, s.log)
	}

	result := make([]*pb.OrgMembershipItem, 0, len(memberships))
	for _, m := range memberships {
		result = append(result, bizMembershipToPb(m))
	}

	return &pb.UserServiceListMembershipsResponse{Result: result}, nil
}

func (s *UserService) SetCurrentMembership(ctx context.Context, req *pb.SetCurrentMembershipRequest) (*pb.SetCurrentMembershipResponse, error) {
	currentUser, err := requireCurrentUser(ctx)
	if err != nil {
		return nil, err
	}

	m, err := s.membershipUC.SetCurrent(ctx, currentUser.ID, req.MembershipId)
	if err != nil && biz.IsNotFound(err) {
		return nil, errors.NotFound("not found", err.Error())
	} else if err != nil {
		return nil, handleUseCaseErr(err, s.log)
	}

	return &pb.SetCurrentMembershipResponse{Result: bizMembershipToPb(m)}, nil
}

func (s *UserService) DeleteMembership(ctx context.Context, req *pb.DeleteMembershipRequest) (*pb.DeleteMembershipResponse, error) {
	currentUser, err := requireCurrentUser(ctx)
	if err != nil {
		return nil, err
	}

	err = s.membershipUC.LeaveAndDeleteOrg(ctx, currentUser.ID, req.MembershipId)
	if err != nil && biz.IsNotFound(err) {
		return nil, errors.NotFound("not found", err.Error())
	} else if err != nil {
		return nil, handleUseCaseErr(err, s.log)
	}

	return &pb.DeleteMembershipResponse{}, nil
}

func pbRoleToBiz(r pb.MembershipRole) authz.Role {
	switch r {
	case pb.MembershipRole_MEMBERSHIP_ROLE_ORG_OWNER:
		return authz.RoleOwner
	case pb.MembershipRole_MEMBERSHIP_ROLE_ORG_ADMIN:
		return authz.RoleAdmin
	case pb.MembershipRole_MEMBERSHIP_ROLE_ORG_VIEWER:
		return authz.RoleViewer
	default:
		return ""
	}
}

func bizMembershipToPb(m *biz.Membership) *pb.OrgMembershipItem {
	item := &pb.OrgMembershipItem{
		Id: m.ID.String(), Current: m.Current,
		CreatedAt: timestamppb.New(*m.CreatedAt),
		Org:       bizOrgToPb(m.Org),
		Role:      bizRoleToPb(m.Role),
	}

	if m.UpdatedAt != nil {
		item.UpdatedAt = timestamppb.New(*m.UpdatedAt)
	}

	if m.User != nil {
		item.User = bizUserToPb(m.User)
	}

	return item
}

func bizRoleToPb(r authz.Role) pb.MembershipRole {
	switch r {
	case authz.RoleOwner:
		return pb.MembershipRole_MEMBERSHIP_ROLE_ORG_OWNER
	case authz.RoleAdmin:
		return pb.MembershipRole_MEMBERSHIP_ROLE_ORG_ADMIN
	case authz.RoleViewer:
		return pb.MembershipRole_MEMBERSHIP_ROLE_ORG_VIEWER
	default:
		return pb.MembershipRole_MEMBERSHIP_ROLE_UNSPECIFIED
	}
}
