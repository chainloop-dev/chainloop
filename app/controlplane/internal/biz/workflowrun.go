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
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/chainloop-dev/chainloop/app/controlplane/internal/pagination"
	"github.com/secure-systems-lab/go-securesystemslib/dsse"

	"github.com/go-kratos/kratos/v2/log"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/uuid"
)

type WorkflowRun struct {
	ID                    uuid.UUID
	State, Reason         string
	CreatedAt, FinishedAt *time.Time
	Workflow              *Workflow
	RunURL, RunnerType    string
	ContractVersionID     uuid.UUID
	Attestation           *Attestation
	CASBackends           []*CASBackend
	// The revision of the contract that was used
	ContractRevisionUsed int
	// The max revision of the contract at the time of the run
	ContractRevisionLatest int
}

type Attestation struct {
	Envelope *dsse.Envelope
	Digest   string
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
	Create(ctx context.Context, opts *WorkflowRunRepoCreateOpts) (*WorkflowRun, error)
	FindByID(ctx context.Context, ID uuid.UUID) (*WorkflowRun, error)
	FindByAttestationDigest(ctx context.Context, digest string) (*WorkflowRun, error)
	FindByIDInOrg(ctx context.Context, orgID, ID uuid.UUID) (*WorkflowRun, error)
	MarkAsFinished(ctx context.Context, ID uuid.UUID, status WorkflowRunStatus, reason string) error
	SaveAttestation(ctx context.Context, ID uuid.UUID, att *dsse.Envelope, digest string) error
	List(ctx context.Context, orgID uuid.UUID, f *RunListFilters, p *pagination.CursorOptions) ([]*WorkflowRun, string, error)
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
	ContractRevision           *WorkflowContractWithVersion
	RunnerRunURL               string
	RunnerType                 string
	CASBackendID               uuid.UUID
}

type WorkflowRunRepoCreateOpts struct {
	WorkflowID, RobotaccountID, SchemaVersionID uuid.UUID
	RunURL, RunnerType                          string
	Backends                                    []uuid.UUID
	LatestRevision, UsedRevision                int
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

	if opts.CASBackendID == uuid.Nil {
		return nil, errors.New("CASBackendID cannot be nil")
	}

	if opts.ContractRevision == nil {
		return nil, errors.New("contract revision cannot be nil")
	}
	contractRevision := opts.ContractRevision

	// For now we only associate the workflow run to one backend.
	// This might change in the future so we prepare the data layer to support an array of associated backends
	backends := []uuid.UUID{opts.CASBackendID}
	run, err := uc.wfRunRepo.Create(ctx,
		&WorkflowRunRepoCreateOpts{
			WorkflowID:      workflowUUID,
			RobotaccountID:  robotaccountUUID,
			SchemaVersionID: contractRevision.Version.ID,
			RunURL:          opts.RunnerRunURL,
			RunnerType:      opts.RunnerType,
			Backends:        backends,
			LatestRevision:  contractRevision.Contract.LatestRevision,
			UsedRevision:    contractRevision.Version.Revision,
		})
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

func (uc *WorkflowRunUseCase) SaveAttestation(ctx context.Context, id string, envelope *dsse.Envelope) (string, error) {
	runID, err := uuid.Parse(id)
	if err != nil {
		return "", NewErrInvalidUUID(err)
	}

	// Calculate the digest
	_, digest, err := jsonEnvelopeWithDigest(envelope)
	if err != nil {
		return "", NewErrValidation(fmt.Errorf("marshaling the envelope: %w", err))
	}

	if err := uc.wfRunRepo.SaveAttestation(ctx, runID, envelope, digest.String()); err != nil {
		return "", fmt.Errorf("saving attestation: %w", err)
	}

	return digest.String(), nil
}

type RunListFilters struct {
	WorkflowID uuid.UUID
	Status     WorkflowRunStatus
}

// List the workflowruns associated with an org and optionally filtered by a workflow
func (uc *WorkflowRunUseCase) List(ctx context.Context, orgID string, f *RunListFilters, p *pagination.CursorOptions) ([]*WorkflowRun, string, error) {
	orgUUID, err := uuid.Parse(orgID)
	if err != nil {
		return nil, "", NewErrInvalidUUID(err)
	}

	return uc.wfRunRepo.List(ctx, orgUUID, f, p)
}

// Returns the workflow run with the provided ID if it belongs to the org or its public
func (uc *WorkflowRunUseCase) GetByIDInOrgOrPublic(ctx context.Context, orgID, runID string) (*WorkflowRun, error) {
	orgUUID, err := uuid.Parse(orgID)
	if err != nil {
		return nil, NewErrInvalidUUID(err)
	}

	runUUID, err := uuid.Parse(runID)
	if err != nil {
		return nil, NewErrInvalidUUID(err)
	}

	wfrun, err := uc.wfRunRepo.FindByID(ctx, runUUID)
	if err != nil {
		return nil, fmt.Errorf("finding workflow run: %w", err)
	}

	// If the workflow is public or belongs to the org we can return it
	return workflowRunInOrgOrPublic(wfrun, orgUUID)
}

func (uc *WorkflowRunUseCase) GetByDigestInOrgOrPublic(ctx context.Context, orgID, digest string) (*WorkflowRun, error) {
	orgUUID, err := uuid.Parse(orgID)
	if err != nil {
		return nil, NewErrInvalidUUID(err)
	}

	if _, err := v1.NewHash(digest); err != nil {
		return nil, NewErrValidation(fmt.Errorf("invalid digest format: %w", err))
	}

	wfrun, err := uc.wfRunRepo.FindByAttestationDigest(ctx, digest)
	if err != nil {
		return nil, fmt.Errorf("finding workflow run: %w", err)
	}

	// If the workflow is public or belongs to the org we can return it
	return workflowRunInOrgOrPublic(wfrun, orgUUID)
}

// filter the workflow runs that belong to the org or are public
func workflowRunInOrgOrPublic(wfRun *WorkflowRun, orgID uuid.UUID) (*WorkflowRun, error) {
	if wfRun == nil || (wfRun.Workflow.OrgID != orgID && !wfRun.Workflow.Public) {
		return nil, NewErrNotFound("workflow run")
	}

	return wfRun, nil
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
