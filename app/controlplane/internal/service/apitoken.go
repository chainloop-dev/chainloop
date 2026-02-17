//
// Copyright 2023-2026 The Chainloop Authors.
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
	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/authz"
	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/biz"
	errors "github.com/go-kratos/kratos/v2/errors"
	"github.com/google/uuid"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type APITokenService struct {
	pb.UnimplementedAPITokenServiceServer
	*service

	APITokenUseCase *biz.APITokenUseCase
}

func NewAPITokenService(uc *biz.APITokenUseCase, opts ...NewOpt) *APITokenService {
	return &APITokenService{
		service:         newService(opts...),
		APITokenUseCase: uc,
	}
}

func (s *APITokenService) Create(ctx context.Context, req *pb.APITokenServiceCreateRequest) (*pb.APITokenServiceCreateResponse, error) {
	currentOrg, err := requireCurrentOrg(ctx)
	if err != nil {
		return nil, err
	}

	// Authorization checks
	// Force setting a project scope if RBAC is enabled
	if rbacEnabled(ctx) && !req.ProjectReference.IsSet() {
		return nil, errors.BadRequest("invalid", "project is required")
	}

	// if the project is provided we make sure it exists and the user has permission to it
	var project *biz.Project
	if req.ProjectReference.IsSet() {
		// Make sure the provided project exists and the user has permission to create tokens in it
		project, err = s.userHasPermissionOnProject(ctx, currentOrg.ID, req.GetProjectReference(), authz.PolicyAPITokenCreate)
		if err != nil {
			return nil, err
		}
	}

	var expiresIn *time.Duration
	if req.ExpiresIn != nil {
		expiresIn = new(time.Duration)
		*expiresIn = req.ExpiresIn.AsDuration()
	}

	token, err := s.APITokenUseCase.Create(ctx, req.Name, req.Description, expiresIn, &currentOrg.ID, biz.APITokenWithProject(project))
	if err != nil {
		return nil, handleUseCaseErr(err, s.log)
	}

	return &pb.APITokenServiceCreateResponse{
		Result: &pb.APITokenServiceCreateResponse_APITokenFull{
			Item: apiTokenBizToPb(token),
			Jwt:  token.JWT,
		},
	}, nil
}

func (s *APITokenService) List(ctx context.Context, req *pb.APITokenServiceListRequest) (*pb.APITokenServiceListResponse, error) {
	currentOrg, err := requireCurrentOrg(ctx)
	if err != nil {
		return nil, err
	}

	// default to visible projects for the user
	defaultProjectFilter := s.visibleProjects(ctx)
	// or the user has provided a project filter
	if req.Project.IsSet() {
		project, err := s.userHasPermissionOnProject(ctx, currentOrg.ID, req.GetProject(), authz.PolicyAPITokenList)
		if err != nil {
			return nil, err
		}

		defaultProjectFilter = []uuid.UUID{project.ID}
	}

	tokens, err := s.APITokenUseCase.List(ctx, currentOrg.ID, biz.WithAPITokenStatusFilter(mapTokenStatusFilter(req.GetStatusFilter())), biz.WithAPITokenProjectFilter(defaultProjectFilter), biz.WithAPITokenScope(mapTokenScope(req.Scope)))
	if err != nil {
		return nil, handleUseCaseErr(err, s.log)
	}

	result := make([]*pb.APITokenItem, 0, len(tokens))
	for _, p := range tokens {
		result = append(result, apiTokenBizToPb(p))
	}

	return &pb.APITokenServiceListResponse{Result: result}, nil
}

func mapTokenScope(scope pb.APITokenServiceListRequest_Scope) biz.APITokenScope {
	switch scope {
	case pb.APITokenServiceListRequest_SCOPE_PROJECT:
		return biz.APITokenScopeProject
	case pb.APITokenServiceListRequest_SCOPE_GLOBAL:
		return biz.APITokenScopeGlobal
	}

	return ""
}

func mapTokenStatusFilter(f pb.APITokenServiceListRequest_StatusFilter) biz.APITokenStatusFilter {
	switch f {
	case pb.APITokenServiceListRequest_STATUS_FILTER_REVOKED:
		return biz.APITokenStatusFilterRevoked
	case pb.APITokenServiceListRequest_STATUS_FILTER_ALL:
		return biz.APITokenStatusFilterAll
	default:
		return biz.APITokenStatusFilterActive
	}
}

func (s *APITokenService) Revoke(ctx context.Context, req *pb.APITokenServiceRevokeRequest) (*pb.APITokenServiceRevokeResponse, error) {
	currentOrg, err := requireCurrentOrg(ctx)
	if err != nil {
		return nil, err
	}

	t, err := s.APITokenUseCase.FindByIDInOrg(ctx, currentOrg.ID, req.GetId())
	if err != nil {
		return nil, handleUseCaseErr(err, s.log)
	}

	// 1 - Only admins can manage global contracts
	if t.ProjectID == nil && rbacEnabled(ctx) {
		return nil, errors.BadRequest("invalid", "you can not manage a global API token")
	}

	// Make sure the user has permission to revoke the token in the project
	if t.ProjectID != nil {
		if err := s.authorizeResource(ctx, authz.PolicyAPITokenRevoke, authz.ResourceTypeProject, *t.ProjectID); err != nil {
			return nil, err
		}
	}

	if err := s.APITokenUseCase.Revoke(ctx, currentOrg.ID, t.ID.String()); err != nil {
		return nil, handleUseCaseErr(err, s.log)
	}

	return &pb.APITokenServiceRevokeResponse{}, nil
}

func apiTokenBizToPb(in *biz.APIToken) *pb.APITokenItem {
	res := &pb.APITokenItem{
		Id:               in.ID.String(),
		Name:             in.Name,
		OrganizationId:   in.OrganizationID.String(),
		OrganizationName: in.OrganizationName,
		Description:      in.Description,
		CreatedAt:        timestamppb.New(*in.CreatedAt),
	}

	if in.ExpiresAt != nil {
		res.ExpiresAt = timestamppb.New(*in.ExpiresAt)
	}

	if in.RevokedAt != nil {
		res.RevokedAt = timestamppb.New(*in.RevokedAt)
	}

	if in.LastUsedAt != nil {
		res.LastUsedAt = timestamppb.New(*in.LastUsedAt)
	}

	if in.ProjectID != nil {
		res.ScopedEntity = &pb.ScopedEntity{
			Type: string(biz.ContractScopeProject),
			Id:   in.ProjectID.String(),
			Name: *in.ProjectName,
		}
	}

	return res
}
