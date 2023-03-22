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
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"sync"
	"time"

	"github.com/cenkalti/backoff/v4"

	cpAPI "github.com/chainloop-dev/chainloop/app/controlplane/api/controlplane/v1"
	contractAPI "github.com/chainloop-dev/chainloop/app/controlplane/api/workflowcontract/v1"
	"github.com/chainloop-dev/chainloop/app/controlplane/internal/biz"
	"github.com/chainloop-dev/chainloop/app/controlplane/internal/integrations/dependencytrack"
	"github.com/chainloop-dev/chainloop/app/controlplane/internal/usercontext"
	"github.com/chainloop-dev/chainloop/internal/attestation/renderer"
	"github.com/chainloop-dev/chainloop/internal/blobmanager/oci"
	"github.com/chainloop-dev/chainloop/internal/credentials"
	casJWT "github.com/chainloop-dev/chainloop/internal/robotaccount/cas"
	sl "github.com/chainloop-dev/chainloop/internal/servicelogger"

	errors "github.com/go-kratos/kratos/v2/errors"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/secure-systems-lab/go-securesystemslib/dsse"
)

type AttestationService struct {
	cpAPI.UnimplementedAttestationServiceServer
	*service

	wrUseCase               *biz.WorkflowRunUseCase
	workflowUseCase         *biz.WorkflowUseCase
	workflowContractUseCase *biz.WorkflowContractUseCase
	ociUC                   *biz.OCIRepositoryUseCase
	attestationUseCase      *biz.AttestationUseCase
	credsReader             credentials.Reader
	integrationUseCase      *biz.IntegrationUseCase
	casCredsUseCase         *biz.CASCredentialsUseCase
}

type NewAttestationServiceOpts struct {
	WorkflowRunUC      *biz.WorkflowRunUseCase
	WorkflowUC         *biz.WorkflowUseCase
	WorkflowContractUC *biz.WorkflowContractUseCase
	OCIUC              *biz.OCIRepositoryUseCase
	AttestationUC      *biz.AttestationUseCase
	CredsReader        credentials.Reader
	IntegrationUseCase *biz.IntegrationUseCase
	CasCredsUseCase    *biz.CASCredentialsUseCase
	Opts               []NewOpt
}

func NewAttestationService(opts *NewAttestationServiceOpts) *AttestationService {
	return &AttestationService{
		service:                 newService(opts.Opts...),
		wrUseCase:               opts.WorkflowRunUC,
		workflowUseCase:         opts.WorkflowUC,
		attestationUseCase:      opts.AttestationUC,
		workflowContractUseCase: opts.WorkflowContractUC,
		ociUC:                   opts.OCIUC,
		credsReader:             opts.CredsReader,
		integrationUseCase:      opts.IntegrationUseCase,
		casCredsUseCase:         opts.CasCredsUseCase,
	}
}

func (s *AttestationService) GetContract(ctx context.Context, req *cpAPI.AttestationServiceGetContractRequest) (*cpAPI.AttestationServiceGetContractResponse, error) {
	robotAccount := usercontext.CurrentRobotAccount(ctx)
	if robotAccount == nil {
		return nil, errors.NotFound("not found", "robot account not found")
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
		Workflow: bizWorkFlowToPb(wf),
		Contract: bizWorkFlowContractVersionToPb(contractVersion.Version),
	}

	return &cpAPI.AttestationServiceGetContractResponse{Result: resp}, nil
}

