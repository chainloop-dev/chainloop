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
	backend "github.com/chainloop-dev/chainloop/internal/blobmanager"
	"github.com/go-kratos/kratos/v2/errors"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type CASBackendService struct {
	pb.UnimplementedCASBackendServiceServer
	*service

	uc        *biz.CASBackendUseCase
	providers backend.Providers
}

func NewCASBackendService(uc *biz.CASBackendUseCase, providers backend.Providers, opts ...NewOpt) *CASBackendService {
	return &CASBackendService{
		service:   newService(opts...),
		uc:        uc,
		providers: providers,
	}
}

func (s *CASBackendService) List(ctx context.Context, _ *pb.CASBackendServiceListRequest) (*pb.CASBackendServiceListResponse, error) {
	currentOrg, err := requireCurrentOrg(ctx)
	if err != nil {
		return nil, err
	}

	backends, err := s.uc.List(ctx, currentOrg.ID)
	if err != nil {
		return nil, handleUseCaseErr(err, s.log)
	}

	res := []*pb.CASBackendItem{}
	for _, backend := range backends {
		res = append(res, bizCASBackendToPb(backend))
	}

	return &pb.CASBackendServiceListResponse{Result: res}, nil
}

func (s *CASBackendService) Create(ctx context.Context, req *pb.CASBackendServiceCreateRequest) (*pb.CASBackendServiceCreateResponse, error) {
	currentOrg, err := requireCurrentOrg(ctx)
	if err != nil {
		return nil, err
	}

	backendP, ok := s.providers[req.Provider]
	if !ok {
		return nil, errors.BadRequest("invalid CAS backend", "invalid CAS backend")
	}

	credsJSON, err := req.Credentials.MarshalJSON()
	if err != nil {
		return nil, errors.BadRequest("invalid config", "config is invalid")
	}

	// Validate and extract the credentials so they can be stored in the next step
	creds, err := backendP.ValidateAndExtractCredentials(req.Location, credsJSON)
	if err != nil {
		return nil, errors.BadRequest("invalid config", err.Error())
	}

	// For now we only support one backend which is set as default
	res, err := s.uc.Create(ctx, currentOrg.ID, req.Name, req.Location, req.Description, biz.CASBackendProvider(req.Provider), creds, req.Default)
	if err != nil {
		return nil, handleUseCaseErr(err, s.log)
	}

	return &pb.CASBackendServiceCreateResponse{Result: bizCASBackendToPb(res)}, nil
}

func (s *CASBackendService) Update(ctx context.Context, req *pb.CASBackendServiceUpdateRequest) (*pb.CASBackendServiceUpdateResponse, error) {
	currentOrg, err := requireCurrentOrg(ctx)
	if err != nil {
		return nil, err
	}

	// find the backend to update
	backend, err := s.uc.FindByIDInOrg(ctx, currentOrg.ID, req.Id)
	if err != nil {
		return nil, handleUseCaseErr(err, s.log)
	}

	// if we are updating credentials we need to validate them
	// to do so we load a backend provider and call ValidateAndExtractCredentials
	var creds any
	if req.Credentials != nil {
		backendP, ok := s.providers[string(backend.Provider)]
		if !ok {
			return nil, errors.BadRequest("invalid CAS backend", "invalid CAS backend")
		}

		credsJSON, err := req.Credentials.MarshalJSON()
		if err != nil {
			return nil, errors.BadRequest("invalid config", "config is invalid")
		}

		// Validate and extract the credentials so they can be stored in the next step
		creds, err = backendP.ValidateAndExtractCredentials(backend.Location, credsJSON)
		if err != nil {
			return nil, errors.BadRequest("invalid config", err.Error())
		}
	}

	// For now we only support one backend which is set as default
	res, err := s.uc.Update(ctx, currentOrg.ID, req.Id, req.Description, creds, req.Default)
	if err != nil {
		return nil, handleUseCaseErr(err, s.log)
	}

	return &pb.CASBackendServiceUpdateResponse{Result: bizCASBackendToPb(res)}, nil
}

// Delete the CAS backend
func (s *CASBackendService) Delete(ctx context.Context, req *pb.CASBackendServiceDeleteRequest) (*pb.CASBackendServiceDeleteResponse, error) {
	currentOrg, err := requireCurrentOrg(ctx)
	if err != nil {
		return nil, err
	}

	// In fact we soft-delete the backend instead
	if err := s.uc.SoftDelete(ctx, currentOrg.ID, req.Id); err != nil {
		return nil, handleUseCaseErr(err, s.log)
	}

	return &pb.CASBackendServiceDeleteResponse{}, nil
}

func bizCASBackendToPb(in *biz.CASBackend) *pb.CASBackendItem {
	r := &pb.CASBackendItem{
		Id: in.ID.String(), Location: in.Location, Description: in.Description,
		Name:        in.Name,
		CreatedAt:   timestamppb.New(*in.CreatedAt),
		ValidatedAt: timestamppb.New(*in.ValidatedAt),
		Provider:    string(in.Provider),
		Default:     in.Default,
		IsInline:    in.Inline,
	}

	if in.Limits != nil {
		r.Limits = &pb.CASBackendItem_Limits{
			MaxBytes: in.Limits.MaxBytes,
		}
	}

	switch in.ValidationStatus {
	case biz.CASBackendValidationOK:
		r.ValidationStatus = pb.CASBackendItem_VALIDATION_STATUS_OK
	case biz.CASBackendValidationFailed:
		r.ValidationStatus = pb.CASBackendItem_VALIDATION_STATUS_INVALID
	}

	return r
}
