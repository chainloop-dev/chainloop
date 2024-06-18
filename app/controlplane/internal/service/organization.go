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

	pb "github.com/chainloop-dev/chainloop/app/controlplane/api/controlplane/v1"
	"github.com/chainloop-dev/chainloop/app/controlplane/internal/authz"
	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/biz"
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

// Create persists an organization with a given name and associate it to the current user.
func (s *OrganizationService) Create(ctx context.Context, req *pb.OrganizationServiceCreateRequest) (*pb.OrganizationServiceCreateResponse, error) {
	currentUser, err := requireCurrentUser(ctx)
	if err != nil {
		return nil, err
	}

	// Create an organization with an associated inline CAS backend
	org, err := s.orgUC.Create(ctx, req.Name, biz.WithCreateInlineBackend())
	if err != nil {
		return nil, handleUseCaseErr(err, s.log)
	}

	if _, err := s.membershipUC.Create(ctx, org.ID, currentUser.ID, biz.WithMembershipRole(authz.RoleOwner), biz.WithCurrentMembership()); err != nil {
		return nil, handleUseCaseErr(err, s.log)
	}

	return &pb.OrganizationServiceCreateResponse{Result: bizOrgToPb(org)}, nil
}

func (s *OrganizationService) Update(ctx context.Context, req *pb.OrganizationServiceUpdateRequest) (*pb.OrganizationServiceUpdateResponse, error) {
	currentUser, err := requireCurrentUser(ctx)
	if err != nil {
		return nil, err
	}

	org, err := s.orgUC.Update(ctx, currentUser.ID, req.Id, req.Name)
	if err != nil {
		return nil, handleUseCaseErr(err, s.log)
	}

	return &pb.OrganizationServiceUpdateResponse{Result: bizOrgToPb(org)}, nil
}

func (s *OrganizationService) ListMemberships(ctx context.Context, _ *pb.OrganizationServiceListMembershipsRequest) (*pb.OrganizationServiceListMembershipsResponse, error) {
	currentOrg, err := requireCurrentOrg(ctx)
	if err != nil {
		return nil, err
	}

	memberships, err := s.membershipUC.ByOrg(ctx, currentOrg.ID)
	if err != nil {
		return nil, handleUseCaseErr(err, s.log)
	}

	result := make([]*pb.OrgMembershipItem, 0, len(memberships))
	for _, m := range memberships {
		result = append(result, bizMembershipToPb(m))
	}

	return &pb.OrganizationServiceListMembershipsResponse{Result: result}, nil
}

func (s *OrganizationService) DeleteMembership(ctx context.Context, req *pb.OrganizationServiceDeleteMembershipRequest) (*pb.OrganizationServiceDeleteMembershipResponse, error) {
	currentOrg, err := requireCurrentOrg(ctx)
	if err != nil {
		return nil, err
	}

	currentUser, err := requireCurrentUser(ctx)
	if err != nil {
		return nil, err
	}

	if err := s.membershipUC.DeleteOther(ctx, currentOrg.ID, currentUser.ID, req.MembershipId); err != nil {
		return nil, handleUseCaseErr(err, s.log)
	}

	return &pb.OrganizationServiceDeleteMembershipResponse{}, nil
}

func (s *OrganizationService) UpdateMembership(ctx context.Context, req *pb.OrganizationServiceUpdateMembershipRequest) (*pb.OrganizationServiceUpdateMembershipResponse, error) {
	currentOrg, err := requireCurrentOrg(ctx)
	if err != nil {
		return nil, err
	}

	currentUser, err := requireCurrentUser(ctx)
	if err != nil {
		return nil, err
	}

	m, err := s.membershipUC.UpdateRole(ctx, currentOrg.ID, currentUser.ID, req.MembershipId, biz.PbRoleToBiz(req.Role))
	if err != nil {
		return nil, handleUseCaseErr(err, s.log)
	}

	return &pb.OrganizationServiceUpdateMembershipResponse{Result: bizMembershipToPb(m)}, nil
}
