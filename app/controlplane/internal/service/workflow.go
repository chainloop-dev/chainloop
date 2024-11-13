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
	"time"

	pb "github.com/chainloop-dev/chainloop/app/controlplane/api/controlplane/v1"
	schema "github.com/chainloop-dev/chainloop/app/controlplane/api/workflowcontract/v1"
	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/biz"
	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/pagination"

	"github.com/go-kratos/kratos/v2/errors"
	"github.com/google/uuid"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type WorkflowService struct {
	pb.UnimplementedWorkflowServiceServer
	*service

	useCase         *biz.WorkflowUseCase
	projectsUseCase *biz.ProjectUseCase
	contractUC      *biz.WorkflowContractUseCase
}

func NewWorkflowService(uc *biz.WorkflowUseCase, wfuc *biz.WorkflowContractUseCase, projectUseCase *biz.ProjectUseCase, opts ...NewOpt) *WorkflowService {
	return &WorkflowService{
		service:         newService(opts...),
		useCase:         uc,
		contractUC:      wfuc,
		projectsUseCase: projectUseCase,
	}
}

func (s *WorkflowService) Create(ctx context.Context, req *pb.WorkflowServiceCreateRequest) (*pb.WorkflowServiceCreateResponse, error) {
	currentOrg, err := requireCurrentOrg(ctx)
	if err != nil {
		return nil, err
	}

	createOpts := &biz.WorkflowCreateOpts{
		OrgID:        currentOrg.ID,
		Name:         req.GetName(),
		Project:      req.GetProjectName(),
		Team:         req.GetTeam(),
		ContractName: req.GetContractName(),
		Description:  req.GetDescription(),
		Public:       req.GetPublic(),
	}

	p, err := s.useCase.Create(ctx, createOpts)
	if err != nil {
		return nil, handleUseCaseErr(err, s.log)
	}

	return &pb.WorkflowServiceCreateResponse{Result: bizWorkflowToPb(p)}, nil
}

func (s *WorkflowService) Update(ctx context.Context, req *pb.WorkflowServiceUpdateRequest) (*pb.WorkflowServiceUpdateResponse, error) {
	currentOrg, err := requireCurrentOrg(ctx)
	if err != nil {
		return nil, err
	}

	wf, err := s.useCase.FindByNameInOrg(ctx, currentOrg.ID, req.ProjectName, req.Name)
	if err != nil {
		return nil, handleUseCaseErr(err, s.log)
	}

	var contractID *string
	if req.ContractName != nil {
		c, err := s.contractUC.FindByNameInOrg(ctx, currentOrg.ID, *req.ContractName)
		if err != nil {
			return nil, handleUseCaseErr(err, s.log)
		} else if c == nil {
			return nil, biz.NewErrNotFound("contract")
		}

		cid := c.ID.String()
		contractID = &cid
	}

	updateOpts := &biz.WorkflowUpdateOpts{
		Team:        req.Team,
		Public:      req.Public,
		Description: req.Description,
		ContractID:  contractID,
	}

	p, err := s.useCase.Update(ctx, currentOrg.ID, wf.ID.String(), updateOpts)
	if err != nil {
		return nil, handleUseCaseErr(err, s.log)
	}

	return &pb.WorkflowServiceUpdateResponse{Result: bizWorkflowToPb(p)}, nil
}

// List returns a list of workflows.
func (s *WorkflowService) List(ctx context.Context, req *pb.WorkflowServiceListRequest) (*pb.WorkflowServiceListResponse, error) {
	currentOrg, err := requireCurrentOrg(ctx)
	if err != nil {
		return nil, err
	}

	// Initialize the pagination options
	paginationOpts := &pagination.OffsetPaginationOpts{}

	if req.GetPagination() != nil {
		paginationOpts, err = pagination.NewOffsetPaginationOpts(
			int(req.GetPagination().GetPage()),
			int(req.GetPagination().GetPageSize()),
		)
		if err != nil {
			return nil, handleUseCaseErr(err, s.log)
		}
	} else {
		// Apply default pagination if not provided
		paginationOpts = pagination.NewDefaultOffsetPaginationOpts()
	}

	// Initialize the filters
	filters := &biz.WorkflowListOpts{}

	// Check the workflow name
	if req.GetWorkflowName() != "" {
		filters.WorkflowName = req.GetWorkflowName()
	}

	// Check the Team
	if req.GetWorkflowTeam() != "" {
		filters.WorkflowTeam = req.GetWorkflowTeam()
	}

	// Check the Project Name
	if len(req.GetProjectNames()) != 0 {
		filters.WorkflowProjectNames = req.GetProjectNames()
	}

	// Workflow visibility
	if req.WorkflowPublic != nil {
		val := req.GetWorkflowPublic()
		filters.WorkflowPublic = &val
	}

	// Workflow Run Runner Type
	if req.GetWorkflowRunRunnerType() != schema.CraftingSchema_Runner_RUNNER_TYPE_UNSPECIFIED {
		filters.WorkflowRunRunnerType = req.GetWorkflowRunRunnerType().String()
	}

	// Workflow Last Known Status
	if req.GetWorkflowRunLastStatus() != pb.RunStatus_RUN_STATUS_UNSPECIFIED {
		status, err := pbWorkflowRunStatusToBiz(req.GetWorkflowRunLastStatus())
		if err != nil {
			return nil, errors.BadRequest("invalid argument", err.Error())
		}
		filters.WorkflowRunLastStatus = status
	}

	// Workflow Last Activity Window
	if req.GetWorkflowLastActivityWindow() != pb.WorkflowActivityWindow_WORKFLOW_ACTIVITY_WINDOW_UNSPECIFIED {
		timeWindow, err := workflowsActivityTimeWindowPbToTimeWindow(req.GetWorkflowLastActivityWindow())
		if err != nil {
			return nil, errors.BadRequest("invalid argument", err.Error())
		}
		filters.WorkflowActiveWindow = timeWindow
	}

	workflows, count, err := s.useCase.List(ctx, currentOrg.ID, filters, paginationOpts)
	if err != nil {
		return nil, handleUseCaseErr(err, s.log)
	}

	result := make([]*pb.WorkflowItem, 0, len(workflows))
	for _, p := range workflows {
		result = append(result, bizWorkflowToPb(p))
	}

	var (
		isLastPage  bool
		currentPage int32
	)
	// Calculate the current page
	if count > 0 {
		currentPage = int32(paginationOpts.Offset()/paginationOpts.Limit()) + 1
		isLastPage = currentPage*int32(paginationOpts.Limit()) >= int32(count)
	}

	return &pb.WorkflowServiceListResponse{
		Result: result,
		Pagination: &pb.OffsetPaginationResponse{
			Page:       currentPage,
			PageSize:   int32(paginationOpts.Limit()),
			LastPage:   isLastPage,
			TotalCount: int32(count),
		},
	}, nil
}

