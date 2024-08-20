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

	pb "github.com/chainloop-dev/chainloop/app/controlplane/api/controlplane/v1"
	"github.com/chainloop-dev/chainloop/app/controlplane/internal/usercontext"
	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/biz"
	errors "github.com/go-kratos/kratos/v2/errors"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
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
		return nil, handleUseCaseErr(err, s.log)
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

	contract, err := s.contractUseCase.FindByNameInOrg(ctx, currentOrg.ID, req.GetName())
	if err != nil {
		return nil, handleUseCaseErr(err, s.log)
	} else if contract == nil {
		return nil, errors.NotFound("not found", "contract not found")
	}

	contractWithVersion, err := s.contractUseCase.Describe(ctx, currentOrg.ID, contract.ID.String(), int(req.GetRevision()))
	if err != nil {
		return nil, handleUseCaseErr(err, s.log)
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

	token, err := usercontext.GetRawToken(ctx)
	if err != nil {
		return nil, status.Error(codes.Unauthenticated, "invalid token")
	}

	if err = s.contractUseCase.ValidateContractPolicies(req.RawContract, token); err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid policies")
	}

	// Currently supporting only v1 version
	schema, err := s.contractUseCase.Create(ctx, &biz.WorkflowContractCreateOpts{
		OrgID: currentOrg.ID,
		Name:  req.Name, Description: req.Description,
		RawSchema: req.RawContract})
	if err != nil {
		return nil, handleUseCaseErr(err, s.log)
	}

	return &pb.WorkflowContractServiceCreateResponse{Result: bizWorkFlowContractToPb(schema)}, nil
}

func (s *WorkflowContractService) Update(ctx context.Context, req *pb.WorkflowContractServiceUpdateRequest) (*pb.WorkflowContractServiceUpdateResponse, error) {
	currentOrg, err := requireCurrentOrg(ctx)
	if err != nil {
		return nil, err
	}

	token, err := usercontext.GetRawToken(ctx)
	if err != nil {
		return nil, status.Error(codes.Unauthenticated, "invalid token")
	}

	if err = s.contractUseCase.ValidateContractPolicies(req.RawContract, token); err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid policies")
	}

	schemaWithVersion, err := s.contractUseCase.Update(ctx, currentOrg.ID, req.Name,
		&biz.WorkflowContractUpdateOpts{
			Description: req.Description,
			RawSchema:   req.RawContract,
		})
	if err != nil {
		return nil, handleUseCaseErr(err, s.log)
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

	contract, err := s.contractUseCase.FindByNameInOrg(ctx, currentOrg.ID, req.GetName())
	if err != nil {
		return nil, handleUseCaseErr(err, s.log)
	} else if contract == nil {
		return nil, errors.NotFound("not found", "contract not found")
	}

	if err := s.contractUseCase.Delete(ctx, currentOrg.ID, contract.ID.String()); err != nil {
		return nil, handleUseCaseErr(err, s.log)
	}

	return &pb.WorkflowContractServiceDeleteResponse{}, nil
}

func bizWorkFlowContractToPb(schema *biz.WorkflowContract) *pb.WorkflowContractItem {
	return &pb.WorkflowContractItem{
		Id:             schema.ID.String(),
		CreatedAt:      timestamppb.New(*schema.CreatedAt),
		Name:           schema.Name,
		LatestRevision: int32(schema.LatestRevision),
		WorkflowNames:  schema.WorkflowNames,
		Description:    schema.Description,
	}
}

func bizWorkFlowContractVersionToPb(schema *biz.WorkflowContractVersion) *pb.WorkflowContractVersionItem {
	formatTranslator := func(biz.ContractRawFormat) pb.WorkflowContractVersionItem_RawBody_Format {
		switch schema.Schema.Format {
		case biz.ContractRawFormatJSON:
			return pb.WorkflowContractVersionItem_RawBody_FORMAT_JSON
		case biz.ContractRawFormatYAML:
			return pb.WorkflowContractVersionItem_RawBody_FORMAT_YAML
		case biz.ContractRawFormatCUE:
			return pb.WorkflowContractVersionItem_RawBody_FORMAT_CUE
		}

		return pb.WorkflowContractVersionItem_RawBody_FORMAT_UNSPECIFIED
	}

	return &pb.WorkflowContractVersionItem{
		Id:        schema.ID.String(),
		CreatedAt: timestamppb.New(*schema.CreatedAt),
		Revision:  int32(schema.Revision),
		Contract: &pb.WorkflowContractVersionItem_V1{
			V1: schema.Schema.Schema,
		},
		RawContract: &pb.WorkflowContractVersionItem_RawBody{
			Body:   schema.Schema.Raw,
			Format: formatTranslator(schema.Schema.Format),
		},
	}
}
