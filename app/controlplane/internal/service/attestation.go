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
	"encoding/json"
	"fmt"
	"sort"
	"time"

	"github.com/cenkalti/backoff/v4"
	cpAPI "github.com/chainloop-dev/chainloop/app/controlplane/api/controlplane/v1"
	"github.com/chainloop-dev/chainloop/app/controlplane/internal/biz"
	"github.com/chainloop-dev/chainloop/app/controlplane/internal/dispatcher"
	"github.com/chainloop-dev/chainloop/app/controlplane/internal/usercontext"
	"github.com/chainloop-dev/chainloop/internal/attestation/renderer/chainloop"
	"github.com/chainloop-dev/chainloop/internal/credentials"
	casJWT "github.com/chainloop-dev/chainloop/internal/robotaccount/cas"

	errors "github.com/go-kratos/kratos/v2/errors"
	"github.com/secure-systems-lab/go-securesystemslib/dsse"
)

type AttestationService struct {
	cpAPI.UnimplementedAttestationServiceServer
	*service

	wrUseCase               *biz.WorkflowRunUseCase
	workflowUseCase         *biz.WorkflowUseCase
	workflowContractUseCase *biz.WorkflowContractUseCase
	casUC                   *biz.CASBackendUseCase
	credsReader             credentials.Reader
	integrationUseCase      *biz.IntegrationUseCase
	integrationDispatcher   *dispatcher.FanOutDispatcher
	casCredsUseCase         *biz.CASCredentialsUseCase
	attestationUseCase      *biz.AttestationUseCase
	casMappingUseCase       *biz.CASMappingUseCase
	referrerUseCase         *biz.ReferrerUseCase
	orgUseCase              *biz.OrganizationUseCase
}

type NewAttestationServiceOpts struct {
	WorkflowRunUC      *biz.WorkflowRunUseCase
	WorkflowUC         *biz.WorkflowUseCase
	WorkflowContractUC *biz.WorkflowContractUseCase
	OCIUC              *biz.CASBackendUseCase
	CredsReader        credentials.Reader
	IntegrationUseCase *biz.IntegrationUseCase
	CasCredsUseCase    *biz.CASCredentialsUseCase
	AttestationUC      *biz.AttestationUseCase
	FanoutDispatcher   *dispatcher.FanOutDispatcher
	CASMappingUseCase  *biz.CASMappingUseCase
	ReferrerUC         *biz.ReferrerUseCase
	OrgUC              *biz.OrganizationUseCase
	Opts               []NewOpt
}

func NewAttestationService(opts *NewAttestationServiceOpts) *AttestationService {
	return &AttestationService{
		service:                 newService(opts.Opts...),
		wrUseCase:               opts.WorkflowRunUC,
		workflowUseCase:         opts.WorkflowUC,
		workflowContractUseCase: opts.WorkflowContractUC,
		casUC:                   opts.OCIUC,
		credsReader:             opts.CredsReader,
		integrationUseCase:      opts.IntegrationUseCase,
		casCredsUseCase:         opts.CasCredsUseCase,
		integrationDispatcher:   opts.FanoutDispatcher,
		attestationUseCase:      opts.AttestationUC,
		casMappingUseCase:       opts.CASMappingUseCase,
		referrerUseCase:         opts.ReferrerUC,
		orgUseCase:              opts.OrgUC,
	}
}

func (s *AttestationService) GetContract(ctx context.Context, req *cpAPI.AttestationServiceGetContractRequest) (*cpAPI.AttestationServiceGetContractResponse, error) {
	robotAccount, err := s.completeRobotAccount(ctx, req.GetWorkflowName())
	if err != nil {
		return nil, fmt.Errorf("%w", err)
	}

	// Find workflow
	wf, err := s.workflowUseCase.FindByID(ctx, robotAccount.WorkflowID)
	if err != nil {
		return nil, errors.NotFound("not found", "workflow not found")
	}

	// Find contract revision
	contractVersion, err := s.workflowContractUseCase.Describe(ctx, wf.OrgID.String(), wf.ContractID.String(), int(req.ContractRevision))
	if err != nil || contractVersion == nil {
		return nil, errors.NotFound("not found", "contract not found")
	}

	resp := &cpAPI.AttestationServiceGetContractResponse_Result{
		Workflow: bizWorkflowToPb(wf),
		Contract: bizWorkFlowContractVersionToPb(contractVersion.Version),
	}

	return &cpAPI.AttestationServiceGetContractResponse{Result: resp}, nil
}

