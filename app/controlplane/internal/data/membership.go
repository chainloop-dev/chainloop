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

	"github.com/chainloop-dev/bedrock/app/controlplane/internal/biz"
	"github.com/chainloop-dev/bedrock/app/controlplane/internal/data/ent"
	"github.com/chainloop-dev/bedrock/app/controlplane/internal/data/ent/membership"
	"github.com/chainloop-dev/bedrock/app/controlplane/internal/data/ent/user"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/google/uuid"
)

type MembershipRepo struct {
	data *Data
	log  *log.Helper
}

func NewMembershipRepo(data *Data, logger log.Logger) biz.MembershipRepo {
	return &MembershipRepo{
		data: data,
		log:  log.NewHelper(logger),
	}
}

func (r *MembershipRepo) Create(ctx context.Context, orgID, userID uuid.UUID, current bool) (*biz.Membership, error) {
	m, err := r.data.db.Membership.Create().
		SetUserID(userID).
		SetOrganizationID(orgID).
		SetCurrent(current).
		Save(ctx)
	if err != nil {
		return nil, err
	}

	// reload it so it includes all the information
	m, err = r.loadMembership(ctx, m.ID)
	if err != nil {
		return nil, err
	}

	return entMembershipToBiz(m), nil
}

func (r *MembershipRepo) loadMembership(ctx context.Context, id uuid.UUID) (*ent.Membership, error) {
	return r.data.db.Membership.Query().WithOrganization().WithUser().Where(membership.ID(id)).First(ctx)
}

func (r *MembershipRepo) FindByUser(ctx context.Context, userID uuid.UUID) ([]*biz.Membership, error) {
	memberships, err := r.data.db.User.Query().Where(user.ID(userID)).QueryMemberships().
		WithOrganization().All(ctx)
	if err != nil {
		return nil, err
	}

	result := make([]*biz.Membership, 0, len(memberships))
	for _, m := range memberships {
		result = append(result, entMembershipToBiz(m))
	}

	return result, nil
}

// FindByOrg finds all memberships for a given organization
func (r *MembershipRepo) FindByOrg(ctx context.Context, orgID uuid.UUID) ([]*biz.Membership, error) {
	memberships, err := orgScopedQuery(r.data.db, orgID).
		QueryMemberships().
		WithOrganization().All(ctx)
	if err != nil {
		return nil, err
	}

	result := make([]*biz.Membership, 0, len(memberships))
	for _, m := range memberships {
		result = append(result, entMembershipToBiz(m))
	}

	return result, nil
}

func (r *MembershipRepo) FindByIDInUser(ctx context.Context, userID, membershipID uuid.UUID) (*biz.Membership, error) {
	m, err := r.data.db.User.Query().Where(user.ID(userID)).
		QueryMemberships().
		Where(membership.ID(membershipID)).
		WithUser().
		WithOrganization().Only(ctx)
	if err != nil && !ent.IsNotFound(err) {
		return nil, err
	}

	return entMembershipToBiz(m), nil
}

func (r *MembershipRepo) SetCurrent(ctx context.Context, membershipID uuid.UUID) (*biz.Membership, error) {
	// Load membership to find user
	m, err := r.loadMembership(ctx, membershipID)
	if err != nil {
		return nil, err
	}

	// For the found user, we must, in a transaction.
	tx, err := r.data.db.Tx(ctx)
	if err != nil {
		return nil, err
	}

	// 1 - Set all the memberships to current=false
	if err = tx.Membership.Update().Where(membership.HasUserWith(user.ID(m.Edges.User.ID))).
		SetCurrent(false).Exec(ctx); err != nil {
		return nil, err
	}

	// 2 - Set the referenced membership to current=true
	if err = tx.Membership.UpdateOneID(membershipID).SetCurrent(true).Exec(ctx); err != nil {
		return nil, err
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}

	// Reload returned data
	m, err = r.loadMembership(ctx, membershipID)
	if err != nil {
		return nil, err
	}

	return entMembershipToBiz(m), nil
}

// Delete deletes a membership by ID.
func (r *MembershipRepo) Delete(ctx context.Context, id uuid.UUID) error {
	return r.data.db.Membership.DeleteOneID(id).Exec(ctx)
}

func entMembershipToBiz(m *ent.Membership) *biz.Membership {
	if m == nil {
		return nil
	}

	res := &biz.Membership{ID: m.ID, Current: m.Current, CreatedAt: toTimePtr(m.CreatedAt), UpdatedAt: toTimePtr(m.UpdatedAt)}

	if m.Edges.Organization != nil {
		res.OrganizationID = m.Edges.Organization.ID
		res.Org = entOrgToBizOrg(m.Edges.Organization)
	}

	if m.Edges.User != nil {
		res.UserID = m.Edges.User.ID
	}

	return res
}
