//
// Copyright 2024-2025 The Chainloop Authors.
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
	"github.com/chainloop-dev/chainloop/app/controlplane/internal/usercontext/entities"
	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/biz"
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
	currentUser, currentAPIToken, err := requireCurrentUserOrAPIToken(ctx)
	if err != nil {
		return nil, err
	}

	if currentUser == nil && currentAPIToken == nil {
		return nil, errors.NotFound("not found", "logged in user")
	}

	res := &pb.ContextServiceCurrentResponse_Result{}

	// Load current org if available since it can be a user with no org
	// This is the case for API tokens
	currentOrg, err := requireCurrentOrg(ctx)
	if err != nil && !errors.IsNotFound(err) {
		return nil, err
	}

	// Load user/API token info
	if currentAPIToken != nil {
		res.CurrentUser = &pb.User{
			Id: currentAPIToken.ID, Email: "API-token@chainloop", CreatedAt: timestamppb.New(*currentAPIToken.CreatedAt),
		}
	} else if currentUser != nil {
		res.CurrentUser = &pb.User{
			Id: currentUser.ID, Email: currentUser.Email, CreatedAt: timestamppb.New(*currentUser.CreatedAt),
		}

		// For regular users, we need to load the membership manually
		// NOTE that we are not using the middleware here because we want to handle the case
		// when there is no organization or membership gracefully
		orgName, err := entities.GetOrganizationNameFromHeader(ctx)
		if err != nil {
			return nil, handleUseCaseErr(err, s.log)
		}

		// It might not be set in the header, so we load it from the DB
		if orgName == "" {
			membership, err := s.userUC.CurrentMembership(ctx, currentUser.ID)
			if err != nil && !biz.IsNotFound(err) {
				return nil, handleUseCaseErr(err, s.log)
			} else if membership != nil {
				orgName = membership.Org.Name
			}
		}

		if orgName != "" {
			m, err := s.userUC.MembershipInOrg(ctx, currentUser.ID, orgName)
			if err != nil && !biz.IsNotFound(err) {
				return nil, handleUseCaseErr(err, s.log)
			} else if err != nil {
				return nil, pb.ErrorUserNotMemberOfOrgErrorNotInOrg("user is not a member of organization %s", orgName)
			}

			res.CurrentMembership = bizMembershipToPb(m)
			currentOrg = &entities.Org{Name: m.Org.Name, ID: m.Org.ID, CreatedAt: m.CreatedAt}
		}
	}

	if currentOrg != nil {
		// Add cas backend
		backend, err := s.uc.FindDefaultBackend(ctx, currentOrg.ID)
		if err != nil && !biz.IsNotFound(err) {
			return nil, handleUseCaseErr(err, s.log)
		}

		if backend != nil {
			res.CurrentCasBackend = bizCASBackendToPb(backend)
		}
	}

	return &pb.ContextServiceCurrentResponse{Result: res}, nil
}

func bizOrgToPb(m *biz.Organization) *pb.OrgItem {
	return &pb.OrgItem{Id: m.ID, Name: m.Name, CreatedAt: timestamppb.New(*m.CreatedAt), DefaultPolicyViolationStrategy: bizPolicyViolationBlockingStrategyToPb(m.BlockOnPolicyViolation)}
}

func bizUserToPb(u *biz.User) *pb.User {
	return &pb.User{Id: u.ID, Email: u.Email, CreatedAt: timestamppb.New(*u.CreatedAt)}
}

func bizPolicyViolationBlockingStrategyToPb(blockOnPolicyViolation bool) pb.OrgItem_PolicyViolationBlockingStrategy {
	if blockOnPolicyViolation {
		return pb.OrgItem_POLICY_VIOLATION_BLOCKING_STRATEGY_BLOCK
	}

	return pb.OrgItem_POLICY_VIOLATION_BLOCKING_STRATEGY_ADVISORY
}
