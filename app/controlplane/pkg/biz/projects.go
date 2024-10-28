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
	"time"

	"github.com/chainloop-dev/chainloop/pkg/servicelogger"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/google/uuid"
)

// ProjectsRepo is a repository for projects
type ProjectsRepo interface {
	FindProjectByOrgIDAndName(ctx context.Context, orgID uuid.UUID, projectName string) (*Project, error)
	FindProjectByOrgIDAndID(ctx context.Context, orgID uuid.UUID, projectID uuid.UUID) (*Project, error)
}

// ProjectUseCase is a use case for projects
type ProjectUseCase struct {
	logger *log.Helper
	// Repositories
	projectsRepository ProjectsRepo
}

// Project is a project in the organization
type Project struct {
	// ID is the unique identifier of the project
	ID uuid.UUID
	// Name is the name of the project
	Name string
	// OrgID is the organization that this project belongs to
	OrgID uuid.UUID
	// CreatedAt is the time when the project was created
	CreatedAt *time.Time
	// UpdatedAt is the time when the project was last updated
	UpdatedAt *time.Time
}

func NewProjectsUseCase(logger log.Logger, projectsRepository ProjectsRepo) *ProjectUseCase {
	return &ProjectUseCase{
		logger:             servicelogger.ScopedHelper(logger, "biz/projects"),
		projectsRepository: projectsRepository,
	}
}

// FindProjectByReference finds a project by reference, which can be either a project name or a project ID.
func (uc *ProjectUseCase) FindProjectByReference(ctx context.Context, orgID string, reference *EntityRef) (*Project, error) {
	if reference == nil || orgID == "" {
		return nil, NewErrValidationStr("orgID or project reference are empty")
	}
	orgUUID, err := uuid.Parse(orgID)
	if err != nil {
		return nil, NewErrInvalidUUID(err)
	}

	switch {
	case reference.Name != "":
		return uc.projectsRepository.FindProjectByOrgIDAndName(ctx, orgUUID, reference.Name)
	case reference.ID != "":
		projectUUID, err := uuid.Parse(reference.ID)
		if err != nil {
			return nil, NewErrInvalidUUID(err)
		}
		return uc.projectsRepository.FindProjectByOrgIDAndID(ctx, orgUUID, projectUUID)
	default:
		return nil, NewErrValidationStr("project reference is empty")
	}
}
