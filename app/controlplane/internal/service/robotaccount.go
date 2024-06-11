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
	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/biz"
	"github.com/go-kratos/kratos/v2/errors"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type RobotAccountService struct {
	pb.UnimplementedRobotAccountServiceServer
	*service

	robotAccountUseCase *biz.RobotAccountUseCase
}

func NewRobotAccountService(uc *biz.RobotAccountUseCase, opts ...NewOpt) *RobotAccountService {
	return &RobotAccountService{
		service:             newService(opts...),
		robotAccountUseCase: uc,
	}
}

func (s *RobotAccountService) Create(ctx context.Context, req *pb.RobotAccountServiceCreateRequest) (*pb.RobotAccountServiceCreateResponse, error) {
	currentOrg, err := requireCurrentOrg(ctx)
	if err != nil {
		return nil, err
	}

	robotAccount, err := s.robotAccountUseCase.Create(ctx, req.Name, currentOrg.ID, req.WorkflowId)
	if err != nil && biz.IsNotFound(err) {
		return nil, errors.NotFound("not found", err.Error())
	} else if err != nil {
		return nil, handleUseCaseErr(err, s.log)
	}

	return &pb.RobotAccountServiceCreateResponse{
		Result: &pb.RobotAccountServiceCreateResponse_RobotAccountFull{
			Id: robotAccount.ID.String(), Name: robotAccount.Name, WorkflowId: robotAccount.WorkflowID.String(), CreatedAt: timestamppb.New(*robotAccount.CreatedAt), Key: robotAccount.JWT,
		},
	}, nil
}

func (s *RobotAccountService) List(ctx context.Context, req *pb.RobotAccountServiceListRequest) (*pb.RobotAccountServiceListResponse, error) {
	currentOrg, err := requireCurrentOrg(ctx)
	if err != nil {
		return nil, err
	}

	robotAccounts, err := s.robotAccountUseCase.List(ctx, currentOrg.ID, req.WorkflowId, req.IncludeRevoked)
	if err != nil && biz.IsNotFound(err) {
		return nil, errors.NotFound("not found", err.Error())
	} else if err != nil {
		return nil, handleUseCaseErr(err, s.log)
	}

	result := make([]*pb.RobotAccountServiceListResponse_RobotAccountItem, 0, len(robotAccounts))
	for _, p := range robotAccounts {
		acc := &pb.RobotAccountServiceListResponse_RobotAccountItem{
			Id: p.ID.String(), WorkflowId: p.WorkflowID.String(),
			Name:      p.Name,
			CreatedAt: timestamppb.New(*p.CreatedAt),
		}

		if p.RevokedAt != nil {
			acc.RevokedAt = timestamppb.New(*p.RevokedAt)
		}

		result = append(result, acc)
	}

	return &pb.RobotAccountServiceListResponse{Result: result}, nil
}

func (s *RobotAccountService) Revoke(ctx context.Context, req *pb.RobotAccountServiceRevokeRequest) (*pb.RobotAccountServiceRevokeResponse, error) {
	currentOrg, err := requireCurrentOrg(ctx)
	if err != nil {
		return nil, err
	}

	if err := s.robotAccountUseCase.Revoke(ctx, currentOrg.ID, req.Id); err != nil {
		return nil, handleUseCaseErr(err, s.log)
	}

	return &pb.RobotAccountServiceRevokeResponse{}, nil
}
