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

package data

import (
	"context"
	"fmt"
	"time"

	"entgo.io/ent/dialect/sql/sqljson"

	"entgo.io/ent/dialect/sql"
	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/authz"
	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/biz"
	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/data/ent"
	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/data/ent/group"
	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/data/ent/groupmembership"
	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/data/ent/membership"
	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/data/ent/organization"
	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/data/ent/orginvitation"
	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/data/ent/user"
	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/data/ent/workflow"
	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/pagination"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/google/uuid"
)

type GroupRepo struct {
	data *Data
	log  *log.Helper
}

func NewGroupRepo(data *Data, logger log.Logger) biz.GroupRepo {
	return &GroupRepo{
		data: data,
		log:  log.NewHelper(logger),
	}
}

func (g GroupRepo) List(ctx context.Context, orgID uuid.UUID, filterOpts *biz.ListGroupOpts, paginationOpts *pagination.OffsetPaginationOpts) ([]*biz.Group, int, error) {
	if filterOpts == nil {
		filterOpts = &biz.ListGroupOpts{}
	}

	query := g.data.DB.Group.Query().
		Where(group.DeletedAtIsNil(), group.OrganizationIDEQ(orgID)).
		WithMembers().WithOrganization()

	if filterOpts.Name != "" {
		query.Where(group.NameContains(filterOpts.Name))
	}

	if filterOpts.Description != "" {
		query.Where(group.DescriptionContains(filterOpts.Description))
	}

	if filterOpts.MemberEmail != "" {
		query.Where(group.HasMembersWith(user.EmailContains(filterOpts.MemberEmail)))
	}

	// Get the count of all filtered rows without the limit and offset
	count, err := query.Count(ctx)
	if err != nil {
		return nil, 0, err
	}

	// Apply pagination options and execute the query
	groups, err := query.
		Order(ent.Desc(group.FieldCreatedAt)).
		Limit(paginationOpts.Limit()).
		Offset(paginationOpts.Offset()).
		All(ctx)
	if err != nil {
		return nil, 0, err
	}

	bizGroups := make([]*biz.Group, 0, len(groups))
	for _, entGroup := range groups {
		bizGroups = append(bizGroups, entGroupToBiz(entGroup))
	}

	return bizGroups, count, nil
}

// ListMembers retrieves the members of a group, optionally filtering by maintainers.
func (g GroupRepo) ListMembers(ctx context.Context, orgID uuid.UUID, groupID uuid.UUID, opts *biz.ListMembersOpts, paginationOpts *pagination.OffsetPaginationOpts) ([]*biz.GroupMembership, int, error) {
	if paginationOpts == nil {
		paginationOpts = pagination.NewDefaultOffsetPaginationOpts()
	}

	// Check the group exists in the organization
	_, err := g.data.DB.Group.Query().
		Where(group.ID(groupID), group.OrganizationIDEQ(orgID), group.DeletedAtIsNil()).
		Only(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, 0, biz.NewErrNotFound("group")
		}
		return nil, 0, err
	}

	// Build the query to list members of the group
	query := g.data.DB.GroupMembership.Query().
		Where(groupmembership.GroupID(groupID), groupmembership.DeletedAtIsNil())

	if opts != nil && opts.Maintainers != nil {
		query.Where(groupmembership.MaintainerEQ(*opts.Maintainers))
	}

	if opts != nil && opts.MemberEmail != nil {
		query.Where(groupmembership.HasUserWith(user.EmailContains(*opts.MemberEmail)))
	}

	// Get the count of all filtered rows without the limit and offset
	count, err := query.Count(ctx)
	if err != nil {
		return nil, 0, err
	}

	members, err := query.
		Order(ent.Desc(workflow.FieldCreatedAt)).
		WithUser().
		Limit(paginationOpts.Limit()).
		Offset(paginationOpts.Offset()).
		All(ctx)
	if err != nil {
		return nil, 0, err
	}

	bizMembers := make([]*biz.GroupMembership, 0, len(members))
	for _, member := range members {
		bizMembers = append(bizMembers, entGroupMembershipToBiz(member))
	}

	return bizMembers, count, nil
}

