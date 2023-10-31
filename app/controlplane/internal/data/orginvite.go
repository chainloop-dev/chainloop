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
	"fmt"
	"time"

	"github.com/chainloop-dev/chainloop/app/controlplane/internal/biz"
	"github.com/chainloop-dev/chainloop/app/controlplane/internal/data/ent"
	"github.com/chainloop-dev/chainloop/app/controlplane/internal/data/ent/orginvite"
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

	return r.FindByID(ctx, invite.ID)
}

func (r *OrgInvite) PendingInvite(ctx context.Context, orgID uuid.UUID, receiverEmail string) (*biz.OrgInvite, error) {
	invite, err := r.query().
		Where(
			orginvite.OrganizationID(orgID),
			orginvite.ReceiverEmail(receiverEmail),
			orginvite.StatusEQ(biz.OrgInviteStatusPending),
			orginvite.DeletedAtIsNil(),
		).Only(ctx)
	if err != nil && !ent.IsNotFound(err) {
		return nil, err
	} else if invite == nil {
		return nil, nil
	}

	return entInviteToBiz(invite), nil
}

func (r *OrgInvite) PendingInvites(ctx context.Context, receiverEmail string) ([]*biz.OrgInvite, error) {
	invites, err := r.query().Where(
		orginvite.ReceiverEmail(receiverEmail),
		orginvite.StatusEQ(biz.OrgInviteStatusPending),
		orginvite.DeletedAtIsNil()).All(ctx)
	if err != nil {
		return nil, fmt.Errorf("error finding invites for user %s: %w", receiverEmail, err)
	}

	res := make([]*biz.OrgInvite, len(invites))
	for i, v := range invites {
		res[i] = entInviteToBiz(v)
	}

	return res, nil
}

func (r *OrgInvite) ChangeStatus(ctx context.Context, id uuid.UUID, status biz.OrgInviteStatus) error {
	return r.data.db.OrgInvite.UpdateOneID(id).SetStatus(status).Exec(ctx)
}

// Full query with dependencies
func (r *OrgInvite) query() *ent.OrgInviteQuery {
	return r.data.db.OrgInvite.Query().WithOrganization().WithSender()
}

func (r *OrgInvite) FindByID(ctx context.Context, id uuid.UUID) (*biz.OrgInvite, error) {
	invite, err := r.query().Where(orginvite.ID(id), orginvite.DeletedAtIsNil()).Only(ctx)
	if err != nil && !ent.IsNotFound(err) {
		return nil, fmt.Errorf("error finding invite %s: %w", id.String(), err)
	} else if invite == nil {
		return nil, nil
	}

	return entInviteToBiz(invite), nil
}

func (r *OrgInvite) SoftDelete(ctx context.Context, id uuid.UUID) error {
	return r.data.db.OrgInvite.UpdateOneID(id).SetDeletedAt(time.Now()).Exec(ctx)
}

func (r *OrgInvite) ListBySender(ctx context.Context, userID uuid.UUID) ([]*biz.OrgInvite, error) {
	invite, err := r.query().Where(orginvite.SenderID(userID), orginvite.DeletedAtIsNil()).All(ctx)
	if err != nil {
		return nil, fmt.Errorf("error finding invites for user %s: %w", userID.String(), err)
	}

	res := make([]*biz.OrgInvite, len(invite))
	for i, v := range invite {
		res[i] = entInviteToBiz(v)
	}

	return res, nil
}

func entInviteToBiz(i *ent.OrgInvite) *biz.OrgInvite {
	if i == nil {
		return nil
	}

	res := &biz.OrgInvite{
		ID:            i.ID,
		ReceiverEmail: i.ReceiverEmail,
		CreatedAt:     toTimePtr(i.CreatedAt),
		Status:        i.Status,
	}

	if i.Edges.Organization != nil {
		res.Org = entOrgToBizOrg(i.Edges.Organization)
	}

	if i.Edges.Sender != nil {
		res.Sender = entUserToBizUser(i.Edges.Sender)
	}

	return res
}
