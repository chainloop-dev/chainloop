//
// Copyright 2024-2026 The Chainloop Authors.
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
	"fmt"
	"slices"

	pb "github.com/chainloop-dev/chainloop/app/controlplane/api/controlplane/v1"
	craftingpb "github.com/chainloop-dev/chainloop/app/controlplane/api/workflowcontract/v1"
	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/authz"
	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/biz"
	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/pagination"
	chainloop "github.com/chainloop-dev/chainloop/pkg/attestation/renderer/chainloop"
	"github.com/chainloop-dev/chainloop/pkg/cache/policyevalbundle"
	"github.com/chainloop-dev/chainloop/pkg/credentials"
	errors "github.com/go-kratos/kratos/v2/errors"
	"github.com/google/uuid"
	intoto "github.com/in-toto/attestation/go/v1"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type WorkflowRunService struct {
	pb.UnimplementedWorkflowRunServiceServer
	*service

	wrUseCase               *biz.WorkflowRunUseCase
	workflowUseCase         *biz.WorkflowUseCase
	workflowContractUseCase *biz.WorkflowContractUseCase
	projectUseCase          *biz.ProjectUseCase
	credsReader             credentials.Reader
	casClient               biz.CASClient
	casMappingUC            *biz.CASMappingUseCase
	policyEvalCache         *policyevalbundle.Cache
}

type NewWorkflowRunServiceOpts struct {
	WorkflowRunUC      *biz.WorkflowRunUseCase
	WorkflowUC         *biz.WorkflowUseCase
	WorkflowContractUC *biz.WorkflowContractUseCase
	ProjectUC          *biz.ProjectUseCase
	CredsReader        credentials.Reader
	CASClient          biz.CASClient
	CASMappingUC       *biz.CASMappingUseCase
	PolicyEvalCache    *policyevalbundle.Cache
	Opts               []NewOpt
}

func NewWorkflowRunService(opts *NewWorkflowRunServiceOpts) *WorkflowRunService {
	return &WorkflowRunService{
		service:                 newService(opts.Opts...),
		wrUseCase:               opts.WorkflowRunUC,
		workflowUseCase:         opts.WorkflowUC,
		workflowContractUseCase: opts.WorkflowContractUC,
		projectUseCase:          opts.ProjectUC,
		credsReader:             opts.CredsReader,
		casClient:               opts.CASClient,
		casMappingUC:            opts.CASMappingUC,
		policyEvalCache:         opts.PolicyEvalCache,
	}
}

type casResolvedPredicate struct {
	chainloop.NormalizablePredicate
	evals map[string][]*chainloop.PolicyEvaluation
}

func (p *casResolvedPredicate) GetPolicyEvaluations() map[string][]*chainloop.PolicyEvaluation {
	return p.evals
}

func (s *WorkflowRunService) resolvePolicyEvaluations(
	ctx context.Context,
	ref *intoto.ResourceDescriptor,
	orgID uuid.UUID,
) (map[string][]*chainloop.PolicyEvaluation, error) {
	if ref == nil {
		return nil, nil
	}

	hexDigest, ok := ref.Digest["sha256"]
	if !ok {
		return nil, fmt.Errorf("no sha256 digest in policy evaluations ref")
	}
	digest := fmt.Sprintf("sha256:%s", hexDigest)

	if cached, found, err := s.policyEvalCache.Get(ctx, digest); err == nil && found {
		return chainloop.PolicyEvaluationsFromBundle(cached)
	}

	mapping, err := s.casMappingUC.FindCASMappingForDownloadByOrg(ctx, digest, []uuid.UUID{orgID}, nil)
	if err != nil {
		return nil, fmt.Errorf("finding CAS mapping: %w", err)
	}

	var buf bytes.Buffer
	if err := s.casClient.Download(ctx, string(mapping.CASBackend.Provider), mapping.CASBackend.SecretName, &buf, digest); err != nil {
		return nil, fmt.Errorf("downloading policy eval bundle: %w", err)
	}

	data := buf.Bytes()
	_ = s.policyEvalCache.Set(ctx, digest, data)

	return chainloop.PolicyEvaluationsFromBundle(data)
}

