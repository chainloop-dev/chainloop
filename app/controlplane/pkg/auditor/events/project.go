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
	"strings"

	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/auditor"

	"github.com/google/uuid"
)

var (
	_ auditor.LogEntry = (*ProjectMemberAdded)(nil)
	_ auditor.LogEntry = (*ProjectMemberRemoved)(nil)
	_ auditor.LogEntry = (*ProjectGroupAdded)(nil)
	_ auditor.LogEntry = (*ProjectGroupRemoved)(nil)
)

const (
	ProjectType                    auditor.TargetType = "Project"
	ProjectMemberAddedActionType   string             = "ProjectMemberAdded"
	ProjectMemberRemovedActionType string             = "ProjectMemberRemoved"
	ProjectGroupAddedActionType    string             = "ProjectGroupAdded"
	ProjectGroupRemovedActionType  string             = "ProjectGroupRemoved"
)

// ProjectBase is the base struct for project events
type ProjectBase struct {
	ProjectID   *uuid.UUID `json:"project_id,omitempty"`
	ProjectName string     `json:"project_name,omitempty"`
}

func (p *ProjectBase) RequiresActor() bool {
	return true
}

func (p *ProjectBase) TargetType() auditor.TargetType {
	return ProjectType
}

func (p *ProjectBase) TargetID() *uuid.UUID {
	return p.ProjectID
}

func (p *ProjectBase) ActionInfo() (json.RawMessage, error) {
	if p.ProjectID == nil || p.ProjectName == "" {
		return nil, errors.New("project id and name are required")
	}

	return json.Marshal(&p)
}

// Helper function to make role names more user-friendly
func prettyRole(role string) string {
	// Convert the role to a prettier format
	prettyRole := role
	if strings.HasPrefix(role, "role:project:") {
		prettyRole = strings.TrimPrefix(role, "role:project:")
	}
	return prettyRole
}

// ProjectMemberAdded represents the addition of a member to a project
type ProjectMemberAdded struct {
	*ProjectBase
	UserID    *uuid.UUID `json:"user_id,omitempty"`
	UserEmail string     `json:"user_email,omitempty"`
	Role      string     `json:"role,omitempty"`
}

func (p *ProjectMemberAdded) ActionType() string {
	return ProjectMemberAddedActionType
}

func (p *ProjectMemberAdded) ActionInfo() (json.RawMessage, error) {
	if _, err := p.ProjectBase.ActionInfo(); err != nil {
		return nil, err
	}

	if p.UserID == nil {
		return nil, fmt.Errorf("user ID is required")
	}

	return json.Marshal(&p)
}

func (p *ProjectMemberAdded) Description() string {
	roleDesc := ""
	if p.Role != "" {
		roleDesc = fmt.Sprintf(" with role '%s'", prettyRole(p.Role))
	}

	return fmt.Sprintf("{{ if .ActorEmail }}{{ .ActorEmail }}{{ else }}API Token {{ .ActorID }}{{ end }} has added user '%s' to the project '%s'%s",
		p.UserEmail, p.ProjectName, roleDesc)
}

// ProjectMemberRemoved represents the removal of a member from a project
type ProjectMemberRemoved struct {
	*ProjectBase
	UserID    *uuid.UUID `json:"user_id,omitempty"`
	UserEmail string     `json:"user_email,omitempty"`
}

func (p *ProjectMemberRemoved) ActionType() string {
	return ProjectMemberRemovedActionType
}

func (p *ProjectMemberRemoved) ActionInfo() (json.RawMessage, error) {
	if _, err := p.ProjectBase.ActionInfo(); err != nil {
		return nil, err
	}

	if p.UserID == nil {
		return nil, fmt.Errorf("user ID is required")
	}

	return json.Marshal(&p)
}

func (p *ProjectMemberRemoved) Description() string {
	return fmt.Sprintf("{{ if .ActorEmail }}{{ .ActorEmail }}{{ else }}API Token {{ .ActorID }}{{ end }} has removed user '%s' from the project '%s'",
		p.UserEmail, p.ProjectName)
}

// ProjectGroupAdded represents the addition of a group to a project
type ProjectGroupAdded struct {
	*ProjectBase
	GroupID   *uuid.UUID `json:"group_id,omitempty"`
	GroupName string     `json:"group_name,omitempty"`
	Role      string     `json:"role,omitempty"`
}

func (p *ProjectGroupAdded) ActionType() string {
	return ProjectGroupAddedActionType
}

func (p *ProjectGroupAdded) ActionInfo() (json.RawMessage, error) {
	if _, err := p.ProjectBase.ActionInfo(); err != nil {
		return nil, err
	}

	if p.GroupID == nil {
		return nil, fmt.Errorf("group ID is required")
	}

	return json.Marshal(&p)
}

func (p *ProjectGroupAdded) Description() string {
	// Create a prettier role description
	roleDesc := ""
	if p.Role != "" {
		roleDesc = fmt.Sprintf(" with role '%s'", prettyRole(p.Role))
	}

	return fmt.Sprintf("{{ if .ActorEmail }}{{ .ActorEmail }}{{ else }}API Token {{ .ActorID }}{{ end }} has added group '%s' to the project '%s'%s",
		p.GroupName, p.ProjectName, roleDesc)
}

// ProjectGroupRemoved represents the removal of a group from a project
type ProjectGroupRemoved struct {
	*ProjectBase
	GroupID   *uuid.UUID `json:"group_id,omitempty"`
	GroupName string     `json:"group_name,omitempty"`
}

func (p *ProjectGroupRemoved) ActionType() string {
	return ProjectGroupRemovedActionType
}

func (p *ProjectGroupRemoved) ActionInfo() (json.RawMessage, error) {
	if _, err := p.ProjectBase.ActionInfo(); err != nil {
		return nil, err
	}

	if p.GroupID == nil {
		return nil, fmt.Errorf("group ID is required")
	}

	return json.Marshal(&p)
}

func (p *ProjectGroupRemoved) Description() string {
	return fmt.Sprintf("{{ if .ActorEmail }}{{ .ActorEmail }}{{ else }}API Token {{ .ActorID }}{{ end }} has removed group '%s' from the project '%s'",
		p.GroupName, p.ProjectName)
}
