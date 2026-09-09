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
	conf "github.com/chainloop-dev/chainloop/app/controlplane/internal/conf/controlplane/config/v1"
	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/authz"
	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/biz"
	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/pagination"
	chainloop "github.com/chainloop-dev/chainloop/pkg/attestation/renderer/chainloop"
	"github.com/chainloop-dev/chainloop/pkg/cache/policyevalbundle"
	"github.com/chainloop-dev/chainloop/pkg/credentials"
	errors "github.com/go-kratos/kratos/v2/errors"
	"github.com/google/uuid"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type WorkflowRunService struct {
	pb.UnimplementedWorkflowRunServiceServer
	*service

	wrUseCase               *biz.WorkflowRunUseCase
	workflowUseCase         *biz.WorkflowUseCase
	workflowContractUseCase *biz.WorkflowContractUseCase
	projectUseCase          *biz.ProjectUseCase
	projectVersionUseCase   *biz.ProjectVersionUseCase
	credsReader             credentials.Reader
	casClient               biz.CASClient
	casMappingUC            *biz.CASMappingUseCase
	policyEvalCache         *policyevalbundle.Cache
	bootstrapConfig         *conf.Bootstrap
}

type NewWorkflowRunServiceOpts struct {
	WorkflowRunUC      *biz.WorkflowRunUseCase
	WorkflowUC         *biz.WorkflowUseCase
	WorkflowContractUC *biz.WorkflowContractUseCase
	ProjectUC          *biz.ProjectUseCase
	ProjectVersionUC   *biz.ProjectVersionUseCase
	CredsReader        credentials.Reader
	CASClient          biz.CASClient
	CASMappingUC       *biz.CASMappingUseCase
	PolicyEvalCache    *policyevalbundle.Cache
	BootstrapConfig    *conf.Bootstrap
	Opts               []NewOpt
}

func NewWorkflowRunService(opts *NewWorkflowRunServiceOpts) *WorkflowRunService {
	return &WorkflowRunService{
		service:                 newService(opts.Opts...),
		wrUseCase:               opts.WorkflowRunUC,
		workflowUseCase:         opts.WorkflowUC,
		workflowContractUseCase: opts.WorkflowContractUC,
		projectUseCase:          opts.ProjectUC,
		projectVersionUseCase:   opts.ProjectVersionUC,
		credsReader:             opts.CredsReader,
		casClient:               opts.CASClient,
		casMappingUC:            opts.CASMappingUC,
		policyEvalCache:         opts.PolicyEvalCache,
		bootstrapConfig:         opts.BootstrapConfig,
	}
}

type casResolvedPredicate struct {
	chainloop.NormalizablePredicate
	evals map[string][]*chainloop.PolicyEvaluation
}

func (p *casResolvedPredicate) GetPolicyEvaluations() map[string][]*chainloop.PolicyEvaluation {
	return p.evals
}

// defaultPolicyEvaluationsMaxInlineBytes bounds the policy-evaluation bundle
// the View API is willing to download and inline in a response. A single
// attestation can carry a six-figure number of violations, and inlining one
// holds the payload in memory several times over (download buffer, decoded
// bundle, regrouped evaluations, response protos), which is enough to exhaust
// the control plane. Bundles above the cap are returned as a reference so the
// caller can fetch them directly from the CAS.
const defaultPolicyEvaluationsMaxInlineBytes = 10 << 20 // 10MiB

// resolvedPolicyEvaluations carries the reference to the policy-evaluation
// bundle and, when it was small enough to be read, the evaluations decoded from
// it. The reference is always set so callers can point at the bundle whether or
// not it was inlined; evaluations are set only on the inlining path.
type resolvedPolicyEvaluations struct {
	evaluations map[string][]*chainloop.PolicyEvaluation
	ref         *pb.PolicyEvaluationsRef
}

func (s *WorkflowRunService) policyEvaluationsMaxInlineBytes() int64 {
	if configured := s.bootstrapConfig.GetAttestations().GetPolicyEvaluationsMaxInlineBytes(); configured > 0 {
		return configured
	}

	return defaultPolicyEvaluationsMaxInlineBytes
}

