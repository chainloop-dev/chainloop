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

package data

import (
	"context"
	"fmt"

	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/authz"
	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/biz"
	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/data/ent"
	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/data/ent/membership"
	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/data/ent/organization"
	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/data/ent/user"
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

func (r *MembershipRepo) Create(ctx context.Context, orgID, userID uuid.UUID, current bool, role authz.Role) (*biz.Membership, error) {
	m, err := r.data.DB.Membership.Create().
		SetUserID(userID).
		SetOrganizationID(orgID).
		SetCurrent(current).
		SetRole(role).
		SetMembershipType(authz.MembershipTypeUser).
		SetMemberID(userID).
		SetResourceType(authz.ResourceTypeOrganization).
		SetResourceID(orgID).
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
	return r.data.DB.Membership.Query().WithOrganization().WithUser().Where(membership.ID(id)).First(ctx)
}

func (r *MembershipRepo) FindByUser(ctx context.Context, userID uuid.UUID) ([]*biz.Membership, error) {
	memberships, err := r.data.DB.Membership.Query().Where(
		membership.ResourceTypeEQ(authz.ResourceTypeOrganization),
		membership.MembershipTypeEQ(authz.MembershipTypeUser),
		membership.MemberID(userID),
	).WithOrganization().All(ctx)
	if err != nil {
		return nil, err
	}

	return entMembershipsToBiz(memberships), nil
}

// FindByOrg finds all memberships for a given organization
func (r *MembershipRepo) FindByOrg(ctx context.Context, orgID uuid.UUID) ([]*biz.Membership, error) {
	memberships, err := r.data.DB.Membership.Query().Where(
		membership.MembershipTypeEQ(authz.MembershipTypeUser),
		membership.ResourceTypeEQ(authz.ResourceTypeOrganization),
		membership.ResourceIDEQ(orgID),
	).WithUser().WithOrganization().All(ctx)
	if err != nil {
		return nil, err
	}

	return entMembershipsToBiz(memberships), nil
}

// FindByOrgAndUser finds the membership for a given organization and user
func (r *MembershipRepo) FindByOrgAndUser(ctx context.Context, orgID, userID uuid.UUID) (*biz.Membership, error) {
	m, err := r.data.DB.Membership.Query().Where(
		membership.MembershipTypeEQ(authz.MembershipTypeUser),
		membership.MemberID(userID),
		membership.ResourceTypeEQ(authz.ResourceTypeOrganization),
		membership.ResourceIDEQ(orgID),
	).WithOrganization().WithUser().Only(ctx)
	if err != nil && !ent.IsNotFound(err) {
		return nil, err
	}

	return entMembershipToBiz(m), nil
}

// FindByOrgIDAndUserEmail finds the membership for a given organization and user email.
func (r *MembershipRepo) FindByOrgIDAndUserEmail(ctx context.Context, orgID uuid.UUID, userEmail string) (*biz.Membership, error) {
	// Find the user by email
	u, err := r.data.DB.User.Query().Where(user.Email(userEmail)).Only(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, biz.NewErrNotFound(fmt.Sprintf("user with email %s not found", userEmail))
		}
		return nil, fmt.Errorf("failed to find user by email %s: %w", userEmail, err)
	}

	// Now find the membership for that user in the organization
	mem, err := r.data.DB.Membership.Query().Where(
		membership.MembershipTypeEQ(authz.MembershipTypeUser),
		membership.MemberID(u.ID),
		membership.ResourceTypeEQ(authz.ResourceTypeOrganization),
		membership.ResourceIDEQ(orgID),
	).WithOrganization().WithUser().Only(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, biz.NewErrNotFound(fmt.Sprintf("membership for user %s in organization %s not found", userEmail, orgID))
		}
		return nil, fmt.Errorf("failed to query memberships: %w", err)
	}

	return entMembershipToBiz(mem), nil
}

func (r *MembershipRepo) FindByOrgNameAndUser(ctx context.Context, orgName string, userID uuid.UUID) (*biz.Membership, error) {
	org, err := r.data.DB.Organization.Query().Where(organization.Name(orgName)).First(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, biz.NewErrNotFound(fmt.Sprintf("organization %s not found", orgName))
		}
		return nil, fmt.Errorf("organization %s not found", orgName)
	}

	m, err := r.data.DB.Membership.Query().Where(
		membership.MembershipTypeEQ(authz.MembershipTypeUser),
		membership.MemberID(userID),
		membership.ResourceTypeEQ(authz.ResourceTypeOrganization),
		membership.ResourceID(org.ID),
	).WithOrganization().WithUser().Only(ctx)
	if err != nil && !ent.IsNotFound(err) {
		return nil, err
	}

	return entMembershipToBiz(m), nil
}

func (r *MembershipRepo) FindByIDInUser(ctx context.Context, userID, membershipID uuid.UUID) (*biz.Membership, error) {
	m, err := r.data.DB.Membership.Query().Where(
		membership.MembershipTypeEQ(authz.MembershipTypeUser),
		membership.MemberID(userID),
		membership.ResourceTypeEQ(authz.ResourceTypeOrganization),
		membership.ID(membershipID),
	).WithUser().WithOrganization().Only(ctx)
	if err != nil && !ent.IsNotFound(err) {
		return nil, err
	}

	return entMembershipToBiz(m), nil
}

