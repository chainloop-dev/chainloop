//
// Copyright 2024-2025 The Chainloop Authors.
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
	"github.com/chainloop-dev/chainloop/app/controlplane/internal/usercontext/entities"
	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/authz"
	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/biz"
	errors "github.com/go-kratos/kratos/v2/errors"

	"github.com/google/uuid"
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

	canCreate, err := s.canCreateOrganization(ctx)
	if err != nil {
		return nil, err
	}

	if !canCreate {
		return nil, errors.Forbidden("forbidden", "creation of organizations is restricted to instance admins")
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

	// we want to differentiate between setting the value to empty or not setting it at all
	// to do that we will use a nil slice to represent not setting it at all
	var policiesAllowedHostnames []string
	if req.UpdatePoliciesAllowedHostnames {
		policiesAllowedHostnames = req.PoliciesAllowedHostnames
		// explicitly set the slice so we can differentiate between an empty slice and a nil slice
		if len(policiesAllowedHostnames) == 0 {
			policiesAllowedHostnames = []string{}
		}
	}

	org, err := s.orgUC.Update(ctx, currentUser.ID, req.Name, req.BlockOnPolicyViolation, policiesAllowedHostnames)
	if err != nil {
		return nil, handleUseCaseErr(err, s.log)
	}

	return &pb.OrganizationServiceUpdateResponse{Result: bizOrgToPb(org)}, nil
}

func (s *OrganizationService) Delete(ctx context.Context, req *pb.OrganizationServiceDeleteRequest) (*pb.OrganizationServiceDeleteResponse, error) {
	if _, err := requireCurrentUser(ctx); err != nil {
		return nil, err
	}

	// Find the organization to get its UUID for authorization
	org, err := s.orgUC.FindByName(ctx, req.Name)
	if err != nil {
		return nil, handleUseCaseErr(err, s.log)
	}

	orgUUID, err := uuid.Parse(org.ID)
	if err != nil {
		return nil, handleUseCaseErr(biz.NewErrInvalidUUID(err), s.log)
	}

	// Check if user has permission to delete this specific organization
	// Force RBAC to ensure only owners can delete, even if they have admin privileges elsewhere
	if err := s.authorizeResource(ctx, authz.PolicyOrganizationDelete, authz.ResourceTypeOrganization, orgUUID, withForceRBAC()); err != nil {
		return nil, errors.Forbidden("forbidden", "only organization owners can delete the organization")
	}

	if err := s.orgUC.Delete(ctx, orgUUID.String()); err != nil {
		return nil, handleUseCaseErr(err, s.log)
	}

	return &pb.OrganizationServiceDeleteResponse{}, nil
}

func (s *OrganizationService) ListMemberships(ctx context.Context, req *pb.OrganizationServiceListMembershipsRequest) (*pb.OrganizationServiceListMembershipsResponse, error) {
	currentOrg, err := requireCurrentOrg(ctx)
	if err != nil {
		return nil, err
	}

	// Initialize the pagination options, with default values
	paginationOpts, err := initializePaginationOpts(req.GetPagination())
	if err != nil {
		return nil, handleUseCaseErr(err, s.log)
	}

	opts := &biz.ListByOrgOpts{
		Name:  req.Name,
		Email: req.Email,
	}

	if req.MembershipId != nil {
		membershipUUID, err := uuid.Parse(req.GetMembershipId())
		if err != nil {
			return nil, handleUseCaseErr(err, s.log)
		}

		opts.MembershipID = &membershipUUID
	}

	if req.Role != nil {
		castedRole := biz.PbRoleToBiz(req.GetRole())
		opts.Role = &castedRole
	}

	memberships, count, err := s.membershipUC.ByOrg(ctx, currentOrg.ID, opts, paginationOpts)
	if err != nil {
		return nil, handleUseCaseErr(err, s.log)
	}

	result := make([]*pb.OrgMembershipItem, 0, len(memberships))
	for _, m := range memberships {
		result = append(result, bizMembershipToPb(m))
	}

	return &pb.OrganizationServiceListMembershipsResponse{
		Result:     result,
		Pagination: paginationToPb(count, paginationOpts.Offset(), paginationOpts.Limit()),
	}, nil
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

func (s *OrganizationService) canCreateOrganization(ctx context.Context) (bool, error) {
	// Restricted org creation is disabled, allow creation
	if !s.enforcer.RestrictOrgCreation {
		return true, nil
	}

	m := entities.CurrentMembership(ctx)
	for _, rm := range m.Resources {
		if rm.ResourceType != authz.ResourceTypeInstance {
			continue
		}

		pass, err := s.enforcer.Enforce(string(rm.Role), authz.PolicyOrganizationCreate)
		if err != nil {
			return false, handleUseCaseErr(err, s.log)
		}
		if pass {
			return true, nil
		}
	}

	return false, nil
}
