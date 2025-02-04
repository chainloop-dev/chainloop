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

	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/pagination"
	"github.com/chainloop-dev/chainloop/pkg/attestation"
	"github.com/chainloop-dev/chainloop/pkg/attestation/renderer/chainloop"
	"github.com/secure-systems-lab/go-securesystemslib/dsse"
	protobundle "github.com/sigstore/protobuf-specs/gen/pb-go/bundle/v1"
	"google.golang.org/protobuf/encoding/protojson"

	schemaapi "github.com/chainloop-dev/chainloop/app/controlplane/api/workflowcontract/v1"
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
	Digest                string
	Bundle                *protobundle.Bundle
	CASBackends           []*CASBackend
	// The revision of the contract that was used
	ContractRevisionUsed int
	// The max revision of the contract at the time of the run
	ContractRevisionLatest int
	ProjectVersion         *ProjectVersion
}

type Attestation struct {
	Envelope *dsse.Envelope
	// Bundle digest, or envelope digest for old attestations
	Digest string
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
	SaveBundle(ctx context.Context, ID uuid.UUID, bundle []byte) error
	GetBundle(ctx context.Context, wrID uuid.UUID) ([]byte, error)
	List(ctx context.Context, orgID uuid.UUID, f *RunListFilters, p *pagination.CursorOptions) ([]*WorkflowRun, string, error)
	// List the runs that have not finished and are older than a given time
	ListNotFinishedOlderThan(ctx context.Context, olderThan time.Time, limit int) ([]*WorkflowRun, error)
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

type PromObservable interface {
	ObserveAttestationIfNeeded(ctx context.Context, run *WorkflowRun, status WorkflowRunStatus) bool
}

type WorkflowRunExpirerUseCase struct {
	wfRunRepo      WorkflowRunRepo
	PromObservable PromObservable
	logger         *log.Helper
}

type WorkflowRunExpirerOpts struct {
	// Maximum time threshold for what a workflowRun will be considered expired
	ExpirationWindow time.Duration
	CheckInterval    time.Duration
}

func NewWorkflowRunExpirerUseCase(wfrRepo WorkflowRunRepo, po PromObservable, logger log.Logger) *WorkflowRunExpirerUseCase {
	logger = log.With(logger, "component", "biz.WorkflowRunExpirer")
	return &WorkflowRunExpirerUseCase{wfrRepo, po, log.NewHelper(logger)}
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
				}
			}

			timer.Reset(opts.CheckInterval)
		}
	}()

	uc.logger.Infof("periodic check enabled. interval=%s, expirationWindow=%s", opts.CheckInterval, opts.ExpirationWindow)
}

// ExpirationSweep looks for runs older than the provider time and marks them as expired
func (uc *WorkflowRunExpirerUseCase) ExpirationSweep(ctx context.Context, olderThan time.Time) error {
	uc.logger.Debugf("expiration sweep - runs older than %s", olderThan.Format(time.RFC822))

	const maxNumberOfRunsToExpire = 100
	toExpire, err := uc.wfRunRepo.ListNotFinishedOlderThan(ctx, olderThan, maxNumberOfRunsToExpire)
	if err != nil {
		return err
	}

	for _, r := range toExpire {
		if err := uc.wfRunRepo.Expire(ctx, r.ID); err != nil {
			return err
		}

		// Record the attestation in the prometheus registry if applicable
		_ = uc.PromObservable.ObserveAttestationIfNeeded(ctx, r, WorkflowRunExpired)
		uc.logger.Debugf("run with id=%q createdAt=%q expired!\n", r.ID, r.CreatedAt.Format(time.RFC822))
	}

	return nil
}

type WorkflowRunCreateOpts struct {
	WorkflowID       string
	ContractRevision *WorkflowContractWithVersion
	RunnerRunURL     string
	RunnerType       string
	CASBackendID     uuid.UUID
	ProjectVersion   string
}

type WorkflowRunRepoCreateOpts struct {
	WorkflowID, SchemaVersionID  uuid.UUID
	RunURL, RunnerType           string
	Backends                     []uuid.UUID
	LatestRevision, UsedRevision int
	ProjectVersion               string
}

