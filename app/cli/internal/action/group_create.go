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

// GroupCreateItem represents the response structure for a created group
type GroupCreateItem struct {
	ID          string `json:"id,omitempty"`
	Name        string `json:"name,omitempty"`
	Description string `json:"description,omitempty"`
	CreatedAt   string `json:"created_at,omitempty"`
	UpdatedAt   string `json:"updated_at,omitempty"`
}

// GroupCreate handles the creation of a new group
type GroupCreate struct {
	cfg *ActionsOpts
}

// NewGroupCreate creates a new instance of GroupCreate
func NewGroupCreate(cfg *ActionsOpts) *GroupCreate {
	return &GroupCreate{cfg}
}

// Run executes the group creation operation
func (action *GroupCreate) Run(ctx context.Context, name, description string) (*GroupCreateItem, error) {
	client := pb.NewGroupServiceClient(action.cfg.CPConnection)
	resp, err := client.Create(ctx, &pb.GroupServiceCreateRequest{
		Name:        name,
		Description: description,
	})
	if err != nil {
		return nil, err
	}

	return pbGroupItemToAction(resp.GetGroup()), nil
}

// pbGroupItemToAction converts a protobuf group item to the action model
func pbGroupItemToAction(group *pb.Group) *GroupCreateItem {
	createdAt := ""
	if group.CreatedAt != nil {
		createdAt = group.CreatedAt.AsTime().Format(time.RFC3339)
	}
	updatedAt := ""
	if group.UpdatedAt != nil {
		updatedAt = group.UpdatedAt.AsTime().Format(time.RFC3339)
	}

	return &GroupCreateItem{
		ID:          group.Id,
		Name:        group.Name,
		Description: group.Description,
		CreatedAt:   createdAt,
		UpdatedAt:   updatedAt,
	}
}
