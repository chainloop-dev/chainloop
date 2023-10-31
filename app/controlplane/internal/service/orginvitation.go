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
	user, _, err := loadCurrentUserAndOrg(ctx)
	if err != nil {
		return nil, err
	}

	// Validations and rbac checks are done in the biz layer
	i, err := s.useCase.Create(ctx, req.OrganizationId, user.ID, req.ReceiverEmail)
	if err != nil {
		return nil, handleUseCaseErr("invitation", err, s.log)
	}

	return &pb.OrgInvitationServiceCreateResponse{Result: bizInvitationToPB(i)}, nil
}

func (s *OrgInvitationService) Revoke(ctx context.Context, req *pb.OrgInvitationServiceRevokeRequest) (*pb.OrgInvitationServiceRevokeResponse, error) {
	user, _, err := loadCurrentUserAndOrg(ctx)
	if err != nil {
		return nil, err
	}

	if err := s.useCase.Revoke(ctx, user.ID, req.Id); err != nil {
		return nil, handleUseCaseErr("invitation", err, s.log)
	}

	return &pb.OrgInvitationServiceRevokeResponse{}, nil
}

func (s *OrgInvitationService) ListSent(ctx context.Context, _ *pb.OrgInvitationServiceListSentRequest) (*pb.OrgInvitationServiceListSentResponse, error) {
	user, _, err := loadCurrentUserAndOrg(ctx)
	if err != nil {
		return nil, err
	}

	invitations, err := s.useCase.ListBySender(ctx, user.ID)
	if err != nil {
		return nil, handleUseCaseErr("invitation", err, s.log)
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
	}
}