func (s *AttestationService) Init(ctx context.Context, req *cpAPI.AttestationServiceInitRequest) (*cpAPI.AttestationServiceInitResponse, error) {
	robotAccount, err := s.completeRobotAccount(ctx, req.GetWorkflowName())
	if err != nil {
		return nil, fmt.Errorf("%w", err)
	}

	// Find workflow
	wf, err := s.workflowUseCase.FindByID(ctx, robotAccount.WorkflowID)
	if err != nil {
		return nil, errors.NotFound("not found", "workflow not found")
	}

	// Find contract revision
	contractVersion, err := s.workflowContractUseCase.Describe(ctx, wf.OrgID.String(), wf.ContractID.String(), int(req.ContractRevision))
	if err != nil || contractVersion == nil {
		return nil, errors.NotFound("not found", "contract not found")
	}

	// find the default CAS backend to associate the workflow
	backend, err := s.casUC.FindDefaultBackend(context.Background(), robotAccount.OrgID)
	if err != nil && !biz.IsNotFound(err) {
		return nil, fmt.Errorf("failed to find default CAS backend: %w", err)
	} else if err != nil {
		return nil, errors.NotFound("not found", "default CAS backend not found")
	}

	// Create workflowRun
	opts := &biz.WorkflowRunCreateOpts{
		WorkflowID:       robotAccount.WorkflowID,
		ContractRevision: contractVersion,
		RunnerRunURL:     req.GetJobUrl(),
		RunnerType:       req.GetRunner().String(),
		CASBackendID:     backend.ID,
	}

	run, err := s.wrUseCase.Create(ctx, opts)
	if err != nil {
		return nil, handleUseCaseErr(err, s.log)
	}

	// Find the organization
	org, err := s.orgUseCase.FindByID(ctx, robotAccount.OrgID)
	if err != nil {
		return nil, handleUseCaseErr(err, s.log)
	}

	wRun := bizWorkFlowRunToPb(run)
	wRun.Workflow = bizWorkflowToPb(wf)
	resp := &cpAPI.AttestationServiceInitResponse_Result{
		WorkflowRun:  wRun,
		Organization: org.Name,
	}

	return &cpAPI.AttestationServiceInitResponse{Result: resp}, nil
}

func (s *AttestationService) Store(ctx context.Context, req *cpAPI.AttestationServiceStoreRequest) (*cpAPI.AttestationServiceStoreResponse, error) {
	robotAccount, err := s.completeRobotAccount(ctx, req.GetWorkflowName())
	if err != nil {
		return nil, fmt.Errorf("%w", err)
	}

	// Check that provided workflowRun belongs to workflow encoded in the robot account
	if exists, err := s.wrUseCase.ExistsInWorkflow(ctx, robotAccount.WorkflowID, req.WorkflowRunId); err != nil || !exists {
		return nil, errors.NotFound("not found", "workflowRun not found")
	}

	// Decode the envelope through json encoding but
	// TODO: Verify the envelope signature before storing it
	// see sigstore's dsse signer/verifier helpers
	envelope := &dsse.Envelope{}
	if err := json.Unmarshal(req.Attestation, envelope); err != nil {
		return nil, handleUseCaseErr(err, s.log)
	}

	wRun, err := s.wrUseCase.GetByIDInOrgOrPublic(ctx, robotAccount.OrgID, req.WorkflowRunId)
	if err != nil {
		return nil, handleUseCaseErr(err, s.log)
	} else if wRun == nil {
		return nil, errors.NotFound("not found", "workflow run not found")
	}

	if len(wRun.CASBackends) == 0 {
		return nil, errors.NotFound("not found", "workflow run has no CAS backend")
	}

	// We currently only support one backend per workflowRun
	casBackend := wRun.CASBackends[0]

	// If we have an external CAS backend, we will push there the attestation
	if !casBackend.Inline {
		b := backoff.NewExponentialBackOff()
		b.MaxElapsedTime = 1 * time.Minute
		err := backoff.Retry(
			func() error {
				// reset context
				ctx := context.Background()
				d, err := s.attestationUseCase.UploadToCAS(ctx, envelope, casBackend, req.WorkflowRunId)
				if err != nil {
					return err
				}
				s.log.Infow("msg", "attestation uploaded to CAS", "digest", d.String(), "runID", req.WorkflowRunId)
				return nil
			}, b)

		if err != nil {
			_ = handleUseCaseErr(err, s.log)
		}
	}

	// Store the attestation including the digest in the CAS backend (if exists)
	digest, err := s.wrUseCase.SaveAttestation(ctx, req.WorkflowRunId, envelope)
	if err != nil {
		return nil, handleUseCaseErr(err, s.log)
	}

	// Store the exploded attestation referrer information in the DB
	if err := s.referrerUseCase.ExtractAndPersist(ctx, envelope, robotAccount.WorkflowID); err != nil {
		return nil, handleUseCaseErr(err, s.log)
	}

	if !casBackend.Inline {
		// Store the mappings in the DB
		references, err := s.casMappingUseCase.LookupDigestsInAttestation(envelope)
		if err != nil {
			return nil, handleUseCaseErr(err, s.log)
		}

		for _, ref := range references {
			s.log.Infow("msg", "creating CAS mapping", "name", ref.Name, "digest", ref.Digest, "workflowRun", req.WorkflowRunId, "casBackend", casBackend.ID.String())
			if _, err := s.casMappingUseCase.Create(ctx, ref.Digest, casBackend.ID.String(), req.WorkflowRunId); err != nil {
				return nil, handleUseCaseErr(err, s.log)
			}
		}
	}

	secretName := casBackend.SecretName

	// Run integrations dispatcher
	go func() {
		if err := s.integrationDispatcher.Run(context.TODO(), &dispatcher.RunOpts{
			Envelope: envelope, OrgID: robotAccount.OrgID, WorkflowID: robotAccount.WorkflowID,
			DownloadBackendType: string(casBackend.Provider),
			DownloadSecretName:  secretName,
			WorkflowRunID:       req.WorkflowRunId,
		}); err != nil {
			_ = handleUseCaseErr(err, s.log)
		}
	}()

	if err := s.wrUseCase.MarkAsFinished(ctx, req.WorkflowRunId, biz.WorkflowRunSuccess, ""); err != nil {
		return nil, handleUseCaseErr(err, s.log)
	}

	return &cpAPI.AttestationServiceStoreResponse{
		Result: &cpAPI.AttestationServiceStoreResponse_Result{Digest: digest},
	}, nil
}

