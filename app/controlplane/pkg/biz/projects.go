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

	"github.com/google/uuid"
)

// ProjectsRepo is a repository for projects
type ProjectsRepo interface {
	FindProjectByOrgIDAndName(ctx context.Context, orgID uuid.UUID, projectName string) (*Project, error)
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