func (s *WorkflowService) Delete(ctx context.Context, req *pb.WorkflowServiceDeleteRequest) (*pb.WorkflowServiceDeleteResponse, error) {
	currentOrg, err := requireCurrentOrg(ctx)
	if err != nil {
		return nil, err
	}

	wf, err := s.useCase.FindByNameInOrg(ctx, currentOrg.ID, req.ProjectName, req.Name)
	if err != nil {
		return nil, handleUseCaseErr(err, s.log)
	}

	if err := s.useCase.Delete(ctx, currentOrg.ID, wf.ID.String()); err != nil {
		return nil, handleUseCaseErr(err, s.log)
	}

	return &pb.WorkflowServiceDeleteResponse{}, nil
}

func (s *WorkflowService) View(ctx context.Context, req *pb.WorkflowServiceViewRequest) (*pb.WorkflowServiceViewResponse, error) {
	currentOrg, err := requireCurrentOrg(ctx)
	if err != nil {
		return nil, err
	}

	var wf *biz.Workflow

	wf, err = s.useCase.FindByNameInOrg(ctx, currentOrg.ID, req.ProjectName, req.Name)
	if err != nil {
		return nil, handleUseCaseErr(err, s.log)
	}

	return &pb.WorkflowServiceViewResponse{Result: bizWorkflowToPb(wf)}, nil
}

func bizWorkflowToPb(wf *biz.Workflow) *pb.WorkflowItem {
	item := &pb.WorkflowItem{
		Id: wf.ID.String(), Name: wf.Name, CreatedAt: timestamppb.New(*wf.CreatedAt),
		Project: wf.Project, Team: wf.Team, RunsCount: int32(wf.RunsCounter), Public: wf.Public,
		Description: wf.Description, ContractRevisionLatest: int32(wf.ContractRevisionLatest),
	}

	if wf.ContractID != uuid.Nil {
		item.ContractName = wf.ContractName
	}

	if wf.LastRun != nil {
		item.LastRun = bizWorkFlowRunToPb(wf.LastRun)
	}

	return item
}

// bizWorkflowRunToPb converts a biz.WorkflowRun to a pb.WorkflowRun.
func bizWorkflowRefToPb(wf *biz.WorkflowRef) *pb.WorkflowRef {
	return &pb.WorkflowRef{Id: wf.ID.String(), Name: wf.Name, ProjectName: wf.ProjectName}
}

// workflowsActivityTimeWindowPbToTimeWindow converts a v1.WorkflowActivityWindow to a biz.TimeWindow.
func workflowsActivityTimeWindowPbToTimeWindow(tw pb.WorkflowActivityWindow) (*biz.TimeWindow, error) {
	timeWindow := &biz.TimeWindow{
		To: time.Now().UTC(),
	}
	switch tw {
	case pb.WorkflowActivityWindow_WORKFLOW_ACTIVITY_WINDOW_LAST_DAY:
		timeWindow.From = timeWindow.To.Add(-24 * time.Hour)
	case pb.WorkflowActivityWindow_WORKFLOW_ACTIVITY_WINDOW_LAST_7_DAYS:
		timeWindow.From = timeWindow.To.Add(-7 * 24 * time.Hour)
	case pb.WorkflowActivityWindow_WORKFLOW_ACTIVITY_WINDOW_LAST_30_DAYS:
		timeWindow.From = timeWindow.To.Add(-30 * 24 * time.Hour)
	case pb.WorkflowActivityWindow_WORKFLOW_ACTIVITY_WINDOW_UNSPECIFIED:
		return nil, fmt.Errorf("invalid time window: %s", tw)
	}

	return timeWindow, nil
}
