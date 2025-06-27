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

// GroupMemberAdd handles adding a member to a group
type GroupMemberAdd struct {
	cfg *ActionsOpts
}

// NewGroupMemberAdd creates a new instance of GroupMemberAdd
func NewGroupMemberAdd(cfg *ActionsOpts) *GroupMemberAdd {
	return &GroupMemberAdd{cfg}
}

// Run executes the group member addition operation
func (action *GroupMemberAdd) Run(ctx context.Context, groupName, memberEmail string, isMaintainer bool) error {
	client := pb.NewGroupServiceClient(action.cfg.CPConnection)

	// Build the request
	req := &pb.GroupServiceAddMemberRequest{
		GroupReference: &pb.IdentityReference{
			Name: &groupName,
		},
		UserEmail:    memberEmail,
		IsMaintainer: isMaintainer,
	}

	_, err := client.AddMember(ctx, req)
	if err != nil {
		return err
	}

	return nil
}
