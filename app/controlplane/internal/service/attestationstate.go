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

package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"strings"

	cpAPI "github.com/chainloop-dev/chainloop/app/controlplane/api/controlplane/v1"
	"github.com/chainloop-dev/chainloop/app/controlplane/internal/biz"
	"github.com/chainloop-dev/chainloop/app/controlplane/internal/usercontext"

	errors "github.com/go-kratos/kratos/v2/errors"
	"github.com/go-kratos/kratos/v2/transport"
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

	encryptionPassphrase, err := encryptionPassphrase(ctx)
	if err != nil {
		return nil, errors.Forbidden("forbidden", "failed to authenticate request")
	}

	if err := s.uc.Save(ctx, robotAccount.WorkflowID, req.WorkflowRunId, req.AttestationState, encryptionPassphrase); err != nil {
		return nil, handleUseCaseErr("state", err, s.log)
	}

	return &cpAPI.AttestationStateServiceSaveResponse{}, nil
}

func (s *AttestationStateService) Read(ctx context.Context, req *cpAPI.AttestationStateServiceReadRequest) (*cpAPI.AttestationStateServiceReadResponse, error) {
	robotAccount := usercontext.CurrentRobotAccount(ctx)
	if robotAccount == nil {
		return nil, errors.NotFound("not found", "robot account not found")
	}

	encryptionPassphrase, err := encryptionPassphrase(ctx)
	if err != nil {
		return nil, errors.Forbidden("forbidden", "failed to authenticate request")
	}

	state, err := s.uc.Read(ctx, robotAccount.WorkflowID, req.WorkflowRunId, encryptionPassphrase)
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

// In order to encrypt the state at rest, we will encrypt the state using a passphrase
// which comes in the request and is used to derive an encryption key.
// The passphrase that we'll use is the robot-account token that only the workload should have.
// NOTE: Using the robot-account as JWT is not ideal but it's a start
// TODO: look into using some identifier from the actual client like machine-uuid
func encryptionPassphrase(ctx context.Context) (string, error) {
	header, ok := transport.FromServerContext(ctx)
	if !ok {
		return "", errors.NotFound("not found", "transport not found")
	}

	auths := strings.SplitN(header.RequestHeader().Get(authorizationKey), " ", 2)
	if len(auths) != 2 || !strings.EqualFold(auths[0], "Bearer") {
		return "", errors.BadRequest("bad request", "missing auth token")
	}

	// We'll use the sha256 of the token as the passphrase
	hash := sha256.Sum256([]byte(auths[1]))
	return hex.EncodeToString(hash[:]), nil
}
