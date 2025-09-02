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
	_ auditor.LogEntry = (*CASBackendCreated)(nil)
	_ auditor.LogEntry = (*CASBackendUpdated)(nil)
	_ auditor.LogEntry = (*CASBackendDeleted)(nil)
	_ auditor.LogEntry = (*CASBackendPermanentDeleted)(nil)
	_ auditor.LogEntry = (*CASBackendStatusChanged)(nil)
)

const (
	CASBackendType                 auditor.TargetType = "CASBackend"
	CASBackendCreatedActionType    string             = "CASBackendCreated"
	CASBackendUpdatedActionType    string             = "CASBackendUpdated"
	CASBackendDeletedActionType    string             = "CASBackendSoftDeleted"
	CASBackendPermanentDeletedType string             = "CASBackendPermanentDeleted"
	CASBackendStatusChangedAction  string             = "CASBackendStatusChanged"
)

// CASBackendBase contains the common fields for all CAS backend events
type CASBackendBase struct {
	CASBackendID   *uuid.UUID `json:"cas_backend_id,omitempty"`
	CASBackendName string     `json:"cas_backend_name,omitempty"`
	Provider       string     `json:"provider,omitempty"`
	Location       string     `json:"location,omitempty"`
	Default        bool       `json:"default,omitempty"`
}

func (c *CASBackendBase) RequiresActor() bool {
	return true
}

func (c *CASBackendBase) TargetType() auditor.TargetType {
	return CASBackendType
}

func (c *CASBackendBase) TargetID() *uuid.UUID {
	return c.CASBackendID
}

func (c *CASBackendBase) ActionInfo() (json.RawMessage, error) {
	if c.CASBackendID == nil || c.CASBackendName == "" {
		return nil, errors.New("cas backend id and name are required")
	}

	return json.Marshal(&c)
}

// CASBackendCreated represents the creation of a CAS backend
type CASBackendCreated struct {
	*CASBackendBase
	CASBackendDescription string `json:"description,omitempty"`
}

func (c *CASBackendCreated) ActionType() string {
	return CASBackendCreatedActionType
}

func (c *CASBackendCreated) ActionInfo() (json.RawMessage, error) {
	if _, err := c.CASBackendBase.ActionInfo(); err != nil {
		return nil, err
	}

	return json.Marshal(&c)
}

func (c *CASBackendCreated) Description() string {
	defaultStatus := "non-default"
	if c.Default {
		defaultStatus = "default"
	}
	return fmt.Sprintf("%s has created CAS backend %s with provider %s (%s)", auditor.GetActorIdentifier(), c.CASBackendName, c.Provider, defaultStatus)
}

// CASBackendUpdated represents an update to a CAS backend
type CASBackendUpdated struct {
	*CASBackendBase
	NewDescription     *string `json:"new_description,omitempty"`
	CredentialsChanged bool    `json:"credentials_changed,omitempty"`
	PreviousDefault    bool    `json:"previous_default,omitempty"`
}

func (c *CASBackendUpdated) ActionType() string {
	return CASBackendUpdatedActionType
}

func (c *CASBackendUpdated) ActionInfo() (json.RawMessage, error) {
	if _, err := c.CASBackendBase.ActionInfo(); err != nil {
		return nil, err
	}

	return json.Marshal(&c)
}

func (c *CASBackendUpdated) Description() string {
	var credentialsInfo string
	if c.CredentialsChanged {
		// nolint: gosec
		credentialsInfo = " and updated credentials"
	}

	if c.PreviousDefault != c.Default {
		defaultStatus := "default"
		if !c.Default {
			defaultStatus = "non-default"
		}
		return fmt.Sprintf("%s has updated CAS backend %s to %s%s",
			auditor.GetActorIdentifier(), c.CASBackendName, defaultStatus, credentialsInfo)
	}

	return fmt.Sprintf("%s has updated CAS backend %s%s",
		auditor.GetActorIdentifier(), c.CASBackendName, credentialsInfo)
}

// CASBackendDeleted represents the deletion of a CAS backend
type CASBackendDeleted struct {
	*CASBackendBase
}

func (c *CASBackendDeleted) ActionType() string {
	return CASBackendDeletedActionType
}

func (c *CASBackendDeleted) ActionInfo() (json.RawMessage, error) {
	if _, err := c.CASBackendBase.ActionInfo(); err != nil {
		return nil, err
	}

	return json.Marshal(&c)
}

func (c *CASBackendDeleted) Description() string {
	return fmt.Sprintf("%s has deleted CAS backend %s", auditor.GetActorIdentifier(), c.CASBackendName)
}

// CASBackendPermanentDeleted represents the permanent deletion of a CAS backend
type CASBackendPermanentDeleted struct {
	*CASBackendBase
}

func (c *CASBackendPermanentDeleted) ActionType() string {
	return CASBackendPermanentDeletedType
}

func (c *CASBackendPermanentDeleted) ActionInfo() (json.RawMessage, error) {
	if _, err := c.CASBackendBase.ActionInfo(); err != nil {
		return nil, err
	}

	return json.Marshal(&c)
}

func (c *CASBackendPermanentDeleted) Description() string {
	return fmt.Sprintf("%s has permanently deleted CAS backend %s", auditor.GetActorIdentifier(), c.CASBackendName)
}

// CASBackendStatusChanged represents a change in the validation status of a CAS backend
type CASBackendStatusChanged struct {
	*CASBackendBase
	PreviousStatus string `json:"previous_status,omitempty"`
	NewStatus      string `json:"new_status,omitempty"`
	IsRecovery     bool   `json:"is_recovery,omitempty"`
}

func (c *CASBackendStatusChanged) ActionType() string {
	return CASBackendStatusChangedAction
}

func (c *CASBackendStatusChanged) ActionInfo() (json.RawMessage, error) {
	if _, err := c.CASBackendBase.ActionInfo(); err != nil {
		return nil, err
	}

	return json.Marshal(&c)
}

func (c *CASBackendStatusChanged) Description() string {
	var statusInfo string
	if c.IsRecovery {
		statusInfo = " has recovered from invalid state"
	} else {
		statusInfo = fmt.Sprintf(" status changed from %s to %s", c.PreviousStatus, c.NewStatus)
	}

	return fmt.Sprintf("CAS backend %s%s", c.CASBackendName, statusInfo)
}

func (c *CASBackendStatusChanged) RequiresActor() bool {
	// Status changes are system-generated, no actor required
	return false
}
