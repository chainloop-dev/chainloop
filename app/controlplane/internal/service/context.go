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

type ContextService struct {
	*service
	pb.UnimplementedContextServiceServer

	uc     *biz.CASBackendUseCase
	userUC *biz.UserUseCase
}

func NewContextService(repoUC *biz.CASBackendUseCase, uUC *biz.UserUseCase, opts ...NewOpt) *ContextService {
	return &ContextService{
		service: newService(opts...),
		uc:      repoUC,
		userUC:  uUC,
	}
}

func (s *ContextService) Current(ctx context.Context, _ *pb.ContextServiceCurrentRequest) (*pb.ContextServiceCurrentResponse, error) {
	currentOrg, err := requireCurrentOrg(ctx)
	if err != nil {
		return nil, err
	}

	currentUser, currentAPIToken, err := requireCurrentUserOrAPIToken(ctx)
	if err != nil {
		return nil, err
	}

	if currentUser == nil && currentAPIToken == nil {
		return nil, errors.NotFound("not found", "logged in user")
	}

	res := &pb.ContextServiceCurrentResponse_Result{}

	// Load user/API token info
	if currentAPIToken != nil {
		res.CurrentUser = &pb.User{
			Id: currentAPIToken.ID, Email: "API-token@chainloop", CreatedAt: timestamppb.New(*currentAPIToken.CreatedAt),
		}
	} else if currentUser != nil {
		res.CurrentUser = &pb.User{
			Id: currentUser.ID, Email: currentUser.Email, CreatedAt: timestamppb.New(*currentUser.CreatedAt),
		}
	}

	// Add cas backend
	backend, err := s.uc.FindDefaultBackend(ctx, currentOrg.ID)
	if err != nil && !biz.IsNotFound(err) {
		return nil, handleUseCaseErr(err, s.log)
	}

	if backend != nil {
		res.CurrentCasBackend = bizCASBackendToPb(backend)
	}

	// Optionally add current membership
	if currentUser != nil {
		m, err := s.userUC.CurrentMembership(ctx, currentUser.ID)
		if err != nil {
			return nil, handleUseCaseErr(err, s.log)
		}

		if m != nil {
			res.CurrentMembership = bizMembershipToPb(m)
		}
	}

	return &pb.ContextServiceCurrentResponse{Result: res}, nil
}

func bizOrgToPb(m *biz.Organization) *pb.OrgItem {
	return &pb.OrgItem{Id: m.ID, Name: m.Name, CreatedAt: timestamppb.New(*m.CreatedAt)}
}

func bizUserToPb(u *biz.User) *pb.User {
	return &pb.User{Id: u.ID, Email: u.Email, CreatedAt: timestamppb.New(*u.CreatedAt)}
}