func (s *WorkflowRunService) List(ctx context.Context, req *pb.WorkflowRunServiceListRequest) (*pb.WorkflowRunServiceListResponse, error) {
	currentOrg, err := requireCurrentOrg(ctx)
	if err != nil {
		return nil, err
	}

	// Configure filters
	filters := &biz.RunListFilters{}

	// Apply RBAC if needed
	visibleProjectIDs := s.visibleProjects(ctx)
	filters.ProjectIDs = visibleProjectIDs

	// by workflow and project name
	if req.GetWorkflowName() != "" && req.GetProjectName() != "" {
		wf, err := s.workflowUseCase.FindByNameInOrg(ctx, currentOrg.ID, req.GetProjectName(), req.GetWorkflowName())
		if err != nil {
			return nil, handleUseCaseErr(err, s.log)
		} else if wf == nil {
			return nil, errors.NotFound("not found", "workflow not found")
		}

		filters.WorkflowID = &wf.ID
	} else if req.GetProjectName() != "" {
		// by project name only
		projectID, err := s.validateAndGetProjectID(ctx, currentOrg.ID, req.GetProjectName(), visibleProjectIDs)
		if err != nil {
			return nil, handleUseCaseErr(err, s.log)
		}

		// Override the filter to only include this specific project
		filters.ProjectIDs = []uuid.UUID{projectID}
	}

	if req.GetProjectVersion() != "" {
		projectUUID, err := uuid.Parse(req.GetProjectVersion())
		if err != nil {
			return nil, errors.BadRequest("invalid", "invalid project version")
		}

		filters.VersionID = &projectUUID
	}

	// by run status
	if req.GetStatus() != pb.RunStatus_RUN_STATUS_UNSPECIFIED {
		st, err := pbWorkflowRunStatusToBiz(req.GetStatus())
		if err != nil {
			return nil, errors.BadRequest("invalid run status", err.Error())
		}
		filters.Status = st
	}

	// by policy violations status (legacy, coarse — kept for back-compat)
	//nolint:staticcheck // honoring the deprecated field for older clients
	if req.GetPolicyViolations() != pb.PolicyViolationsFilter_POLICY_VIOLATIONS_FILTER_UNSPECIFIED {
		//nolint:staticcheck // honoring the deprecated field for older clients
		hasViolations := req.GetPolicyViolations() == pb.PolicyViolationsFilter_POLICY_VIOLATIONS_FILTER_WITH_VIOLATIONS
		filters.PolicyViolationsFilter = &hasViolations
	}

	// by canonical policy status — takes precedence over policy_violations
	// when both are set (documented on the request message).
	if req.GetPolicyStatus() != pb.PolicyStatusFilter_POLICY_STATUS_FILTER_UNSPECIFIED {
		s := pbPolicyStatusFilterToBiz(req.GetPolicyStatus())
		filters.PolicyStatus = &s
	}

	if req.GetPolicyGates() != pb.PolicyGatesFilter_POLICY_GATES_FILTER_UNSPECIFIED {
		hasGates := req.GetPolicyGates() == pb.PolicyGatesFilter_POLICY_GATES_FILTER_WITH_GATES
		filters.PolicyHasGates = &hasGates
	}

	p := req.GetPagination()
	paginationOpts, err := pagination.NewCursor(p.GetCursor(), int(p.GetLimit()))
	if err != nil {
		return nil, errors.InternalServer("invalid", "invalid pagination options")
	}

	workflowRuns, nextCursor, err := s.wrUseCase.List(ctx, currentOrg.ID, filters, paginationOpts)
	if err != nil {
		return nil, handleUseCaseErr(err, s.log)
	}

	result := make([]*pb.WorkflowRunItem, 0, len(workflowRuns))
	for _, wr := range workflowRuns {
		wrResp := bizWorkFlowRunToPb(wr)
		wrResp.Workflow = bizWorkflowToPb(wr.Workflow)
		result = append(result, wrResp)
	}

	return &pb.WorkflowRunServiceListResponse{Result: result, Pagination: bizCursorToPb(nextCursor)}, nil
}

