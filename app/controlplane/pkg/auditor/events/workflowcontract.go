//
// Copyright 2024-2026 The Chainloop Authors.
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
	_ auditor.LogEntry = (*WorkflowContractCreated)(nil)
	_ auditor.LogEntry = (*WorkflowContractUpdated)(nil)
	_ auditor.LogEntry = (*WorkflowContractDeleted)(nil)
	_ auditor.LogEntry = (*WorkflowContractAttached)(nil)
	_ auditor.LogEntry = (*WorkflowContractDetached)(nil)
	_ auditor.LogEntry = (*WorkflowContractScopeChanged)(nil)
)

const (
	WorkflowContractType                       auditor.TargetType = "WorkflowContract"
	WorkflowContractCreatedActionType          string             = "WorkflowContractCreated"
	WorkflowContractUpdatedActionType          string             = "WorkflowContractUpdated"
	WorkflowContractDeletedActionType          string             = "WorkflowContractDeleted"
	WorkflowContractContractAttachedActionType string             = "WorkflowContractContractAttached"
	WorkflowContractContractDetachedActionType string             = "WorkflowContractContractDetached"
	WorkflowContractScopeChangedActionType     string             = "WorkflowContractScopeChanged"
)

const (
	WorkflowContractScopeOrganization = "organization"
	WorkflowContractScopeProject      = "project"
)

// WorkflowContractBase is the base struct for workflow contract events
type WorkflowContractBase struct {
	WorkflowContractID   *uuid.UUID `json:"workflow_contract_id,omitempty"`
	WorkflowContractName string     `json:"workflow_contract_name,omitempty"`
}

func (w *WorkflowContractBase) RequiresActor() bool {
	return true
}

func (w *WorkflowContractBase) TargetType() auditor.TargetType {
	return WorkflowContractType
}

func (w *WorkflowContractBase) TargetID() *uuid.UUID {
	return w.WorkflowContractID
}

func (w *WorkflowContractBase) ActionInfo() (json.RawMessage, error) {
	if w.WorkflowContractID == nil || w.WorkflowContractName == "" {
		return nil, errors.New("workflow contract id and name are required")
	}

	return json.Marshal(&w)
}

type WorkflowContractCreated struct {
	*WorkflowContractBase
}

func (w *WorkflowContractCreated) TargetID() *uuid.UUID {
	return w.WorkflowContractBase.WorkflowContractID
}

func (w *WorkflowContractCreated) Description() string {
	wfContractName := w.WorkflowContractName
	return fmt.Sprintf("%s has created the workflow contract %s", auditor.GetActorIdentifier(), wfContractName)
}

func (w *WorkflowContractCreated) ActionType() string {
	return WorkflowContractCreatedActionType
}

func (w *WorkflowContractCreated) ActionInfo() (json.RawMessage, error) {
	if w.WorkflowContractBase.WorkflowContractID == nil || w.WorkflowContractBase.WorkflowContractName == "" {
		return nil, errors.New("workflow contract id and name are required")
	}

	return json.Marshal(&w)
}

type WorkflowContractUpdated struct {
	*WorkflowContractBase
	NewRevisionID  *uuid.UUID `json:"new_revision_id,omitempty"`
	NewRevision    *int       `json:"new_revision,omitempty"`
	NewDescription *string    `json:"new_description,omitempty"`
}

func (w *WorkflowContractUpdated) ActionType() string {
	return WorkflowContractUpdatedActionType
}

func (w *WorkflowContractUpdated) ActionInfo() (json.RawMessage, error) {
	if w.WorkflowContractBase.WorkflowContractID == nil || w.WorkflowContractBase.WorkflowContractName == "" {
		return nil, errors.New("workflow contract id and name are required")
	}

	return json.Marshal(&w)
}

func (w *WorkflowContractUpdated) Description() string {
	wfContractName := w.WorkflowContractName
	return fmt.Sprintf("%s has updated the workflow contract %s", auditor.GetActorIdentifier(), wfContractName)
}

type WorkflowContractDeleted struct {
	*WorkflowContractBase
}

