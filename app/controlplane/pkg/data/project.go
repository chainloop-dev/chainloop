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

	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/biz"
	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/data/ent"
	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/data/ent/organization"
	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/data/ent/project"

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
	pro, err := r.data.DB.Organization.Query().Where(organization.ID(orgID)).QueryProjects().Where(project.Name(projectName)).Only(ctx)
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
	pro, err := r.data.DB.Organization.Query().Where(organization.ID(orgID)).QueryProjects().Where(project.ID(projectID)).Only(ctx)
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
