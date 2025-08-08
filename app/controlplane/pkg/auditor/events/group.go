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

package events

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/auditor"
	"github.com/google/uuid"
)

var (
	_ auditor.LogEntry = (*GroupCreated)(nil)
	_ auditor.LogEntry = (*GroupUpdated)(nil)
	_ auditor.LogEntry = (*GroupDeleted)(nil)
	_ auditor.LogEntry = (*GroupMemberAdded)(nil)
	_ auditor.LogEntry = (*GroupMemberRemoved)(nil)
	_ auditor.LogEntry = (*GroupMemberUpdated)(nil)
)

const (
	GroupType                        auditor.TargetType = "Group"
	GroupCreatedActionType           string             = "GroupCreated"
	GroupUpdatedActionType           string             = "GroupUpdated"
	GroupDeletedActionType           string             = "GroupDeleted"
	GroupMembershipAddedActionType   string             = "GroupMembershipAdded"
	GroupMembershipRemovedActionType string             = "GroupMembershipRemoved"
)

// GroupBase is the base struct for group events
type GroupBase struct {
	GroupID   *uuid.UUID `json:"group_id,omitempty"`
	GroupName string     `json:"group_name,omitempty"`
}

func (g *GroupBase) RequiresActor() bool {
	// Groups might be created automatically by the system, so we don't require an actor
	return false
}

func (g *GroupBase) TargetType() auditor.TargetType {
	return GroupType
}

func (g *GroupBase) TargetID() *uuid.UUID {
	return g.GroupID
}

func (g *GroupBase) ActionInfo() (json.RawMessage, error) {
	if g.GroupID == nil || g.GroupName == "" {
		return nil, errors.New("group id and name are required")
	}

	return json.Marshal(&g)
}

// GroupCreated represents the creation of a group
type GroupCreated struct {
	*GroupBase
	GroupDescription string `json:"group_description,omitempty"`
}

func (g *GroupCreated) ActionType() string {
	return GroupCreatedActionType
}

func (g *GroupCreated) ActionInfo() (json.RawMessage, error) {
	if _, err := g.GroupBase.ActionInfo(); err != nil {
		return nil, err
	}

	return json.Marshal(&g)
}

func (g *GroupCreated) Description() string {
	return fmt.Sprintf("%s has created the group %s", auditor.GetActorIdentifier(), g.GroupName)
}

// GroupUpdated represents an update to a group
type GroupUpdated struct {
	*GroupBase
	NewDescription *string `json:"new_description,omitempty"`
	OldName        *string `json:"old_name,omitempty"`
	NewName        *string `json:"new_name,omitempty"`
}

func (g *GroupUpdated) ActionType() string {
	return GroupUpdatedActionType
}

func (g *GroupUpdated) ActionInfo() (json.RawMessage, error) {
	if _, err := g.GroupBase.ActionInfo(); err != nil {
		return nil, err
	}

	return json.Marshal(&g)
}

func (g *GroupUpdated) Description() string {
	if g.OldName != nil && g.NewName != nil {
		return fmt.Sprintf("%s has renamed the group from %s to %s", auditor.GetActorIdentifier(), *g.OldName, *g.NewName)
	}
	return fmt.Sprintf("%s has updated the group %s", auditor.GetActorIdentifier(), g.GroupName)
}

// GroupDeleted represents the deletion of a group
type GroupDeleted struct {
	*GroupBase
}

func (g *GroupDeleted) ActionType() string {
	return GroupDeletedActionType
}

func (g *GroupDeleted) ActionInfo() (json.RawMessage, error) {
	if _, err := g.GroupBase.ActionInfo(); err != nil {
		return nil, err
	}

	return json.Marshal(&g)
}

func (g *GroupDeleted) Description() string {
	return fmt.Sprintf("%s has deleted the group %s", auditor.GetActorIdentifier(), g.GroupName)
}

// GroupMemberAdded represents the addition of a member to a group
type GroupMemberAdded struct {
	*GroupBase
	UserID     *uuid.UUID `json:"user_id,omitempty"`
	UserEmail  string     `json:"user_email,omitempty"`
	Maintainer bool       `json:"maintainer,omitempty"`
}

func (g *GroupMemberAdded) ActionType() string {
	return GroupMembershipAddedActionType
}

func (g *GroupMemberAdded) ActionInfo() (json.RawMessage, error) {
	if _, err := g.GroupBase.ActionInfo(); err != nil {
		return nil, err
	}

	if g.UserID == nil {
		return nil, fmt.Errorf("user ID is required")
	}

	return json.Marshal(&g)
}

func (g *GroupMemberAdded) Description() string {
	maintainerStatus := ""
	if g.Maintainer {
		maintainerStatus = " as a maintainer"
	}

	return fmt.Sprintf("%s has added user %s to the group %s%s",
		auditor.GetActorIdentifier(), g.UserEmail, g.GroupName, maintainerStatus)
}

// GroupMemberRemoved represents the removal of a member from a group
type GroupMemberRemoved struct {
	*GroupBase
	UserID    *uuid.UUID `json:"user_id,omitempty"`
	UserEmail string     `json:"user_email,omitempty"`
}

func (g *GroupMemberRemoved) ActionType() string {
	return GroupMembershipRemovedActionType
}

func (g *GroupMemberRemoved) ActionInfo() (json.RawMessage, error) {
	if _, err := g.GroupBase.ActionInfo(); err != nil {
		return nil, err
	}

	if g.UserID == nil {
		return nil, fmt.Errorf("user ID is required")
	}

	return json.Marshal(&g)
}

func (g *GroupMemberRemoved) Description() string {
	return fmt.Sprintf("%s has removed user %s from the group %s",
		auditor.GetActorIdentifier(), g.UserEmail, g.GroupName)
}

// GroupMemberUpdated represents the update of a group member
type GroupMemberUpdated struct {
	*GroupBase
	UserID              *uuid.UUID `json:"user_id,omitempty"`
	UserEmail           string     `json:"user_email,omitempty"`
	NewMaintainerStatus bool       `json:"new_maintainer_status,omitempty"`
	OldMaintainerStatus bool       `json:"old_maintainer_status,omitempty"`
}

func (g *GroupMemberUpdated) ActionType() string {
	return "GroupMembershipUpdated"
}

func (g *GroupMemberUpdated) ActionInfo() (json.RawMessage, error) {
	if _, err := g.GroupBase.ActionInfo(); err != nil {
		return nil, err
	}

	if g.UserID == nil {
		return nil, fmt.Errorf("user ID is required")
	}

	return json.Marshal(&g)
}

func (g *GroupMemberUpdated) Description() string {
	return fmt.Sprintf("%s has updated user %s in the group %s",
		auditor.GetActorIdentifier(), g.UserEmail, g.GroupName)
}
