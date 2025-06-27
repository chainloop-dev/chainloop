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

// GroupDescribe handles retrieving detailed information about a specific group
type GroupDescribe struct {
	cfg *ActionsOpts
}

// NewGroupDescribe creates a new instance of GroupDescribe
func NewGroupDescribe(cfg *ActionsOpts) *GroupDescribe {
	return &GroupDescribe{cfg}
}

// Run executes the group describe operation
func (action *GroupDescribe) Run(ctx context.Context, groupName string) (*GroupCreateItem, error) {
	client := pb.NewGroupServiceClient(action.cfg.CPConnection)

	// Build the request
	req := &pb.GroupServiceGetRequest{
		GroupReference: &pb.IdentityReference{
			Name: &groupName,
		},
	}

	resp, err := client.Get(ctx, req)
	if err != nil {
		return nil, err
	}

	// Convert the response to our model
	return pbGroupItemToAction(resp.GetGroup()), nil
}