// resolvePolicyEvaluations resolves the policy-evaluation bundle referenced by
// an attestation predicate. It returns nil when the predicate carries no
// reference, meaning the caller should keep whatever the predicate itself
// holds.
//
// The cap is enforced against the size the descriptor records, which travels
// signed with the predicate and costs no round trip. A bundle whose size is not
// recorded is read in full, since sizing it would mean asking the CAS on every
// view -- the very cost the recorded size exists to avoid.
//
// Anything that prevents inlining the bundle with confidence -- an oversized
// bundle, an unreachable or undecodable object -- yields a reference rather
// than the evaluations.
func (s *WorkflowRunService) resolvePolicyEvaluations(
	ctx context.Context,
	ref *chainloop.PolicyEvaluationsRef,
	orgID uuid.UUID,
) *resolvedPolicyEvaluations {
	if ref == nil {
		return nil
	}

	mediaType := ref.GetMediaType()
	bundleSize := ref.GetSizeBytes()

	hexDigest, ok := ref.GetDigest()["sha256"]
	if !ok {
		s.log.Warnw("msg", "policy evaluations reference has no sha256 digest")
		return unavailablePolicyEvaluations("", 0, mediaType)
	}
	digest := fmt.Sprintf("sha256:%s", hexDigest)

	maxInlineBytes := s.policyEvaluationsMaxInlineBytes()
	if bundleSize > maxInlineBytes {
		s.log.Infow("msg", "policy evaluations bundle too large to inline", "digest", digest, "size", bundleSize, "max", maxInlineBytes)
		return tooLargePolicyEvaluations(digest, bundleSize, mediaType)
	}

	// A cached bundle costs no CAS round trip.
	if cached, found, err := s.policyEvalCache.Get(ctx, digest); err == nil && found {
		return s.decodePolicyEvaluations(cached, digest, mediaType)
	}

	mapping, err := s.casMappingUC.FindCASMappingForDownloadByOrg(ctx, digest, []uuid.UUID{orgID}, nil)
	if err != nil {
		s.log.Warnw("msg", "finding CAS mapping for policy evaluations", "digest", digest, "err", err)
		return unavailablePolicyEvaluations(digest, bundleSize, mediaType)
	}

	buf := &bytes.Buffer{}
	err = s.casClient.Download(ctx, string(mapping.CASBackend.Provider), mapping.CASBackend.SecretName, mapping.CASBackend.OrganizationID, buf, digest)
	if err != nil {
		s.log.Warnw("msg", "downloading policy evaluations bundle", "digest", digest, "err", err)
		return unavailablePolicyEvaluations(digest, bundleSize, mediaType)
	}

	data := buf.Bytes()
	_ = s.policyEvalCache.Set(ctx, digest, data)

	return s.decodePolicyEvaluations(data, digest, mediaType)
}

// decodePolicyEvaluations reports the bytes actually read rather than the size
// the attestation records: the two can disagree, and only the former is known
// to be right.
func (s *WorkflowRunService) decodePolicyEvaluations(data []byte, digest string, mediaType string) *resolvedPolicyEvaluations {
	evaluations, err := chainloop.PolicyEvaluationsFromBundle(data)
	if err != nil {
		s.log.Warnw("msg", "decoding policy evaluations bundle", "digest", digest, "err", err)
		return unavailablePolicyEvaluations(digest, int64(len(data)), mediaType)
	}

	return &resolvedPolicyEvaluations{
		evaluations: evaluations,
		ref:         inlinedPolicyEvaluations(digest, int64(len(data)), mediaType),
	}
}

// inlinedPolicyEvaluations references a bundle whose evaluations are being
// returned alongside it. No reason is set: nothing was left out.
func inlinedPolicyEvaluations(digest string, size int64, mediaType string) *pb.PolicyEvaluationsRef {
	return &pb.PolicyEvaluationsRef{
		Digest:    digest,
		SizeBytes: size,
		MediaType: mediaType,
		Inlined:   true,
	}
}

func tooLargePolicyEvaluations(digest string, size int64, mediaType string) *resolvedPolicyEvaluations {
	return &resolvedPolicyEvaluations{ref: &pb.PolicyEvaluationsRef{
		Digest:    digest,
		SizeBytes: size,
		MediaType: mediaType,
		Reason:    pb.PolicyEvaluationsRef_REASON_TOO_LARGE,
	}}
}