func (r *MembershipRepo) FindByIDInOrg(ctx context.Context, orgID, membershipID uuid.UUID) (*biz.Membership, error) {
	m, err := r.data.DB.Membership.Query().Where(
		membership.MembershipTypeEQ(authz.MembershipTypeUser),
		membership.ResourceTypeEQ(authz.ResourceTypeOrganization),
		membership.ResourceIDEQ(orgID),
		membership.ID(membershipID),
	).WithUser().WithOrganization().Only(ctx)
	if err != nil && !ent.IsNotFound(err) {
		return nil, err
	}

	return entMembershipToBiz(m), nil
}

func (r *MembershipRepo) SetCurrent(ctx context.Context, membershipID uuid.UUID) (mr *biz.Membership, err error) {
	// Load membership to find user
	m, err := r.loadMembership(ctx, membershipID)
	if err != nil {
		return nil, err
	}

	if err = WithTx(ctx, r.data.DB, func(tx *ent.Tx) error {
		// 1 - Set all the memberships to current=false
		if err = tx.Membership.Update().Where(
			membership.ResourceTypeEQ(authz.ResourceTypeOrganization),
			membership.MembershipTypeEQ(authz.MembershipTypeUser),
			membership.MemberID(m.MemberID)).
			SetCurrent(false).Exec(ctx); err != nil {
			return err
		}

		// 2 - Set the referenced membership to current=true
		if err = tx.Membership.UpdateOneID(membershipID).SetCurrent(true).Exec(ctx); err != nil {
			return err
		}
		return nil
	}); err != nil {
		return nil, err
	}

	// Reload returned data
	m, err = r.loadMembership(ctx, membershipID)
	if err != nil {
		return nil, err
	}

	return entMembershipToBiz(m), nil
}

func (r *MembershipRepo) SetRole(ctx context.Context, membershipID uuid.UUID, role authz.Role) (*biz.Membership, error) {
	if err := r.data.DB.Membership.UpdateOneID(membershipID).SetRole(role).Exec(ctx); err != nil {
		return nil, fmt.Errorf("failed to update membership: %w", err)
	}

	m, err := r.loadMembership(ctx, membershipID)
	if err != nil {
		return nil, fmt.Errorf("failed to load membership: %w", err)
	}

	return entMembershipToBiz(m), nil
}

// Delete deletes a membership by ID.
func (r *MembershipRepo) Delete(ctx context.Context, id uuid.UUID) error {
	return r.data.DB.Membership.DeleteOneID(id).Exec(ctx)
}

// RBAC methods

func (r *MembershipRepo) ListAllByUser(ctx context.Context, userID uuid.UUID) ([]*biz.Membership, error) {
	mm, err := r.data.DB.Membership.Query().Where(
		membership.MembershipTypeEQ(authz.MembershipTypeUser),
		membership.MemberID(userID),
	).All(ctx)

	if err != nil {
		return nil, fmt.Errorf("failed to query memberships: %w", err)
	}

	return entMembershipsToBiz(mm), nil
}

func (r *MembershipRepo) ListAllByResource(ctx context.Context, rt authz.ResourceType, id uuid.UUID) ([]*biz.Membership, error) {
	mm, err := r.data.DB.Membership.Query().Where(
		membership.ResourceTypeEQ(rt),
		membership.ResourceID(id),
	).All(ctx)

	if err != nil {
		return nil, fmt.Errorf("failed to query memberships: %w", err)
	}

	return entMembershipsToBiz(mm), nil
}

func (r *MembershipRepo) AddResourceRole(ctx context.Context, resourceType authz.ResourceType, resID uuid.UUID, mType authz.MembershipType, memberID uuid.UUID, role authz.Role) error {
	err := r.data.DB.Membership.Create().
		SetMembershipType(mType).
		SetMemberID(memberID).
		SetResourceType(resourceType).
		SetResourceID(resID).
		SetRole(role).Exec(ctx)

	if err != nil {
		if !ent.IsConstraintError(err) {
			return fmt.Errorf("failed to add resource role: %w", err)
		}

		// combination already existed, ignore error
		return nil
	}

	return nil
}

func entMembershipsToBiz(memberships []*ent.Membership) []*biz.Membership {
	result := make([]*biz.Membership, 0, len(memberships))
	for _, m := range memberships {
		result = append(result, entMembershipToBiz(m))
	}

	return result
}

func entMembershipToBiz(m *ent.Membership) *biz.Membership {
	if m == nil {
		return nil
	}

	res := &biz.Membership{ID: m.ID,
		Current: m.Current, CreatedAt: toTimePtr(m.CreatedAt), UpdatedAt: toTimePtr(m.UpdatedAt),
		Role: m.Role,
	}

	if m.Edges.Organization != nil {
		res.OrganizationID = m.Edges.Organization.ID
		res.Org = entOrgToBizOrg(m.Edges.Organization)
	}

	if m.Edges.User != nil {
		res.User = entUserToBizUser(m.Edges.User)
	}

	res.MembershipType = m.MembershipType
	res.MemberID = m.MemberID
	res.ResourceType = m.ResourceType
	res.ResourceID = m.ResourceID

	return res
}
