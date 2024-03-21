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
	"fmt"

	pb "github.com/chainloop-dev/chainloop/app/controlplane/api/controlplane/v1"
	"github.com/chainloop-dev/chainloop/app/controlplane/internal/biz"
	"github.com/google/uuid"
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

func (s *ReferrerService) DiscoverPrivate(ctx context.Context, req *pb.ReferrerServiceDiscoverPrivateRequest) (*pb.ReferrerServiceDiscoverPrivateResponse, error) {
	currentUser, currentToken, err := requireCurrentUserOrAPIToken(ctx)
	if err != nil {
		return nil, err
	}

	currentOrg, err := requireCurrentOrg(ctx)
	if err != nil {
		return nil, err
	}

	// if we are logged in as user we find the referrer from the user
	// otherwise for the current organization associated with the API token
	var referrer *biz.StoredReferrer
	if currentUser != nil {
		referrer, err = s.referrerUC.GetFromRootUser(ctx, req.GetDigest(), req.GetKind(), currentUser.ID)
	} else if currentToken != nil {
		var orgUUID uuid.UUID
		orgUUID, err = uuid.Parse(currentOrg.ID)
		if err != nil {
			return nil, fmt.Errorf("invalid org UUID: %w", err)
		}

		referrer, err = s.referrerUC.GetFromRoot(ctx, req.GetDigest(), req.GetKind(), []uuid.UUID{orgUUID})
	}
	if err != nil {
		return nil, handleUseCaseErr(err, s.log)
	}

	return &pb.ReferrerServiceDiscoverPrivateResponse{
		Result: bizReferrerToPb(referrer),
	}, nil
}

func (s *ReferrerService) DiscoverPublicShared(ctx context.Context, req *pb.DiscoverPublicSharedRequest) (*pb.DiscoverPublicSharedResponse, error) {
	res, err := s.referrerUC.GetFromRootInPublicSharedIndex(ctx, req.GetDigest(), req.GetKind())
	if err != nil {
		return nil, handleUseCaseErr(err, s.log)
	}

	return &pb.DiscoverPublicSharedResponse{
		Result: bizReferrerToPb(res),
	}, nil
}

func bizReferrerToPb(r *biz.StoredReferrer) *pb.ReferrerItem {
	item := &pb.ReferrerItem{
		Digest:       r.Digest,
		Downloadable: r.Downloadable,
		Public:       r.InPublicWorkflow,
		Kind:         r.Kind,
		CreatedAt:    timestamppb.New(*r.CreatedAt),
		Metadata:     r.Metadata,
		Annotations:  r.Annotations,
	}

	for _, r := range r.References {
		item.References = append(item.References, bizReferrerToPb(r))
	}

	return item
}
