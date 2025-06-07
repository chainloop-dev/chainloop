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
	"fmt"
	"strings"

	cpAPI "github.com/chainloop-dev/chainloop/app/controlplane/api/controlplane/v1"
	"github.com/chainloop-dev/chainloop/app/controlplane/internal/usercontext"
	"github.com/chainloop-dev/chainloop/app/controlplane/internal/usercontext/multijwtmiddleware"
	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/biz"

	errors "github.com/go-kratos/kratos/v2/errors"
	"github.com/go-kratos/kratos/v2/transport"
)

type AttestationStateService struct {
	cpAPI.UnimplementedAttestationStateServiceServer
	*service

	attestationStateUseCase *biz.AttestationStateUseCase
	workflowUseCase         *biz.WorkflowUseCase
	wrUseCase               *biz.WorkflowRunUseCase
}

type NewAttestationStateServiceOpt struct {
	AttestationStateUseCase *biz.AttestationStateUseCase
	WorkflowUseCase         *biz.WorkflowUseCase
	WorkflowRunUseCase      *biz.WorkflowRunUseCase
	Opts                    []NewOpt
}

func NewAttestationStateService(opts *NewAttestationStateServiceOpt) *AttestationStateService {
	return &AttestationStateService{
		service:                 newService(opts.Opts...),
		attestationStateUseCase: opts.AttestationStateUseCase,
		workflowUseCase:         opts.WorkflowUseCase,
		wrUseCase:               opts.WorkflowRunUseCase,
	}
}

func (s *AttestationStateService) Initialized(ctx context.Context, req *cpAPI.AttestationStateServiceInitializedRequest) (*cpAPI.AttestationStateServiceInitializedResponse, error) {
	robotAccount := usercontext.CurrentAPIToken(ctx)
	if robotAccount == nil {
		return nil, errors.NotFound("not found", "robot account not found")
	}

	wf, err := s.findWorkflowFromTokenOrRunID(ctx, robotAccount.OrgID, robotAccount.WorkflowID, req.GetWorkflowRunId())
	if err != nil {
		return nil, handleUseCaseErr(err, s.log)
	}

	initialized, err := s.attestationStateUseCase.Initialized(ctx, wf.ID.String(), req.WorkflowRunId)
	if err != nil {
		return nil, handleUseCaseErr(err, s.log)
	}

	return &cpAPI.AttestationStateServiceInitializedResponse{
		Result: &cpAPI.AttestationStateServiceInitializedResponse_Result{
			Initialized: initialized,
		}}, nil
}

func (s *AttestationStateService) Save(ctx context.Context, req *cpAPI.AttestationStateServiceSaveRequest) (*cpAPI.AttestationStateServiceSaveResponse, error) {
	robotAccount := usercontext.CurrentAPIToken(ctx)
	if robotAccount == nil {
		return nil, errors.NotFound("not found", "robot account not found")
	}

	wf, err := s.findWorkflowFromTokenOrRunID(ctx, robotAccount.OrgID, robotAccount.WorkflowID, req.GetWorkflowRunId())
	if err != nil {
		return nil, handleUseCaseErr(err, s.log)
	}

	encryptionPassphrase, err := encryptionPassphrase(ctx)
	if err != nil {
		return nil, errors.Forbidden("forbidden", "failed to authenticate request")
	}

	err = s.attestationStateUseCase.Save(ctx, wf.ID.String(), req.WorkflowRunId, req.AttestationState, encryptionPassphrase, biz.WithAttStateBaseDigest(req.GetBaseDigest()))
	if err != nil {
		if biz.IsErrAttestationStateConflict(err) {
			return nil, cpAPI.ErrorAttestationStateErrorConflict("saving attestation: %s", err.Error())
		}

		return nil, handleUseCaseErr(err, s.log)
	}

	return &cpAPI.AttestationStateServiceSaveResponse{}, nil
}

