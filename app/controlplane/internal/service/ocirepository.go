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
	"github.com/chainloop-dev/chainloop/internal/blobmanager/oci"
	"github.com/chainloop-dev/chainloop/internal/ociauth"
	sl "github.com/chainloop-dev/chainloop/internal/servicelogger"
	errors "github.com/go-kratos/kratos/v2/errors"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type OCIRepositoryService struct {
	pb.UnimplementedOCIRepositoryServiceServer
	*service

	uc *biz.OCIRepositoryUseCase
}

func NewOCIRepositoryService(uc *biz.OCIRepositoryUseCase, opts ...NewOpt) *OCIRepositoryService {
	return &OCIRepositoryService{
		service: newService(opts...),
		uc:      uc,
	}
}

func (s *OCIRepositoryService) Save(ctx context.Context, req *pb.OCIRepositoryServiceSaveRequest) (*pb.OCIRepositoryServiceSaveResponse, error) {
	_, currentOrg, err := loadCurrentUserAndOrg(ctx)
	if err != nil {
		return nil, err
	}

	username := req.GetKeyPair().Username
	password := req.GetKeyPair().Password

	// Create and validate credentials
	k, err := ociauth.NewCredentials(req.Repository, username, password)
	if err != nil {
		return nil, errors.BadRequest("wrong credentials", err.Error())
	}

	// Check credentials
	b, err := oci.NewBackend(req.Repository, &oci.RegistryOptions{Keychain: k})
	if err != nil {
		return nil, sl.LogAndMaskErr(err, s.log)
	}

	if err := b.CheckWritePermissions(context.TODO()); err != nil {
		s.log.Error(err)
		return nil, errors.BadRequest("wrong credentials", "the provided registry credentials are invalid")
	}

	_, err = s.uc.CreateOrUpdate(ctx, currentOrg.ID, req.Repository, username, password)
	if err != nil {
		return nil, sl.LogAndMaskErr(err, s.log)
	}

	return &pb.OCIRepositoryServiceSaveResponse{}, nil
}

func bizOCIRepoToPb(repo *biz.OCIRepository) *pb.OCIRepositoryItem {
	r := &pb.OCIRepositoryItem{
		Id: repo.ID, Repo: repo.Repo, CreatedAt: timestamppb.New(*repo.CreatedAt),
	}

	switch repo.ValidationStatus {
	case biz.OCIRepoValidationOK:
		r.ValidationStatus = pb.OCIRepositoryItem_VALIDATION_STATUS_OK
	case biz.OCIRepoValidationFailed:
		r.ValidationStatus = pb.OCIRepositoryItem_VALIDATION_STATUS_INVALID
	}

	return r
}
