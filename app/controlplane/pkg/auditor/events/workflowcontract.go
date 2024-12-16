//
// Copyright 2024 The Chainloop Authors.
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
)

const (
	WorkflowContractType                       auditor.TargetType = "WorkflowContract"
	WorkflowContractCreatedActionType          string             = "WorkflowContractCreated"
	WorkflowContractUpdatedActionType          string             = "WorkflowContractUpdated"
	WorkflowContractDeletedActionType          string             = "WorkflowContractDeleted"
	WorkflowContractContractAttachedActionType string             = "WorkflowContractContractAttached"
	WorkflowContractContractDetachedActionType string             = "WorkflowContractContractDetached"
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
	return fmt.Sprintf("{{ if .ActorEmail }}{{ .ActorEmail }}{{ else }}API Token {{ .ActorID }}{{ end }} has created the workflow contract %s", wfContractName)
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
	return fmt.Sprintf("{{ if .ActorEmail }}{{ .ActorEmail }}{{ else }}API Token {{ .ActorID }}{{ end }} has updated the workflow contract %s", wfContractName)
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
	return fmt.Sprintf("{{ if .ActorEmail }}{{ .ActorEmail }}{{ else }}API Token {{ .ActorID }}{{ end }} has deleted the workflow contract %s", wfContractName)
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
	return fmt.Sprintf("{{ if .ActorEmail }}{{ .ActorEmail }}{{ else }}API Token {{ .ActorID }}{{ end }} has attached the workflow %s to the workflow contract %s", w.WorkflowName, w.WorkflowContractName)
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
	return fmt.Sprintf("{{ if .ActorEmail }}{{ .ActorEmail }}{{ else }}API Token {{ .ActorID }}{{ end }} has detached the workflow %s from the workflow contract %s", w.WorkflowName, w.WorkflowContractName)
}
