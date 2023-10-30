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

package data

import (
	"context"

	"github.com/chainloop-dev/chainloop/app/controlplane/internal/biz"
	"github.com/chainloop-dev/chainloop/app/controlplane/internal/data/ent"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/google/uuid"
)

type OrgInvite struct {
	data *Data
	log  *log.Helper
}

func NewOrgInvite(data *Data, logger log.Logger) biz.OrgInviteRepo {
	return &OrgInvite{
		data: data,
		log:  log.NewHelper(logger),
	}
}

func (r *OrgInvite) Create(ctx context.Context, orgID, senderID uuid.UUID, receiverEmail string) (*biz.OrgInvite, error) {
	invite, err := r.data.db.OrgInvite.Create().
		SetOrganizationID(orgID).
		SetSenderID(senderID).
		SetReceiverEmail(receiverEmail).
		Save(ctx)

	if err != nil {
		return nil, err
	}

	return entInviteToBiz(invite), nil
}

func entInviteToBiz(i *ent.OrgInvite) *biz.OrgInvite {
	if i == nil {
		return nil
	}

	return &biz.OrgInvite{
		ID:            i.ID,
		ReceiverEmail: i.ReceiverEmail,
		CreatedAt:     toTimePtr(i.CreatedAt),
		OrgID:         i.OrganizationID,
		SenderID:      i.SenderID,
		Status:        i.Status,
	}
}