func unavailablePolicyEvaluations(digest string, size int64, mediaType string) *resolvedPolicyEvaluations {
	return &resolvedPolicyEvaluations{ref: &pb.PolicyEvaluationsRef{
		Digest:    digest,
		SizeBytes: size,
		MediaType: mediaType,
		Reason:    pb.PolicyEvaluationsRef_REASON_UNAVAILABLE,
	}}
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

	// Track the resolved project so a project version can be looked up by name.
	var projectID *uuid.UUID

	// by workflow and project name
	if req.GetWorkflowName() != "" && req.GetProjectName() != "" {
		wf, err := s.workflowUseCase.FindByNameInOrg(ctx, currentOrg.ID, req.GetProjectName(), req.GetWorkflowName())
		if err != nil {
			return nil, handleUseCaseErr(err, s.log)
		} else if wf == nil {
			return nil, errors.NotFound("not found", "workflow not found")
		}

		filters.WorkflowID = &wf.ID
		projectID = &wf.ProjectID
	} else if req.GetProjectName() != "" {
		// by project name only
		pID, err := s.validateAndGetProjectID(ctx, currentOrg.ID, req.GetProjectName(), visibleProjectIDs)
		if err != nil {
			return nil, handleUseCaseErr(err, s.log)
		}

		// Override the filter to only include this specific project
		filters.ProjectIDs = []uuid.UUID{pID}
		projectID = &pID
	}

	// by project version
	switch {
	case req.GetProjectVersionName() != "":
		// A version name is unique only within a project, so project_name is
		// required. Enforced at the API layer (CEL) and re-checked here.
		if projectID == nil {
			return nil, errors.BadRequest("invalid", "project_name must be set when project_version_name is set")
		}

		pv, err := s.projectVersionUseCase.FindByProjectAndVersion(ctx, projectID.String(), req.GetProjectVersionName())
		if err != nil {
			return nil, handleUseCaseErr(err, s.log)
		}

		filters.VersionID = &pv.ID
	default:
		// Deprecated: honor the project_version UUID for clients that have not
		// migrated to project_version_name.
		//nolint:staticcheck // honoring the deprecated field for older clients
		if rawVersion := req.GetProjectVersion(); rawVersion != "" {
			projectUUID, err := uuid.Parse(rawVersion)
			if err != nil {
				return nil, errors.BadRequest("invalid", "invalid project version")
			}

			filters.VersionID = &projectUUID
		}
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
		run, err = s.wrUseCase.GetByIDInOrg(ctx, currentOrg.ID, req.GetId())
		if err != nil {
			return nil, handleUseCaseErr(err, s.log)
		}
	case req.GetDigest() != "":
		run, err = s.wrUseCase.GetByDigestInOrg(ctx, currentOrg.ID, req.GetDigest())
		if err != nil {
			return nil, handleUseCaseErr(err, s.log)
		}
	default:
		return nil, errors.BadRequest("invalid", "id or digest required")
	}

	// Enforce project-scoped RBAC on the workflow run
	if err = s.authorizeResource(ctx, authz.PolicyWorkflowRunRead, authz.ResourceTypeProject, run.Workflow.ProjectID); err != nil {
		return nil, err
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
	var policyEvaluationsRef *pb.PolicyEvaluationsRef
	if run.Attestation != nil && run.Attestation.Envelope != nil {
		predicate, err = chainloop.ExtractPredicate(run.Attestation.Envelope)
		if err != nil {
			return nil, handleUseCaseErr(err, s.log)
		}

		if resolved := s.resolvePolicyEvaluations(ctx, predicate.GetPolicyEvaluationsRef(), run.Workflow.OrgID); resolved != nil {
			// The reference always travels back so the caller can fetch the
			// bundle from the CAS; the evaluations only when they were inlined.
			policyEvaluationsRef = resolved.ref
			if resolved.evaluations != nil {
				predicate = &casResolvedPredicate{NormalizablePredicate: predicate, evals: resolved.evaluations}
			}
		}
	}

	attestation, err := bizAttestationToPb(run.Attestation, predicate)
	if err != nil {
		return nil, handleUseCaseErr(err, s.log)
	}

	if attestation != nil {
		attestation.PolicyEvaluationsRef = policyEvaluationsRef
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
		Latest:     v.Latest,
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
		Status:     bizPolicyStatusToPb(s.Status),
		Total:      int32(s.Total),
		Passed:     int32(s.Passed),
		Skipped:    int32(s.Skipped),
		Violated:   int32(s.Violated),
		HasGates:   s.HasGates,
		Suppressed: int32(s.Suppressed),
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
