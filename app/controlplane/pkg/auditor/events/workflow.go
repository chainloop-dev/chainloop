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
	_ auditor.LogEntry = (*WorkflowCreated)(nil)
)

const (
	WorkflowType              auditor.TargetType = "Workflow"
	WorkflowCreatedActionType string             = "WorkflowCreated"
	WorkflowUpdatedActionType string             = "WorkflowUpdated"
	WorkflowDeletedActionType string             = "WorkflowDeleted"
)

// WorkflowBase is the base struct for workflow events
type WorkflowBase struct {
	WorkflowID   *uuid.UUID `json:"workflow_id,omitempty"`
	WorkflowName string     `json:"workflow_name,omitempty"`
	ProjectName  string     `json:"project_name,omitempty"`
}

func (w *WorkflowBase) RequiresActor() bool {
	return true
}

func (w *WorkflowBase) TargetType() auditor.TargetType {
	return WorkflowType
}

func (w *WorkflowBase) TargetID() *uuid.UUID {
	return w.WorkflowID
}

func (w *WorkflowBase) ActionInfo() (json.RawMessage, error) {
	if w.WorkflowID == nil || w.WorkflowName == "" || w.ProjectName == "" {
		return nil, errors.New("workflow id, name and project name are required")
	}

	return json.Marshal(&w)
}

type WorkflowCreated struct {
	*WorkflowBase
	WorkflowContractID   *uuid.UUID `json:"workflow_contract_id,omitempty"`
	WorkflowContractName string     `json:"workflow_contract_name,omitempty"`
	WorkflowDescription  *string    `json:"description,omitempty"`
	Team                 *string    `json:"team,omitempty"`
	Public               bool       `json:"public,omitempty"`
}

func (w *WorkflowCreated) TargetID() *uuid.UUID {
	return w.WorkflowID
}

func (w *WorkflowCreated) Description() string {
	workflowName := w.WorkflowName
	projectName := w.ProjectName
	workflowContractName := w.WorkflowContractName
	return fmt.Sprintf("{{ if .ActorEmail }}{{ .ActorEmail }}{{ else }}API Token {{ .ActorID }}{{ end }} has created the workflow %s on project %s with the contract %s", workflowName, projectName, workflowContractName)
}

func (w *WorkflowCreated) ActionType() string {
	return WorkflowCreatedActionType
}

func (w *WorkflowCreated) ActionInfo() (json.RawMessage, error) {
	if w.WorkflowID == nil || w.WorkflowName == "" || w.ProjectName == "" || w.WorkflowContractID == nil || w.WorkflowContractName == "" {
		return nil, errors.New("workflow id, name, project name, contract id and contract name required")
	}

	return json.Marshal(&w)
}

type WorkflowUpdated struct {
	*WorkflowBase
	NewDescription          *string    `json:"new_description,omitempty"`
	NewTeam                 *string    `json:"new_team,omitempty"`
	NewPublic               *bool      `json:"new_public,omitempty"`
	NewWorkflowContractID   *uuid.UUID `json:"new_workflow_contract_id,omitempty"`
	NewWorkflowContractName *string    `json:"new_workflow_contract_name,omitempty"`
}

func (w *WorkflowUpdated) ActionType() string {
	return WorkflowUpdatedActionType
}

func (w *WorkflowUpdated) ActionInfo() (json.RawMessage, error) {
	if w.WorkflowID == nil || w.WorkflowName == "" || w.ProjectName == "" {
		return nil, errors.New("workflow id, name and project name are required")
	}

	return json.Marshal(&w)
}

func (w *WorkflowUpdated) Description() string {
	workflowName := w.WorkflowName
	projectName := w.ProjectName
	return fmt.Sprintf("{{ if .ActorEmail }}{{ .ActorEmail }}{{ else }}API Token {{ .ActorID }}{{ end }} has updated the workflow %s on project %s", workflowName, projectName)
}

type WorkflowDeleted struct {
	*WorkflowBase
}

func (w *WorkflowDeleted) ActionType() string {
	return WorkflowDeletedActionType
}

func (w *WorkflowDeleted) ActionInfo() (json.RawMessage, error) {
	if w.WorkflowID == nil || w.WorkflowName == "" || w.ProjectName == "" {
		return nil, errors.New("workflow id, name and project name are required")
	}

	return json.Marshal(&w)
}

func (w *WorkflowDeleted) Description() string {
	wfName := w.WorkflowName
	return fmt.Sprintf("{{ if .ActorEmail }}{{ .ActorEmail }}{{ else }}API Token {{ .ActorID }}{{ end }} has deleted the workflow %s", wfName)
}