func (w *WorkflowContractDeleted) ActionType() string {
	return WorkflowContractDeletedActionType
}

func (w *WorkflowContractDeleted) ActionInfo() (json.RawMessage, error) {
	if w.WorkflowContractBase.WorkflowContractID == nil || w.WorkflowContractBase.WorkflowContractName == "" {
		return nil, errors.New("workflow contract id and name are required")
	}

	return json.Marshal(&w)
}

func (w *WorkflowContractDeleted) Description() string {
	wfContractName := w.WorkflowContractName
	return fmt.Sprintf("%s has deleted the workflow contract %s", auditor.GetActorIdentifier(), wfContractName)
}

type WorkflowContractAttached struct {
	*WorkflowContractBase
	WorkflowID   *uuid.UUID `json:"workflow_id,omitempty"`
	WorkflowName string     `json:"workflow_name,omitempty"`
}

func (w *WorkflowContractAttached) ActionType() string {
	return WorkflowContractContractAttachedActionType
}

func (w *WorkflowContractAttached) ActionInfo() (json.RawMessage, error) {
	if w.WorkflowContractBase.WorkflowContractID == nil || w.WorkflowContractBase.WorkflowContractName == "" || w.WorkflowID == nil || w.WorkflowName == "" {
		return nil, errors.New("workflow contract id and name, workflow id and name are required")
	}

	return json.Marshal(&w)
}

func (w *WorkflowContractAttached) Description() string {
	return fmt.Sprintf("%s has attached the workflow %s to the workflow contract %s", auditor.GetActorIdentifier(), w.WorkflowName, w.WorkflowContractName)
}

type WorkflowContractDetached struct {
	*WorkflowContractBase
	WorkflowID   *uuid.UUID `json:"workflow_id,omitempty"`
	WorkflowName string     `json:"workflow_name,omitempty"`
}

func (w *WorkflowContractDetached) ActionType() string {
	return WorkflowContractContractDetachedActionType
}

func (w *WorkflowContractDetached) ActionInfo() (json.RawMessage, error) {
	if w.WorkflowContractBase.WorkflowContractID == nil || w.WorkflowContractBase.WorkflowContractName == "" || w.WorkflowID == nil || w.WorkflowName == "" {
		return nil, errors.New("workflow contract id and name, workflow id and name are required")
	}

	return json.Marshal(&w)
}

func (w *WorkflowContractDetached) Description() string {
	return fmt.Sprintf("%s has detached the workflow %s from the workflow contract %s", auditor.GetActorIdentifier(), w.WorkflowName, w.WorkflowContractName)
}

// WorkflowContractScope is the level of a contract: the organization, or a project.
type WorkflowContractScope struct {
	// Type is WorkflowContractScopeOrganization or WorkflowContractScopeProject
	Type        string     `json:"type"`
	ProjectID   *uuid.UUID `json:"project_id,omitempty"`
	ProjectName string     `json:"project_name,omitempty"`
}

func (s *WorkflowContractScope) String() string {
	if s.Type == WorkflowContractScopeProject {
		return fmt.Sprintf("the project %s", s.ProjectName)
	}

	return "the organization level"
}

// WorkflowContractScopeChanged records the conversion of a contract between the organization
// level and a project level.
type WorkflowContractScopeChanged struct {
	*WorkflowContractBase
	OldScope *WorkflowContractScope `json:"old_scope"`
	NewScope *WorkflowContractScope `json:"new_scope"`
}

func (w *WorkflowContractScopeChanged) ActionType() string {
	return WorkflowContractScopeChangedActionType
}

func (w *WorkflowContractScopeChanged) ActionInfo() (json.RawMessage, error) {
	if w.WorkflowContractID == nil || w.WorkflowContractName == "" || w.OldScope == nil || w.NewScope == nil {
		return nil, errors.New("workflow contract id and name, old and new scope are required")
	}

	return json.Marshal(&w)
}

func (w *WorkflowContractScopeChanged) Description() string {
	return fmt.Sprintf("%s has moved the workflow contract %s from %s to %s", auditor.GetActorIdentifier(), w.WorkflowContractName, w.OldScope, w.NewScope)
}
