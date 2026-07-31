//
// Copyright 2024-2026 The Chainloop Authors.
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
	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/data/ent/projectversion"
	"github.com/chainloop-dev/chainloop/pkg/otelx"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/google/uuid"
)

var projectVersionRepoTracer = otelx.Tracer("chainloop-controlplane", "data/projectversion")

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
	ctx, span := otelx.Start(ctx, projectVersionRepoTracer, "ProjectVersionRepo.FindByProjectAndVersion")
	defer span.End()

	pv, err := findProjectVersionWithClient(ctx, r.data.DB, projectID, version)
	if err != nil && !ent.IsNotFound(err) {
		return nil, err
	} else if pv == nil {
		return nil, biz.NewErrNotFound("Version")
	}

	return entProjectVersionToBiz(pv), nil
}

func (r *ProjectVersionRepo) Update(ctx context.Context, id uuid.UUID, updates *biz.ProjectVersionUpdateOpts) (*biz.ProjectVersion, error) {
	ctx, span := otelx.Start(ctx, projectVersionRepoTracer, "ProjectVersionRepo.Update")
	defer span.End()

	if updates == nil {
		updates = &biz.ProjectVersionUpdateOpts{}
	}
	// Only set released_at if it's not already set
	existing, err := r.data.DB.ProjectVersion.Query().Where(projectversion.IDEQ(id), projectversion.DeletedAtIsNil()).Only(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, biz.NewErrNotFound("Version")
		}

		return nil, err
	}

	now := time.Now()
	q := existing.Update().SetNillablePrerelease(updates.Prerelease).SetUpdatedAt(now)
	// we are setting the value either false or true
	if updates.Prerelease != nil {
		// We are marking it as a release
		if !*updates.Prerelease {
			// if not set
			if existing.ReleasedAt.IsZero() {
				// Only set released_at if it's not already set
				q.SetReleasedAt(now)
			}
		} else {
			// We are resetting it to pre-release
			q.ClearReleasedAt()
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
	ctx, span := otelx.Start(ctx, projectVersionRepoTracer, "ProjectVersionRepo.Create")
	defer span.End()

	var res *ent.ProjectVersion
	if err := WithTx(ctx, r.data.DB, func(tx *ent.Tx) error {
		var err error
		res, err = createProjectVersionWithTx(ctx, tx, projectID, version, prerelease, nil)
		return err
	}); err != nil {
		return nil, err
	}

	return entProjectVersionToBiz(res), nil
}

func createProjectVersionWithTx(ctx context.Context, tx *ent.Tx, projectID uuid.UUID, version string, prerelease bool, markAsLatest *bool) (*ent.ProjectVersion, error) {
	if version == "" {
		return nil, biz.NewErrValidationStr("version must not be empty")
	}

	// nil means the caller didn't opt in/out, so new versions default to latest (preserves original behavior)
	shouldBeLatest := markAsLatest == nil || *markAsLatest

	if shouldBeLatest {
		// Update all existing versions of this project to not be the latest
		if err := tx.ProjectVersion.Update().
			Where(
				projectversion.ProjectID(projectID),
				projectversion.DeletedAtIsNil(),
				projectversion.Latest(true),
			).SetLatest(false).Exec(ctx); err != nil {
			return nil, err
		}
	}

	return tx.ProjectVersion.Create().
		SetProjectID(projectID).
		SetVersion(version).
		SetPrerelease(prerelease).
		SetLatest(shouldBeLatest).
		Save(ctx)
}

func (r *ProjectVersionRepo) MarkAsLatest(ctx context.Context, projectID, versionID uuid.UUID) (*biz.ProjectVersionPromotion, error) {
	ctx, span := otelx.Start(ctx, projectVersionRepoTracer, "ProjectVersionRepo.MarkAsLatest")
	defer span.End()

	// The audit context is read up front, before anything is written: the
	// project's name and organization are stable, and reading them here keeps
	// them off both the locked window and the post-commit path. A promotion that
	// commits can then always be reported, whereas a read placed after the
	// commit could fail and strand the promotion with no event — and the retry
	// would see it as already latest and stay silent forever.
	p, err := r.data.DB.Project.Get(ctx, projectID)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, biz.NewErrNotFound("Project")
		}
		return nil, fmt.Errorf("loading project: %w", err)
	}

	var promotion *biz.ProjectVersionPromotion
	if err := WithTx(ctx, r.data.DB, func(tx *ent.Tx) error {
		v, err := tx.ProjectVersion.Query().ForUpdate().
			Where(projectversion.ID(versionID), projectversion.ProjectID(projectID), projectversion.DeletedAtIsNil()).
			Only(ctx)
		if err != nil {
			if ent.IsNotFound(err) {
				return biz.NewErrNotFound("Version")
			}
			return err
		}

		promoted, err := promoteVersionToLatestWithTx(ctx, tx, v)
		if err != nil {
			return err
		}

		// v is the pre-update snapshot and latest is the only field the
		// promotion touches, so this describes the committed row.
		version := entProjectVersionToBiz(v)
		version.Latest = true

		promotion = &biz.ProjectVersionPromotion{
			Promoted: promoted,
			Version:  version,
			Project:  entProjectToBiz(p),
		}

		return nil
	}); err != nil {
		return nil, err
	}

	return promotion, nil
}

// promoteVersionToLatestWithTx makes v the only latest version of its project.
// v must have been read in this transaction with a row lock, which serialises
// concurrent promotions of that same version.
//
// It reports whether v's own latest flag changed, which callers use to decide
// whether the transition is worth announcing.
//
// The lock covers v alone, not the project's other versions, so promotions of
// two different versions of the same project are not serialised against each
// other and can both report success. Closing that needs a lock on the project
// row or a unique partial index on (project_id) WHERE latest.
func promoteVersionToLatestWithTx(ctx context.Context, tx *ent.Tx, v *ent.ProjectVersion) (bool, error) {
	if !v.Prerelease {
		return false, biz.NewErrValidationStr("cannot promote a released version to latest")
	}

	if err := tx.ProjectVersion.Update().
		Where(
			projectversion.ProjectID(v.ProjectID),
			projectversion.DeletedAtIsNil(),
			projectversion.Latest(true),
		).SetLatest(false).Exec(ctx); err != nil {
		return false, err
	}

	if err := tx.ProjectVersion.UpdateOneID(v.ID).
		SetLatest(true).
		SetUpdatedAt(time.Now()).
		Exec(ctx); err != nil {
		return false, err
	}

	return !v.Latest, nil
}

func findProjectVersionWithClient(ctx context.Context, client *ent.Client, projectID uuid.UUID, version string) (*ent.ProjectVersion, error) {
	return client.ProjectVersion.Query().
		Where(
			projectversion.ProjectID(projectID),
			projectversion.VersionEQ(version),
			projectversion.DeletedAtIsNil(),
		).Only(ctx)
}

func entProjectVersionToBiz(v *ent.ProjectVersion) *biz.ProjectVersion {
	pv := &biz.ProjectVersion{
		ID:                v.ID,
		Version:           v.Version,
		Prerelease:        v.Prerelease,
		Latest:            v.Latest,
		TotalWorkflowRuns: v.WorkflowRunCount,
		CreatedAt:         toTimePtr(v.CreatedAt),
		ReleasedAt:        toTimePtr(v.ReleasedAt),
		LastRunAt:         toTimePtr(v.LastRunAt),
		ProjectID:         v.ProjectID,
	}

	return pv
}
