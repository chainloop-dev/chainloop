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
		return nil, errors.New("user id and org name are required")
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
	return fmt.Sprintf("{{ .ActorEmail }} has created the organization %s", p.OrgName)
}

// user joined the organization
type OrgUserJoined struct {
	*OrgBase
}

func (p *OrgUserJoined) ActionType() string {
	return userJoinedOrgActionType
}

func (p *OrgUserJoined) Description() string {
	return fmt.Sprintf("{{ .ActorEmail }} has joined the organization %s", p.OrgName)
}

// user left the organization
type OrgUserLeft struct {
	*OrgBase
}

func (p *OrgUserLeft) ActionType() string {
	return userLeftOrgActionType
}

func (p *OrgUserLeft) Description() string {
	return fmt.Sprintf("{{ .ActorEmail }} has left the organization %s", p.OrgName)
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
	return fmt.Sprintf("{{ .ActorEmail }} has invited %s to the organization %s with role %s", p.ReceiverEmail, p.OrgName, p.Role)
}

func (p *OrgUserInvited) ActionInfo() (json.RawMessage, error) {
	if p.OrgName == "" || p.ReceiverEmail == "" {
		return nil, errors.New("org name and receiver emails are required")
	}

	return json.Marshal(&p)
}
