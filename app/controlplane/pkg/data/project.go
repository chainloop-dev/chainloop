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

	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/data/ent/group"

	"entgo.io/ent/dialect/sql"
	"entgo.io/ent/dialect/sql/sqljson"
	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/authz"
	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/biz"
	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/data/ent"
	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/data/ent/membership"
	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/data/ent/organization"
	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/data/ent/orginvitation"
	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/data/ent/project"
	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/pagination"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/google/uuid"
)

type ProjectRepo struct {
	data *Data
	log  *log.Helper
}

func NewProjectsRepo(data *Data, logger log.Logger) biz.ProjectsRepo {
	return &ProjectRepo{
		data: data,
		log:  log.NewHelper(logger),
	}
}

// FindProjectByOrgIDAndName gets a project by organization ID and project name
func (r *ProjectRepo) FindProjectByOrgIDAndName(ctx context.Context, orgID uuid.UUID, projectName string) (*biz.Project, error) {
	pro, err := r.data.DB.Organization.Query().Where(
		organization.ID(orgID),
	).QueryProjects().Where(
		project.Name(projectName),
		project.DeletedAtIsNil(),
	).Only(ctx)

	if err != nil {
		if ent.IsNotFound(err) {
			return nil, biz.NewErrNotFound(fmt.Sprintf("project %s", projectName))
		}
		return nil, fmt.Errorf("project query failed: %w", err)
	}

	return entProjectToBiz(pro), nil
}

// FindProjectByOrgIDAndID gets a project by organization ID and project ID
func (r *ProjectRepo) FindProjectByOrgIDAndID(ctx context.Context, orgID uuid.UUID, projectID uuid.UUID) (*biz.Project, error) {
	pro, err := r.data.DB.Organization.Query().Where(
		organization.ID(orgID),
	).QueryProjects().Where(
		project.ID(projectID),
		project.DeletedAtIsNil(),
	).Only(ctx)

	if err != nil {
		if ent.IsNotFound(err) {
			return nil, biz.NewErrNotFound(fmt.Sprintf("project %s", projectID.String()))
		}
		return nil, fmt.Errorf("project query failed: %w", err)
	}

	return entProjectToBiz(pro), nil
}

func (r *ProjectRepo) ListProjectsByOrgID(ctx context.Context, orgID uuid.UUID) ([]*biz.Project, error) {
	prs, err := r.data.DB.Project.Query().Where(
		project.OrganizationID(orgID),
		project.DeletedAtIsNil()).All(ctx)
	if err != nil {
		return nil, fmt.Errorf("list projects failed: %w", err)
	}

	res := make([]*biz.Project, 0, len(prs))
	for _, p := range prs {
		res = append(res, entProjectToBiz(p))
	}

	return res, nil
}

func (r *ProjectRepo) Create(ctx context.Context, orgID uuid.UUID, name string) (*biz.Project, error) {
	pro, err := r.data.DB.Project.Create().SetOrganizationID(orgID).SetName(name).Save(ctx)
	if err != nil && !ent.IsNotFound(err) {
		return nil, err
	}

	return entProjectToBiz(pro), nil
}

// ListMembers lists all members of a project (both users and groups)
func (r *ProjectRepo) ListMembers(ctx context.Context, orgID uuid.UUID, projectID uuid.UUID, paginationOpts *pagination.OffsetPaginationOpts) ([]*biz.ProjectMembership, int, error) {
	// Check if the project exists and belongs to the organization
	existingProject, err := r.FindProjectByOrgIDAndID(ctx, orgID, projectID)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to find project: %w", err)
	}
	if existingProject == nil {
		return nil, 0, biz.NewErrNotFound("project")
	}

	// Build the query with base conditions for all membership types
	query := r.data.DB.Membership.Query().
		Where(
			membership.ResourceTypeEQ(authz.ResourceTypeProject),
			membership.ResourceID(projectID),
		)

	// Get total count before applying pagination
	totalCount, err := query.Count(ctx)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to count project members: %w", err)
	}

	// Apply pagination
	if paginationOpts != nil {
		query = query.
			Order(ent.Desc(membership.FieldCreatedAt)).
			Limit(paginationOpts.Limit()).
			Offset(paginationOpts.Offset())
	}

	// Execute the query
	memberships, err := query.All(ctx)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to list project members: %w", err)
	}

	// Convert to biz.ProjectMembership objects
	result := make([]*biz.ProjectMembership, 0, len(memberships))
	for _, m := range memberships {
		var mems *biz.ProjectMembership

		switch m.MembershipType {
		case authz.MembershipTypeUser:
			// Fetch the user details for user memberships
			u, uErr := r.data.DB.User.Get(ctx, m.MemberID)
			if uErr != nil {
				if ent.IsNotFound(uErr) {
					return nil, 0, biz.NewErrNotFound("user")
				}
				return nil, 0, fmt.Errorf("failed to find user: %w", uErr)
			}
			mems = entProjectMembershipToBiz(m, u, nil)
		case authz.MembershipTypeGroup:
			// Fetch the group details for group memberships
			g, gErr := r.data.DB.Group.Get(ctx, m.MemberID)
			if gErr != nil {
				if ent.IsNotFound(gErr) {
					return nil, 0, biz.NewErrNotFound("group")
				}
				return nil, 0, fmt.Errorf("failed to find group: %w", gErr)
			}
			mems = entProjectMembershipToBiz(m, nil, g)
		}

		if mems != nil {
			result = append(result, mems)
		}
	}

	return result, totalCount, nil
}

