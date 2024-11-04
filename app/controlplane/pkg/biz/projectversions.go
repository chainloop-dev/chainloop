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
	ID         uuid.UUID
	Version    string
	Prerelease bool
	CreatedAt  *time.Time
}

type ProjectVersionRepo interface {
	FindByProjectAndVersion(ctx context.Context, projectID uuid.UUID, version string) (*ProjectVersion, error)
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