// ListPendingInvitationsByGroup retrieves pending invitations for a specific group in an organization.
func (g GroupRepo) ListPendingInvitationsByGroup(ctx context.Context, orgID uuid.UUID, groupID uuid.UUID, paginationOpts *pagination.OffsetPaginationOpts) ([]*biz.OrgInvitation, int, error) {
	if paginationOpts == nil {
		paginationOpts = pagination.NewDefaultOffsetPaginationOpts()
	}

	// Check the group exists in the organization
	_, err := g.data.DB.Group.Query().
		Where(group.ID(groupID), group.OrganizationIDEQ(orgID), group.DeletedAtIsNil()).
		Only(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, 0, biz.NewErrNotFound("group")
		}
		return nil, 0, err
	}

	// Build the query for pending invitations related to the group
	query := g.data.DB.OrgInvitation.Query().
		Where(
			orginvitation.OrganizationIDEQ(orgID),
			orginvitation.DeletedAtIsNil(),
			orginvitation.StatusEQ(biz.OrgInvitationStatusPending),
			func(s *sql.Selector) {
				s.Where(sqljson.ValueEQ(orginvitation.FieldContext, groupID.String(), sqljson.DotPath("group_id_to_join")))
			},
		).
		WithOrganization().
		WithSender()

	// Get the count of all filtered rows without the limit and offset
	count, err := query.Count(ctx)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to count pending invitations: %w", err)
	}

	// Apply pagination options and execute the query
	invitations, err := query.
		Order(ent.Desc(orginvitation.FieldCreatedAt)).
		Limit(paginationOpts.Limit()).
		Offset(paginationOpts.Offset()).
		All(ctx)
	if err != nil {
		return nil, 0, err
	}

	// Convert ent.OrgInvitation entities to biz.OrgInvitation
	result := make([]*biz.OrgInvitation, 0, len(invitations))
	for _, inv := range invitations {
		result = append(result, entInviteToBiz(inv))
	}

	return result, count, nil
}

// Create creates a new group in the specified organization.
func (g GroupRepo) Create(ctx context.Context, orgID uuid.UUID, opts *biz.CreateGroupOpts) (*biz.Group, error) {
	if opts == nil {
		return nil, biz.NewErrValidationStr("create group options cannot be nil")
	}

	var entGroup ent.Group

	err := WithTx(ctx, g.data.DB, func(tx *ent.Tx) error {
		// Create the group with the provided options
		gr, err := tx.Group.Create().
			SetName(opts.Name).
			SetDescription(opts.Description).
			AddMemberIDs(opts.UserID).
			SetOrganizationID(orgID).
			SetMemberCount(1).
			Save(ctx)
		if err != nil {
			if ent.IsConstraintError(err) {
				return biz.NewErrAlreadyExistsStr("group with the same name already exists")
			}
			return err
		}

		// Update the group-user member to set it's a group maintainer
		if _, grUerr := tx.GroupMembership.Update().
			Where(
				groupmembership.GroupIDEQ(gr.ID),
				groupmembership.UserIDEQ(opts.UserID),
			).
			SetMaintainer(true).
			Save(ctx); grUerr != nil {
			if ent.IsNotFound(grUerr) {
				return biz.NewErrNotFound("group user")
			}
			return grUerr
		}

		// Update the user membership with the role of maintainer
		_, err = tx.Membership.Create().
			SetUserID(opts.UserID).
			SetOrganizationID(orgID).
			SetRole(authz.RoleGroupMaintainer).
			SetMembershipType(authz.MembershipTypeUser).
			SetMemberID(opts.UserID).
			SetResourceType(authz.ResourceTypeGroup).
			SetResourceID(gr.ID).
			Save(ctx)
		if err != nil {
			return fmt.Errorf("failed to create membership for user %s in group %s: %w", opts.UserID, gr.ID, err)
		}

		entGroup = *gr

		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create group: %w", err)
	}

	return g.FindByOrgAndID(ctx, orgID, entGroup.ID)
}

// FindByOrgAndID retrieves a group by its ID and org, ensuring it is not deleted.
func (g GroupRepo) FindByOrgAndID(ctx context.Context, orgID uuid.UUID, groupID uuid.UUID) (*biz.Group, error) {
	entGroup, err := g.data.DB.Group.Query().
		Where(group.DeletedAtIsNil(), group.ID(groupID), group.OrganizationID(orgID)).
		WithOrganization().
		Only(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, biz.NewErrNotFound("group")
		}
		return nil, err
	}

	return entGroupToBiz(entGroup), nil
}

