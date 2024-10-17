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
	"fmt"

	pb "github.com/chainloop-dev/chainloop/app/controlplane/api/controlplane/v1"
	craftingpb "github.com/chainloop-dev/chainloop/app/controlplane/api/workflowcontract/v1"
	"github.com/chainloop-dev/chainloop/app/controlplane/internal/pagination"
	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/biz"
	"github.com/chainloop-dev/chainloop/pkg/credentials"
	errors "github.com/go-kratos/kratos/v2/errors"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type WorkflowRunService struct {
	pb.UnimplementedWorkflowRunServiceServer
	*service

	wrUseCase               *biz.WorkflowRunUseCase
	workflowUseCase         *biz.WorkflowUseCase
	workflowContractUseCase *biz.WorkflowContractUseCase
	credsReader             credentials.Reader
}

type NewWorkflowRunServiceOpts struct {
	WorkflowRunUC      *biz.WorkflowRunUseCase
	WorkflowUC         *biz.WorkflowUseCase
	WorkflowContractUC *biz.WorkflowContractUseCase
	CredsReader        credentials.Reader
	Opts               []NewOpt
}

func NewWorkflowRunService(opts *NewWorkflowRunServiceOpts) *WorkflowRunService {
	return &WorkflowRunService{
		service:                 newService(opts.Opts...),
		wrUseCase:               opts.WorkflowRunUC,
		workflowUseCase:         opts.WorkflowUC,
		workflowContractUseCase: opts.WorkflowContractUC,
		credsReader:             opts.CredsReader,
	}
}

func (s *WorkflowRunService) List(ctx context.Context, req *pb.WorkflowRunServiceListRequest) (*pb.WorkflowRunServiceListResponse, error) {
	currentOrg, err := requireCurrentOrg(ctx)
	if err != nil {
		return nil, err
	}

	// Configure filters
	filters := &biz.RunListFilters{}

	// by workflow
	if req.GetWorkflowName() != "" && req.GetProjectName() != "" {
		wf, err := s.workflowUseCase.FindByNameInOrg(ctx, currentOrg.ID, req.GetProjectName(), req.GetWorkflowName())
		if err != nil {
			return nil, handleUseCaseErr(err, s.log)
		} else if wf == nil {
			return nil, errors.NotFound("not found", "workflow not found")
		}

		filters.WorkflowID = wf.ID
	}

	// by run status
	if req.GetStatus() != pb.RunStatus_RUN_STATUS_UNSPECIFIED {
		st, err := pbWorkflowRunStatusToBiz(req.GetStatus())
		if err != nil {
			return nil, errors.BadRequest("invalid run status", err.Error())
		}
		filters.Status = st
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
	if req.GetId() != "" {
		run, err = s.wrUseCase.GetByIDInOrgOrPublic(ctx, currentOrg.ID, req.GetId())
		if err != nil {
			return nil, handleUseCaseErr(err, s.log)
		}
	} else if req.GetDigest() != "" {
		run, err = s.wrUseCase.GetByDigestInOrgOrPublic(ctx, currentOrg.ID, req.GetDigest())
		if err != nil {
			return nil, handleUseCaseErr(err, s.log)
		}
	} else {
		return nil, errors.BadRequest("invalid", "id or digest required")
	}

	attestation, err := bizAttestationToPb(run.Attestation)
	if err != nil {
		return nil, handleUseCaseErr(err, s.log)
	}

	contractAndVersion, err := s.workflowContractUseCase.FindVersionByID(ctx, run.ContractVersionID.String())
	if err != nil {
		return nil, handleUseCaseErr(err, s.log)
	}

	wr := bizWorkFlowRunToPb(run)
	wr.Workflow = bizWorkflowToPb(run.Workflow)
	wr.ContractVersion = bizWorkFlowContractVersionToPb(contractAndVersion.Version)
	wr.ContractVersion.ContractName = contractAndVersion.Contract.Name
	res := &pb.WorkflowRunServiceViewResponse_Result{
		WorkflowRun: wr,
		Attestation: attestation,
	}

	return &pb.WorkflowRunServiceViewResponse{Result: res}, nil
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
