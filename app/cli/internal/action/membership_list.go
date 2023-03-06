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

package action

import (
	"context"
	"time"

	pb "github.com/chainloop-dev/chainloop/app/controlplane/api/controlplane/v1"
)

type MembershipList struct {
	cfg *ActionsOpts
}

type OrgItem struct {
	ID, Name  string
	CreatedAt *time.Time
}

type MembershipItem struct {
	ID        string     `json:"id"`
	Current   bool       `json:"current"`
	CreatedAt *time.Time `json:"joinedAt"`
	UpdatedAt *time.Time `json:"updatedAt"`
	Org       *OrgItem
}

func NewMembershipList(cfg *ActionsOpts) *MembershipList {
	return &MembershipList{cfg}
}

func (action *MembershipList) Run() ([]*MembershipItem, error) {
	client := pb.NewOrganizationServiceClient(action.cfg.CPConnecction)
	resp, err := client.ListMemberships(context.Background(), &pb.OrganizationServiceListMembershipsRequest{})
	if err != nil {
		return nil, err
	}

	result := make([]*MembershipItem, 0, len(resp.Result))
	for _, p := range resp.Result {
		result = append(result, pbMembershipItemToAction(p))
	}

	return result, nil
}

func pbOrgItemToAction(in *pb.Org) *OrgItem {
	return &OrgItem{
		ID:        in.Id,
		Name:      in.Name,
		CreatedAt: toTimePtr(in.CreatedAt.AsTime()),
	}
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
		Current:   in.Current,
	}
}