// FindGroupMembershipByGroupAndID retrieves a group membership for a specific user in a group.
func (g GroupRepo) FindGroupMembershipByGroupAndID(ctx context.Context, groupID uuid.UUID, userID uuid.UUID) (*biz.GroupMembership, error) {
	// Query the group user membership for the specified user in the group
	groupUser, err := g.data.DB.GroupMembership.Query().
		Where(
			groupmembership.GroupIDEQ(groupID),
			groupmembership.UserIDEQ(userID),
			groupmembership.DeletedAtIsNil(),
		).
		WithUser().
		Only(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, biz.NewErrNotFound("group membership")
		}
		return nil, err
	}

	return entGroupMembershipToBiz(groupUser), nil
}

// Update updates an existing group in the specified organization.
func (g GroupRepo) Update(ctx context.Context, orgID uuid.UUID, groupID uuid.UUID, opts *biz.UpdateGroupOpts) (*biz.Group, error) {
	if opts == nil {
		return nil, biz.NewErrValidationStr("update group options cannot be nil")
	}

	// Update the group with the provided options
	entGroup, err := g.data.DB.Group.UpdateOneID(groupID).
		SetNillableName(opts.NewName).
		SetNillableDescription(opts.NewDescription).
		Where(group.OrganizationIDEQ(orgID), group.DeletedAtIsNil()).
		Save(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, biz.NewErrNotFound("group")
		}
		if ent.IsConstraintError(err) {
			return nil, biz.NewErrAlreadyExistsStr("group with the same name already exists")
		}

		return nil, err
	}

	return g.FindByOrgAndID(ctx, orgID, entGroup.ID)
}

// SoftDelete soft-deletes a group by setting the DeletedAt field to the current time.
func (g GroupRepo) SoftDelete(ctx context.Context, orgID uuid.UUID, groupID uuid.UUID) error {
	// Softly delete the group by setting the DeletedAt field
	_, err := g.data.DB.Group.UpdateOneID(groupID).
		SetDeletedAt(time.Now()).
		Where(group.OrganizationIDEQ(orgID), group.DeletedAtIsNil()).
		Save(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return biz.NewErrNotFound("group")
		}
		return err
	}

	return nil
}

// AddMemberToGroup adds a user to a group, creating a new membership if they are not already a member.
func (g GroupRepo) AddMemberToGroup(ctx context.Context, orgID uuid.UUID, groupID uuid.UUID, userID uuid.UUID, maintainer bool) (*biz.GroupMembership, error) {
	if err := WithTx(ctx, g.data.DB, func(tx *ent.Tx) error {
		// Check if the user is already a member of this group
		existingMember, err := tx.GroupMembership.Query().
			Where(groupmembership.UserIDEQ(userID), groupmembership.GroupIDEQ(groupID), groupmembership.DeletedAtIsNil()).
			Exist(ctx)
		if err != nil && !ent.IsNotFound(err) {
			return fmt.Errorf("failed to check existing group membership: %w", err)
		}

		// If the user is already a member, return an error
		if existingMember {
			return biz.NewErrAlreadyExistsStr("user is already a member of this group")
		}

		// Create a new group-user relationship
		if _, err := tx.GroupMembership.Create().
			SetGroupID(groupID).
			SetUserID(userID).
			SetMaintainer(maintainer).
			Save(ctx); err != nil {
			return fmt.Errorf("failed to add user to group: %w", err)
		}

		// Update the user membership with the role of maintainer
		if maintainer {
			_, err = tx.Membership.Create().
				SetUserID(userID).
				SetOrganizationID(orgID).
				SetRole(authz.RoleGroupMaintainer).
				SetMembershipType(authz.MembershipTypeUser).
				SetMemberID(userID).
				SetResourceType(authz.ResourceTypeGroup).
				SetResourceID(groupID).
				Save(ctx)
			if err != nil {
				return fmt.Errorf("failed to create membership for user %s in group %s: %w", userID, groupID, err)
			}
		}

		// Increment the member count of the group
		if err := tx.Group.UpdateOneID(groupID).AddMemberCount(1).Exec(ctx); err != nil {
			return fmt.Errorf("failed to increment group member count: %w", err)
		}

		return nil
	}); err != nil {
		return nil, fmt.Errorf("failed to add member to group: %w", err)
	}

	// Return the newly created membership
	return g.FindGroupMembershipByGroupAndID(ctx, groupID, userID)
}

