//
// Copyright 2024-2025 The Chainloop Authors.
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
	"time"

	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/auditor"
	"github.com/google/uuid"
)

var (
	_ auditor.LogEntry = (*UserSignedUp)(nil)
	_ auditor.LogEntry = (*UserLoggedIn)(nil)
)

const (
	UserType                  auditor.TargetType = "User"
	UserSignedUpActionType    string             = "SignedUp"
	UserLoggedInActionType    string             = "LoggedIn"
	UserRoleChangedActionType string             = "RoleChanged"
)

// UserBase is the base struct for policy events
type UserBase struct {
	UserID    *uuid.UUID `json:"user_id,omitempty"`
	Email     string     `json:"email,omitempty"`
	SSOGroups []string   `json:"sso_groups,omitempty"`
}

func (p *UserBase) RequiresActor() bool {
	return true
}

func (p *UserBase) TargetType() auditor.TargetType {
	return UserType
}

func (p *UserBase) TargetID() *uuid.UUID {
	return p.UserID
}

func (p *UserBase) ActionInfo() (json.RawMessage, error) {
	if p.UserID == nil || p.Email == "" {
		return nil, errors.New("user id and email are required")
	}

	return json.Marshal(&p)
}

type UserSignedUp struct {
	*UserBase
}

func (p *UserSignedUp) ActionType() string {
	return UserSignedUpActionType
}

func (p *UserSignedUp) Description() string {
	return fmt.Sprintf("%s has signed up", p.Email)
}

type UserLoggedIn struct {
	*UserBase
	// This timestamp can be used to generate a new digest for the user and burst the cache
	LoggedIn time.Time
}

func (p *UserLoggedIn) ActionType() string {
	return UserLoggedInActionType
}

func (p *UserLoggedIn) Description() string {
	return fmt.Sprintf("%s has logged in", p.Email)
}

func (p *UserLoggedIn) ActionInfo() (json.RawMessage, error) {
	if p.UserID == nil || p.Email == "" || p.LoggedIn.IsZero() {
		return nil, errors.New("user id and email are required")
	}

	return json.Marshal(&p)
}

type UserRoleChanged struct {
	*UserBase
	OldRole string `json:"old_role,omitempty"`
	NewRole string `json:"new_role,omitempty"`
}

func (p *UserRoleChanged) ActionType() string {
	return UserRoleChangedActionType
}

func (p *UserRoleChanged) Description() string {
	return fmt.Sprintf("%s has changed %s role from '%s' to '%s'", auditor.GetActorIdentifier(), p.Email, p.OldRole, p.NewRole)
}

func (p *UserRoleChanged) ActionInfo() (json.RawMessage, error) {
	if p.UserID == nil || p.Email == "" || p.OldRole == "" || p.NewRole == "" {
		return nil, errors.New("user id, email, old role and new role are required")
	}

	return json.Marshal(&p)
}
