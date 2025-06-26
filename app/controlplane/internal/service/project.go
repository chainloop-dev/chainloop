//
// Copyright 2025 The Chainloop Authors.
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
	"time"

	pb "github.com/chainloop-dev/chainloop/app/controlplane/api/controlplane/v1"
	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/biz"
)

type ProjectService struct {
	pb.UnimplementedProjectServiceServer
	*service

	APITokenUseCase *biz.APITokenUseCase
}

func NewProjectService(uc *biz.APITokenUseCase, opts ...NewOpt) *ProjectService {
	return &ProjectService{
		service:         newService(opts...),
		APITokenUseCase: uc,
	}
}

func (s *ProjectService) APITokenCreate(ctx context.Context, req *pb.ProjectServiceAPITokenCreateRequest) (*pb.ProjectServiceAPITokenCreateResponse, error) {
	currentOrg, err := requireCurrentOrg(ctx)
	if err != nil {
		return nil, err
	}

	var expiresIn *time.Duration
	if req.ExpiresIn != nil {
		expiresIn = new(time.Duration)
		*expiresIn = req.ExpiresIn.AsDuration()
	}

	token, err := s.APITokenUseCase.Create(ctx, req.Name, req.Description, expiresIn, currentOrg.ID)
	if err != nil {
		return nil, handleUseCaseErr(err, s.log)
	}

	return &pb.ProjectServiceAPITokenCreateResponse{
		Result: &pb.ProjectServiceAPITokenCreateResponse_APITokenFull{
			Item: apiTokenBizToPb(token),
			Jwt:  token.JWT,
		},
	}, nil
}

func (s *ProjectService) APITokenList(ctx context.Context, req *pb.ProjectServiceAPITokenListRequest) (*pb.ProjectServiceAPITokenListResponse, error) {
	currentOrg, err := requireCurrentOrg(ctx)
	if err != nil {
		return nil, err
	}

	tokens, err := s.APITokenUseCase.List(ctx, currentOrg.ID, req.IncludeRevoked)
	if err != nil {
		return nil, handleUseCaseErr(err, s.log)
	}

	result := make([]*pb.APITokenItem, 0, len(tokens))
	for _, p := range tokens {
		result = append(result, apiTokenBizToPb(p))
	}

	return &pb.ProjectServiceAPITokenListResponse{Result: result}, nil
}

func (s *ProjectService) APITokenRevoke(ctx context.Context, req *pb.ProjectServiceAPITokenRevokeRequest) (*pb.ProjectServiceAPITokenRevokeResponse, error) {
	currentOrg, err := requireCurrentOrg(ctx)
	if err != nil {
		return nil, err
	}

	t, err := s.APITokenUseCase.FindByNameInOrg(ctx, currentOrg.ID, req.Name)
	if err != nil {
		return nil, handleUseCaseErr(err, s.log)
	}

	if err := s.APITokenUseCase.Revoke(ctx, currentOrg.ID, t.ID.String()); err != nil {
		return nil, handleUseCaseErr(err, s.log)
	}

	return &pb.ProjectServiceAPITokenRevokeResponse{}, nil
}
