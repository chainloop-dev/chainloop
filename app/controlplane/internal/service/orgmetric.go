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
	"github.com/chainloop-dev/chainloop/app/controlplane/internal/biz"
	sl "github.com/chainloop-dev/chainloop/internal/servicelogger"
)

type OrgMetricsService struct {
	pb.UnimplementedOrgMetricsServiceServer
	*service

	uc *biz.OrgMetricsUseCase
}

func NewOrgMetricsService(uc *biz.OrgMetricsUseCase, opts ...NewOpt) *OrgMetricsService {
	return &OrgMetricsService{
		service: newService(opts...),
		uc:      uc,
	}
}

func (s *OrgMetricsService) Totals(ctx context.Context, req *pb.OrgMetricsServiceTotalsRequest) (*pb.OrgMetricsServiceTotalsResponse, error) {
	currentOrg, err := requireCurrentOrg(ctx)
	if err != nil {
		return nil, err
	}

	// totals
	totals, err := s.uc.RunsTotal(ctx, currentOrg.ID, *req.TimeWindow.ToDuration())
	if err != nil {
		return nil, sl.LogAndMaskErr(err, s.log)
	}

	totalsByStatus, err := s.uc.RunsTotalByStatus(ctx, currentOrg.ID, *req.TimeWindow.ToDuration())
	if err != nil {
		return nil, sl.LogAndMaskErr(err, s.log)
	}

	totalsByRunnerType, err := s.uc.RunsTotalByRunnerType(ctx, currentOrg.ID, *req.TimeWindow.ToDuration())
	if err != nil {
		return nil, sl.LogAndMaskErr(err, s.log)
	}

	return &pb.OrgMetricsServiceTotalsResponse{Result: &pb.OrgMetricsServiceTotalsResponse_Result{
		RunsTotal:             totals,
		RunsTotalByStatus:     totalsByStatus,
		RunsTotalByRunnerType: totalsByRunnerType,
	}}, nil
}

func (s *OrgMetricsService) TopWorkflowsByRunsCount(ctx context.Context, req *pb.TopWorkflowsByRunsCountRequest) (*pb.TopWorkflowsByRunsCountResponse, error) {
	currentOrg, err := requireCurrentOrg(ctx)
	if err != nil {
		return nil, err
	}

	res, err := s.uc.TopWorkflowsByRunsCount(ctx, currentOrg.ID, int(req.GetNumWorkflows()), *req.TimeWindow.ToDuration())
	if err != nil {
		return nil, sl.LogAndMaskErr(err, s.log)
	}

	var result = []*pb.TopWorkflowsByRunsCountResponse_TotalByStatus{}
	for _, r := range res {
		result = append(result, &pb.TopWorkflowsByRunsCountResponse_TotalByStatus{
			Workflow:          bizWorkflowToPb(r.Workflow),
			RunsTotalByStatus: r.ByStatus,
		})
	}

	return &pb.TopWorkflowsByRunsCountResponse{Result: result}, nil
}
