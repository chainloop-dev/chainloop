//
// Copyright 2023 The Chainloop Authors.
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

package biz

import (
	"context"
	"time"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/google/uuid"
)

type OrgInviteUseCase struct {
	logger *log.Helper
	repo   OrgInviteRepo
}

type OrgInvite struct {
	ID            uuid.UUID
	OrgID         uuid.UUID
	SenderID      uuid.UUID
	ReceiverEmail string
	CreatedAt     *time.Time
	Status        OrgInviteStatus
}

type OrgInviteRepo interface {
	Create(ctx context.Context, orgID, senderID uuid.UUID, receiverEmail string) (*OrgInvite, error)
}

func NewOrgInviteUseCase(r OrgInviteRepo, l log.Logger) (*OrgInviteUseCase, error) {
	return &OrgInviteUseCase{logger: log.NewHelper(l), repo: r}, nil
}

type OrgInviteStatus string

var (
	OrgInviteStatusPending  OrgInviteStatus = "pending"
	OrgInviteStatusAccepted OrgInviteStatus = "accepted"
	OrgInviteStatusDeclined OrgInviteStatus = "declined"
)

// Implements https://pkg.go.dev/entgo.io/ent/schema/field#EnumValues
func (OrgInviteStatus) Values() (kinds []string) {
	for _, s := range []OrgInviteStatus{OrgInviteStatusAccepted, OrgInviteStatusDeclined, OrgInviteStatusPending} {
		kinds = append(kinds, string(s))
	}

	return
}
