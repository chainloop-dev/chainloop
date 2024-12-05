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

	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/biz"
	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/data/ent"
	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/data/ent/project"
	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/data/ent/projectversion"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/google/uuid"
)

type ProjectVersionRepo struct {
	data *Data
	log  *log.Helper
}

func NewProjectVersionRepo(data *Data, logger log.Logger) biz.ProjectVersionRepo {
	return &ProjectVersionRepo{
		data: data,
		log:  log.NewHelper(logger),
	}
}

func (r *ProjectVersionRepo) FindByProjectAndVersion(ctx context.Context, projectID uuid.UUID, version string) (*biz.ProjectVersion, error) {
	pv, err := r.data.DB.ProjectVersion.Query().Where(projectversion.HasProjectWith(project.ID(projectID)), projectversion.VersionEQ(version)).Only(ctx)
	if err != nil && !ent.IsNotFound(err) {
		return nil, err
	} else if pv == nil {
		return nil, biz.NewErrNotFound("Version")
	}

	return entProjectVersionToBiz(pv), nil
}

func (r *ProjectVersionRepo) Update(ctx context.Context, id uuid.UUID, updates *biz.ProjectVersionUpdateOpts) (*biz.ProjectVersion, error) {
	if updates == nil {
		updates = &biz.ProjectVersionUpdateOpts{}
	}

	res, err := r.data.DB.ProjectVersion.UpdateOneID(id).SetNillablePrerelease(updates.Prerelease).Save(ctx)
	if err != nil && !ent.IsNotFound(err) {
		return nil, err
	} else if res == nil {
		return nil, biz.NewErrNotFound("Version")
	}

	return entProjectVersionToBiz(res), nil
}

func (r *ProjectVersionRepo) Create(ctx context.Context, projectID uuid.UUID, version string, prerelease bool) (*biz.ProjectVersion, error) {
	pv, err := r.data.DB.ProjectVersion.Create().
		SetProjectID(projectID).
		SetVersion(version).
		SetPrerelease(prerelease).
		Save(ctx)
	if err != nil {
		return nil, err
	}

	return entProjectVersionToBiz(pv), nil
}

func entProjectVersionToBiz(v *ent.ProjectVersion) *biz.ProjectVersion {
	return &biz.ProjectVersion{
		ID:                v.ID,
		Version:           v.Version,
		Prerelease:        v.Prerelease,
		TotalWorkflowRuns: v.WorkflowRunCount,
	}
}
