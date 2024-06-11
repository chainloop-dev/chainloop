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

	"github.com/chainloop-dev/chainloop/app/controlplane/internal/authz"
	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/biz"
	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/data/ent"
	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/data/ent/orginvitation"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/google/uuid"
)

type OrgInvitation struct {
	data *Data
	log  *log.Helper
}

func NewOrgInvitation(data *Data, logger log.Logger) biz.OrgInvitationRepo {
	return &OrgInvitation{
		data: data,
		log:  log.NewHelper(logger),
	}
}

func (r *OrgInvitation) Create(ctx context.Context, orgID, senderID uuid.UUID, receiverEmail string, role authz.Role) (*biz.OrgInvitation, error) {
	invite, err := r.data.DB.OrgInvitation.Create().
		SetOrganizationID(orgID).
		SetSenderID(senderID).
		SetRole(role).
		SetReceiverEmail(receiverEmail).
		Save(ctx)

	if err != nil {
		return nil, err
	}

	return r.FindByID(ctx, invite.ID)
}

func (r *OrgInvitation) PendingInvitation(ctx context.Context, orgID uuid.UUID, receiverEmail string) (*biz.OrgInvitation, error) {
	invite, err := r.query().
		Where(
			orginvitation.OrganizationID(orgID),
			orginvitation.ReceiverEmail(receiverEmail),
			orginvitation.StatusEQ(biz.OrgInvitationStatusPending),
		).Only(ctx)
	if err != nil && !ent.IsNotFound(err) {
		return nil, err
	} else if invite == nil {
		return nil, nil
	}

	return entInviteToBiz(invite), nil
}

func (r *OrgInvitation) PendingInvitations(ctx context.Context, receiverEmail string) ([]*biz.OrgInvitation, error) {
	invites, err := r.query().Where(
		orginvitation.ReceiverEmail(receiverEmail),
		orginvitation.StatusEQ(biz.OrgInvitationStatusPending),
	).All(ctx)
	if err != nil {
		return nil, fmt.Errorf("error finding invites for user %s: %w", receiverEmail, err)
	}

	res := make([]*biz.OrgInvitation, len(invites))
	for i, v := range invites {
		res[i] = entInviteToBiz(v)
	}

	return res, nil
}

func (r *OrgInvitation) ChangeStatus(ctx context.Context, id uuid.UUID, status biz.OrgInvitationStatus) error {
	return r.data.DB.OrgInvitation.UpdateOneID(id).SetStatus(status).Exec(ctx)
}

// Full query of non-deleted invites with all edges loaded
func (r *OrgInvitation) query() *ent.OrgInvitationQuery {
	return r.data.DB.OrgInvitation.Query().WithOrganization().WithSender().Where(orginvitation.DeletedAtIsNil())
}

func (r *OrgInvitation) FindByID(ctx context.Context, id uuid.UUID) (*biz.OrgInvitation, error) {
	invite, err := r.query().Where(orginvitation.ID(id)).Only(ctx)
	if err != nil && !ent.IsNotFound(err) {
		return nil, fmt.Errorf("error finding invite %s: %w", id.String(), err)
	} else if invite == nil {
		return nil, nil
	}

	return entInviteToBiz(invite), nil
}

func (r *OrgInvitation) SoftDelete(ctx context.Context, id uuid.UUID) error {
	return r.data.DB.OrgInvitation.UpdateOneID(id).SetDeletedAt(time.Now()).Exec(ctx)
}

func (r *OrgInvitation) ListByOrg(ctx context.Context, orgID uuid.UUID) ([]*biz.OrgInvitation, error) {
	invite, err := r.query().Where(orginvitation.OrganizationID(orgID)).All(ctx)
	if err != nil {
		return nil, fmt.Errorf("error finding invites for org %s: %w", orgID.String(), err)
	}

	res := make([]*biz.OrgInvitation, len(invite))
	for i, v := range invite {
		res[i] = entInviteToBiz(v)
	}

	return res, nil
}

func entInviteToBiz(i *ent.OrgInvitation) *biz.OrgInvitation {
	if i == nil {
		return nil
	}

	res := &biz.OrgInvitation{
		ID:            i.ID,
		ReceiverEmail: i.ReceiverEmail,
		CreatedAt:     toTimePtr(i.CreatedAt),
		Status:        i.Status,
		Role:          i.Role,
	}

	if i.Edges.Organization != nil {
		res.Org = entOrgToBizOrg(i.Edges.Organization)
	}

	if i.Edges.Sender != nil {
		res.Sender = entUserToBizUser(i.Edges.Sender)
	}

	return res
}
