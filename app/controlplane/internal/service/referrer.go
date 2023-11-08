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

type ReferrerService struct {
	pb.UnimplementedReferrerServiceServer
	*service

	referrerUC *biz.ReferrerUseCase
}

func NewReferrerService(uc *biz.ReferrerUseCase, opts ...NewOpt) *ReferrerService {
	return &ReferrerService{
		service:    newService(opts...),
		referrerUC: uc,
	}
}

func (s *ReferrerService) Discover(ctx context.Context, req *pb.ReferrerServiceDiscoverRequest) (*pb.ReferrerServiceDiscoverResponse, error) {
	currentUser, _, err := loadCurrentUserAndOrg(ctx)
	if err != nil {
		return nil, err
	}

	res, err := s.referrerUC.GetFromRoot(ctx, req.GetDigest(), currentUser.ID)
	if err != nil {
		return nil, handleUseCaseErr("referrer discovery", err, s.log)
	}

	return &pb.ReferrerServiceDiscoverResponse{
		Result: bizReferrerToPb(res),
	}, nil
}

func bizReferrerToPb(r *biz.StoredReferrer) *pb.ReferrerItem {
	item := &pb.ReferrerItem{
		Digest:       r.Digest,
		Downloadable: r.Downloadable,
		ArtifactType: r.ArtifactType,
		CreatedAt:    timestamppb.New(*r.CreatedAt),
	}

	for _, r := range r.References {
		item.References = append(item.References, bizReferrerToPb(r))
	}

	return item
}
