//
// Copyright 2023 The Chainloop Authors.
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

	"github.com/chainloop-dev/chainloop/app/controlplane/api/controlplane/conf/v1"
	"github.com/chainloop-dev/chainloop/app/controlplane/internal/jwt"
	"github.com/chainloop-dev/chainloop/app/controlplane/internal/jwt/robotaccount"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/google/uuid"
)

type RobotAccount struct {
	Name                 string
	ID                   uuid.UUID
	JWT                  string
	WorkflowID           uuid.UUID
	CreatedAt, RevokedAt *time.Time
}

type RobotAccountRepo interface {
	Create(ctx context.Context, name string, workflowID uuid.UUID) (*RobotAccount, error)
	List(ctx context.Context, workflowID uuid.UUID, includeRevoked bool) ([]*RobotAccount, error)
	FindByID(ctx context.Context, ID uuid.UUID) (*RobotAccount, error)
	Revoke(ctx context.Context, orgID, ID uuid.UUID) error
}

type RobotAccountUseCase struct {
	robotAccountRepo RobotAccountRepo
	workflowRepo     WorkflowRepo
	authConf         *conf.Auth
	logger           *log.Helper
}

func NewRootAccountUseCase(robotAccountRepo RobotAccountRepo, workflowRepo WorkflowRepo, conf *conf.Auth, logger log.Logger) *RobotAccountUseCase {
	return &RobotAccountUseCase{
		robotAccountRepo: robotAccountRepo,
		workflowRepo:     workflowRepo,
		authConf:         conf,
		logger:           log.NewHelper(logger),
	}
}

func (uc *RobotAccountUseCase) Create(ctx context.Context, name string, orgID, workflowID string) (*RobotAccount, error) {
	workflowUUID, err := uuid.Parse(workflowID)
	if err != nil {
		return nil, NewErrInvalidUUID(err)
	}

	orgUUID, err := uuid.Parse(orgID)
	if err != nil {
		return nil, NewErrInvalidUUID(err)
	}

	// Make sure that the given workflow belongs to the provided org
	if wf, err := uc.workflowRepo.GetOrgScoped(ctx, orgUUID, workflowUUID); err != nil {
		return nil, err
	} else if wf == nil {
		return nil, NewErrNotFound("workflow")
	}

	res, err := uc.robotAccountRepo.Create(ctx, name, workflowUUID)
	if err != nil {
		return nil, err
	}

	// Create Key
	b, err := robotaccount.NewBuilder(
		robotaccount.WithIssuer(jwt.DefaultIssuer),
		robotaccount.WithKeySecret(uc.authConf.GeneratedJwsHmacSecret),
	)
	if err != nil {
		return nil, err
	}

	jwt, err := b.GenerateJWT(orgID, workflowID, res.ID.String(), jwt.DefaultAudience)
	if err != nil {
		return nil, err
	}

	res.JWT = jwt
	return res, nil
}

func (uc *RobotAccountUseCase) List(ctx context.Context, orgID, workflowID string, includeRevoked bool) ([]*RobotAccount, error) {
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
	uuid, err := uuid.Parse(id)
	if err != nil {
		return nil, NewErrInvalidUUID(err)
	}

	return uc.robotAccountRepo.FindByID(ctx, uuid)
}

func (uc *RobotAccountUseCase) Revoke(ctx context.Context, orgID, id string) error {
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
