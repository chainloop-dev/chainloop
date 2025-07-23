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
	"time"

	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/authz"
	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/biz"
	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/data/ent"
	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/data/ent/group"
	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/data/ent/groupmembership"
	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/data/ent/membership"
	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/data/ent/organization"
	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/data/ent/predicate"
	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/data/ent/user"
	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/pagination"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/google/uuid"
)

type MembershipRepo struct {
	data      *Data
	log       *log.Helper
	groupRepo biz.GroupRepo
}

func NewMembershipRepo(data *Data, groupRepo biz.GroupRepo, logger log.Logger) biz.MembershipRepo {
	return &MembershipRepo{
		data:      data,
		groupRepo: groupRepo,
		log:       log.NewHelper(logger),
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
func (r *MembershipRepo) FindByOrg(ctx context.Context, orgID uuid.UUID, opts *biz.ListByOrgOpts, paginationOpts *pagination.OffsetPaginationOpts) ([]*biz.Membership, int, error) {
	if paginationOpts == nil {
		paginationOpts = pagination.NewDefaultOffsetPaginationOpts()
	}

	if opts == nil {
		opts = &biz.ListByOrgOpts{}
	}

	// Build the base query
	query := r.data.DB.Membership.Query().Where(
		membership.MembershipTypeEQ(authz.MembershipTypeUser),
		membership.ResourceTypeEQ(authz.ResourceTypeOrganization),
		membership.ResourceIDEQ(orgID),
	).WithUser().WithOrganization()

	// Apply filters if provided
	var predicates []predicate.Membership
	if opts.Name != nil && *opts.Name != "" {
		// Filter by user's first name or last name containing the search term
		predicates = append(predicates, membership.HasUserWith(
			user.Or(
				user.FirstNameContainsFold(*opts.Name),
				user.LastNameContainsFold(*opts.Name),
			),
		))
	}

	// Filter by user's email containing the search term
	if opts.Email != nil && *opts.Email != "" {
		predicates = append(predicates, membership.HasUserWith(user.EmailContainsFold(*opts.Email)))
	}

	// Apply OR predicates if any exist
	if len(predicates) > 0 {
		query = query.Where(membership.Or(predicates...))
	}

	// Filter by the membership ID if provided
	if opts.MembershipID != nil {
		query = query.Where(membership.IDEQ(*opts.MembershipID))
	}

	// Filter by role if provided
	if opts.Role != nil {
		query = query.Where(membership.RoleEQ(*opts.Role))
	}

	// Get the count of all filtered rows without the limit and offset
	count, err := query.Count(ctx)
	if err != nil {
		return nil, 0, err
	}

	// Apply pagination and execute the query
	memberships, err := query.
		Order(ent.Desc(membership.FieldCreatedAt)).
		Limit(paginationOpts.Limit()).
		Offset(paginationOpts.Offset()).
		All(ctx)
	if err != nil {
		return nil, 0, err
	}

	// Fetch all member IDs from the memberships, in this context they are user IDs
	memberIDs := make([]uuid.UUID, 0, len(memberships))
	for _, m := range memberships {
		memberIDs = append(memberIDs, m.MemberID)
	}

	// Fetch user data for all the member IDs
	users, err := r.data.DB.User.Query().Where(user.IDIn(memberIDs...)).All(ctx)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to fetch user data: %w", err)
	}

	// Create a map of users by ID
	userMap := make(map[uuid.UUID]*ent.User, len(users))
	for _, u := range users {
		userMap[u.ID] = u
	}

	// Convert to biz.Membership objects and attach user data manually
	result := make([]*biz.Membership, 0, len(memberships))
	for _, m := range memberships {
		bizMembership := entMembershipToBiz(m)
		if u, ok := userMap[m.MemberID]; ok {
			bizMembership.User = entUserToBizUser(u)
		}
		result = append(result, bizMembership)
	}

	return result, count, nil
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
// When deleting a membership, it's important to ensure we're not leaving any dangling references.
func (r *MembershipRepo) Delete(ctx context.Context, id uuid.UUID) error {
	// First, fetch the membership to understand what we're deleting
	membershipToDelete, err := r.data.DB.Membership.Query().Where(membership.ID(id)).WithOrganization().Only(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil // Already deleted, nothing to do
		}
		return fmt.Errorf("failed to get membership: %w", err)
	}

	// Prepare a slice to hold group IDs that need to be updated
	var groupIDs []uuid.UUID

	if trErr := WithTx(ctx, r.data.DB, func(tx *ent.Tx) error {
		// Delete the specific membership
		if err := tx.Membership.DeleteOneID(id).Exec(ctx); err != nil {
			return fmt.Errorf("failed to delete membership: %w", err)
		}

		// If this is an organization membership and the member type is a user,
		// we also need to clean up any resource memberships for this user in the organization
		if membershipToDelete.ResourceType == authz.ResourceTypeOrganization && membershipToDelete.MembershipType == authz.MembershipTypeUser {
			// Extract the organization id and user ID from the membership
			orgID := membershipToDelete.Edges.Organization.ID
			userID := membershipToDelete.MemberID

			// Delete all other resource memberships for this user in the organization
			// This will cover all membership types including group-related ones
			if _, err := tx.Membership.Delete().Where(
				membership.IDNEQ(id), // Don't try to delete the one we already deleted
				membership.MemberID(userID),
				membership.HasOrganizationWith(organization.ID(orgID)),
			).Exec(ctx); err != nil {
				return fmt.Errorf("failed to delete related memberships: %w", err)
			}

			// Remove the user from all groups in the organization by soft-deleting group memberships
			now := time.Now()

			// Find all group IDs where this user is a member in this organization
			groupMemberships, grpMemErr := tx.GroupMembership.Query().Where(
				groupmembership.UserID(userID),
				groupmembership.DeletedAtIsNil(),
				groupmembership.HasGroupWith(group.OrganizationID(orgID)),
			).Select(groupmembership.FieldGroupID).All(ctx)
			if grpMemErr != nil {
				return fmt.Errorf("failed to fetch group IDs for user %s in organization %s: %w", userID, orgID, err)
			}

			// Collect group IDs to update member counts later
			for _, gm := range groupMemberships {
				groupIDs = append(groupIDs, gm.GroupID)
			}

			// Soft delete all group memberships for this user in this organization
			if _, err := tx.GroupMembership.Update().Where(
				groupmembership.UserID(userID),
				groupmembership.DeletedAtIsNil(),
				groupmembership.HasGroupWith(group.OrganizationID(orgID)),
			).SetDeletedAt(now).SetUpdatedAt(now).Save(ctx); err != nil {
				return fmt.Errorf("failed to delete group memberships for user %s in organization %s: %w", userID, orgID, err)
			}
		}

		return nil
	}); trErr != nil {
		return trErr
	}

	// For each affected group, update the member count based on actual query
	updated := map[uuid.UUID]struct{}{}
	for _, gid := range groupIDs {
		if _, seen := updated[gid]; seen {
			// deduplicate group IDs
			continue
		}
		updated[gid] = struct{}{}
		if err := r.groupRepo.UpdateGroupMemberCount(ctx, gid); err != nil {
			return fmt.Errorf("failed to update group member count for group %s: %w", gid, err)
		}
	}

	return nil
}

// RBAC methods

func (r *MembershipRepo) ListAllByUser(ctx context.Context, userID uuid.UUID) ([]*biz.Membership, error) {
	mm, err := r.data.DB.Membership.Query().Where(
		membership.MembershipTypeEQ(authz.MembershipTypeUser),
		membership.MemberID(userID),
	).WithOrganization().All(ctx)

	if err != nil {
		return nil, fmt.Errorf("failed to query memberships: %w", err)
	}

	return entMembershipsToBiz(mm), nil
}

// ListGroupMembershipsByUser returns all memberships of the users inherited from groups
func (r *MembershipRepo) ListGroupMembershipsByUser(ctx context.Context, userID uuid.UUID) ([]*biz.Membership, error) {
	// First query all group memberships for the user directly
	groupMemberships, err := r.data.DB.GroupMembership.Query().Where(
		groupmembership.UserID(userID),
		groupmembership.DeletedAtIsNil(),
	).All(ctx)

	if err != nil {
		return nil, fmt.Errorf("failed to query user's group memberships: %w", err)
	}

	// Extract group IDs
	groupIDs := make([]uuid.UUID, 0, len(groupMemberships))
	for _, gm := range groupMemberships {
		groupIDs = append(groupIDs, gm.GroupID)
	}

	var res []*ent.Membership

	// If user belongs to groups, query those group memberships
	if len(groupIDs) > 0 {
		groupRoleMemberships, err := r.data.DB.Membership.Query().Where(
			membership.MembershipTypeEQ(authz.MembershipTypeGroup),
			membership.MemberIDIn(groupIDs...),
		).WithOrganization().All(ctx)

		if err != nil {
			return nil, fmt.Errorf("failed to query group role memberships: %w", err)
		}

		// Append group role memberships to the result
		res = append(res, groupRoleMemberships...)
	}

	return entMembershipsToBiz(res), nil
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

func (r *MembershipRepo) AddResourceRole(ctx context.Context, orgID uuid.UUID, resourceType authz.ResourceType, resID uuid.UUID, mType authz.MembershipType, memberID uuid.UUID, role authz.Role, parentID *uuid.UUID) error {
	err := r.data.DB.Membership.Create().
		SetOrganizationID(orgID).
		SetMembershipType(mType).
		SetMemberID(memberID).
		SetResourceType(resourceType).
		SetResourceID(resID).
		SetNillableParentID(parentID).
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

// FindByUserAndResourceID finds a membership by user ID and resource ID.
func (r *MembershipRepo) FindByUserAndResourceID(ctx context.Context, userID, resourceID uuid.UUID) (*biz.Membership, error) {
	m, err := r.data.DB.Membership.Query().Where(
		membership.MemberID(userID),
		membership.ResourceID(resourceID),
	).WithUser().WithOrganization().Only(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, biz.NewErrNotFound(fmt.Sprintf("membership for user %s and resource %s not found", userID, resourceID))
		}
		return nil, fmt.Errorf("failed to query memberships: %w", err)
	}

	return entMembershipToBiz(m), nil
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

	// Deprecated branch, remove when no longer needed
	if m.Edges.Organization != nil {
		res.OrganizationID = m.Edges.Organization.ID
		res.Org = entOrgToBizOrg(m.Edges.Organization)
	}

	// New branch to populate organization ID
	if m.ResourceType == authz.ResourceTypeOrganization {
		res.OrganizationID = m.ResourceID
	}

	if m.Edges.User != nil {
		res.User = entUserToBizUser(m.Edges.User)
	}

	res.MembershipType = m.MembershipType
	res.MemberID = m.MemberID
	res.ResourceType = m.ResourceType
	res.ResourceID = m.ResourceID
	res.ParentID = m.ParentID

	return res
}
