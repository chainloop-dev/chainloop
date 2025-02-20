//
// Copyright 2024 The Chainloop Authors.
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

	v1 "github.com/chainloop-dev/chainloop/app/controlplane/api/controlplane/v1"
	"github.com/chainloop-dev/chainloop/app/controlplane/internal/usercontext"
	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/biz"
	"github.com/go-kratos/kratos/v2/errors"
)

type SigningService struct {
	v1.UnimplementedSigningServiceServer
	*service

	signing *biz.SigningUseCase
}

var _ v1.SigningServiceServer = (*SigningService)(nil)

func NewSigningService(signing *biz.SigningUseCase, opts ...NewOpt) *SigningService {
	return &SigningService{
		service: newService(opts...),
		signing: signing,
	}
}

func (s *SigningService) GenerateSigningCert(ctx context.Context, req *v1.GenerateSigningCertRequest) (*v1.GenerateSigningCertResponse, error) {
	ra := usercontext.CurrentRobotAccount(ctx)
	if ra == nil {
		return nil, errors.Unauthorized("missing org", "authentication data is required")
	}

	certs, err := s.signing.CreateSigningCert(ctx, ra.OrgID, req.GetCertificateSigningRequest())
	if err != nil {
		return nil, handleUseCaseErr(err, s.log)
	}

	return &v1.GenerateSigningCertResponse{Chain: &v1.CertificateChain{Certificates: certs}}, nil
}

func (s *SigningService) GetTrustedRoot(ctx context.Context, _ *v1.GetTrustedRootRequest) (*v1.GetTrustedRootResponse, error) {
	tr, err := s.signing.GetTrustedRoot(ctx)
	if err != nil {
		return nil, handleUseCaseErr(err, s.log)
	}
	resp := &v1.GetTrustedRootResponse{Keys: make(map[string]*v1.CertificateChain), TimestampAuthorities: make(map[string]*v1.CertificateChain)}
	for k, v := range tr.Keys {
		resp.Keys[k] = &v1.CertificateChain{Certificates: v}
	}
	for k, v := range tr.TimestampAuthorities {
		resp.TimestampAuthorities[k] = &v1.CertificateChain{Certificates: v}
	}
	return resp, nil
}