// AddMemberToProject adds a user or group to a project with a specific role
func (r *ProjectRepo) AddMemberToProject(ctx context.Context, orgID uuid.UUID, projectID uuid.UUID, memberID uuid.UUID, membershipType authz.MembershipType, role authz.Role) (*biz.ProjectMembership, error) {
	// Check if the project exists and belongs to the organization
	existingProject, err := r.FindProjectByOrgIDAndID(ctx, orgID, projectID)
	if err != nil {
		return nil, fmt.Errorf("failed to find project: %w", err)
	}
	if existingProject == nil {
		return nil, biz.NewErrNotFound("project")
	}

	if role != authz.RoleProjectAdmin && role != authz.RoleProjectViewer {
		return nil, biz.NewErrValidationStr("invalid role, must be either 'admin' or 'viewer'")
	}

	// Create the membership
	if _, err := r.data.DB.Membership.Create().
		SetOrganizationID(orgID).
		SetMembershipType(membershipType).
		SetMemberID(memberID).
		SetResourceType(authz.ResourceTypeProject).
		SetResourceID(projectID).
		SetRole(role).
		Save(ctx); err != nil {
		return nil, fmt.Errorf("failed to create project membership: %w", err)
	}

	// Return the created membership
	return r.FindProjectMembershipByProjectAndID(ctx, orgID, projectID, memberID, membershipType)
}

// RemoveMemberFromProject removes a user or group from a project
func (r *ProjectRepo) RemoveMemberFromProject(ctx context.Context, orgID uuid.UUID, projectID uuid.UUID, memberID uuid.UUID, membershipType authz.MembershipType) error {
	// Check if the project exists and belongs to the organization
	existingProject, err := r.FindProjectByOrgIDAndID(ctx, orgID, projectID)
	if err != nil {
		return fmt.Errorf("failed to find project: %w", err)
	}
	if existingProject == nil {
		return biz.NewErrNotFound("project")
	}

	// Find the membership to delete
	m, err := r.queryMembership(orgID, projectID, memberID, membershipType).Only(ctx)

	if err != nil {
		if ent.IsNotFound(err) {
			return biz.NewErrNotFound("membership")
		}
		return fmt.Errorf("failed to find membership: %w", err)
	}

	// Delete the membership
	if err := r.data.DB.Membership.DeleteOne(m).Exec(ctx); err != nil {
		return fmt.Errorf("failed to delete membership: %w", err)
	}

	return nil
}

// FindProjectMembershipByProjectAndID finds a project membership by project ID and member ID (user or group)
func (r *ProjectRepo) FindProjectMembershipByProjectAndID(ctx context.Context, orgID uuid.UUID, projectID uuid.UUID, memberID uuid.UUID, membershipType authz.MembershipType) (*biz.ProjectMembership, error) {
	// Find the membership
	m, err := r.queryMembership(orgID, projectID, memberID, membershipType).Only(ctx)

	if err != nil {
		if ent.IsNotFound(err) {
			return nil, nil // Return nil when no membership found
		}
		return nil, fmt.Errorf("failed to find membership: %w", err)
	}

	// Build the membership response based on the membership type
	projectMembership := &biz.ProjectMembership{
		MembershipType: m.MembershipType,
		Role:           m.Role,
		CreatedAt:      &m.CreatedAt,
		UpdatedAt:      &m.UpdatedAt,
	}

	switch membershipType {
	case authz.MembershipTypeUser:
		// Fetch the user details for user memberships
		u, err := r.data.DB.User.Get(ctx, memberID)
		if err != nil {
			if ent.IsNotFound(err) {
				return nil, biz.NewErrNotFound("user")
			}
			return nil, fmt.Errorf("failed to find user: %w", err)
		}
		projectMembership.User = entUserToBizUser(u)
	case authz.MembershipTypeGroup:
		// Fetch the group details for group memberships
		g, err := r.data.DB.Group.Query().Where(group.ID(memberID), group.DeletedAtIsNil()).Only(ctx)
		if err != nil {
			if ent.IsNotFound(err) {
				return nil, biz.NewErrNotFound("group")
			}
			return nil, fmt.Errorf("failed to find group: %w", err)
		}
		projectMembership.Group = entGroupToBiz(g)
	}

	return projectMembership, nil
}