func bizCursorToPb(cursor string) *pb.CursorPaginationResponse {
	return &pb.CursorPaginationResponse{NextCursor: cursor}
}

func (s *WorkflowRunService) View(ctx context.Context, req *pb.WorkflowRunServiceViewRequest) (*pb.WorkflowRunServiceViewResponse, error) {
	currentOrg, err := requireCurrentOrg(ctx)
	if err != nil {
		return nil, err
	}

	// retrieve the workflow run either by ID or by digest
	var run *biz.WorkflowRun
	switch {
	case req.GetId() != "":
		run, err = s.wrUseCase.GetByIDInOrgOrPublic(ctx, currentOrg.ID, req.GetId())
		if err != nil {
			return nil, handleUseCaseErr(err, s.log)
		}
	case req.GetDigest() != "":
		run, err = s.wrUseCase.GetByDigestInOrgOrPublic(ctx, currentOrg.ID, req.GetDigest())
		if err != nil {
			return nil, handleUseCaseErr(err, s.log)
		}
	default:
		return nil, errors.BadRequest("invalid", "id or digest required")
	}

	// Apply RBAC only if workflow is not public
	if !run.Workflow.Public {
		if err = s.authorizeResource(ctx, authz.PolicyWorkflowRunRead, authz.ResourceTypeProject, run.Workflow.ProjectID); err != nil {
			return nil, err
		}
	}

	var verificationResult *pb.WorkflowRunServiceViewResponse_VerificationResult
	if req.Verify {
		// it might be nil if it doesn't apply
		vr, err := s.wrUseCase.VerifyRun(ctx, run)
		if err != nil {
			return nil, handleUseCaseErr(err, s.log)
		}
		verificationResult = bizVerificationToPb(vr)
	}

	var predicate chainloop.NormalizablePredicate
	if run.Attestation != nil && run.Attestation.Envelope != nil {
		predicate, err = chainloop.ExtractPredicate(run.Attestation.Envelope)
		if err != nil {
			return nil, handleUseCaseErr(err, s.log)
		}

		if ref := predicate.GetPolicyEvaluationsRef(); ref != nil {
			resolved, resolveErr := s.resolvePolicyEvaluations(ctx, ref, run.Workflow.OrgID)
			if resolveErr != nil {
				s.log.Warnw("msg", "failed to resolve policy evaluations from CAS, using inline", "err", resolveErr)
			} else if resolved != nil {
				predicate = &casResolvedPredicate{NormalizablePredicate: predicate, evals: resolved}
			}
		}
	}

	attestation, err := bizAttestationToPb(run.Attestation, predicate)
	if err != nil {
		return nil, handleUseCaseErr(err, s.log)
	}

	contractAndVersion, err := s.workflowContractUseCase.FindVersionByID(ctx, run.ContractVersionID.String())
	if err != nil {
		return nil, handleUseCaseErr(err, s.log)
	}

	wr := bizWorkFlowRunToPb(run)
	wr.Workflow = bizWorkflowToPb(run.Workflow)
	wr.ContractVersion = bizWorkFlowContractVersionToPb(contractAndVersion.Version, contractAndVersion.Contract)
	wr.ContractVersion.ContractName = contractAndVersion.Contract.Name
	res := &pb.WorkflowRunServiceViewResponse_Result{
		OrgName:      currentOrg.Name,
		WorkflowRun:  wr,
		Attestation:  attestation,
		Verification: verificationResult,
	}

	return &pb.WorkflowRunServiceViewResponse{Result: res}, nil
}

