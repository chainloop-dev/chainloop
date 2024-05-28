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

	v1 "github.com/chainloop-dev/chainloop/app/artifact-cas/api/cas/v1"
	backend "github.com/chainloop-dev/chainloop/internal/blobmanager"
	sl "github.com/chainloop-dev/chainloop/pkg/servicelogger"
	"github.com/go-kratos/kratos/v2/errors"
)

type ResourceService struct {
	v1.UnimplementedResourceServiceServer
	*commonService
}

func NewResourceService(bp backend.Providers, opts ...NewOpt) *ResourceService {
	return &ResourceService{
		commonService: newCommonService(bp, opts...),
	}
}

// Return the metadata if an artifact referenced by its content digest
func (s *ResourceService) Describe(ctx context.Context, req *v1.ResourceServiceDescribeRequest) (*v1.ResourceServiceDescribeResponse, error) {
	info, err := infoFromAuth(ctx)
	if err != nil {
		return nil, err
	}

	b, err := s.loadBackend(ctx, info.BackendType, info.StoredSecretID)
	if err != nil && errors.IsNotFound(err) {
		return nil, err
	} else if err != nil {
		return nil, sl.LogAndMaskErr(err, s.log)
	}

	res, err := b.Describe(ctx, req.Digest)
	if err != nil && backend.IsNotFound(err) {
		return nil, errors.NotFound("not found", err.Error())
	} else if err != nil {
		return nil, sl.LogAndMaskErr(err, s.log)
	}

	return &v1.ResourceServiceDescribeResponse{
		Result: &v1.CASResource{Digest: res.Digest, FileName: res.FileName, Size: res.Size},
	}, nil
}