// Create will add a new WorkflowRun, associate it to a schemaVersion and increment the counter in the associated workflow
func (uc *WorkflowRunUseCase) Create(ctx context.Context, opts *WorkflowRunCreateOpts) (*WorkflowRun, error) {
	workflowUUID, err := uuid.Parse(opts.WorkflowID)
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

	if opts.ProjectVersion != "" {
		if err := ValidateVersion(opts.ProjectVersion); err != nil {
			return nil, err
		}
	}

	// For now we only associate the workflow run to one backend.
	// This might change in the future so we prepare the data layer to support an array of associated backends
	backends := []uuid.UUID{opts.CASBackendID}
	run, err := uc.wfRunRepo.Create(ctx,
		&WorkflowRunRepoCreateOpts{
			WorkflowID:      workflowUUID,
			SchemaVersionID: contractRevision.Version.ID,
			RunURL:          opts.RunnerRunURL,
			RunnerType:      opts.RunnerType,
			Backends:        backends,
			LatestRevision:  contractRevision.Contract.LatestRevision,
			UsedRevision:    contractRevision.Version.Revision,
			ProjectVersion:  opts.ProjectVersion,
		})
	if err != nil {
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

func (uc *WorkflowRunUseCase) SaveAttestation(ctx context.Context, id string, envelope *dsse.Envelope, bundle *protobundle.Bundle) (string, error) {
	runID, err := uuid.Parse(id)
	if err != nil {
		return "", NewErrInvalidUUID(err)
	}

	// extract statement to run some validations in the content
	predicate, err := chainloop.ExtractPredicate(envelope)
	if err != nil {
		return "", fmt.Errorf("extracting predicate: %w", err)
	}

	// Run some validations on the predicate
	// Attestations can include dependent attestations and we want to make sure they exist in the system
	// Find any material of kind attestation and make sure they exist already
	for _, m := range predicate.GetMaterials() {
		if m.Type == schemaapi.CraftingSchema_Material_ATTESTATION.String() {
			run, err := uc.wfRunRepo.FindByAttestationDigest(ctx, m.Hash.String())
			if err != nil {
				return "", fmt.Errorf("finding attestation: %w", err)
			} else if run == nil {
				return "", NewErrValidation(fmt.Errorf("dependent attestation not found: %s", m.Hash))
			}
		}
	}

	// Calculate the digest
	var digest v1.Hash

	// envelope digest
	_, digest, err = attestation.JSONEnvelopeWithDigest(envelope)
	if err != nil {
		return "", NewErrValidation(fmt.Errorf("marshaling the envelope: %w", err))
	}

	// Save bundle if provided, as it might come as an empty struct
	if bundle != nil && bundle.GetDsseEnvelope() != nil {
		var bundleBytes []byte
		// calculate digest from bundle instead
		bundleBytes, digest, err = attestation.JSONBundleWithDigest(bundle)
		if err != nil {
			return "", NewErrValidation(fmt.Errorf("marshaling the envelope: %w", err))
		}

		// Save bundle
		if err = uc.wfRunRepo.SaveBundle(ctx, runID, bundleBytes); err != nil {
			return "", fmt.Errorf("saving bundle: %w", err)
		}
	}

	if err := uc.wfRunRepo.SaveAttestation(ctx, runID, envelope, digest.String()); err != nil {
		return "", fmt.Errorf("saving attestation: %w", err)
	}

	return digest.String(), nil
}

type RunListFilters struct {
	WorkflowID *uuid.UUID
	VersionID  *uuid.UUID
	Status     WorkflowRunStatus
}

// List the workflowruns associated with an org and optionally filtered by a workflow
func (uc *WorkflowRunUseCase) List(ctx context.Context, orgID string, f *RunListFilters, p *pagination.CursorOptions) ([]*WorkflowRun, string, error) {
	orgUUID, err := uuid.Parse(orgID)
	if err != nil {
		return nil, "", NewErrInvalidUUID(err)
	}

	if f.WorkflowID != nil && f.VersionID != nil {
		return nil, "", NewErrValidation(errors.New("cannot filter by workflow and version at the same time"))
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

	// if available, add attestation from attestation bundles
	if err = uc.addAttestationFromBundle(ctx, wfrun); err != nil {
		return nil, fmt.Errorf("retrieving attestation from bundle: %w", err)
	}

	// If the workflow is public or belongs to the org we can return it
	return workflowRunInOrgOrPublic(wfrun, orgUUID)
}

// Returns the workflow run with the provided ID if it belongs to the org
func (uc *WorkflowRunUseCase) GetByIDInOrg(ctx context.Context, orgID, runID string) (*WorkflowRun, error) {
	orgUUID, err := uuid.Parse(orgID)
	if err != nil {
		return nil, NewErrInvalidUUID(err)
	}

	runUUID, err := uuid.Parse(runID)
	if err != nil {
		return nil, NewErrInvalidUUID(err)
	}

	wfRun, err := uc.wfRunRepo.FindByIDInOrg(ctx, orgUUID, runUUID)
	if err != nil {
		return nil, fmt.Errorf("finding workflow run: %w", err)
	}

	// if available, add attestation from attestation bundles
	if err = uc.addAttestationFromBundle(ctx, wfRun); err != nil {
		return nil, fmt.Errorf("retrieving attestation from bundle: %w", err)
	}

	return wfRun, nil
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

	// if available, add attestation from attestation bundles
	if err = uc.addAttestationFromBundle(ctx, wfrun); err != nil {
		return nil, fmt.Errorf("retrieving attestation from bundle: %w", err)
	}

	// If the workflow is public or belongs to the org we can return it
	return workflowRunInOrgOrPublic(wfrun, orgUUID)
}

func (uc *WorkflowRunUseCase) addAttestationFromBundle(ctx context.Context, wfRun *WorkflowRun) error {
	// missing workflow run or attestation already there, do nothing
	if wfRun == nil || wfRun.Attestation != nil || wfRun.State != string(WorkflowRunSuccess) {
		return nil
	}

	var bundle protobundle.Bundle
	bundleBytes, err := uc.wfRunRepo.GetBundle(ctx, wfRun.ID)
	if err != nil {
		return err
	}
	if err = protojson.Unmarshal(bundleBytes, &bundle); err != nil {
		return err
	}
	wfRun.Bundle = &bundle
	wfRun.Attestation = &Attestation{
		Envelope: attestation.DSSEEnvelopeFromBundle(&bundle),
		// the bundle digest
		Digest: wfRun.Digest,
	}
	return nil
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