func (s *AttestationService) Cancel(ctx context.Context, req *cpAPI.AttestationServiceCancelRequest) (*cpAPI.AttestationServiceCancelResponse, error) {
	robotAccount, err := s.completeRobotAccount(ctx, req.GetWorkflowName())
	if err != nil {
		return nil, fmt.Errorf("%w", err)
	}

	// Check that provided workflowRun belongs to workflow encoded in the robot account
	if exists, err := s.wrUseCase.ExistsInWorkflow(ctx, robotAccount.WorkflowID, req.WorkflowRunId); err != nil || !exists {
		return nil, errors.NotFound("not found", "workflowRun not found")
	}

	var status biz.WorkflowRunStatus
	switch req.Trigger {
	case cpAPI.AttestationServiceCancelRequest_TRIGGER_TYPE_FAILURE:
		status = biz.WorkflowRunError
	case cpAPI.AttestationServiceCancelRequest_TRIGGER_TYPE_CANCELLATION:
		status = biz.WorkflowRunCancelled
	default:
		return nil, fmt.Errorf("invalid trigger %s", req.Trigger)
	}

	if err := s.wrUseCase.MarkAsFinished(ctx, req.WorkflowRunId, status, req.Reason); err != nil {
		return nil, err
	}

	return &cpAPI.AttestationServiceCancelResponse{}, nil
}

// There is another endpoint to get credentials via casCredentialsService.Get
// This one is kept since it leverages robot-accounts in the context of a workflow
func (s *AttestationService) GetUploadCreds(ctx context.Context, req *cpAPI.AttestationServiceGetUploadCredsRequest) (*cpAPI.AttestationServiceGetUploadCredsResponse, error) {
	robotAccount, err := s.completeRobotAccount(ctx, req.GetWorkflowName())
	if err != nil {
		return nil, fmt.Errorf("%w", err)
	}

	// Find workflow in DB to extract the organization
	wf, err := s.workflowUseCase.FindByID(ctx, robotAccount.WorkflowID)
	if err != nil {
		return nil, errors.NotFound("not found", "workflow not found")
	}

	// Find the CAS backend associated with this workflowRun, that's the one that will be used to upload the materials
	// NOTE: currently we only support one backend per workflowRun but this will change in the future

	// DEPRECATED: if no workflow run is provided, we use the default repository
	// Maintained for compatibility reasons with older versions of the CLI
	var backend *biz.CASBackend
	if req.WorkflowRunId == "" {
		s.log.Warn("DEPRECATED: using main repository to get upload creds")
		backend, err = s.casUC.FindDefaultBackend(ctx, wf.OrgID.String())
		if err != nil && !biz.IsNotFound(err) {
			return nil, handleUseCaseErr(err, s.log)
		} else if backend == nil {
			return nil, errors.NotFound("not found", "main repository not found")
		}
	} else {
		// This is the new mode, where the CAS backend ref is stored in the workflow run since initialization
		wRun, err := s.wrUseCase.GetByIDInOrgOrPublic(ctx, robotAccount.OrgID, req.WorkflowRunId)
		if err != nil {
			return nil, handleUseCaseErr(err, s.log)
		} else if wRun == nil {
			return nil, errors.NotFound("not found", "workflow run not found")
		}

		if len(wRun.CASBackends) == 0 {
			return nil, errors.NotFound("not found", "workflow run has no CAS backend")
		}

		s.log.Infow("msg", "generating upload credentials for CAS backend", "ID", wRun.CASBackends[0].ID, "name", wRun.CASBackends[0].Location, "workflowRun", req.WorkflowRunId)

		backend = wRun.CASBackends[0]
	}

	// Return the backend information and associated credentials (if applicable)
	resp := &cpAPI.AttestationServiceGetUploadCredsResponse_Result{Backend: bizCASBackendToPb(backend)}
	if backend.SecretName != "" {
		ref := &biz.CASCredsOpts{BackendType: string(backend.Provider), SecretPath: backend.SecretName, Role: casJWT.Uploader}
		t, err := s.casCredsUseCase.GenerateTemporaryCredentials(ref)
		if err != nil {
			return nil, handleUseCaseErr(err, s.log)
		}

		resp.Token = t
	}

	return &cpAPI.AttestationServiceGetUploadCredsResponse{Result: resp}, nil
}

