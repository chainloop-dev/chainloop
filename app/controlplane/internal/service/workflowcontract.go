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
	errors "github.com/go-kratos/kratos/v2/errors"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type WorkflowContractService struct {
	pb.UnimplementedWorkflowContractServiceServer
	*service

	contractUseCase *biz.WorkflowContractUseCase
}

func NewWorkflowSchemaService(uc *biz.WorkflowContractUseCase, opts ...NewOpt) *WorkflowContractService {
	return &WorkflowContractService{
		service:         newService(opts...),
		contractUseCase: uc,
	}
}

func (s *WorkflowContractService) List(ctx context.Context, _ *pb.WorkflowContractServiceListRequest) (*pb.WorkflowContractServiceListResponse, error) {
	currentOrg, err := requireCurrentOrg(ctx)
	if err != nil {
		return nil, err
	}

	contracts, err := s.contractUseCase.List(ctx, currentOrg.ID)
	if err != nil {
		return nil, handleUseCaseErr("contract", err, s.log)
	}

	result := make([]*pb.WorkflowContractItem, 0, len(contracts))
	for _, s := range contracts {
		result = append(result, bizWorkFlowContractToPb(s))
	}

	return &pb.WorkflowContractServiceListResponse{Result: result}, nil
}

func (s *WorkflowContractService) Describe(ctx context.Context, req *pb.WorkflowContractServiceDescribeRequest) (*pb.WorkflowContractServiceDescribeResponse, error) {
	currentOrg, err := requireCurrentOrg(ctx)
	if err != nil {
		return nil, err
	}

	contractWithVersion, err := s.contractUseCase.Describe(ctx, currentOrg.ID, req.GetId(), int(req.GetRevision()))
	if err != nil {
		return nil, handleUseCaseErr("contract", err, s.log)
	} else if contractWithVersion == nil {
		return nil, errors.NotFound("not found", "contract not found")
	}

	result := &pb.WorkflowContractServiceDescribeResponse_Result{
		Contract: bizWorkFlowContractToPb(contractWithVersion.Contract),
		Revision: bizWorkFlowContractVersionToPb(contractWithVersion.Version),
	}

	return &pb.WorkflowContractServiceDescribeResponse{Result: result}, nil
}

func (s *WorkflowContractService) Create(ctx context.Context, req *pb.WorkflowContractServiceCreateRequest) (*pb.WorkflowContractServiceCreateResponse, error) {
	currentOrg, err := requireCurrentOrg(ctx)
	if err != nil {
		return nil, err
	}

	// Currently supporting only v1 version
	schema, err := s.contractUseCase.Create(ctx, &biz.WorkflowContractCreateOpts{OrgID: currentOrg.ID, Name: req.Name, Schema: req.GetV1()})
	if err != nil {
		return nil, handleUseCaseErr("contract", err, s.log)
	}

	return &pb.WorkflowContractServiceCreateResponse{Result: bizWorkFlowContractToPb(schema)}, nil
}

func (s *WorkflowContractService) Update(ctx context.Context, req *pb.WorkflowContractServiceUpdateRequest) (*pb.WorkflowContractServiceUpdateResponse, error) {
	currentOrg, err := requireCurrentOrg(ctx)
	if err != nil {
		return nil, err
	}

	schemaWithVersion, err := s.contractUseCase.Update(ctx, currentOrg.ID, req.GetId(), req.GetName(), req.GetV1())
	if err != nil {
		return nil, handleUseCaseErr("contract", err, s.log)
	}

	result := &pb.WorkflowContractServiceUpdateResponse_Result{
		Contract: bizWorkFlowContractToPb(schemaWithVersion.Contract),
		Revision: bizWorkFlowContractVersionToPb(schemaWithVersion.Version),
	}

	return &pb.WorkflowContractServiceUpdateResponse{Result: result}, nil
}

func (s *WorkflowContractService) Delete(ctx context.Context, req *pb.WorkflowContractServiceDeleteRequest) (*pb.WorkflowContractServiceDeleteResponse, error) {
	currentOrg, err := requireCurrentOrg(ctx)
	if err != nil {
		return nil, err
	}

	if err := s.contractUseCase.Delete(ctx, currentOrg.ID, req.Id); err != nil {
		return nil, handleUseCaseErr("contract", err, s.log)
	}

	return &pb.WorkflowContractServiceDeleteResponse{}, nil
}

func bizWorkFlowContractToPb(schema *biz.WorkflowContract) *pb.WorkflowContractItem {
	return &pb.WorkflowContractItem{
		Id:             schema.ID.String(),
		CreatedAt:      timestamppb.New(*schema.CreatedAt),
		Name:           schema.Name,
		LatestRevision: int32(schema.LatestRevision),
		WorkflowIds:    schema.WorkflowIDs,
	}
}

func bizWorkFlowContractVersionToPb(schema *biz.WorkflowContractVersion) *pb.WorkflowContractVersionItem {
	return &pb.WorkflowContractVersionItem{
		Id:        schema.ID.String(),
		CreatedAt: timestamppb.New(*schema.CreatedAt),
		Revision:  int32(schema.Revision),
		Contract: &pb.WorkflowContractVersionItem_V1{
			V1: schema.BodyV1,
		},
	}
}
