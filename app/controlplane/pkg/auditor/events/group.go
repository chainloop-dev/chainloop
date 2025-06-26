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
)

const (
	GroupType              auditor.TargetType = "Group"
	GroupCreatedActionType string             = "GroupCreated"
	GroupUpdatedActionType string             = "GroupUpdated"
	GroupDeletedActionType string             = "GroupDeleted"
)

// GroupBase is the base struct for group events
type GroupBase struct {
	GroupID   *uuid.UUID `json:"group_id,omitempty"`
	GroupName string     `json:"group_name,omitempty"`
}

func (g *GroupBase) RequiresActor() bool {
	return true
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
	return fmt.Sprintf("{{ if .ActorEmail }}{{ .ActorEmail }}{{ else }}API Token {{ .ActorID }}{{ end }} has created the group %s", g.GroupName)
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
		return fmt.Sprintf("{{ if .ActorEmail }}{{ .ActorEmail }}{{ else }}API Token {{ .ActorID }}{{ end }} has renamed the group from %s to %s", *g.OldName, *g.NewName)
	}
	return fmt.Sprintf("{{ if .ActorEmail }}{{ .ActorEmail }}{{ else }}API Token {{ .ActorID }}{{ end }} has updated the group %s", g.GroupName)
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
	return fmt.Sprintf("{{ if .ActorEmail }}{{ .ActorEmail }}{{ else }}API Token {{ .ActorID }}{{ end }} has deleted the group %s", g.GroupName)
}