func (s *AttestationStateService) Read(ctx context.Context, req *cpAPI.AttestationStateServiceReadRequest) (*cpAPI.AttestationStateServiceReadResponse, error) {
	robotAccount := usercontext.CurrentAPIToken(ctx)
	if robotAccount == nil {
		return nil, errors.NotFound("not found", "robot account not found")
	}

	wf, err := s.findWorkflowFromTokenOrRunID(ctx, robotAccount.OrgID, robotAccount.WorkflowID, req.GetWorkflowRunId())
	if err != nil {
		return nil, handleUseCaseErr(err, s.log)
	}

	encryptionPassphrase, err := encryptionPassphrase(ctx)
	if err != nil {
		return nil, errors.Forbidden("forbidden", "failed to authenticate request")
	}

	state, err := s.attestationStateUseCase.Read(ctx, wf.ID.String(), req.WorkflowRunId, encryptionPassphrase)
	if err != nil {
		return nil, handleUseCaseErr(err, s.log)
	}

	return &cpAPI.AttestationStateServiceReadResponse{
		Result: &cpAPI.AttestationStateServiceReadResponse_Result{
			AttestationState: state.State,
			Digest:           state.Digest,
		},
	}, nil
}

func (s *AttestationStateService) Reset(ctx context.Context, req *cpAPI.AttestationStateServiceResetRequest) (*cpAPI.AttestationStateServiceResetResponse, error) {
	robotAccount := usercontext.CurrentAPIToken(ctx)
	if robotAccount == nil {
		return nil, errors.NotFound("not found", "robot account not found")
	}

	wf, err := s.findWorkflowFromTokenOrRunID(ctx, robotAccount.OrgID, robotAccount.WorkflowID, req.GetWorkflowRunId())
	if err != nil {
		return nil, handleUseCaseErr(err, s.log)
	}

	if err := s.attestationStateUseCase.Reset(ctx, wf.ID.String(), req.WorkflowRunId); err != nil {
		return nil, handleUseCaseErr(err, s.log)
	}

	return &cpAPI.AttestationStateServiceResetResponse{}, nil
}

// In order to encrypt the state at rest, we will encrypt the state using a passphrase
// which comes in the request and is used to derive an encryption key.
// The passphrase that we'll use is the robot-account token that only the workload should have.
// NOTE: Using the robot-account as JWT is not ideal but it's a start
// TODO: look into using some identifier from the actual client like machine-uuid
func encryptionPassphrase(ctx context.Context) (string, error) {
	robotAccount := usercontext.CurrentAPIToken(ctx)
	if robotAccount == nil {
		return "", errors.NotFound("not found", "robot account not found")
		// If we are using a federated provider, we'll use the provider key as the passphrase since we can not guarantee the stability of the token
		// In practice this means disabling the state encryption at rest but this state in practice is a subset of the resulting attestation that we end up storing
	} else if robotAccount.ProviderKey == multijwtmiddleware.FederatedProviderKey {
		return robotAccount.ProviderKey, nil
	}

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

// Cascade-based way of retrieving the workflow from the robot-account or the run_ID
func (s *AttestationStateService) findWorkflowFromTokenOrRunID(ctx context.Context, orgID string, workflowID, runID string) (*biz.Workflow, error) {
	if orgID == "" {
		return nil, biz.NewErrValidationStr("orgID must be provided")
	}

	// This is the case of the workflowID encoded in the robot account
	if workflowID != "" {
		return s.workflowUseCase.FindByIDInOrg(ctx, orgID, workflowID)
	}

	// This is the case when the workflow is found by its reference to the run
	if runID != "" {
		run, err := s.wrUseCase.GetByIDInOrg(ctx, orgID, runID)
		if err != nil {
			return nil, fmt.Errorf("error retrieving the workflow run: %w", err)
		}

		return run.Workflow, nil
	}

	return nil, biz.NewErrValidationStr("workflowRunId must be provided")
}
