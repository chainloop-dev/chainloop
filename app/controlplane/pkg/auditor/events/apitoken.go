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
	"time"

	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/auditor"

	"github.com/google/uuid"
)

var (
	_ auditor.LogEntry = (*APITokenCreated)(nil)
	_ auditor.LogEntry = (*APITokenRevoked)(nil)
)

const (
	APITokenType              auditor.TargetType = "APIToken"
	APITokenCreatedActionType string             = "APITokenCreated"
	APITokenRevokedActionType string             = "APITokenRevoked"
)

type APITokenBase struct {
	APITokenID   *uuid.UUID `json:"api_token_id,omitempty"`
	APITokenName string     `json:"api_token_name,omitempty"`
}

func (a *APITokenBase) RequiresActor() bool {
	return true
}

func (a *APITokenBase) TargetType() auditor.TargetType {
	return APITokenType
}

func (a *APITokenBase) TargetID() *uuid.UUID {
	return a.APITokenID
}

func (a *APITokenBase) ActionInfo() (json.RawMessage, error) {
	if a.APITokenID == nil {
		return nil, errors.New("api token id is required")
	}
	if a.APITokenName == "" {
		return nil, errors.New("api token name is required")
	}

	return json.Marshal(&a)
}

type APITokenCreated struct {
	*APITokenBase
	APITokenDescription *string    `json:"description,omitempty"`
	ExpiresAt           *time.Time `json:"expires_at,omitempty"`
}

func (a *APITokenCreated) ActionType() string {
	return APITokenCreatedActionType
}

func (a *APITokenCreated) ActionInfo() (json.RawMessage, error) {
	_, err := a.APITokenBase.ActionInfo()
	if err != nil {
		return nil, fmt.Errorf("getting action info: %w", err)
	}

	return json.Marshal(&a)
}

func (a *APITokenCreated) Description() string {
	if a.ExpiresAt != nil {
		return fmt.Sprintf("{{ .ActorEmail }} has created the API token %s expiring at %s", a.APITokenName, a.ExpiresAt.Format(time.RFC3339))
	}
	return fmt.Sprintf("{{ .ActorEmail }} has created the API token %s", a.APITokenName)
}

type APITokenRevoked struct {
	*APITokenBase
}

func (a *APITokenRevoked) ActionType() string {
	return APITokenRevokedActionType
}

func (a *APITokenRevoked) ActionInfo() (json.RawMessage, error) {
	_, err := a.APITokenBase.ActionInfo()
	if err != nil {
		return nil, fmt.Errorf("getting action info: %w", err)
	}

	return json.Marshal(&a)
}

func (a *APITokenRevoked) Description() string {
	return fmt.Sprintf("{{ .ActorEmail }} has revoked the API token %s", a.APITokenName)
}
