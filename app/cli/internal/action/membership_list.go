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

package action

import (
	"context"
	"fmt"
	"strings"
	"time"

	pb "github.com/chainloop-dev/chainloop/app/controlplane/api/controlplane/v1"
)

type MembershipList struct {
	cfg *ActionsOpts
}

type OrgItem struct {
	ID, Name                        string
	CreatedAt                       *time.Time
	PolicyViolationBlockingStrategy string
}

type MembershipItem struct {
	ID        string     `json:"id"`
	Default   bool       `json:"current"`
	CreatedAt *time.Time `json:"joinedAt"`
	UpdatedAt *time.Time `json:"updatedAt"`
	Org       *OrgItem
	User      *UserItem
	Role      Role `json:"role"`
}

type ListMembersOpts struct {
	// MembershipID Optional, if provided, filters by a specific membership ID
	MembershipID *string
	// Name is the name of the user to filter by
	Name *string
	// Email is the email of the user to filter by
	Email *string
	// Role is the role of the user to filter by
	Role *string
}

type ListMembershipResult struct {
	Memberships    []*MembershipItem
	PaginationMeta *OffsetPagination
}

func NewMembershipList(cfg *ActionsOpts) *MembershipList {
	return &MembershipList{cfg}
}

// List organizations for the current user
func (action *MembershipList) ListOrgs(ctx context.Context) ([]*MembershipItem, error) {
	client := pb.NewUserServiceClient(action.cfg.CPConnection)
	resp, err := client.ListMemberships(ctx, &pb.UserServiceListMembershipsRequest{})
	if err != nil {
		return nil, err
	}

	result := make([]*MembershipItem, 0, len(resp.Result))
	for _, p := range resp.Result {
		result = append(result, pbMembershipItemToAction(p))
	}

	return result, nil
}

// ListMembers lists the members of an organization with pagination and optional filters.
func (action *MembershipList) ListMembers(ctx context.Context, page int, pageSize int, opts *ListMembersOpts) (*ListMembershipResult, error) {
	if page < 1 {
		return nil, fmt.Errorf("page must be greater or equal to 1")
	}
	if pageSize < 1 {
		return nil, fmt.Errorf("page-size must be greater or equal to 1")
	}

	client := pb.NewOrganizationServiceClient(action.cfg.CPConnection)
	req := &pb.OrganizationServiceListMembershipsRequest{
		MembershipId: opts.MembershipID,
		Name:         opts.Name,
		Email:        opts.Email,
		Pagination: &pb.OffsetPaginationRequest{
			Page:     int32(page),
			PageSize: int32(pageSize),
		},
	}

	// If a role is specified, convert it to the protobuf enum
	if opts.Role != nil {
		casted := stringToPbRole(Role(*opts.Role))
		req.Role = &casted
	}

	resp, err := client.ListMemberships(ctx, req)
	if err != nil {
		return nil, err
	}

	result := make([]*MembershipItem, 0, len(resp.Result))
	for _, p := range resp.Result {
		result = append(result, pbMembershipItemToAction(p))
	}

	return &ListMembershipResult{
		Memberships: result,
		PaginationMeta: &OffsetPagination{
			Page:       int(resp.GetPagination().GetPage()),
			PageSize:   int(resp.GetPagination().GetPageSize()),
			TotalPages: int(resp.GetPagination().GetTotalPages()),
			TotalCount: int(resp.GetPagination().GetTotalCount()),
		},
	}, nil
}

func pbOrgItemToAction(in *pb.OrgItem) *OrgItem {
	i := &OrgItem{
		ID:        in.Id,
		Name:      in.Name,
		CreatedAt: toTimePtr(in.CreatedAt.AsTime()),
	}

	if in.DefaultPolicyViolationStrategy == pb.OrgItem_POLICY_VIOLATION_BLOCKING_STRATEGY_BLOCK {
		i.PolicyViolationBlockingStrategy = PolicyViolationBlockingStrategyEnforced
	} else {
		i.PolicyViolationBlockingStrategy = PolicyViolationBlockingStrategyAdvisory
	}

	return i
}

func pbMembershipItemToAction(in *pb.OrgMembershipItem) *MembershipItem {
	if in == nil {
		return nil
	}

	return &MembershipItem{
		ID:        in.GetId(),
		CreatedAt: toTimePtr(in.GetCreatedAt().AsTime()),
		UpdatedAt: toTimePtr(in.GetCreatedAt().AsTime()),
		Org:       pbOrgItemToAction(in.Org),
		Default:   in.Current,
		Role:      pbRoleToString(in.Role),
		User:      pbUserItemToAction(in.User),
	}
}

type Role string

const (
	RoleAdmin       Role = "admin"
	RoleOwner       Role = "owner"
	RoleViewer      Role = "viewer"
	RoleMember      Role = "member"
	RoleContributor Role = "contributor"
)

type Roles []Role

var AvailableRoles = Roles{
	RoleAdmin,
	RoleOwner,
	RoleViewer,
	RoleMember,
	RoleContributor,
}

func (roles Roles) String() string {
	result := make([]string, 0, len(roles))
	for _, role := range roles {
		result = append(result, string(role))
	}
	return strings.Join(result, ", ")
}

func pbRoleToString(role pb.MembershipRole) Role {
	switch role {
	case pb.MembershipRole_MEMBERSHIP_ROLE_ORG_ADMIN:
		return RoleAdmin
	case pb.MembershipRole_MEMBERSHIP_ROLE_ORG_VIEWER:
		return RoleViewer
	case pb.MembershipRole_MEMBERSHIP_ROLE_ORG_OWNER:
		return RoleOwner
	case pb.MembershipRole_MEMBERSHIP_ROLE_ORG_MEMBER:
		return RoleMember
	case pb.MembershipRole_MEMBERSHIP_ROLE_ORG_CONTRIBUTOR:
		return RoleContributor
	}
	return ""
}

func stringToPbRole(role Role) pb.MembershipRole {
	switch role {
	case RoleAdmin:
		return pb.MembershipRole_MEMBERSHIP_ROLE_ORG_ADMIN
	case RoleViewer:
		return pb.MembershipRole_MEMBERSHIP_ROLE_ORG_VIEWER
	case RoleOwner:
		return pb.MembershipRole_MEMBERSHIP_ROLE_ORG_OWNER
	case RoleMember:
		return pb.MembershipRole_MEMBERSHIP_ROLE_ORG_MEMBER
	case RoleContributor:
		return pb.MembershipRole_MEMBERSHIP_ROLE_ORG_CONTRIBUTOR
	}
	return pb.MembershipRole_MEMBERSHIP_ROLE_UNSPECIFIED
}
