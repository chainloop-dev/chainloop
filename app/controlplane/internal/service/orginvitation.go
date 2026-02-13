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
	"github.com/google/uuid"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type OrgInvitationService struct {
	pb.UnimplementedOrgInvitationServiceServer
	*service

	useCase *biz.OrgInvitationUseCase
}

func NewOrgInvitationService(uc *biz.OrgInvitationUseCase, opts ...NewOpt) *OrgInvitationService {
	return &OrgInvitationService{
		service: newService(opts...),
		useCase: uc,
	}
}

func (s *OrgInvitationService) Create(ctx context.Context, req *pb.OrgInvitationServiceCreateRequest) (*pb.OrgInvitationServiceCreateResponse, error) {
	user, _, err := requireCurrentUserOrAPIToken(ctx)
	if err != nil {
		return nil, err
	}

	org, err := requireCurrentOrg(ctx)
	if err != nil {
		return nil, err
	}

	opts := []biz.InvitationCreateOpt{biz.WithInvitationRole(biz.PbRoleToBiz(req.Role))}
	if user != nil {
		userID, err := uuid.Parse(user.ID)
		if err != nil {
			return nil, handleUseCaseErr(err, s.log)
		}
		opts = append(opts, biz.WithSender(userID))
	}

	// Validations are done in the biz layer
	i, err := s.useCase.Create(ctx, org.ID, req.ReceiverEmail, opts...)
	if err != nil {
		return nil, handleUseCaseErr(err, s.log)
	}

	return &pb.OrgInvitationServiceCreateResponse{Result: bizInvitationToPB(i)}, nil
}

func (s *OrgInvitationService) Revoke(ctx context.Context, req *pb.OrgInvitationServiceRevokeRequest) (*pb.OrgInvitationServiceRevokeResponse, error) {
	org, err := requireCurrentOrg(ctx)
	if err != nil {
		return nil, err
	}

	if err := s.useCase.Revoke(ctx, org.ID, req.Id); err != nil {
		return nil, handleUseCaseErr(err, s.log)
	}

	return &pb.OrgInvitationServiceRevokeResponse{}, nil
}

func (s *OrgInvitationService) ListSent(ctx context.Context, _ *pb.OrgInvitationServiceListSentRequest) (*pb.OrgInvitationServiceListSentResponse, error) {
	org, err := requireCurrentOrg(ctx)
	if err != nil {
		return nil, err
	}

	invitations, err := s.useCase.ListByOrg(ctx, org.ID)
	if err != nil {
		return nil, handleUseCaseErr(err, s.log)
	}

	res := []*pb.OrgInvitationItem{}
	for _, invitation := range invitations {
		res = append(res, bizInvitationToPB(invitation))
	}

	return &pb.OrgInvitationServiceListSentResponse{Result: res}, nil
}

func bizInvitationToPB(e *biz.OrgInvitation) *pb.OrgInvitationItem {
	return &pb.OrgInvitationItem{
		Id: e.ID.String(), CreatedAt: timestamppb.New(*e.CreatedAt),
		ReceiverEmail: e.ReceiverEmail, Status: string(e.Status),
		Organization: bizOrgToPb(e.Org), Sender: bizUserToPb(e.Sender),
		Role: bizRoleToPb(e.Role),
	}
}
