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
	"fmt"
	"time"

	"github.com/go-kratos/kratos/v2/log"
	cr_v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/uuid"
)

type CASMapping struct {
	ID, CASBackendID, WorkflowRunID uuid.UUID
	Digest                          string
	CreatedAt                       *time.Time
}

type CASMappingRepo interface {
	Create(ctx context.Context, digest string, casBackendID, workflowRunID uuid.UUID) (*CASMapping, error)
}

type CASMappingUseCase struct {
	repo   CASMappingRepo
	logger *log.Helper
}

func NewCASMappingUseCase(repo CASMappingRepo, logger log.Logger) *CASMappingUseCase {
	return &CASMappingUseCase{repo, log.NewHelper(logger)}
}

func (uc *CASMappingUseCase) Create(ctx context.Context, digest string, casBackendID, workflowRunID string) (*CASMapping, error) {
	casBackendUUID, err := uuid.Parse(casBackendID)
	if err != nil {
		return nil, NewErrInvalidUUID(err)
	}

	workflowRunUUID, err := uuid.Parse(workflowRunID)
	if err != nil {
		return nil, NewErrInvalidUUID(err)
	}

	// parse the digest to make sure is a valid sha256 sum
	if _, err = cr_v1.NewHash(digest); err != nil {
		return nil, NewErrValidation(fmt.Errorf("invalid digest format: %w", err))
	}

	return uc.repo.Create(ctx, digest, casBackendUUID, workflowRunUUID)
}
