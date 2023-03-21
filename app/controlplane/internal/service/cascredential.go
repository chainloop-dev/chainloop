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
	sl "github.com/chainloop-dev/chainloop/internal/servicelogger"
	errors "github.com/go-kratos/kratos/v2/errors"

	"github.com/chainloop-dev/chainloop/app/controlplane/internal/biz"
	casJWT "github.com/chainloop-dev/chainloop/internal/robotaccount/cas"
)

type CASCredentialsService struct {
	*service
	pb.UnimplementedCASCredentialsServiceServer

	casUC *biz.CASCredentialsUseCase
	ociUC *biz.OCIRepositoryUseCase
}

func NewCASCredentialsService(casUC *biz.CASCredentialsUseCase, ociUC *biz.OCIRepositoryUseCase, opts ...NewOpt) *CASCredentialsService {
	return &CASCredentialsService{
		service: newService(opts...),
		casUC:   casUC,
		ociUC:   ociUC,
	}
}

// Get will generate temporary credentials to be used against the CAS service for the current organization
func (s *CASCredentialsService) Get(ctx context.Context, req *pb.CASCredentialsServiceGetRequest) (*pb.CASCredentialsServiceGetResponse, error) {
	_, currentOrg, err := loadCurrentUserAndOrg(ctx)
	if err != nil {
		return nil, err
	}

	var role casJWT.Role
	switch req.GetRole() {
	case pb.CASCredentialsServiceGetRequest_ROLE_DOWNLOADER:
		role = casJWT.Downloader
	case pb.CASCredentialsServiceGetRequest_ROLE_UPLOADER:
		role = casJWT.Uploader
	}

	// Get repository to provide the secret name
	repo, err := s.ociUC.FindMainRepo(ctx, currentOrg.ID)
	if err != nil {
		return nil, sl.LogAndMaskErr(err, s.log)
	}

	if repo == nil {
		return nil, errors.NotFound("not found", "main repository not found")
	}

	t, err := s.casUC.GenerateTemporaryCredentials(repo.SecretName, role)
	if err != nil {
		return nil, sl.LogAndMaskErr(err, s.log)
	}

	return &pb.CASCredentialsServiceGetResponse{
		Result: &pb.CASCredentialsServiceGetResponse_Result{Token: t},
	}, nil
}
