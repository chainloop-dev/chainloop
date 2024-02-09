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
	"github.com/chainloop-dev/chainloop/app/controlplane/internal/usercontext"

	errors "github.com/go-kratos/kratos/v2/errors"
)

type AttestationStateService struct {
	cpAPI.UnimplementedAttestationStateServiceServer
	*service
}

func NewAttestationStateService(opts ...NewOpt) *AttestationStateService {
	return &AttestationStateService{
		service: newService(opts...),
	}
}

func (s *AttestationStateService) Initialized(ctx context.Context, req *cpAPI.AttestationStateServiceInitializedRequest) (*cpAPI.AttestationStateServiceInitializedResponse, error) {
	robotAccount := usercontext.CurrentRobotAccount(ctx)
	if robotAccount == nil {
		return nil, errors.NotFound("not found", "robot account not found")
	}

	return &cpAPI.AttestationStateServiceInitializedResponse{}, nil
}

func (s *AttestationService) Save(ctx context.Context, req *cpAPI.AttestationStateServiceSaveRequest) (*cpAPI.AttestationStateServiceSaveResponse, error) {
	robotAccount := usercontext.CurrentRobotAccount(ctx)
	if robotAccount == nil {
		return nil, errors.NotFound("not found", "robot account not found")
	}

	return &cpAPI.AttestationStateServiceSaveResponse{}, nil
}

func (s *AttestationService) Read(ctx context.Context, req *cpAPI.AttestationStateServiceReadRequest) (*cpAPI.AttestationStateServiceReadResponse, error) {
	robotAccount := usercontext.CurrentRobotAccount(ctx)
	if robotAccount == nil {
		return nil, errors.NotFound("not found", "robot account not found")
	}

	return &cpAPI.AttestationStateServiceReadResponse{}, nil
}

func (s *AttestationService) Reset(ctx context.Context, req *cpAPI.AttestationStateServiceResetRequest) (*cpAPI.AttestationStateServiceResetResponse, error) {
	robotAccount := usercontext.CurrentRobotAccount(ctx)
	if robotAccount == nil {
		return nil, errors.NotFound("not found", "robot account not found")
	}

	return &cpAPI.AttestationStateServiceResetResponse{}, nil
}
