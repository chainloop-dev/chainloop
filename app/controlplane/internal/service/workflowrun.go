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
	"context"

	pb "github.com/chainloop-dev/chainloop/app/controlplane/api/controlplane/v1"
	craftingpb "github.com/chainloop-dev/chainloop/app/controlplane/api/workflowcontract/v1"
	"github.com/chainloop-dev/chainloop/app/controlplane/internal/biz"
	"github.com/chainloop-dev/chainloop/app/controlplane/internal/pagination"
	"github.com/chainloop-dev/chainloop/internal/credentials"
	sl "github.com/chainloop-dev/chainloop/internal/servicelogger"
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

	if req.GetWorkflowId() != "" {
		wf, err := s.workflowUseCase.FindByIDInOrg(ctx, currentOrg.ID, req.GetWorkflowId())
		if err != nil {
			return nil, handleUseCaseErr(workflowRunEntity, err, s.log)
		} else if wf == nil {
			return nil, errors.NotFound("not found", "workflow not found")
		}
	}

	p := req.GetPagination()
	paginationOpts, err := pagination.New(p.GetCursor(), int(p.GetLimit()))
	if err != nil {
		return nil, errors.InternalServer("invalid", "invalid pagination options")
	}

	workflowRuns, nextCursor, err := s.wrUseCase.List(ctx, currentOrg.ID, req.GetWorkflowId(), paginationOpts)
	if err != nil {
		return nil, sl.LogAndMaskErr(err, s.log)
	}

	result := make([]*pb.WorkflowRunItem, 0, len(workflowRuns))
	for _, wr := range workflowRuns {
		wrResp := bizWorkFlowRunToPb(wr)
		wrResp.Workflow = bizWorkflowToPb(wr.Workflow)
		result = append(result, wrResp)
	}

	return &pb.WorkflowRunServiceListResponse{Result: result, Pagination: bizCursorToPb(nextCursor)}, nil
}

func bizCursorToPb(cursor string) *pb.PaginationResponse {
	return &pb.PaginationResponse{NextCursor: cursor}
}

const workflowRunEntity = "Workflow Run"

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
			return nil, handleUseCaseErr(workflowRunEntity, err, s.log)
		}
	} else if req.GetDigest() != "" {
		run, err = s.wrUseCase.GetByDigestInOrgOrPublic(ctx, currentOrg.ID, req.GetDigest())
		if err != nil {
			return nil, handleUseCaseErr(workflowRunEntity, err, s.log)
		}
	}

	attestation, err := bizAttestationToPb(run.Attestation)
	if err != nil {
		return nil, sl.LogAndMaskErr(err, s.log)
	}

	contractVersion, err := s.workflowContractUseCase.FindVersionByID(ctx, run.ContractVersionID.String())
	if err != nil {
		return nil, handleUseCaseErr(workflowRunEntity, err, s.log)
	}

	wr := bizWorkFlowRunToPb(run)
	wr.Workflow = bizWorkflowToPb(run.Workflow)
	wr.ContractVersion = bizWorkFlowContractVersionToPb(contractVersion)
	res := &pb.WorkflowRunServiceViewResponse_Result{
		WorkflowRun: wr,
		Attestation: attestation,
	}

	return &pb.WorkflowRunServiceViewResponse{Result: res}, nil
}

func bizWorkFlowRunToPb(wfr *biz.WorkflowRun) *pb.WorkflowRunItem {
	r := craftingpb.CraftingSchema_Runner_RunnerType_value[wfr.RunnerType]
	item := &pb.WorkflowRunItem{
		Id:                     wfr.ID.String(),
		CreatedAt:              timestamppb.New(*wfr.CreatedAt),
		State:                  wfr.State,
		Reason:                 wfr.Reason,
		JobUrl:                 wfr.RunURL,
		RunnerType:             craftingpb.CraftingSchema_Runner_RunnerType(r),
		ContractRevisionUsed:   int32(wfr.ContractRevisionUsed),
		ContractRevisionLatest: int32(wfr.ContractRevisionLatestAvailable),
	}

	if wfr.FinishedAt != nil {
		item.FinishedAt = timestamppb.New(*wfr.FinishedAt)
	}

	return item
}
