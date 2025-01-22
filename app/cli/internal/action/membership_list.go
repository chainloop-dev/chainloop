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

// List members of the current organization
func (action *MembershipList) ListMembers(ctx context.Context) ([]*MembershipItem, error) {
	client := pb.NewOrganizationServiceClient(action.cfg.CPConnection)
	resp, err := client.ListMemberships(ctx, &pb.OrganizationServiceListMembershipsRequest{})
	if err != nil {
		return nil, err
	}

	result := make([]*MembershipItem, 0, len(resp.Result))
	for _, p := range resp.Result {
		result = append(result, pbMembershipItemToAction(p))
	}

	return result, nil
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
	RoleAdmin  Role = "admin"
	RoleOwner  Role = "owner"
	RoleViewer Role = "viewer"
)

type Roles []Role

var AvailableRoles = Roles{
	RoleAdmin,
	RoleOwner,
	RoleViewer,
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
	}
	return pb.MembershipRole_MEMBERSHIP_ROLE_UNSPECIFIED
}
