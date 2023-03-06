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
	"github.com/chainloop-dev/chainloop/app/controlplane/internal/integrations/dependencytrack"
	sl "github.com/chainloop-dev/chainloop/internal/servicelogger"
	errors "github.com/go-kratos/kratos/v2/errors"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type IntegrationsService struct {
	pb.UnimplementedIntegrationsServiceServer
	*service

	integrationUC *biz.IntegrationUseCase
	workflowUC    *biz.WorkflowUseCase
}

func NewIntegrationsService(uc *biz.IntegrationUseCase, wuc *biz.WorkflowUseCase, opts ...NewOpt) *IntegrationsService {
	return &IntegrationsService{
		service:       newService(opts...),
		integrationUC: uc,
		workflowUC:    wuc,
	}
}

func (s *IntegrationsService) AddDependencyTrack(ctx context.Context, req *pb.AddDependencyTrackRequest) (*pb.AddDependencyTrackResponse, error) {
	_, org, err := loadCurrentUserAndOrg(ctx)
	if err != nil {
		return nil, err
	}

	domain, enableProjectCreation := req.GetConfig().GetDomain(), req.GetConfig().GetAllowAutoCreate()
	// TODO: Move validation logic to biz use-case
	checker, err := dependencytrack.NewIntegration(domain, req.ApiKey, enableProjectCreation)
	if err != nil {
		return nil, sl.LogAndMaskErr(err, s.log)
	}

	if err := checker.Validate(ctx); err != nil {
		return nil, errors.BadRequest("invalid configuration", err.Error())
	}

	i, err := s.integrationUC.AddDependencyTrack(ctx, org.ID, domain, req.ApiKey, enableProjectCreation)
	if err != nil {
		return nil, sl.LogAndMaskErr(err, s.log)
	}

	return &pb.AddDependencyTrackResponse{Result: bizIntegrationToPb(i)}, nil
}

func (s *IntegrationsService) List(ctx context.Context, req *pb.IntegrationsServiceListRequest) (*pb.IntegrationsServiceListResponse, error) {
	_, org, err := loadCurrentUserAndOrg(ctx)
	if err != nil {
		return nil, err
	}

	integrations, err := s.integrationUC.List(ctx, org.ID)
	if err != nil {
		return nil, sl.LogAndMaskErr(err, s.log)
	}

	result := make([]*pb.IntegrationItem, 0, len(integrations))
	for _, i := range integrations {
		result = append(result, bizIntegrationToPb(i))
	}

	return &pb.IntegrationsServiceListResponse{Result: result}, nil
}

func (s *IntegrationsService) Delete(ctx context.Context, req *pb.IntegrationsServiceDeleteRequest) (*pb.IntegrationsServiceDeleteResponse, error) {
	_, org, err := loadCurrentUserAndOrg(ctx)
	if err != nil {
		return nil, err
	}

	err = s.integrationUC.Delete(ctx, org.ID, req.Id)
	if err != nil && biz.IsNotFound(err) {
		return nil, errors.NotFound("not found", err.Error())
	} else if err != nil {
		return nil, sl.LogAndMaskErr(err, s.log)
	}

	return &pb.IntegrationsServiceDeleteResponse{}, nil
}

func (s *IntegrationsService) Attach(ctx context.Context, req *pb.IntegrationsServiceAttachRequest) (*pb.IntegrationsServiceAttachResponse, error) {
	_, org, err := loadCurrentUserAndOrg(ctx)
	if err != nil {
		return nil, err
	}

	res, err := s.integrationUC.AttachToWorkflow(ctx, &biz.AttachOpts{
		OrgID: org.ID, IntegrationID: req.IntegrationId, WorkflowID: req.WorkflowId, Config: req.Config,
	})
	if err != nil {
		if biz.IsNotFound(err) {
			return nil, errors.NotFound("not found", err.Error())
		} else if biz.IsErrValidation(err) {
			return nil, errors.BadRequest("wrong validation", err.Error())
		}

		return nil, sl.LogAndMaskErr(err, s.log)
	}

	result, err := s.bizIntegrationAttachmentToPb(ctx, res, org.ID)
	if err != nil {
		return nil, sl.LogAndMaskErr(err, s.log)
	}

	return &pb.IntegrationsServiceAttachResponse{Result: result}, nil
}

func (s *IntegrationsService) ListAttachments(ctx context.Context, req *pb.ListAttachmentsRequest) (*pb.ListAttachmentsResponse, error) {
	_, org, err := loadCurrentUserAndOrg(ctx)
	if err != nil {
		return nil, err
	}

	integrations, err := s.integrationUC.ListAttachments(ctx, org.ID, req.GetWorkflowId())
	if err != nil {
		return nil, sl.LogAndMaskErr(err, s.log)
	}

	result := make([]*pb.IntegrationAttachmentItem, 0, len(integrations))
	for _, i := range integrations {
		r, err := s.bizIntegrationAttachmentToPb(ctx, i, org.ID)
		if err != nil {
			return nil, sl.LogAndMaskErr(err, s.log)
		}
		result = append(result, r)
	}

	return &pb.ListAttachmentsResponse{Result: result}, nil
}

func (s *IntegrationsService) Detach(ctx context.Context, req *pb.IntegrationsServiceDetachRequest) (*pb.IntegrationsServiceDetachResponse, error) {
	_, org, err := loadCurrentUserAndOrg(ctx)
	if err != nil {
		return nil, err
	}

	if err := s.integrationUC.Detach(ctx, org.ID, req.Id); err != nil {
		if biz.IsNotFound(err) {
			return nil, errors.NotFound("not found", err.Error())
		}

		return nil, sl.LogAndMaskErr(err, s.log)
	}

	return &pb.IntegrationsServiceDetachResponse{}, nil
}

func bizIntegrationToPb(e *biz.Integration) *pb.IntegrationItem {
	return &pb.IntegrationItem{
		Id: e.ID.String(), CreatedAt: timestamppb.New(*e.CreatedAt),
		Kind: e.Kind, Config: e.Config,
	}
}

func (s *IntegrationsService) bizIntegrationAttachmentToPb(ctx context.Context, e *biz.IntegrationAttachment, orgID string) (*pb.IntegrationAttachmentItem, error) {
	a := &pb.IntegrationAttachmentItem{
		Id: e.ID.String(), CreatedAt: timestamppb.New(*e.CreatedAt),
		Config: e.Config,
	}

	i, err := s.integrationUC.FindByIDInOrg(ctx, orgID, e.IntegrationID.String())
	if err != nil {
		return nil, err
	} else if i != nil {
		a.Integration = bizIntegrationToPb(i)
	}

	wf, err := s.workflowUC.FindByIDInOrg(ctx, orgID, e.WorkflowID.String())
	if err != nil {
		return nil, err
	} else if wf != nil {
		a.Workflow = bizWorkFlowToPb(wf)
	}

	return a, nil
}
