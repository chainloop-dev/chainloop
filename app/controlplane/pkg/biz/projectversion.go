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

package biz

import (
	"context"
	"io"
	"time"

	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/auditor/events"
	"github.com/chainloop-dev/chainloop/pkg/otelx"
	"github.com/chainloop-dev/chainloop/pkg/servicelogger"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/google/uuid"
)

var projectVersionTracer = otelx.Tracer("chainloop-controlplane", "biz/projectversion")

// DefaultVersionName is the canonical name for the default/unversioned project version.
const DefaultVersionName = "v0"

type ProjectVersion struct {
	// ID is the UUID of the project version.
	ID uuid.UUID
	// Version is the version of the project.
	Version string
	// Prerelease indicates whether the version is a prerelease.
	Prerelease bool
	// Latest indicates whether this is the latest version of the project.
	Latest bool
	// TotalWorkflowRuns is the total number of workflow runs for this version.
	TotalWorkflowRuns int
	// CreatedAt is the time when the project version was created.
	CreatedAt *time.Time
	// ReleasedAt is the time when the version was released.
	ReleasedAt *time.Time
	// LastRunAt is the time when the last workflow run occurred for this version.
	LastRunAt *time.Time
	ProjectID uuid.UUID
}

// ProjectVersionPromotion is the outcome of promoting a project version to be
// the latest one.
type ProjectVersionPromotion struct {
	// Promoted reports whether the promotion changed which version is the
	// latest one. It is false when the version already was the latest.
	Promoted bool
	// Version and Project describe the promotion for auditing purposes. Both are
	// populated whenever the promotion succeeded, so that a committed promotion
	// can always be reported.
	Version *ProjectVersion
	Project *Project
}

// dispatchProjectVersionPromoted reports that a project version became the
// latest one for its project. Every path that promotes a version funnels
// through here: downstream consumers reconcile off this event, so a promotion
// must be as loud as a creation, otherwise anything tracking the latest version
// of the project silently falls behind.
//
// A promotion deliberately reuses ProjectVersionUpdated rather than declaring
// its own action type, and that choice is load-bearing. Subjects are published
// as "audit.<target_type>.<action_type>", and consumers subscribe to a fixed
// list of them, so a new action type lands on a subject nobody is listening to
// and every promotion is dropped — the very failure this event exists to
// prevent, but silent. If this action type ever changes, the consumers must
// subscribe to the new subject and be released FIRST; MarkedAsLatest is what
// lets a consumer tell a promotion from a rename in the meantime.
func dispatchProjectVersionPromoted(ctx context.Context, auditorUC *AuditorUseCase, project *Project, version *ProjectVersion) {
	if auditorUC == nil || project == nil || version == nil {
		return
	}

	auditorUC.Dispatch(ctx, &events.ProjectVersionUpdated{
		ProjectBase: &events.ProjectBase{
			ProjectID:   &project.ID,
			ProjectName: project.Name,
		},
		VersionID:      &version.ID,
		Version:        version.Version,
		MarkedAsLatest: true,
	}, &project.OrgID)
}

type ProjectVersionRepo interface {
	FindByProjectAndVersion(ctx context.Context, projectID uuid.UUID, version string) (*ProjectVersion, error)
	Update(ctx context.Context, versionID uuid.UUID, updates *ProjectVersionUpdateOpts) (*ProjectVersion, error)
	Create(ctx context.Context, projectID uuid.UUID, version string, prerelease bool) (*ProjectVersion, error)
	MarkAsLatest(ctx context.Context, projectID, versionID uuid.UUID) (*ProjectVersionPromotion, error)
}

type ProjectVersionUseCase struct {
	projectRepo ProjectVersionRepo
	auditorUC   *AuditorUseCase
	logger      *log.Helper
}

func NewProjectVersionUseCase(repo ProjectVersionRepo, auditorUC *AuditorUseCase, l log.Logger) *ProjectVersionUseCase {
	if l == nil {
		l = log.NewStdLogger(io.Discard)
	}

	return &ProjectVersionUseCase{
		projectRepo: repo,
		auditorUC:   auditorUC,
		logger:      servicelogger.ScopedHelper(l, "biz/project-version"),
	}
}

func (uc *ProjectVersionUseCase) FindByProjectAndVersion(ctx context.Context, projectID string, version string) (*ProjectVersion, error) {
	ctx, span := otelx.Start(ctx, projectVersionTracer, "ProjectVersionUseCase.FindByProjectAndVersion")
	defer span.End()

	projectUUID, err := uuid.Parse(projectID)
	if err != nil {
		return nil, NewErrInvalidUUID(err)
	}

	return uc.projectRepo.FindByProjectAndVersion(ctx, projectUUID, version)
}

type ProjectVersionUpdateOpts struct {
	Prerelease *bool
}

func (uc *ProjectVersionUseCase) UpdateReleaseStatus(ctx context.Context, version string, isRelease bool) (*ProjectVersion, error) {
	ctx, span := otelx.Start(ctx, projectVersionTracer, "ProjectVersionUseCase.UpdateReleaseStatus")
	defer span.End()

	versionUUID, err := uuid.Parse(version)
	if err != nil {
		return nil, NewErrInvalidUUID(err)
	}

	preReleaseValue := !isRelease
	return uc.projectRepo.Update(ctx, versionUUID, &ProjectVersionUpdateOpts{Prerelease: &preReleaseValue})
}

// MarkAsLatest promotes a pre-release version to latest. The platform repo builds the
// "project version mark-latest" CLI command and service endpoint on top of this method.
func (uc *ProjectVersionUseCase) MarkAsLatest(ctx context.Context, projectID, versionID string) error {
	ctx, span := otelx.Start(ctx, projectVersionTracer, "ProjectVersionUseCase.MarkAsLatest")
	defer span.End()

	projectUUID, err := uuid.Parse(projectID)
	if err != nil {
		return NewErrInvalidUUID(err)
	}

	versionUUID, err := uuid.Parse(versionID)
	if err != nil {
		return NewErrInvalidUUID(err)
	}

	promotion, err := uc.projectRepo.MarkAsLatest(ctx, projectUUID, versionUUID)
	if err != nil {
		return err
	}

	if promotion.Promoted {
		dispatchProjectVersionPromoted(ctx, uc.auditorUC, promotion.Project, promotion.Version)
	}

	return nil
}

func (uc *ProjectVersionUseCase) Create(ctx context.Context, projectID, version string, prerelease bool) (*ProjectVersion, error) {
	ctx, span := otelx.Start(ctx, projectVersionTracer, "ProjectVersionUseCase.Create")
	defer span.End()

	projectUUID, err := uuid.Parse(projectID)
	if err != nil {
		return nil, NewErrInvalidUUID(err)
	}

	// Treat empty version as the default for backward compatibility
	if version == "" {
		version = DefaultVersionName
	}

	if err := ValidateVersion(version); err != nil {
		return nil, err
	}

	return uc.projectRepo.Create(ctx, projectUUID, version, prerelease)
}
