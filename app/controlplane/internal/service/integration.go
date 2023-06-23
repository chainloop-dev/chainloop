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
	"fmt"

	pb "github.com/chainloop-dev/chainloop/app/controlplane/api/controlplane/v1"
	"github.com/chainloop-dev/chainloop/app/controlplane/internal/biz"
	"github.com/chainloop-dev/chainloop/app/controlplane/plugins/sdk/v1"
	sl "github.com/chainloop-dev/chainloop/internal/servicelogger"
	errors "github.com/go-kratos/kratos/v2/errors"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type IntegrationsService struct {
	pb.UnimplementedIntegrationsServiceServer
	*service

	integrationUC *biz.IntegrationUseCase
	workflowUC    *biz.WorkflowUseCase
	integrations  sdk.AvailablePlugins
}

func NewIntegrationsService(uc *biz.IntegrationUseCase, wuc *biz.WorkflowUseCase, integrations sdk.AvailablePlugins, opts ...NewOpt) *IntegrationsService {
	return &IntegrationsService{
		service:       newService(opts...),
		integrationUC: uc,
		workflowUC:    wuc,
		integrations:  integrations,
	}
}

func (s *IntegrationsService) ListAvailable(_ context.Context, _ *pb.IntegrationsServiceListAvailableRequest) (*pb.IntegrationsServiceListAvailableResponse, error) {
	result := make([]*pb.IntegrationAvailableItem, 0, len(s.integrations))

	for _, i := range s.integrations {
		d := i.Describe()

		var subscribedMaterials = make([]string, 0)
		for _, m := range d.SubscribedMaterials {
			subscribedMaterials = append(subscribedMaterials, m.Type.String())
		}

		item := &pb.IntegrationAvailableItem{
			Id:          d.ID,
			Version:     d.Version,
			Description: d.Description,
			Type: &pb.IntegrationAvailableItem_Fanout{
				Fanout: &pb.PluginFanout{
					AttachmentSchema:    d.AttachmentJSONSchema,
					RegistrationSchema:  d.RegistrationJSONSchema,
					SubscribedMaterials: subscribedMaterials,
				},
			},
		}

		result = append(result, item)
	}

	return &pb.IntegrationsServiceListAvailableResponse{Result: result}, nil
}

func (s *IntegrationsService) Register(ctx context.Context, req *pb.IntegrationsServiceRegisterRequest) (*pb.IntegrationsServiceRegisterResponse, error) {
	_, org, err := loadCurrentUserAndOrg(ctx)
	if err != nil {
		return nil, err
	}

	// lookup the integration
	integration, err := s.integrations.FindByID(req.PluginId)
	if err != nil {
		return nil, errors.NotFound("not found", err.Error())
	}

	i, err := s.integrationUC.RegisterAndSave(ctx, org.ID, req.Description, integration, req.Config)
	if err != nil {
		if biz.IsNotFound(err) {
			return nil, errors.NotFound("not found", err.Error())
		} else if biz.IsErrValidation(err) {
			return nil, errors.BadRequest("wrong validation", err.Error())
		}

		return nil, sl.LogAndMaskErr(err, s.log)
	}

	return &pb.IntegrationsServiceRegisterResponse{Result: bizIntegrationToPb(i)}, nil
}

func (s *IntegrationsService) Attach(ctx context.Context, req *pb.IntegrationsServiceAttachRequest) (*pb.IntegrationsServiceAttachResponse, error) {
	_, org, err := loadCurrentUserAndOrg(ctx)
	if err != nil {
		return nil, err
	}

	integration, err := s.integrationUC.FindByIDInOrg(ctx, org.ID, req.IntegrationId)
	if err != nil {
		if biz.IsNotFound(err) {
			return nil, errors.NotFound("not found", err.Error())
		}
		return nil, sl.LogAndMaskErr(err, s.log)
	}

	// lookup the integration
	attachable, err := s.integrations.FindByID(integration.Kind)
	if err != nil {
		return nil, fmt.Errorf("loading integration: %w", err)
	}

	res, err := s.integrationUC.AttachToWorkflow(ctx, &biz.AttachOpts{
		OrgID: org.ID, IntegrationID: req.IntegrationId, WorkflowID: req.WorkflowId,
		AttachmentConfig:  req.Config,
		FanOutIntegration: attachable,
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

func (s *IntegrationsService) ListRegistrations(ctx context.Context, _ *pb.IntegrationsServiceListRegistrationsRequest) (*pb.IntegrationsServiceListRegistrationsResponse, error) {
	_, org, err := loadCurrentUserAndOrg(ctx)
	if err != nil {
		return nil, err
	}

	integrations, err := s.integrationUC.List(ctx, org.ID)
	if err != nil {
		return nil, sl.LogAndMaskErr(err, s.log)
	}

	result := make([]*pb.RegisteredIntegrationItem, 0, len(integrations))
	for _, i := range integrations {
		result = append(result, bizIntegrationToPb(i))
	}

	return &pb.IntegrationsServiceListRegistrationsResponse{Result: result}, nil
}

func (s *IntegrationsService) DescribeRegistration(ctx context.Context, req *pb.IntegrationsServiceDescribeRegistrationRequest) (*pb.IntegrationsServiceDescribeRegistrationResponse, error) {
	_, org, err := loadCurrentUserAndOrg(ctx)
	if err != nil {
		return nil, err
	}

	i, err := s.integrationUC.FindByIDInOrg(ctx, org.ID, req.Id)
	if err != nil {
		return nil, sl.LogAndMaskErr(err, s.log)
	} else if i == nil {
		return nil, errors.NotFound("not found", "integration not found")
	}

	return &pb.IntegrationsServiceDescribeRegistrationResponse{Result: bizIntegrationToPb(i)}, nil
}

func (s *IntegrationsService) Deregister(ctx context.Context, req *pb.IntegrationsServiceDeregisterRequest) (*pb.IntegrationsServiceDeregisterResponse, error) {
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

	return &pb.IntegrationsServiceDeregisterResponse{}, nil
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

func bizIntegrationToPb(e *biz.Integration) *pb.RegisteredIntegrationItem {
	return &pb.RegisteredIntegrationItem{
		Id: e.ID.String(), CreatedAt: timestamppb.New(*e.CreatedAt),
		Description: e.Description,
		Kind:        e.Kind, Config: e.Config,
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
