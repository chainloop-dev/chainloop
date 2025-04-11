//
// Copyright 2024-2025 The Chainloop Authors.
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
	"time"

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

	q := r.data.DB.ProjectVersion.UpdateOneID(id).SetNillablePrerelease(updates.Prerelease)
	// we are setting the value either false or true
	if updates.Prerelease != nil {
		// We are marking it as a release
		if !*updates.Prerelease {
			q = q.SetReleasedAt(time.Now())
		} else {
			// We are resetting it to pre-release
			q = q.ClearReleasedAt()
		}
	}

	res, err := q.Save(ctx)
	if err != nil && !ent.IsNotFound(err) {
		return nil, err
	} else if res == nil {
		return nil, biz.NewErrNotFound("Version")
	}

	return entProjectVersionToBiz(res), nil
}

func (r *ProjectVersionRepo) Create(ctx context.Context, projectID uuid.UUID, version string, prerelease bool) (*biz.ProjectVersion, error) {
	var res *ent.ProjectVersion
	if err := WithTx(ctx, r.data.DB, func(tx *ent.Tx) error {
		var err error
		res, err = createProjectWithTx(ctx, tx, projectID, version, prerelease)
		return err
	}); err != nil {
		return nil, err
	}

	return entProjectVersionToBiz(res), nil
}

func createProjectWithTx(ctx context.Context, tx *ent.Tx, projectID uuid.UUID, version string, prerelease bool) (*ent.ProjectVersion, error) {
	// Update all existing versions of this project to not be the latest
	if err := tx.ProjectVersion.Update().
		Where(
			projectversion.ProjectID(projectID),
			projectversion.DeletedAtIsNil(),
			projectversion.Latest(true),
		).SetLatest(false).Exec(ctx); err != nil {
		return nil, err
	}

	return tx.ProjectVersion.Create().
		SetProjectID(projectID).
		SetVersion(version).
		SetPrerelease(prerelease).
		SetLatest(true).
		Save(ctx)
}

func entProjectVersionToBiz(v *ent.ProjectVersion) *biz.ProjectVersion {
	pv := &biz.ProjectVersion{
		ID:                v.ID,
		Version:           v.Version,
		Prerelease:        v.Prerelease,
		TotalWorkflowRuns: v.WorkflowRunCount,
		CreatedAt:         toTimePtr(v.CreatedAt),
		ReleasedAt:        toTimePtr(v.ReleasedAt),
	}

	return pv
}