func (s *AttestationService) Init(ctx context.Context, req *cpAPI.AttestationServiceInitRequest) (*cpAPI.AttestationServiceInitResponse, error) {
	robotAccount := usercontext.CurrentRobotAccount(ctx)
	if robotAccount == nil {
		return nil, errors.NotFound("not found", "robot account not found")
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

	// Create workflowRun
	opts := &biz.WorkflowRunCreateOpts{
		WorkflowID: robotAccount.WorkflowID, RobotaccountID: robotAccount.ID,
		ContractRevisionUUID: contractVersion.Version.ID,
		RunnerRunURL:         req.GetJobUrl(),
		RunnerType:           contractVersion.Version.BodyV1.GetRunner().GetType().String(),
	}
	run, err := s.wrUseCase.Create(ctx, opts)
	if err != nil {
		return nil, sl.LogAndMaskErr(err, s.log)
	}

	wRun := bizWorkFlowRunToPb(run)
	wRun.Workflow = bizWorkFlowToPb(wf)
	resp := &cpAPI.AttestationServiceInitResponse_Result{
		WorkflowRun: wRun,
	}

	return &cpAPI.AttestationServiceInitResponse{Result: resp}, nil
}

func (s *AttestationService) Store(ctx context.Context, req *cpAPI.AttestationServiceStoreRequest) (*cpAPI.AttestationServiceStoreResponse, error) {
	robotAccount := usercontext.CurrentRobotAccount(ctx)
	if robotAccount == nil {
		return nil, errors.NotFound("not found", "robot account not found")
	}

	// Check that provided workflowRun belongs to workflow encoded in the robot account
	if exists, err := s.wrUseCase.ExistsInWorkflow(ctx, robotAccount.WorkflowID, req.WorkflowRunId); err != nil || !exists {
		return nil, errors.NotFound("not found", "workflowRun not found")
	}

	// Decode the envelope through json encoding but
	// TODO: Verify the envelope signature before storing it
	// see sigstore's dsee signer/verifier helpers
	envelope := &dsse.Envelope{}
	if err := json.Unmarshal(req.Attestation, envelope); err != nil {
		return nil, sl.LogAndMaskErr(err, s.log)
	}

	// TODO: Move to event bus and background processing
	// https://github.com/chainloop-dev/chainloop/issues/396
	// Upload to OCI
	go func() {
		b := backoff.NewExponentialBackOff()
		b.MaxElapsedTime = 1 * time.Minute
		err := backoff.RetryNotify(
			func() error {
				// Reset context
				ctx := context.Background()
				repo, err := s.ociUC.FindMainRepo(ctx, robotAccount.OrgID)
				if err != nil {
					return backoff.Permanent(err)
				} else if repo == nil {
					return backoff.Permanent(errors.NotFound("not found", "main repository not found"))
				}

				digest, err := s.attestationUseCase.UploadToCAS(ctx, envelope, repo.SecretName, req.WorkflowRunId)
				if err != nil {
					return err
				}

				// associate the attestation stored in the CAS with the workflow run
				if err := s.wrUseCase.AssociateAttestation(ctx, req.WorkflowRunId, &biz.AttestationRef{Sha256: digest, SecretRef: repo.SecretName}); err != nil {
					return err
				}

				s.log.Infow("msg", "attestation associated", "digest", digest, "runID", req.WorkflowRunId)

				return s.wrUseCase.MarkAsFinished(ctx, req.WorkflowRunId, biz.WorkflowRunSuccess, "")
			},
			b,
			func(err error, delay time.Duration) {
				s.log.Warnf("error uploading attestation to CAS, retrying in %s - %s", delay, err)
			},
		)
		if err != nil {
			// Send a notification
			_ = sl.LogAndMaskErr(err, s.log)
		}
	}()

	// Upload to dependency track (if applicable)
	go func() {
		b := backoff.NewExponentialBackOff()
		b.MaxElapsedTime = 1 * time.Minute
		err := backoff.RetryNotify(
			func() error {
				opts := &uploadSBOMToDepTrackOpts{
					envelope: envelope,
					orgID:    robotAccount.OrgID, workflowID: robotAccount.WorkflowID,
					integrationUC: s.integrationUseCase, ociUC: s.ociUC, credsReader: s.credsReader, log: s.log,
				}
				return uploadSBOMsToDependencyTrack(opts)
			},
			b,
			func(_ error, delay time.Duration) {
				s.log.Warnf("error uploading SBOM to dependency-track, retrying in %s", delay)
			},
		)
		if err != nil {
			// Send a notification
			_ = sl.LogAndMaskErr(err, s.log)
		}
	}()

	return &cpAPI.AttestationServiceStoreResponse{}, nil
}

func (s *AttestationService) Cancel(ctx context.Context, req *cpAPI.AttestationServiceCancelRequest) (*cpAPI.AttestationServiceCancelResponse, error) {
	robotAccount := usercontext.CurrentRobotAccount(ctx)
	if robotAccount == nil {
		return nil, errors.NotFound("not found", "robot account not found")
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
func (s *AttestationService) GetUploadCreds(ctx context.Context, _ *cpAPI.AttestationServiceGetUploadCredsRequest) (*cpAPI.AttestationServiceGetUploadCredsResponse, error) {
	robotAccount := usercontext.CurrentRobotAccount(ctx)
	if robotAccount == nil {
		return nil, errors.NotFound("not found", "robot account not found")
	}

	// Find workflow in DB to extract the organization
	wf, err := s.workflowUseCase.FindByID(ctx, robotAccount.WorkflowID)
	if err != nil {
		return nil, errors.NotFound("not found", "workflow not found")
	}

	repo, err := s.ociUC.FindMainRepo(ctx, wf.OrgID.String())
	if err != nil {
		return nil, sl.LogAndMaskErr(err, s.log)
	}
	if repo == nil {
		return nil, errors.NotFound("not found", "main repository not found")
	}

	t, err := s.casCredsUseCase.GenerateTemporaryCredentials(repo.SecretName, casJWT.Uploader)
	if err != nil {
		return nil, sl.LogAndMaskErr(err, s.log)
	}

	return &cpAPI.AttestationServiceGetUploadCredsResponse{Result: &cpAPI.AttestationServiceGetUploadCredsResponse_Result{Token: t}}, nil
}

func bizAttestationToPb(att *biz.Attestation) (*cpAPI.AttestationItem, error) {
	encodedAttestation, err := json.Marshal(att.Envelope)
	if err != nil {
		return nil, err
	}

	predicate, err := renderer.ExtractPredicate(att.Envelope)
	if err != nil {
		return nil, err
	}

	return &cpAPI.AttestationItem{
		Envelope:  encodedAttestation,
		EnvVars:   extractEnvVariables(predicate.Env),
		Materials: extractMaterials(predicate.Materials),
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

func extractMaterials(in []*renderer.ChainloopProvenanceMaterial) []*cpAPI.AttestationItem_Material {
	res := make([]*cpAPI.AttestationItem_Material, 0, len(in))
	for _, m := range in {
		res = append(res, &cpAPI.AttestationItem_Material{Name: m.Name, Value: m.Material.String(), Type: m.Type})
	}
	return res
}

type uploadSBOMToDepTrackOpts struct {
	envelope          *dsse.Envelope
	orgID, workflowID string
	integrationUC     *biz.IntegrationUseCase
	ociUC             *biz.OCIRepositoryUseCase
	credsReader       credentials.Reader
	log               *log.Helper
}

func uploadSBOMsToDependencyTrack(opts *uploadSBOMToDepTrackOpts) error {
	ctx := context.Background()
	opts.log.Infow("msg", "looking for integration", "workflowID", opts.workflowID, "integration", biz.DependencyTrackKind)

	// List enabled integrations with this workflow
	attachments, err := opts.integrationUC.ListAttachments(ctx, opts.orgID, opts.workflowID)
	if err != nil {
		return err
	}

	// Load the ones about dependency track
	var depTrackIntegrations []*biz.IntegrationAndAttachment
	for _, at := range attachments {
		integration, err := opts.integrationUC.FindByIDInOrg(ctx, opts.orgID, at.IntegrationID.String())
		if err != nil {
			return err
		} else if integration == nil {
			continue
		}
		if integration.Kind == biz.DependencyTrackKind {
			depTrackIntegrations = append(depTrackIntegrations, &biz.IntegrationAndAttachment{Integration: integration, IntegrationAttachment: at})
		}
	}

	if len(depTrackIntegrations) == 0 {
		opts.log.Infow("msg", "no attached integrations", "workflowID", opts.workflowID, "integration", biz.DependencyTrackKind)
		return nil
	}

	predicate, err := renderer.ExtractPredicate(opts.envelope)
	if err != nil {
		return err
	}

	repo, err := opts.ociUC.FindMainRepo(ctx, opts.orgID)
	if err != nil {
		return err
	} else if repo == nil {
		return errors.NotFound("not found", "main repository not found")
	}

	backend, err := oci.NewBackendProvider(opts.credsReader).FromCredentials(ctx, repo.SecretName)
	if err != nil {
		return err
	}

	for _, m := range predicate.Materials {
		if m.Type != contractAPI.CraftingSchema_Material_SBOM_CYCLONEDX_JSON.String() {
			continue
		}

		buf := bytes.NewBuffer(nil)
		digest, ok := m.Material.SLSA.Digest["sha256"]
		if !ok {
			continue
		}

		opts.log.Infow("msg", "SBOM present, downloading", "workflowID", opts.workflowID, "integration", biz.DependencyTrackKind, "name", m.Name)
		// Download SBOM
		if err := backend.Download(ctx, buf, digest); err != nil {
			return err
		}
		opts.log.Infow("msg", "SBOM downloaded", "digest", digest, "workflowID", opts.workflowID, "integration", biz.DependencyTrackKind, "name", m.Name)

		// Run integrations with that sbom
		var wg sync.WaitGroup
		var errs = make(chan error)
		var wgDone = make(chan bool)

		for _, i := range depTrackIntegrations {
			wg.Add(1)
			b := backoff.NewExponentialBackOff()
			b.MaxElapsedTime = 10 * time.Second

			go func(i *biz.IntegrationAndAttachment) {
				defer wg.Done()
				err := backoff.RetryNotify(
					func() error {
						return doSendToDependencyTrack(ctx, opts.credsReader, opts.workflowID, buf, i, opts.log)
					},
					b,
					func(_ error, delay time.Duration) {
						opts.log.Warnw("msg", "error uploading SBOM", "retry", delay)
					},
				)
				if err != nil {
					errs <- err
					log.Error(err)
				}
			}(i)
		}

		go func() {
			wg.Wait()
			close(wgDone)
		}()

		select {
		case <-wgDone:
			break
		case err := <-errs:
			return err
		}
	}

	return nil
}

func doSendToDependencyTrack(ctx context.Context, credsReader credentials.Reader, workflowID string, sbom io.Reader, i *biz.IntegrationAndAttachment, log *log.Helper) error {
	integrationConfig := i.Integration.Config.GetDependencyTrack()
	attachmentConfig := i.IntegrationAttachment.Config.GetDependencyTrack()

	creds := &credentials.APICreds{}
	if err := credsReader.ReadCredentials(ctx, i.SecretName, creds); err != nil {
		return err
	}

	log.Infow("msg", "Sending SBOM to Dependency-Track",
		"host", integrationConfig.Domain,
		"projectID", attachmentConfig.GetProjectId(), "projectName", attachmentConfig.GetProjectName(),
		"workflowID", workflowID, "integration", biz.DependencyTrackKind,
	)

	d, err := dependencytrack.NewSBOMUploader(integrationConfig.Domain, creds.Key, sbom, attachmentConfig.GetProjectId(), attachmentConfig.GetProjectName())
	if err != nil {
		return err
	}

	if err := d.Validate(ctx); err != nil {
		return err
	}

	if err := d.Do(ctx); err != nil {
		return err
	}

	log.Infow("msg", "SBOM Sent to Dependency-Track",
		"host", integrationConfig.Domain,
		"projectID", attachmentConfig.GetProjectId(), "projectName", attachmentConfig.GetProjectName(),
		"workflowID", workflowID, "integration", biz.DependencyTrackKind,
	)

	return nil
}
