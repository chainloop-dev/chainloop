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

package action

import (
	"context"
	"time"

	pb "github.com/chainloop-dev/chainloop/app/controlplane/api/controlplane/v1"
)

// GroupMemberItem represents a member in a group
type GroupMemberItem struct {
	User    UserItem `json:"user"`
	Role    string   `json:"role"`
	AddedAt string   `json:"added_at"`
}

// GroupMemberListResult represents the response for a list of group members
type GroupMemberListResult struct {
	Members    []*GroupMemberItem `json:"members"`
	Pagination *OffsetPagination  `json:"pagination"`
}

// GroupMemberListFilterOpts contains the filters for group member listing
type GroupMemberListFilterOpts struct {
	GroupName   string
	MemberEmail string
	Role        string // can be "maintainer" or "member"
}

// GroupMemberList handles the listing of group members
type GroupMemberList struct {
	cfg *ActionsOpts
}

// NewGroupMemberList creates a new instance of GroupMemberList
func NewGroupMemberList(cfg *ActionsOpts) *GroupMemberList {
	return &GroupMemberList{cfg}
}

// Run executes the group member list operation with pagination and filtering
func (action *GroupMemberList) Run(ctx context.Context, page, limit int, filterOpts *GroupMemberListFilterOpts) (*GroupMemberListResult, error) {
	client := pb.NewGroupServiceClient(action.cfg.CPConnection)

	// Build the request
	req := &pb.GroupServiceListMembersRequest{
		GroupReference: &pb.IdentityReference{
			Name: &filterOpts.GroupName,
		},
		Pagination: &pb.OffsetPaginationRequest{
			Page:     int32(page),
			PageSize: int32(limit),
		},
	}

	// Apply filters
	if filterOpts.MemberEmail != "" {
		req.MemberEmail = &filterOpts.MemberEmail
	}
	if filterOpts.Role != "" {
		isMaintainer := filterOpts.Role == "maintainer"
		req.Maintainers = &isMaintainer
	}

	resp, err := client.ListMembers(ctx, req)
	if err != nil {
		return nil, err
	}

	// Convert the response to our model
	result := &GroupMemberListResult{
		Members: make([]*GroupMemberItem, 0, len(resp.GetMembers())),
		Pagination: &OffsetPagination{
			Page:       int(resp.GetPagination().GetPage()),
			PageSize:   int(resp.GetPagination().GetPageSize()),
			TotalCount: int(resp.GetPagination().GetTotalCount()),
			TotalPages: int(resp.GetPagination().GetTotalPages()),
		},
	}

	// Process each member
	for _, member := range resp.GetMembers() {
		result.Members = append(result.Members, pbGroupMemberToAction(member))
	}

	return result, nil
}

// pbGroupMemberToAction converts a protobuf group member to the action model
func pbGroupMemberToAction(member *pb.GroupMember) *GroupMemberItem {
	addedAt := ""
	if member.CreatedAt != nil {
		addedAt = member.CreatedAt.AsTime().Format(time.RFC3339)
	}

	return &GroupMemberItem{
		User: UserItem{
			ID:        member.User.GetId(),
			Email:     member.User.GetEmail(),
			FirstName: member.User.GetFirstName(),
			LastName:  member.User.GetLastName(),
		},
		Role:    getRoleName(member.IsMaintainer),
		AddedAt: addedAt,
	}
}

// getRoleName returns a human-readable role name from the role enum
func getRoleName(maintainer bool) string {
	if maintainer {
		return "Maintainer"
	} else {
		return "Member"
	}
}