func bizVerificationToPb(vr *biz.VerificationResult) *pb.WorkflowRunServiceViewResponse_VerificationResult {
	if vr == nil {
		return nil
	}
	return &pb.WorkflowRunServiceViewResponse_VerificationResult{
		Verified:      vr.Result,
		FailureReason: vr.FailureReason,
	}
}

func bizRunnerToPb(runner string) craftingpb.CraftingSchema_Runner_RunnerType {
	runnerType := craftingpb.CraftingSchema_Runner_RunnerType_value[runner]
	return craftingpb.CraftingSchema_Runner_RunnerType(runnerType)
}

func bizWorkFlowRunToPb(wfr *biz.WorkflowRun) *pb.WorkflowRunItem {
	item := &pb.WorkflowRunItem{
		Id:        wfr.ID.String(),
		CreatedAt: timestamppb.New(*wfr.CreatedAt),
		// state is deprecated
		State:                  wfr.State,
		Status:                 bizWorkflowRunStatusToPb(biz.WorkflowRunStatus(wfr.State)),
		Reason:                 wfr.Reason,
		JobUrl:                 wfr.RunURL,
		RunnerType:             bizRunnerToPb(wfr.RunnerType),
		ContractRevisionUsed:   int32(wfr.ContractRevisionUsed),
		ContractRevisionLatest: int32(wfr.ContractRevisionLatest),
		Version:                bizProjectVersionToPb(wfr.ProjectVersion),
		HasPolicyViolations:    wfr.HasPolicyViolations,
		PolicySummary:          bizPolicyStatusSummaryToPb(wfr.PolicyStatus),
	}

	if wfr.FinishedAt != nil {
		item.FinishedAt = timestamppb.New(*wfr.FinishedAt)
	}

	return item
}

// Transform pb run status to biz run status
func pbWorkflowRunStatusToBiz(st pb.RunStatus) (biz.WorkflowRunStatus, error) {
	m := map[pb.RunStatus]biz.WorkflowRunStatus{
		pb.RunStatus_RUN_STATUS_INITIALIZED: biz.WorkflowRunInitialized,
		pb.RunStatus_RUN_STATUS_SUCCEEDED:   biz.WorkflowRunSuccess,
		pb.RunStatus_RUN_STATUS_FAILED:      biz.WorkflowRunError,
		pb.RunStatus_RUN_STATUS_EXPIRED:     biz.WorkflowRunExpired,
		pb.RunStatus_RUN_STATUS_CANCELLED:   biz.WorkflowRunCancelled,
	}

	// not in the list
	if _, ok := m[st]; !ok {
		return "", fmt.Errorf("invalid run status: %s", st.String())
	}

	return m[st], nil
}

func bizProjectVersionToPb(v *biz.ProjectVersion) *pb.ProjectVersion {
	if v == nil {
		return nil
	}

	pv := &pb.ProjectVersion{
		Id:         v.ID.String(),
		Version:    v.Version,
		Prerelease: v.Prerelease,
	}

	if v.CreatedAt != nil {
		pv.CreatedAt = timestamppb.New(*v.CreatedAt)
	}

	if v.ReleasedAt != nil {
		pv.ReleasedAt = timestamppb.New(*v.ReleasedAt)
	}

	return pv
}

func bizWorkflowRunStatusToPb(st biz.WorkflowRunStatus) pb.RunStatus {
	m := map[biz.WorkflowRunStatus]pb.RunStatus{
		biz.WorkflowRunInitialized: pb.RunStatus_RUN_STATUS_INITIALIZED,
		biz.WorkflowRunSuccess:     pb.RunStatus_RUN_STATUS_SUCCEEDED,
		biz.WorkflowRunError:       pb.RunStatus_RUN_STATUS_FAILED,
		biz.WorkflowRunExpired:     pb.RunStatus_RUN_STATUS_EXPIRED,
		biz.WorkflowRunCancelled:   pb.RunStatus_RUN_STATUS_CANCELLED,
	}

	// not in the list
	if _, ok := m[st]; !ok {
		return pb.RunStatus_RUN_STATUS_UNSPECIFIED
	}

	return m[st]
}

