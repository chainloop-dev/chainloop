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

// GroupMemberDelete handles removing a member from a group
type GroupMemberDelete struct {
	cfg *ActionsOpts
}

// NewGroupMemberDelete creates a new instance of GroupMemberDelete
func NewGroupMemberDelete(cfg *ActionsOpts) *GroupMemberDelete {
	return &GroupMemberDelete{cfg}
}

// Run executes the group member removal operation
func (action *GroupMemberDelete) Run(ctx context.Context, groupName, memberEmail string) error {
	client := pb.NewGroupServiceClient(action.cfg.CPConnection)

	// Build the request
	req := &pb.GroupServiceRemoveMemberRequest{
		GroupReference: &pb.IdentityReference{
			Name: &groupName,
		},
		UserEmail: memberEmail,
	}

	_, err := client.RemoveMember(ctx, req)
	return err
}