// RemoveMemberFromGroup removes a user from a group.
func (g GroupRepo) RemoveMemberFromGroup(ctx context.Context, orgID uuid.UUID, groupID uuid.UUID, userID uuid.UUID) error {
	err := WithTx(ctx, g.data.DB, func(tx *ent.Tx) error {
		// Check if the user is a member of this group
		existingMembership, err := tx.GroupMembership.Query().
			Where(groupmembership.UserIDEQ(userID), groupmembership.GroupIDEQ(groupID), groupmembership.DeletedAtIsNil()).
			Only(ctx)
		if err != nil {
			if ent.IsNotFound(err) {
				return biz.NewErrNotFound("group membership")
			}
			return fmt.Errorf("failed to check existing group membership: %w", err)
		}

		now := time.Now()

		// Mark the membership as deleted
		_, err = tx.GroupMembership.UpdateOne(existingMembership).
			SetDeletedAt(now).
			SetUpdatedAt(now).
			Save(ctx)
		if err != nil {
			return fmt.Errorf("failed to remove user from group: %w", err)
		}

		if existingMembership.Maintainer {
			// Also remove the user membership if it exists
			if _, err := tx.Membership.Delete().Where(
				membership.MemberID(userID),
				membership.ResourceID(groupID),
				membership.ResourceTypeEQ(authz.ResourceTypeGroup),
				membership.HasOrganizationWith(
					organization.ID(orgID),
				),
			).Exec(ctx); err != nil {
				return fmt.Errorf("failed to remove user from group: %w", err)
			}
		}

		// Decrement the member count of the group
		if err := tx.Group.UpdateOneID(groupID).AddMemberCount(-1).Exec(ctx); err != nil {
			return fmt.Errorf("failed to increment group member count: %w", err)
		}

		return nil
	})

	if err != nil {
		return err
	}

	return nil
}

// entGroupToBiz converts an ent.Group to a biz.Group.
func entGroupToBiz(gr *ent.Group) *biz.Group {
	grp := &biz.Group{
		ID:          gr.ID,
		Name:        gr.Name,
		Description: gr.Description,
		MemberCount: gr.MemberCount,
		CreatedAt:   toTimePtr(gr.CreatedAt),
		UpdatedAt:   toTimePtr(gr.UpdatedAt),
		DeletedAt:   toTimePtr(gr.DeletedAt),
	}

	// Include the organization to which the group belongs
	if gr.Edges.Organization != nil {
		grp.Organization = entOrgToBizOrg(gr.Edges.Organization)
	}

	return grp
}

// entGroupMembershipToBiz converts an ent.GroupMembership to a biz.GroupMembership.
func entGroupMembershipToBiz(gu *ent.GroupMembership) *biz.GroupMembership {
	return &biz.GroupMembership{
		User:       entUserToBizUser(gu.Edges.User),
		Maintainer: gu.Maintainer,
		CreatedAt:  toTimePtr(gu.CreatedAt),
		UpdatedAt:  toTimePtr(gu.UpdatedAt),
		DeletedAt:  toTimePtr(gu.DeletedAt),
	}
}
