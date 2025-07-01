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

// ListMembers lists all members of a project
func (r *ProjectRepo) ListMembers(ctx context.Context, orgID uuid.UUID, projectID uuid.UUID, paginationOpts *pagination.OffsetPaginationOpts) ([]*biz.ProjectMembership, int, error) {
	// Check if the project exists and belongs to the organization
	existingProject, err := r.FindProjectByOrgIDAndID(ctx, orgID, projectID)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to find project: %w", err)
	}
	if existingProject == nil {
		return nil, 0, biz.NewErrNotFound("project")
	}

	// Build the query with base conditions
	query := r.data.DB.Membership.Query().
		Where(
			membership.ResourceTypeEQ(authz.ResourceTypeProject),
			membership.ResourceID(projectID),
			membership.MembershipTypeEQ(authz.MembershipTypeUser),
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
		u, uErr := r.data.DB.User.Get(ctx, m.MemberID)
		if uErr != nil {
			if ent.IsNotFound(uErr) {
				return nil, 0, biz.NewErrNotFound("user")
			}
			return nil, 0, fmt.Errorf("failed to find user: %w", uErr)
		}

		result = append(result, entProjectMembershipToBiz(m, u))
	}

	return result, totalCount, nil
}

// AddMemberToProject adds a user to a project with a specific role
func (r *ProjectRepo) AddMemberToProject(ctx context.Context, orgID uuid.UUID, projectID uuid.UUID, userID uuid.UUID, role authz.Role) (*biz.ProjectMembership, error) {
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
		SetMembershipType(authz.MembershipTypeUser).
		SetMemberID(userID).
		SetResourceType(authz.ResourceTypeProject).
		SetResourceID(projectID).
		SetRole(role).
		Save(ctx); err != nil {
		return nil, fmt.Errorf("failed to create project membership: %w", err)
	}

	// Return the created membership
	return r.FindProjectMembershipByProjectAndID(ctx, projectID, userID)
}

// RemoveMemberFromProject removes a user from a project
func (r *ProjectRepo) RemoveMemberFromProject(ctx context.Context, orgID uuid.UUID, projectID uuid.UUID, userID uuid.UUID) error {
	// Check if the project exists and belongs to the organization
	existingProject, err := r.FindProjectByOrgIDAndID(ctx, orgID, projectID)
	if err != nil {
		return fmt.Errorf("failed to find project: %w", err)
	}
	if existingProject == nil {
		return biz.NewErrNotFound("project")
	}

	// Find the membership to delete
	m, err := r.data.DB.Membership.Query().
		Where(
			membership.MembershipTypeEQ(authz.MembershipTypeUser),
			membership.MemberID(userID),
			membership.ResourceTypeEQ(authz.ResourceTypeProject),
			membership.ResourceID(projectID),
		).Only(ctx)

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

// FindProjectMembershipByProjectAndID finds a project membership by project ID and user ID
func (r *ProjectRepo) FindProjectMembershipByProjectAndID(ctx context.Context, projectID uuid.UUID, userID uuid.UUID) (*biz.ProjectMembership, error) {
	// Find the membership
	m, err := r.data.DB.Membership.Query().
		Where(
			membership.MembershipTypeEQ(authz.MembershipTypeUser),
			membership.MemberID(userID),
			membership.ResourceTypeEQ(authz.ResourceTypeProject),
			membership.ResourceID(projectID),
		).
		Only(ctx)

	if err != nil {
		if ent.IsNotFound(err) {
			return nil, nil // Return nil when no membership found
		}
		return nil, fmt.Errorf("failed to find membership: %w", err)
	}

	u, err := r.data.DB.User.Get(ctx, userID)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, biz.NewErrNotFound("user")
		}
		return nil, fmt.Errorf("failed to find user: %w", err)
	}

	return entProjectMembershipToBiz(m, u), nil
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
func entProjectMembershipToBiz(m *ent.Membership, u *ent.User) *biz.ProjectMembership {
	return &biz.ProjectMembership{
		User:      entUserToBizUser(u),
		Role:      m.Role,
		CreatedAt: &m.CreatedAt,
		UpdatedAt: &m.UpdatedAt,
	}
}