// bizPolicyStatusSummaryToPb maps the domain summary onto its protobuf shape.
// Returns nil for nil input so rows predating the materialization change
// travel over the wire as "no policy_summary present" and clients can fall
// back to has_policy_violations.
func bizPolicyStatusSummaryToPb(s *chainloop.PolicyStatusSummary) *pb.PolicyStatusSummary {
	if s == nil {
		return nil
	}
	return &pb.PolicyStatusSummary{
		Status:   bizPolicyStatusToPb(s.Status),
		Total:    int32(s.Total),
		Passed:   int32(s.Passed),
		Skipped:  int32(s.Skipped),
		Violated: int32(s.Violated),
		HasGates: s.HasGates,
	}
}

func bizPolicyStatusToPb(s chainloop.PolicyStatus) pb.PolicyStatus {
	switch s {
	case chainloop.PolicyStatusNotApplicable:
		return pb.PolicyStatus_POLICY_STATUS_NOT_APPLICABLE
	case chainloop.PolicyStatusPassed:
		return pb.PolicyStatus_POLICY_STATUS_PASSED
	case chainloop.PolicyStatusSkipped:
		return pb.PolicyStatus_POLICY_STATUS_SKIPPED
	case chainloop.PolicyStatusWarning:
		return pb.PolicyStatus_POLICY_STATUS_WARNING
	case chainloop.PolicyStatusBlocked:
		return pb.PolicyStatus_POLICY_STATUS_BLOCKED
	case chainloop.PolicyStatusBypassed:
		return pb.PolicyStatus_POLICY_STATUS_BYPASSED
	default:
		return pb.PolicyStatus_POLICY_STATUS_UNSPECIFIED
	}
}

func pbPolicyStatusFilterToBiz(f pb.PolicyStatusFilter) chainloop.PolicyStatus {
	switch f {
	case pb.PolicyStatusFilter_POLICY_STATUS_FILTER_NOT_APPLICABLE:
		return chainloop.PolicyStatusNotApplicable
	case pb.PolicyStatusFilter_POLICY_STATUS_FILTER_PASSED:
		return chainloop.PolicyStatusPassed
	case pb.PolicyStatusFilter_POLICY_STATUS_FILTER_SKIPPED:
		return chainloop.PolicyStatusSkipped
	case pb.PolicyStatusFilter_POLICY_STATUS_FILTER_WARNING:
		return chainloop.PolicyStatusWarning
	case pb.PolicyStatusFilter_POLICY_STATUS_FILTER_BLOCKED:
		return chainloop.PolicyStatusBlocked
	case pb.PolicyStatusFilter_POLICY_STATUS_FILTER_BYPASSED:
		return chainloop.PolicyStatusBypassed
	default:
		return chainloop.PolicyStatusUnspecified
	}
}

// validateAndGetProjectID finds a project by name and verifies it's in the visible projects list
func (s *WorkflowRunService) validateAndGetProjectID(ctx context.Context, orgID, projectName string, visibleProjectIDs []uuid.UUID) (uuid.UUID, error) {
	project, err := s.projectUseCase.FindProjectByReference(ctx, orgID, &biz.IdentityReference{Name: &projectName})
	if err != nil {
		return uuid.Nil, err
	} else if project == nil {
		return uuid.Nil, biz.NewErrNotFound("project")
	}

	// Check if the project is in the visible projects list (RBAC)
	// nil means all projects are visible
	if visibleProjectIDs != nil && !slices.Contains(visibleProjectIDs, project.ID) {
		return uuid.Nil, biz.NewErrNotFound("project")
	}

	return project.ID, nil
}
