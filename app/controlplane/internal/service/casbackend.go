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
	sl "github.com/chainloop-dev/chainloop/internal/servicelogger"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type CASBackendService struct {
	pb.UnimplementedCASBackendServiceServer
	*service

	uc *biz.CASBackendUseCase
}

func NewCASBackendService(uc *biz.CASBackendUseCase, opts ...NewOpt) *CASBackendService {
	return &CASBackendService{
		service: newService(opts...),
		uc:      uc,
	}
}

func (s *CASBackendService) List(ctx context.Context, _ *pb.CASBackendServiceListRequest) (*pb.CASBackendServiceListResponse, error) {
	_, currentOrg, err := loadCurrentUserAndOrg(ctx)
	if err != nil {
		return nil, err
	}

	backends, err := s.uc.List(ctx, currentOrg.ID)
	if err != nil {
		return nil, sl.LogAndMaskErr(err, s.log)
	}

	res := []*pb.CASBackendItem{}
	for _, backend := range backends {
		res = append(res, bizOCASBackendToPb(backend))
	}

	return &pb.CASBackendServiceListResponse{Result: res}, nil
}

func bizOCASBackendToPb(repo *biz.CASBackend) *pb.CASBackendItem {
	r := &pb.CASBackendItem{
		Id: repo.ID.String(), Name: repo.Name,
		CreatedAt:   timestamppb.New(*repo.CreatedAt),
		ValidatedAt: timestamppb.New(*repo.ValidatedAt),
		Provider:    string(repo.Provider),
		Default:     repo.Default,
	}

	switch repo.ValidationStatus {
	case biz.CASBackendValidationOK:
		r.ValidationStatus = pb.CASBackendItem_VALIDATION_STATUS_OK
	case biz.CASBackendValidationFailed:
		r.ValidationStatus = pb.CASBackendItem_VALIDATION_STATUS_INVALID
	}

	return r
}
