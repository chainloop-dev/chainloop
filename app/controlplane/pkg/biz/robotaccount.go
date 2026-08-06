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
	"github.com/google/uuid"
)

var robotAccountTracer = otelx.Tracer("chainloop-controlplane", "biz/robotaccount")

// RobotAccount is a legacy, workflow-scoped credential superseded by API tokens. Its management API
// (RobotAccountService) was removed (CP-N4): accounts can no longer be created, listed or revoked.
// Existing tokens are still honored during attestation, which is why lookup remains.
type RobotAccount struct {
	ID, WorkflowID uuid.UUID
	RevokedAt      *time.Time
}

type RobotAccountRepo interface {
	// FindByID returns (nil, nil) when no account with the given ID exists.
	FindByID(ctx context.Context, ID uuid.UUID) (*RobotAccount, error)
}

type RobotAccountUseCase struct {
	robotAccountRepo RobotAccountRepo
}

func NewRobotAccountUseCase(robotAccountRepo RobotAccountRepo) *RobotAccountUseCase {
	return &RobotAccountUseCase{
		robotAccountRepo: robotAccountRepo,
	}
}

func (uc *RobotAccountUseCase) FindByID(ctx context.Context, id string) (*RobotAccount, error) {
	ctx, span := otelx.Start(ctx, robotAccountTracer, "RobotAccountUseCase.FindByID")
	defer span.End()

	accountUUID, err := uuid.Parse(id)
	if err != nil {
		return nil, NewErrInvalidUUID(err)
	}

	return uc.robotAccountRepo.FindByID(ctx, accountUUID)
}
