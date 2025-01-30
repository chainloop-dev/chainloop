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

package biz

import (
	"context"
	"io"
	"time"

	"github.com/chainloop-dev/chainloop/pkg/servicelogger"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/google/uuid"
)

type ProjectVersion struct {
	// ID is the UUID of the project version.
	ID uuid.UUID
	// Version is the version of the project.
	Version string
	// Prerelease indicates whether the version is a prerelease.
	Prerelease bool
	// TotalWorkflowRuns is the total number of workflow runs for this version.
	TotalWorkflowRuns int
	// CreatedAt is the time when the project version was created.
	CreatedAt *time.Time
	// ReleasedAt is the time when the version was released.
	ReleasedAt *time.Time
}

type ProjectVersionRepo interface {
	FindByProjectAndVersion(ctx context.Context, projectID uuid.UUID, version string) (*ProjectVersion, error)
	Update(ctx context.Context, versionID uuid.UUID, updates *ProjectVersionUpdateOpts) (*ProjectVersion, error)
	Create(ctx context.Context, projectID uuid.UUID, version string, prerelease bool) (*ProjectVersion, error)
}

type ProjectVersionUseCase struct {
	projectRepo ProjectVersionRepo
	logger      *log.Helper
}

func NewProjectVersionUseCase(repo ProjectVersionRepo, l log.Logger) *ProjectVersionUseCase {
	if l == nil {
		l = log.NewStdLogger(io.Discard)
	}

	return &ProjectVersionUseCase{projectRepo: repo, logger: servicelogger.ScopedHelper(l, "biz/project-version")}
}

func (uc *ProjectVersionUseCase) FindByProjectAndVersion(ctx context.Context, projectID string, version string) (*ProjectVersion, error) {
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
	versionUUID, err := uuid.Parse(version)
	if err != nil {
		return nil, NewErrInvalidUUID(err)
	}

	preRelease := !isRelease
	return uc.projectRepo.Update(ctx, versionUUID, &ProjectVersionUpdateOpts{Prerelease: &preRelease})
}

func (uc *ProjectVersionUseCase) Create(ctx context.Context, projectID, version string, prerelease bool) (*ProjectVersion, error) {
	projectUUID, err := uuid.Parse(projectID)
	if err != nil {
		return nil, NewErrInvalidUUID(err)
	}

	return uc.projectRepo.Create(ctx, projectUUID, version, prerelease)
}
