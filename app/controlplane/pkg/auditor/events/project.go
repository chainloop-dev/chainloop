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
	_ auditor.LogEntry = (*ProjectMembershipAdded)(nil)
	_ auditor.LogEntry = (*ProjectMembershipRemoved)(nil)
	_ auditor.LogEntry = (*ProjectMemberRoleUpdated)(nil)
)

const (
	ProjectType                        auditor.TargetType = "Project"
	ProjectMembershipAddedActionType   string             = "ProjectMembershipAdded"
	ProjectMembershipRemovedActionType string             = "ProjectMembershipRemoved"
	ProjectMemberRoleUpdatedType       string             = "ProjectMemberRoleUpdated"
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

// ProjectMemberRoleUpdated represents the update of a member's (user or group) role in a project
type ProjectMemberRoleUpdated struct {
	*ProjectBase
	// User-specific fields
	UserID    *uuid.UUID `json:"user_id,omitempty"`
	UserEmail string     `json:"user_email,omitempty"`
	// Group-specific fields
	GroupID   *uuid.UUID `json:"group_id,omitempty"`
	GroupName string     `json:"group_name,omitempty"`
	// Common fields
	OldRole string `json:"old_role,omitempty"`
	NewRole string `json:"new_role,omitempty"`
}

func (p *ProjectMemberRoleUpdated) ActionType() string {
	return ProjectMemberRoleUpdatedType
}

func (p *ProjectMemberRoleUpdated) ActionInfo() (json.RawMessage, error) {
	if _, err := p.ProjectBase.ActionInfo(); err != nil {
		return nil, err
	}

	// Validate that either user or group info is provided
	if p.UserID == nil && p.GroupID == nil {
		return nil, fmt.Errorf("either user ID or group ID is required")
	}

	return json.Marshal(&p)
}

func (p *ProjectMemberRoleUpdated) Description() string {
	if p.UserID != nil {
		// User role update
		return fmt.Sprintf("{{ if .ActorEmail }}{{ .ActorEmail }}{{ else }}system@chainloop.dev{{ end }} has updated user '%s' role in project '%s' from '%s' to '%s'",
			p.UserEmail, p.ProjectName, prettyRole(p.OldRole), prettyRole(p.NewRole))
	}

	// Group role update
	return fmt.Sprintf("{{ if .ActorEmail }}{{ .ActorEmail }}{{ else }}system@chainloop.dev{{ end }} has updated group '%s' role in project '%s' from '%s' to '%s'",
		p.GroupName, p.ProjectName, prettyRole(p.OldRole), prettyRole(p.NewRole))
}

// ProjectMembershipAdded represents the addition of a member (user or group) to a project
type ProjectMembershipAdded struct {
	*ProjectBase
	// User-specific fields
	UserID    *uuid.UUID `json:"user_id,omitempty"`
	UserEmail string     `json:"user_email,omitempty"`
	// Group-specific fields
	GroupID   *uuid.UUID `json:"group_id,omitempty"`
	GroupName string     `json:"group_name,omitempty"`
	// Common fields
	Role string `json:"role,omitempty"`
}

func (p *ProjectMembershipAdded) ActionType() string {
	return ProjectMembershipAddedActionType
}

func (p *ProjectMembershipAdded) ActionInfo() (json.RawMessage, error) {
	if _, err := p.ProjectBase.ActionInfo(); err != nil {
		return nil, err
	}

	// Validate that either user or group info is provided
	if p.UserID == nil && p.GroupID == nil {
		return nil, fmt.Errorf("either user ID or group ID is required")
	}

	return json.Marshal(&p)
}

func (p *ProjectMembershipAdded) Description() string {
	roleDesc := ""
	if p.Role != "" {
		roleDesc = fmt.Sprintf(" with role '%s'", prettyRole(p.Role))
	}

	if p.UserID != nil {
		// User addition
		return fmt.Sprintf("{{ if .ActorEmail }}{{ .ActorEmail }}{{ else }}system@chainloop.dev{{ end }} has added user '%s' to the project '%s'%s",
			p.UserEmail, p.ProjectName, roleDesc)
	}

	// Group addition
	return fmt.Sprintf("{{ if .ActorEmail }}{{ .ActorEmail }}{{ else }}system@chainloop.dev{{ end }} has added group '%s' to the project '%s'%s",
		p.GroupName, p.ProjectName, roleDesc)
}

// ProjectMembershipRemoved represents the removal of a member (user or group) from a project
type ProjectMembershipRemoved struct {
	*ProjectBase
	// User-specific fields
	UserID    *uuid.UUID `json:"user_id,omitempty"`
	UserEmail string     `json:"user_email,omitempty"`
	// Group-specific fields
	GroupID   *uuid.UUID `json:"group_id,omitempty"`
	GroupName string     `json:"group_name,omitempty"`
}

func (p *ProjectMembershipRemoved) ActionType() string {
	return ProjectMembershipRemovedActionType
}

func (p *ProjectMembershipRemoved) ActionInfo() (json.RawMessage, error) {
	if _, err := p.ProjectBase.ActionInfo(); err != nil {
		return nil, err
	}

	// Validate that either user or group info is provided
	if p.UserID == nil && p.GroupID == nil {
		return nil, fmt.Errorf("either user ID or group ID is required")
	}

	return json.Marshal(&p)
}

func (p *ProjectMembershipRemoved) Description() string {
	if p.UserID != nil {
		// User removal
		return fmt.Sprintf("{{ if .ActorEmail }}{{ .ActorEmail }}{{ else }}system@chainloop.dev{{ end }} has removed user '%s' from the project '%s'",
			p.UserEmail, p.ProjectName)
	}

	// Group removal
	return fmt.Sprintf("{{ if .ActorEmail }}{{ .ActorEmail }}{{ else }}system@chainloop.dev{{ end }} has removed group '%s' from the project '%s'",
		p.GroupName, p.ProjectName)
}
