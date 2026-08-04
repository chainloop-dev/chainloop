//
// Copyright 2023-2026 The Chainloop Authors.
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

	"github.com/chainloop-dev/chainloop/pkg/otelx"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/google/uuid"
)

var robotAccountTracer = otelx.Tracer("chainloop-controlplane", "biz/robotaccount")

type RobotAccount struct {
	Name                 string
	ID                   uuid.UUID
	WorkflowID           uuid.UUID
	CreatedAt, RevokedAt *time.Time
}

type RobotAccountRepo interface {
	List(ctx context.Context, workflowID uuid.UUID, includeRevoked bool) ([]*RobotAccount, error)
	FindByID(ctx context.Context, ID uuid.UUID) (*RobotAccount, error)
	Revoke(ctx context.Context, orgID, ID uuid.UUID) error
}

type RobotAccountUseCase struct {
	robotAccountRepo RobotAccountRepo
	workflowRepo     WorkflowRepo
	logger           *log.Helper
}

func NewRootAccountUseCase(robotAccountRepo RobotAccountRepo, workflowRepo WorkflowRepo, logger log.Logger) *RobotAccountUseCase {
	return &RobotAccountUseCase{
		robotAccountRepo: robotAccountRepo,
		workflowRepo:     workflowRepo,
		logger:           log.NewHelper(logger),
	}
}

func (uc *RobotAccountUseCase) List(ctx context.Context, orgID, workflowID string, includeRevoked bool) ([]*RobotAccount, error) {
	ctx, span := otelx.Start(ctx, robotAccountTracer, "RobotAccountUseCase.List")
	defer span.End()

	workflowUUID, err := uuid.Parse(workflowID)
	if err != nil {
		return nil, NewErrInvalidUUID(err)
	}

	orgUUID, err := uuid.Parse(orgID)
	if err != nil {
		return nil, NewErrInvalidUUID(err)
	}
	// Check that the workflow is from the provided user
	if wf, err := uc.workflowRepo.GetOrgScoped(ctx, orgUUID, workflowUUID); err != nil {
		return nil, err
	} else if wf == nil {
		return nil, NewErrNotFound("workflow")
	}

	return uc.robotAccountRepo.List(ctx, workflowUUID, includeRevoked)
}

func (uc *RobotAccountUseCase) FindByID(ctx context.Context, id string) (*RobotAccount, error) {
	ctx, span := otelx.Start(ctx, robotAccountTracer, "RobotAccountUseCase.FindByID")
	defer span.End()

	uuid, err := uuid.Parse(id)
	if err != nil {
		return nil, NewErrInvalidUUID(err)
	}

	return uc.robotAccountRepo.FindByID(ctx, uuid)
}

func (uc *RobotAccountUseCase) Revoke(ctx context.Context, orgID, id string) error {
	ctx, span := otelx.Start(ctx, robotAccountTracer, "RobotAccountUseCase.Revoke")
	defer span.End()

	orgUUID, err := uuid.Parse(orgID)
	if err != nil {
		return NewErrInvalidUUID(err)
	}

	uuid, err := uuid.Parse(id)
	if err != nil {
		return NewErrInvalidUUID(err)
	}
	return uc.robotAccountRepo.Revoke(ctx, orgUUID, uuid)
}
