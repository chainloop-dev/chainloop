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
	"errors"
	"io"
	"time"

	"github.com/chainloop-dev/chainloop/app/controlplane/internal/pagination"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/google/uuid"
)

type WorkflowRun struct {
	ID                    uuid.UUID
	State, Reason         string
	CreatedAt, FinishedAt *time.Time
	Workflow              *Workflow
	AttestationID         uuid.UUID
	RunURL, RunnerType    string
	ContractVersionID     uuid.UUID
	AttestationRef        *AttestationRef
}

type WorkflowRunWithContract struct {
	*WorkflowRun
	*WorkflowContractVersion
}

type WorkflowRunStatus string

const (
	WorkflowRunInitialized WorkflowRunStatus = "initialized"
	WorkflowRunSuccess     WorkflowRunStatus = "success"
	WorkflowRunError       WorkflowRunStatus = "error"
	WorkflowRunExpired     WorkflowRunStatus = "expired"
	WorkflowRunCancelled   WorkflowRunStatus = "canceled"
)

type WorkflowRunRepo interface {
	Create(ctx context.Context, workflowID, robotaccountID, contractVersion uuid.UUID, runURL, runnerType string) (*WorkflowRun, error)
	FindByID(ctx context.Context, ID uuid.UUID) (*WorkflowRun, error)
	FindByIDInOrg(ctx context.Context, orgID, ID uuid.UUID) (*WorkflowRun, error)
	MarkAsFinished(ctx context.Context, ID uuid.UUID, status WorkflowRunStatus, reason string) error
	SaveAttestationRef(ctx context.Context, ID uuid.UUID, ref *AttestationRef) error
	List(ctx context.Context, orgID, workflowID uuid.UUID, p *pagination.Options) ([]*WorkflowRun, string, error)
	// List the runs that have not finished and are older than a given time
	ListNotFinishedOlderThan(ctx context.Context, olderThan time.Time) ([]*WorkflowRun, error)
	// Set run as expired
	Expire(ctx context.Context, id uuid.UUID) error
}

type WorkflowRunUseCase struct {
	wfRunRepo WorkflowRunRepo
	wfRepo    WorkflowRepo
	logger    *log.Helper
}

func NewWorkflowRunUseCase(wfrRepo WorkflowRunRepo, wfRepo WorkflowRepo, logger log.Logger) (*WorkflowRunUseCase, error) {
	if logger == nil {
		logger = log.NewStdLogger(io.Discard)
	}

	return &WorkflowRunUseCase{
		wfRunRepo: wfrRepo, wfRepo: wfRepo,
		logger: log.NewHelper(logger),
	}, nil
}

type WorkflowRunExpirerUseCase struct {
	wfRunRepo WorkflowRunRepo
	logger    *log.Helper
}

type WorkflowRunExpirerOpts struct {
	// Maximum time threshold for what a workflowRun will be considered expired
	ExpirationWindow time.Duration
	CheckInterval    time.Duration
}

func NewWorkflowRunExpirerUseCase(wfrRepo WorkflowRunRepo, logger log.Logger) *WorkflowRunExpirerUseCase {
	logger = log.With(logger, "component", "biz.WorkflowRunExpirer")
	return &WorkflowRunExpirerUseCase{wfrRepo, log.NewHelper(logger)}
}

func (uc *WorkflowRunExpirerUseCase) Run(ctx context.Context, opts *WorkflowRunExpirerOpts) {
	timer := time.NewTimer(0)

	go func() {
		defer timer.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-timer.C:
				threshold := time.Now().Add(-opts.ExpirationWindow)

				if err := uc.ExpirationSweep(ctx, threshold); err != nil {
					uc.logger.Error(err)
					continue
				}
			}

			timer.Reset(opts.CheckInterval)
		}
	}()

	uc.logger.Infof("periodic check enabled. interval=%s, expirationWindow=%s", opts.CheckInterval, opts.ExpirationWindow)
}

// ExpirationSweep looks for runs older than the provider time and marks them as expired
func (uc *WorkflowRunExpirerUseCase) ExpirationSweep(ctx context.Context, olderThan time.Time) error {
	uc.logger.Infof("expiration sweep - runs older than %s", olderThan.Format(time.RFC822))

	toExpire, err := uc.wfRunRepo.ListNotFinishedOlderThan(ctx, olderThan)
	if err != nil {
		return err
	}

	for _, r := range toExpire {
		if err := uc.wfRunRepo.Expire(ctx, r.ID); err != nil {
			return err
		}
		uc.logger.Infof("run with id=%q createdAt=%q expired!\n", r.ID, r.CreatedAt.Format(time.RFC822))
	}

	return nil
}

