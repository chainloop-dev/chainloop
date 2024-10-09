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
	"github.com/chainloop-dev/chainloop/app/controlplane/internal/dispatcher"
	"github.com/chainloop-dev/chainloop/app/controlplane/internal/usercontext"
	"github.com/chainloop-dev/chainloop/app/controlplane/internal/usercontext/attjwtmiddleware"
	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/biz"
	casJWT "github.com/chainloop-dev/chainloop/internal/robotaccount/cas"
	"github.com/chainloop-dev/chainloop/pkg/attestation/renderer/chainloop"
	"github.com/chainloop-dev/chainloop/pkg/credentials"

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
	prometheusUseCase       *biz.PrometheusUseCase
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
	PromUC             *biz.PrometheusUseCase
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
		prometheusUseCase:       opts.PromUC,
	}
}

func (s *AttestationService) GetContract(ctx context.Context, req *cpAPI.AttestationServiceGetContractRequest) (*cpAPI.AttestationServiceGetContractResponse, error) {
	robotAccount := usercontext.CurrentRobotAccount(ctx)
	if robotAccount == nil {
		return nil, errors.NotFound("not found", "neither robot account nor API token found")
	}

	if err := checkAuthRequirements(robotAccount, req.GetWorkflowName()); err != nil {
		return nil, err
	}

	wf, err := s.findWorkflowFromTokenOrNameOrRunID(ctx, robotAccount.OrgID, req.GetProjectName(), req.GetWorkflowName(), "")
	if err != nil {
		return nil, handleUseCaseErr(err, s.log)
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
	robotAccount := usercontext.CurrentRobotAccount(ctx)
	if robotAccount == nil {
		return nil, errors.NotFound("not found", "neither robot account nor API token found")
	}

	if err := checkAuthRequirements(robotAccount, req.GetWorkflowName()); err != nil {
		return nil, err
	}

	wf, err := s.findWorkflowFromTokenOrNameOrRunID(ctx, robotAccount.OrgID, req.GetProjectName(), req.GetWorkflowName(), "")
	if err != nil {
		return nil, handleUseCaseErr(err, s.log)
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
		WorkflowID:       wf.ID.String(),
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
	robotAccount := usercontext.CurrentRobotAccount(ctx)
	if robotAccount == nil {
		return nil, errors.NotFound("not found", "robot account not found")
	}

	// This will make sure the provided workflowRunID belongs to the org encoded in the robot account
	wf, err := s.findWorkflowFromTokenOrNameOrRunID(ctx, robotAccount.OrgID, "", "", req.WorkflowRunId)
	if err != nil {
		return nil, handleUseCaseErr(err, s.log)
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
	if err := s.referrerUseCase.ExtractAndPersist(ctx, envelope, wf.ID.String()); err != nil {
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
			Envelope: envelope, OrgID: robotAccount.OrgID, WorkflowID: wf.ID.String(),
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

	// Record the attestation in the prometheus registry
	_ = s.prometheusUseCase.ObserveAttestationIfNeeded(ctx, wRun, biz.WorkflowRunSuccess)

	return &cpAPI.AttestationServiceStoreResponse{
		Result: &cpAPI.AttestationServiceStoreResponse_Result{Digest: digest},
	}, nil
}

func (s *AttestationService) Cancel(ctx context.Context, req *cpAPI.AttestationServiceCancelRequest) (*cpAPI.AttestationServiceCancelResponse, error) {
	robotAccount := usercontext.CurrentRobotAccount(ctx)
	if robotAccount == nil {
		return nil, errors.NotFound("not found", "robot account not found")
	}

	// This will make sure the provided workflowRunID belongs to the org encoded in the robot account
	if _, err := s.findWorkflowFromTokenOrNameOrRunID(ctx, robotAccount.OrgID, "", "", req.WorkflowRunId); err != nil {
		return nil, handleUseCaseErr(err, s.log)
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

	wRun, err := s.wrUseCase.GetByIDInOrgOrPublic(ctx, robotAccount.OrgID, req.WorkflowRunId)
	if err != nil {
		return nil, handleUseCaseErr(err, s.log)
	} else if wRun == nil {
		return nil, errors.NotFound("not found", "workflow run not found")
	}

	// Record the attestation in the prometheus registry
	_ = s.prometheusUseCase.ObserveAttestationIfNeeded(ctx, wRun, status)

	return &cpAPI.AttestationServiceCancelResponse{}, nil
}

// There is another endpoint to get credentials via casCredentialsService.Get
// This one is kept since it leverages robot-accounts in the context of a workflow
func (s *AttestationService) GetUploadCreds(ctx context.Context, req *cpAPI.AttestationServiceGetUploadCredsRequest) (*cpAPI.AttestationServiceGetUploadCredsResponse, error) {
	robotAccount := usercontext.CurrentRobotAccount(ctx)
	if robotAccount == nil {
		return nil, errors.NotFound("not found", "robot account not found")
	}

	// Find the CAS backend associated with this workflowRun, that's the one that will be used to upload the materials
	// NOTE: currently we only support one backend per workflowRun but this will change in the future
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

	backend := wRun.CASBackends[0]
	s.log.Infow("msg", "generating upload credentials for CAS backend", "ID", wRun.CASBackends[0].ID, "name", wRun.CASBackends[0].Location, "workflowRun", req.WorkflowRunId)

	// Return the backend information and associated credentials (if applicable)
	resp := &cpAPI.AttestationServiceGetUploadCredsResponse_Result{Backend: bizCASBackendToPb(backend)}
	if backend.SecretName != "" {
		ref := &biz.CASCredsOpts{BackendType: string(backend.Provider), SecretPath: backend.SecretName, Role: casJWT.Uploader, MaxBytes: backend.Limits.MaxBytes}
		t, err := s.casCredsUseCase.GenerateTemporaryCredentials(ref)
		if err != nil {
			return nil, handleUseCaseErr(err, s.log)
		}

		resp.Token = t
	}

	return &cpAPI.AttestationServiceGetUploadCredsResponse{Result: resp}, nil
}

func (s *AttestationService) GetPolicy(ctx context.Context, req *cpAPI.AttestationServiceGetPolicyRequest) (*cpAPI.AttestationServiceGetPolicyResponse, error) {
	token, ok := attjwtmiddleware.FromJWTAuthContext(ctx)
	if !ok {
		return nil, errors.Forbidden("forbidden", "token not found")
	}

	remotePolicy, err := s.workflowContractUseCase.GetPolicy(req.GetProvider(), req.GetPolicyName(), req.GetOrgName(), token.Token)
	if err != nil {
		return nil, handleUseCaseErr(err, s.log)
	}

	return &cpAPI.AttestationServiceGetPolicyResponse{Policy: remotePolicy.Policy, Reference: &cpAPI.RemotePolicyReference{
		Url:    remotePolicy.ProviderRef.URL,
		Digest: remotePolicy.ProviderRef.Digest,
	}}, nil
}

func (s *AttestationService) GetPolicyGroup(ctx context.Context, req *cpAPI.AttestationServiceGetPolicyGroupRequest) (*cpAPI.AttestationServiceGetPolicyGroupResponse, error) {
	token, ok := attjwtmiddleware.FromJWTAuthContext(ctx)
	if !ok {
		return nil, errors.Forbidden("forbidden", "token not found")
	}

	remoteGroup, err := s.workflowContractUseCase.GetPolicyGroup(req.GetProvider(), req.GetGroupName(), req.GetOrgName(), token.Token)
	if err != nil {
		return nil, handleUseCaseErr(err, s.log)
	}

	return &cpAPI.AttestationServiceGetPolicyGroupResponse{Group: remoteGroup.PolicyGroup, Reference: &cpAPI.RemotePolicyReference{
		Url:    remoteGroup.ProviderRef.URL,
		Digest: remoteGroup.ProviderRef.Digest,
	}}, nil
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
		PolicyEvaluations:  extractPolicyEvaluations(predicate.GetPolicyEvaluations()),
	}, nil
}

// extract policy evaluations in form of a Go map of arrays, into a map of protobuf messages
// (needed to be added to the response message)
func extractPolicyEvaluations(in map[string][]*chainloop.PolicyEvaluation) map[string]*cpAPI.PolicyEvaluations {
	res := make(map[string]*cpAPI.PolicyEvaluations)
	for k, v := range in {
		evaluations := make([]*cpAPI.PolicyEvaluation, 0, len(v))
		for _, ev := range v {
			violations := make([]*cpAPI.PolicyViolation, 0, len(ev.Violations))
			for _, vi := range ev.Violations {
				violations = append(violations, &cpAPI.PolicyViolation{
					Subject: vi.Subject,
					Message: vi.Message,
				})
			}

			eval := &cpAPI.PolicyEvaluation{
				Name:         ev.Name,
				MaterialName: ev.MaterialName,
				Body:         ev.Body,
				Sources:      ev.Sources,
				Annotations:  ev.Annotations,
				Description:  ev.Description,
				With:         ev.With,
				Type:         ev.Type,
				Violations:   violations,
			}

			if ev.PolicyReference != nil {
				eval.PolicyReference = &cpAPI.PolicyReference{
					Name:   ev.PolicyReference.Name,
					Digest: ev.PolicyReference.Digest,
				}
			}

			evaluations = append(evaluations, eval)
		}

		res[k] = &cpAPI.PolicyEvaluations{
			Evaluations: evaluations,
		}
	}

	return res
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
			Tag:            m.Tag,
		}

		if m.Hash != nil {
			materialItem.Hash = m.Hash.String()
		}

		res = append(res, materialItem)
	}

	return res, nil
}

func (s *AttestationService) findWorkflowFromTokenOrNameOrRunID(ctx context.Context, orgID string, projectName, workflowName, runID string) (*biz.Workflow, error) {
	if orgID == "" {
		return nil, biz.NewErrValidationStr("orgID must be provided")
	}

	// This is the case when the workflow if found by name
	if workflowName != "" {
		return s.workflowUseCase.FindByNameInOrg(ctx, orgID, projectName, workflowName)
	}

	// This is the case when the workflow is found by its reference to the run
	if runID != "" {
		run, err := s.wrUseCase.GetByIDInOrg(ctx, orgID, runID)
		if err != nil {
			return nil, fmt.Errorf("error retrieving the workflow run: %w", err)
		}

		return run.Workflow, nil
	}

	return nil, biz.NewErrValidationStr("workflowName or workflowRunId must be provided")
}

func checkAuthRequirements(attToken *usercontext.RobotAccount, workflowName string) error {
	if attToken == nil {
		return errors.Forbidden("forbidden", "authentication not found")
	}

	// For API tokens we do not support explicit workflowName. It is inside the token
	if attToken.ProviderKey == attjwtmiddleware.APITokenProviderKey && workflowName == "" {
		return errors.BadRequest("bad request", "when using an API Token, workflow name is required as parameter")
	} else if attToken.ProviderKey == attjwtmiddleware.RobotAccountProviderKey && workflowName != "" {
		return errors.BadRequest("bad request", "workflow name is not compatible with robot-account based attestations")
	}

	return nil
}
