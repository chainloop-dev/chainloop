//
// Copyright 2023 The Chainloop Authors.
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
	"github.com/chainloop-dev/chainloop/app/controlplane/internal/biz"
	sl "github.com/chainloop-dev/chainloop/internal/servicelogger"
	errors "github.com/go-kratos/kratos/v2/errors"
)

type OrganizationService struct {
	pb.UnimplementedOrganizationServiceServer
	*service

	membershipUC *biz.MembershipUseCase
	orgUC        *biz.OrganizationUseCase
}

func NewOrganizationService(muc *biz.MembershipUseCase, ouc *biz.OrganizationUseCase, opts ...NewOpt) *OrganizationService {
	return &OrganizationService{
		service:      newService(opts...),
		membershipUC: muc,
		orgUC:        ouc,
	}
}

func (s *OrganizationService) ListMemberships(ctx context.Context, _ *pb.OrganizationServiceListMembershipsRequest) (*pb.OrganizationServiceListMembershipsResponse, error) {
	currentUser, _, err := loadCurrentUserAndOrg(ctx)
	if err != nil {
		return nil, err
	}

	memberships, err := s.membershipUC.ByUser(ctx, currentUser.ID)
	if err != nil && biz.IsNotFound(err) {
		return nil, errors.NotFound("not found", err.Error())
	} else if err != nil {
		return nil, sl.LogAndMaskErr(err, s.log)
	}

	result := make([]*pb.OrgMembershipItem, 0, len(memberships))
	for _, m := range memberships {
		result = append(result, bizMembershipToPb(m))
	}

	return &pb.OrganizationServiceListMembershipsResponse{Result: result}, nil
}

func (s *OrganizationService) Update(ctx context.Context, req *pb.OrganizationServiceUpdateRequest) (*pb.OrganizationServiceUpdateResponse, error) {
	currentUser, _, err := loadCurrentUserAndOrg(ctx)
	if err != nil {
		return nil, err
	}

	org, err := s.orgUC.Update(ctx, currentUser.ID, req.Id, req.Name)
	if err != nil {
		return nil, handleUseCaseErr("organization", err, s.log)
	}

	return &pb.OrganizationServiceUpdateResponse{Result: bizOrgToPb(org)}, nil
}

func (s *OrganizationService) SetCurrentMembership(ctx context.Context, req *pb.SetCurrentMembershipRequest) (*pb.SetCurrentMembershipResponse, error) {
	currentUser, _, err := loadCurrentUserAndOrg(ctx)
	if err != nil {
		return nil, err
	}

	m, err := s.membershipUC.SetCurrent(ctx, currentUser.ID, req.MembershipId)
	if err != nil && biz.IsNotFound(err) {
		return nil, errors.NotFound("not found", err.Error())
	} else if err != nil {
		return nil, sl.LogAndMaskErr(err, s.log)
	}

	return &pb.SetCurrentMembershipResponse{Result: bizMembershipToPb(m)}, nil
}

func bizMembershipToPb(m *biz.Membership) *pb.OrgMembershipItem {
	item := &pb.OrgMembershipItem{
		Id: m.ID.String(), Current: m.Current,
		CreatedAt: timestamppb.New(*m.CreatedAt),
		UpdatedAt: timestamppb.New(*m.UpdatedAt),
		Org:       bizOrgToPb(m.Org),
	}

	return item
}
