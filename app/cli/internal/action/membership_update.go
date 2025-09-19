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

	pb "github.com/chainloop-dev/chainloop/app/controlplane/api/controlplane/v1"
)

type MembershipUpdate struct {
	cfg *ActionsOpts
}

func NewMembershipUpdate(cfg *ActionsOpts) *MembershipUpdate {
	return &MembershipUpdate{cfg}
}

// List organizations for the current user
func (action *MembershipUpdate) ChangeRole(ctx context.Context, membershipID, role string) (*MembershipItem, error) {
	client := pb.NewOrganizationServiceClient(action.cfg.CPConnection)
	resp, err := client.UpdateMembership(ctx, &pb.OrganizationServiceUpdateMembershipRequest{
		MembershipId: membershipID,
		Role:         stringToPbRole(Role(role)),
	})
	if err != nil {
		return nil, err
	}

	return pbMembershipItemToAction(resp.Result), nil
}
