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
	_ auditor.LogEntry = (*OrgUserJoined)(nil)
	_ auditor.LogEntry = (*OrgUserLeft)(nil)
	_ auditor.LogEntry = (*OrgCreated)(nil)
)

const (
	OrgType                    auditor.TargetType = "Organization"
	userJoinedOrgActionType    string             = "UserJoined"
	userLeftOrgActionType      string             = "UserLeft"
	userInvitedToOrgActionType string             = "InvitationCreated"
	orgCreatedActionType       string             = "OrganizationCreated"
)

type OrgBase struct {
	OrgID   *uuid.UUID `json:"org_id,omitempty"`
	OrgName string     `json:"org_name,omitempty"`
}

func (p *OrgBase) RequiresActor() bool {
	return true
}

func (p *OrgBase) TargetType() auditor.TargetType {
	return OrgType
}

func (p *OrgBase) TargetID() *uuid.UUID {
	return p.OrgID
}

func (p *OrgBase) ActionInfo() (json.RawMessage, error) {
	if p.OrgName == "" || p.OrgID == nil {
		return nil, errors.New("org name and org id are required")
	}

	return json.Marshal(&p)
}

// Org created
type OrgCreated struct {
	*OrgBase
}

func (p *OrgCreated) ActionType() string {
	return orgCreatedActionType
}

func (p *OrgCreated) Description() string {
	return fmt.Sprintf("%s has created the organization %s", auditor.GetActorIdentifier(), p.OrgName)
}

// user joined the organization
type OrgUserJoined struct {
	*OrgBase
	// UserID of the user that joined the organization
	UserID uuid.UUID `json:"user_id,omitempty"`
	// UserEmail of the user that joined the organization
	UserEmail string `json:"user_email,omitempty"`
	// InvitationID is the ID of the invitation that was used to join the organization
	InvitationID uuid.UUID `json:"invitation_id,omitempty"`
}

func (p *OrgUserJoined) ActionType() string {
	return userJoinedOrgActionType
}

func (p *OrgUserJoined) Description() string {
	return fmt.Sprintf("%s has joined the organization %s", auditor.GetActorIdentifier(), p.OrgName)
}

func (p *OrgUserJoined) ActionInfo() (json.RawMessage, error) {
	if p.OrgName == "" || p.OrgID == nil || p.UserID == uuid.Nil || p.UserEmail == "" || p.InvitationID == uuid.Nil {
		return nil, errors.New("org name, org id, user id, user email and invitation id are required")
	}

	return json.Marshal(&p)
}

// user left the organization
type OrgUserLeft struct {
	*OrgBase
}

func (p *OrgUserLeft) ActionType() string {
	return userLeftOrgActionType
}

func (p *OrgUserLeft) Description() string {
	return fmt.Sprintf("%s has left the organization %s", auditor.GetActorIdentifier(), p.OrgName)
}

// user got invited to the organization
type OrgUserInvited struct {
	*OrgBase
	ReceiverEmail string
	Role          string
}

func (p *OrgUserInvited) ActionType() string {
	return userInvitedToOrgActionType
}

func (p *OrgUserInvited) Description() string {
	return fmt.Sprintf("%s has invited %s to the organization %s with role %s", auditor.GetActorIdentifier(), p.ReceiverEmail, p.OrgName, p.Role)
}

func (p *OrgUserInvited) ActionInfo() (json.RawMessage, error) {
	if p.OrgName == "" || p.ReceiverEmail == "" {
		return nil, errors.New("org name and receiver emails are required")
	}

	return json.Marshal(&p)
}