// completeRobotAccount auxiliary method to complete a RobotAccount instance. This is needed since when coming from
// an API Token request, the only information we have set is the workflow name not the ID
func (s *AttestationService) completeRobotAccount(ctx context.Context, workflowName string) (*usercontext.RobotAccount, error) {
	robotAccount := usercontext.CurrentRobotAccount(ctx)
	if robotAccount == nil {
		return nil, errors.NotFound("not found", "neither robot account nor API token found")
	}

	// If WorkflowID is not set and no workflowName is provided, return BadRequest.
	if workflowName == "" && robotAccount.WorkflowID == "" {
		return nil, errors.BadRequest("bad request", "when using an API Token, workflow name is required as parameter")
	}

	// If WorkflowID is not set, retrieve it using the workflow name.
	if robotAccount.WorkflowID == "" {
		workflow, err := s.workflowUseCase.FindByNameInOrg(ctx, robotAccount.OrgID, workflowName)
		if err != nil {
			return nil, fmt.Errorf("error retrieving the workflow: %w", err)
		}
		robotAccount.WorkflowID = workflow.ID.String()
	}

	return robotAccount, nil
}

func bizAttestationToPb(att *biz.Attestation) (*cpAPI.AttestationItem, error) {
	if att == nil || att.Envelope == nil {
		return nil, nil
	}

	encodedAttestation, err := json.Marshal(att.Envelope)
	if err != nil {
		return nil, err
	}

	predicate, err := chainloop.ExtractPredicate(att.Envelope)
	if err != nil {
		return nil, fmt.Errorf("error extracting predicate from attestation: %w", err)
	}

	materials, err := extractMaterials(predicate.GetMaterials())
	if err != nil {
		return nil, fmt.Errorf("error extracting materials from attestation: %w", err)
	}

	return &cpAPI.AttestationItem{
		Envelope:           encodedAttestation,
		EnvVars:            extractEnvVariables(predicate.GetEnvVars()),
		DigestInCasBackend: att.Digest,
		Materials:          materials,
		Annotations:        predicate.GetAnnotations(),
	}, nil
}

func extractEnvVariables(in map[string]string) []*cpAPI.AttestationItem_EnvVariable {
	res := make([]*cpAPI.AttestationItem_EnvVariable, 0, len(in))
	for k, v := range in {
		res = append(res, &cpAPI.AttestationItem_EnvVariable{Name: k, Value: v})
	}

	// Sort result
	sort.Slice(res, func(i, j int) bool {
		return res[i].Name < res[j].Name
	})

	return res
}

func extractMaterials(in []*chainloop.NormalizedMaterial) ([]*cpAPI.AttestationItem_Material, error) {
	res := make([]*cpAPI.AttestationItem_Material, 0, len(in))
	for _, m := range in {
		materialItem := &cpAPI.AttestationItem_Material{
			Name:           m.Name,
			Type:           m.Type,
			Filename:       m.Filename,
			Annotations:    m.Annotations,
			Value:          m.Value,
			UploadedToCas:  m.UploadedToCAS,
			EmbeddedInline: m.EmbeddedInline,
		}

		if m.Hash != nil {
			materialItem.Hash = m.Hash.String()
		}

		res = append(res, materialItem)
	}

	return res, nil
}