type WorkflowRunCreateOpts struct {
	WorkflowID, RobotaccountID string
	ContractRevisionUUID       uuid.UUID
	RunnerRunURL               string
	RunnerType                 string
}

// Create will add a new WorkflowRun, associate it to a schemaVersion and increment the counter in the associated workflow
func (uc *WorkflowRunUseCase) Create(ctx context.Context, opts *WorkflowRunCreateOpts) (*WorkflowRun, error) {
	workflowUUID, err := uuid.Parse(opts.WorkflowID)
	if err != nil {
		return nil, err
	}

	robotaccountUUID, err := uuid.Parse(opts.RobotaccountID)
	if err != nil {
		return nil, err
	}

	run, err := uc.wfRunRepo.Create(ctx, workflowUUID, robotaccountUUID, opts.ContractRevisionUUID, opts.RunnerRunURL, opts.RunnerType)
	if err != nil {
		return nil, err
	}

	if err := uc.wfRepo.IncRunsCounter(ctx, workflowUUID); err != nil {
		return nil, err
	}

	return run, nil
}

// The workflowRun belongs to the provided workflowRun
func (uc *WorkflowRunUseCase) ExistsInWorkflow(ctx context.Context, workflowID, id string) (bool, error) {
	runUUID, err := uuid.Parse(id)
	if err != nil {
		return false, err
	}

	res, err := uc.wfRunRepo.FindByID(ctx, runUUID)
	if err != nil {
		return false, err
	}

	return res != nil && res.Workflow.ID.String() == workflowID, nil
}

func (uc *WorkflowRunUseCase) MarkAsFinished(ctx context.Context, id string, status WorkflowRunStatus, reason string) error {
	runID, err := uuid.Parse(id)
	if err != nil {
		return NewErrInvalidUUID(err)
	}

	return uc.wfRunRepo.MarkAsFinished(ctx, runID, status, reason)
}

// Store the attestation digest for the workflowrun
func (uc *WorkflowRunUseCase) AssociateAttestation(ctx context.Context, id string, ref *AttestationRef) error {
	if ref == nil || ref.SecretRef == "" || ref.Sha256 == "" {
		return NewErrValidation(errors.New("attestation ref is nil or invalid"))
	}

	runID, err := uuid.Parse(id)
	if err != nil {
		return NewErrInvalidUUID(err)
	}

	return uc.wfRunRepo.SaveAttestationRef(ctx, runID, ref)
}

// List the workflowruns associated with an org and optionally filtered by a workflow
func (uc *WorkflowRunUseCase) List(ctx context.Context, orgID, workflowID string, p *pagination.Options) ([]*WorkflowRun, string, error) {
	orgUUID, err := uuid.Parse(orgID)
	if err != nil {
		return nil, "", err
	}

	var workflowUUID uuid.UUID
	if workflowID != "" {
		workflowUUID, err = uuid.Parse(workflowID)
		if err != nil {
			return nil, "", err
		}
	}

	return uc.wfRunRepo.List(ctx, orgUUID, workflowUUID, p)
}

func (uc *WorkflowRunUseCase) View(ctx context.Context, orgID, runID string) (*WorkflowRun, error) {
	orgUUID, err := uuid.Parse(orgID)
	if err != nil {
		return nil, NewErrInvalidUUID(err)
	}

	runUUID, err := uuid.Parse(runID)
	if err != nil {
		return nil, NewErrInvalidUUID(err)
	}

	return uc.wfRunRepo.FindByIDInOrg(ctx, orgUUID, runUUID)
}

// Implements https://pkg.go.dev/entgo.io/ent/schema/field#EnumValues
func (WorkflowRunStatus) Values() (kinds []string) {
	for _, s := range []WorkflowRunStatus{
		WorkflowRunInitialized,
		WorkflowRunSuccess,
		WorkflowRunError,
		WorkflowRunExpired,
		WorkflowRunCancelled,
	} {
		kinds = append(kinds, string(s))
	}

	return
}
