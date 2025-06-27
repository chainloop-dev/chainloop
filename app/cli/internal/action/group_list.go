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

	pb "github.com/chainloop-dev/chainloop/app/controlplane/api/controlplane/v1"
)

// GroupListResult represents the response for a list of groups
type GroupListResult struct {
	Groups     []*GroupCreateItem `json:"groups"`
	Pagination *OffsetPagination  `json:"pagination"`
}

// GroupListFilterOpts contains the filters for group listing
type GroupListFilterOpts struct {
	GroupName   string
	Description string
	MemberEmail string
}

// GroupList handles the listing of groups
type GroupList struct {
	cfg *ActionsOpts
}

// NewGroupList creates a new instance of GroupList
func NewGroupList(cfg *ActionsOpts) *GroupList {
	return &GroupList{cfg}
}

// Run executes the group list operation with pagination and filtering
func (action *GroupList) Run(ctx context.Context, page, limit int, filterOpts *GroupListFilterOpts) (*GroupListResult, error) {
	client := pb.NewGroupServiceClient(action.cfg.CPConnection)

	// Build the request
	req := &pb.GroupServiceListRequest{
		Pagination: &pb.OffsetPaginationRequest{
			Page:     int32(page),
			PageSize: int32(limit),
		},
	}

	// Create filters for the request
	if filterOpts.GroupName != "" {
		req.Name = &filterOpts.GroupName
	}
	if filterOpts.Description != "" {
		req.Description = &filterOpts.Description
	}
	if filterOpts.MemberEmail != "" {
		req.MemberEmail = &filterOpts.MemberEmail
	}

	resp, err := client.List(ctx, req)
	if err != nil {
		return nil, err
	}

	// Convert the response to our model
	result := &GroupListResult{
		Groups: make([]*GroupCreateItem, 0, len(resp.GetGroups())),
		Pagination: &OffsetPagination{
			Page:       int(resp.GetPagination().GetPage()),
			PageSize:   int(resp.GetPagination().GetPageSize()),
			TotalCount: int(resp.GetPagination().GetTotalCount()),
			TotalPages: int(resp.GetPagination().GetTotalPages()),
		},
	}

	// Process each group
	for _, group := range resp.GetGroups() {
		result.Groups = append(result.Groups, pbGroupItemToAction(group))
	}

	return result, nil
}
