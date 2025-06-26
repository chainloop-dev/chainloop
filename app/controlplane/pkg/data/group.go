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

	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/biz"
	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/data/ent"
	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/data/ent/group"
	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/data/ent/groupmembership"
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
			Save(ctx)
		if err != nil {
			if ent.IsConstraintError(err) {
				return biz.NewErrAlreadyExists(err)
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

// Update updates an existing group in the specified organization.
func (g GroupRepo) Update(ctx context.Context, orgID uuid.UUID, groupID uuid.UUID, opts *biz.UpdateGroupOpts) (*biz.Group, error) {
	if opts == nil {
		return nil, biz.NewErrValidationStr("update group options cannot be nil")
	}

	// Update the group with the provided options
	entGroup, err := g.data.DB.Group.UpdateOneID(groupID).
		SetNillableName(opts.Name).
		SetNillableDescription(opts.Description).
		Where(group.OrganizationIDEQ(orgID), group.DeletedAtIsNil()).
		Save(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, biz.NewErrNotFound("group")
		}
		if ent.IsConstraintError(err) {
			return nil, biz.NewErrAlreadyExists(err)
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

// entGroupToBiz converts an ent.Group to a biz.Group.
func entGroupToBiz(gr *ent.Group) *biz.Group {
	grp := &biz.Group{
		ID:          gr.ID,
		Name:        gr.Name,
		Description: gr.Description,
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
