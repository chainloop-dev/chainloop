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

// GroupUpdateItem represents the response structure for an updated group
type GroupUpdateItem struct {
	ID          string `json:"id,omitempty"`
	Name        string `json:"name,omitempty"`
	Description string `json:"description,omitempty"`
	CreatedAt   string `json:"created_at,omitempty"`
	UpdatedAt   string `json:"updated_at,omitempty"`
}

// GroupUpdate handles the update of an existing group
type GroupUpdate struct {
	cfg *ActionsOpts
}

// NewGroupUpdate creates a new instance of GroupUpdate
func NewGroupUpdate(cfg *ActionsOpts) *GroupUpdate {
	return &GroupUpdate{cfg}
}

// Run executes the group update operation
func (action *GroupUpdate) Run(ctx context.Context, groupName string, newName, newDescription *string) (*GroupUpdateItem, error) {
	client := pb.NewGroupServiceClient(action.cfg.CPConnection)

	// Create the group reference using the name
	groupRef := &pb.IdentityReference{
		Name: &groupName,
	}

	// Create the update request
	req := &pb.GroupServiceUpdateRequest{
		GroupReference: groupRef,
		NewName:        newName,
		NewDescription: newDescription,
	}

	// Make the update request
	resp, err := client.Update(ctx, req)
	if err != nil {
		return nil, err
	}

	return pbGroupUpdateItemToAction(resp.GetGroup()), nil
}

// pbGroupUpdateItemToAction converts a protobuf group item to the action model
func pbGroupUpdateItemToAction(group *pb.Group) *GroupUpdateItem {
	createdAt := ""
	if group.CreatedAt != nil {
		createdAt = group.CreatedAt.AsTime().Format(time.RFC3339)
	}
	updatedAt := ""
	if group.UpdatedAt != nil {
		updatedAt = group.UpdatedAt.AsTime().Format(time.RFC3339)
	}

	return &GroupUpdateItem{
		ID:          group.Id,
		Name:        group.Name,
		Description: group.Description,
		CreatedAt:   createdAt,
		UpdatedAt:   updatedAt,
	}
}
