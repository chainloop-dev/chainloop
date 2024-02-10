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

package service

import (
	"context"

	cpAPI "github.com/chainloop-dev/chainloop/app/controlplane/api/controlplane/v1"
	"github.com/chainloop-dev/chainloop/app/controlplane/internal/biz"
	"github.com/chainloop-dev/chainloop/app/controlplane/internal/usercontext"

	errors "github.com/go-kratos/kratos/v2/errors"
)

type AttestationStateService struct {
	cpAPI.UnimplementedAttestationStateServiceServer
	uc *biz.AttestationStateUseCase
	*service
}

func NewAttestationStateService(uc *biz.AttestationStateUseCase, opts ...NewOpt) *AttestationStateService {
	return &AttestationStateService{
		service: newService(opts...), uc: uc,
	}
}

func (s *AttestationStateService) Initialized(ctx context.Context, req *cpAPI.AttestationStateServiceInitializedRequest) (*cpAPI.AttestationStateServiceInitializedResponse, error) {
	robotAccount := usercontext.CurrentRobotAccount(ctx)
	if robotAccount == nil {
		return nil, errors.NotFound("not found", "robot account not found")
	}

	initialized, err := s.uc.Initialized(ctx, robotAccount.WorkflowID, req.WorkflowRunId)
	if err != nil {
		return nil, handleUseCaseErr("state", err, s.log)
	}

	return &cpAPI.AttestationStateServiceInitializedResponse{
		Result: &cpAPI.AttestationStateServiceInitializedResponse_Result{
			Initialized: initialized,
		}}, nil
}

func (s *AttestationStateService) Save(ctx context.Context, req *cpAPI.AttestationStateServiceSaveRequest) (*cpAPI.AttestationStateServiceSaveResponse, error) {
	robotAccount := usercontext.CurrentRobotAccount(ctx)
	if robotAccount == nil {
		return nil, errors.NotFound("not found", "robot account not found")
	}

	if err := s.uc.Save(ctx, robotAccount.WorkflowID, req.WorkflowRunId, req.AttestationState); err != nil {
		return nil, handleUseCaseErr("state", err, s.log)
	}

	return &cpAPI.AttestationStateServiceSaveResponse{}, nil
}

func (s *AttestationStateService) Read(ctx context.Context, req *cpAPI.AttestationStateServiceReadRequest) (*cpAPI.AttestationStateServiceReadResponse, error) {
	robotAccount := usercontext.CurrentRobotAccount(ctx)
	if robotAccount == nil {
		return nil, errors.NotFound("not found", "robot account not found")
	}

	state, err := s.uc.Read(ctx, robotAccount.WorkflowID, req.WorkflowRunId)
	if err != nil {
		return nil, handleUseCaseErr("state", err, s.log)
	}

	return &cpAPI.AttestationStateServiceReadResponse{
		Result: &cpAPI.AttestationStateServiceReadResponse_Result{
			AttestationState: state.State,
		},
	}, nil
}

func (s *AttestationStateService) Reset(ctx context.Context, req *cpAPI.AttestationStateServiceResetRequest) (*cpAPI.AttestationStateServiceResetResponse, error) {
	robotAccount := usercontext.CurrentRobotAccount(ctx)
	if robotAccount == nil {
		return nil, errors.NotFound("not found", "robot account not found")
	}

	if err := s.uc.Reset(ctx, robotAccount.WorkflowID, req.WorkflowRunId); err != nil {
		return nil, handleUseCaseErr("state", err, s.log)
	}

	return &cpAPI.AttestationStateServiceResetResponse{}, nil
}
