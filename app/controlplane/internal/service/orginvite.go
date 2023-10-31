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

type OrgInviteService struct {
	pb.UnimplementedOrgInviteServiceServer
	*service

	useCase *biz.OrgInviteUseCase
}

func NewOrgInviteService(uc *biz.OrgInviteUseCase, opts ...NewOpt) *OrgInviteService {
	return &OrgInviteService{
		service: newService(opts...),
		useCase: uc,
	}
}

func (s *OrgInviteService) Create(ctx context.Context, req *pb.OrgInviteServiceCreateRequest) (*pb.OrgInviteServiceCreateResponse, error) {
	user, _, err := loadCurrentUserAndOrg(ctx)
	if err != nil {
		return nil, err
	}

	// Validations and rbac checks are done in the biz layer
	i, err := s.useCase.Create(ctx, req.OrganizationId, user.ID, req.ReceiverEmail)
	if err != nil {
		return nil, handleUseCaseErr("invite", err, s.log)
	}

	return &pb.OrgInviteServiceCreateResponse{Result: bizInviteToPB(i)}, nil
}

func (s *OrgInviteService) Revoke(ctx context.Context, req *pb.OrgInviteServiceRevokeRequest) (*pb.OrgInviteServiceRevokeResponse, error) {
	user, _, err := loadCurrentUserAndOrg(ctx)
	if err != nil {
		return nil, err
	}

	if err := s.useCase.Revoke(ctx, user.ID, req.Id); err != nil {
		return nil, handleUseCaseErr("invite", err, s.log)
	}

	return &pb.OrgInviteServiceRevokeResponse{}, nil
}

func (s *OrgInviteService) ListSent(ctx context.Context, _ *pb.OrgInviteServiceListSentRequest) (*pb.OrgInviteServiceListSentResponse, error) {
	user, _, err := loadCurrentUserAndOrg(ctx)
	if err != nil {
		return nil, err
	}

	invites, err := s.useCase.ListBySender(ctx, user.ID)
	if err != nil {
		return nil, handleUseCaseErr("invite", err, s.log)
	}

	res := []*pb.OrgInviteItem{}
	for _, invite := range invites {
		res = append(res, bizInviteToPB(invite))
	}

	return &pb.OrgInviteServiceListSentResponse{Result: res}, nil
}

func bizInviteToPB(e *biz.OrgInvite) *pb.OrgInviteItem {
	return &pb.OrgInviteItem{
		Id: e.ID.String(), CreatedAt: timestamppb.New(*e.CreatedAt),
		ReceiverEmail: e.ReceiverEmail, Status: string(e.Status),
		Organization: bizOrgToPb(e.Org), Sender: bizUserToPb(e.Sender),
	}
}