// UpdateMemberRoleInProject updates the role of a member in a project
func (r *ProjectRepo) UpdateMemberRoleInProject(ctx context.Context, orgID uuid.UUID, projectID uuid.UUID, memberID uuid.UUID, membershipType authz.MembershipType, newRole authz.Role) (*biz.ProjectMembership, error) {
	// Check if the project exists and belongs to the organization
	existingProject, err := r.FindProjectByOrgIDAndID(ctx, orgID, projectID)
	if err != nil {
		return nil, fmt.Errorf("failed to find project: %w", err)
	}
	if existingProject == nil {
		return nil, biz.NewErrNotFound("project")
	}

	if newRole != authz.RoleProjectAdmin && newRole != authz.RoleProjectViewer {
		return nil, biz.NewErrValidationStr("invalid role, must be either 'admin' or 'viewer'")
	}

	// Find the membership to update
	m, err := r.queryMembership(orgID, projectID, memberID, membershipType).Only(ctx)

	if err != nil {
		if ent.IsNotFound(err) {
			return nil, biz.NewErrNotFound("membership")
		}
		return nil, fmt.Errorf("failed to find membership: %w", err)
	}

	// Update the role
	m, err = m.Update().SetUpdatedAt(time.Now()).SetRole(newRole).Save(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to update membership role: %w", err)
	}

	return entProjectMembershipToBiz(m, nil, nil), nil
}

// queryMembership is a helper function to build a common membership query
func (r *ProjectRepo) queryMembership(orgID uuid.UUID, projectID uuid.UUID, memberID uuid.UUID, membershipType authz.MembershipType) *ent.MembershipQuery {
	return r.data.DB.Membership.Query().
		Where(
			membership.HasOrganizationWith(
				organization.ID(orgID),
			),
			membership.MembershipTypeEQ(membershipType),
			membership.MemberID(memberID),
			membership.ResourceTypeEQ(authz.ResourceTypeProject),
			membership.ResourceID(projectID),
		).WithOrganization()
}

// entProjectToBiz converts an ent.Project to a biz.Project
func entProjectToBiz(pro *ent.Project) *biz.Project {
	return &biz.Project{
		ID:        pro.ID,
		Name:      pro.Name,
		OrgID:     pro.OrganizationID,
		CreatedAt: &pro.CreatedAt,
		UpdatedAt: &pro.CreatedAt,
	}
}

// entProjectMembershipToBiz converts an ent.Membership to a biz.ProjectMembership
// and includes user or group details if available
func entProjectMembershipToBiz(m *ent.Membership, u *ent.User, g *ent.Group) *biz.ProjectMembership {
	mem := &biz.ProjectMembership{
		MembershipType: m.MembershipType,
		Role:           m.Role,
		CreatedAt:      &m.CreatedAt,
		UpdatedAt:      &m.UpdatedAt,
	}

	if u != nil {
		mem.User = entUserToBizUser(u)
	}

	if g != nil {
		mem.Group = entGroupToBiz(g)
	}

	return mem
}

// ListPendingInvitationsByProject retrieves pending invitations for a specific project in an organization.
func (r *ProjectRepo) ListPendingInvitationsByProject(ctx context.Context, orgID uuid.UUID, projectID uuid.UUID, paginationOpts *pagination.OffsetPaginationOpts) ([]*biz.OrgInvitation, int, error) {
	if paginationOpts == nil {
		paginationOpts = pagination.NewDefaultOffsetPaginationOpts()
	}

	// Check the project exists in the organization
	_, err := r.FindProjectByOrgIDAndID(ctx, orgID, projectID)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to find project: %w", err)
	}

	// Build the query for pending invitations related to the project
	query := r.data.DB.OrgInvitation.Query().
		Where(
			orginvitation.OrganizationIDEQ(orgID),
			orginvitation.DeletedAtIsNil(),
			orginvitation.StatusEQ(biz.OrgInvitationStatusPending),
			func(s *sql.Selector) {
				s.Where(sqljson.ValueEQ(orginvitation.FieldContext, projectID.String(), sqljson.DotPath("project_id_to_join")))
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
		return nil, 0, fmt.Errorf("failed to retrieve pending invitations: %w", err)
	}

	// Convert ent.OrgInvitation entities to biz.OrgInvitation
	result := make([]*biz.OrgInvitation, 0, len(invitations))
	for _, inv := range invitations {
		result = append(result, entInviteToBiz(inv))
	}

	return result, count, nil
}
