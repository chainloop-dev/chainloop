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

	backend "github.com/chainloop-dev/chainloop/internal/blobmanager"
	casJWT "github.com/chainloop-dev/chainloop/internal/robotaccount/cas"
	"github.com/chainloop-dev/chainloop/internal/servicelogger"
	kerrors "github.com/go-kratos/kratos/v2/errors"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/go-kratos/kratos/v2/middleware/auth/jwt"
	"github.com/google/wire"
)

// ProviderSet is service providers.
var ProviderSet = wire.NewSet(NewByteStreamService, NewResourceService, NewDownloadService)

type commonService struct {
	log      *log.Helper
	backends backend.Providers
}

func (s *commonService) selectProvider(id string) (backend.Provider, error) {
	// get the OCI provider from the map
	p, ok := s.backends[id]
	if !ok {
		return nil, fmt.Errorf("provider %s not found", id)
	}

	s.log.Infow("msg", "selected provider", "provider", id)
	return p, nil
}

type NewOpt func(s *commonService)

func WithLogger(logger log.Logger) NewOpt {
	return func(s *commonService) {
		s.log = servicelogger.ScopedHelper(logger, "service")
	}
}

func newCommonService(backends backend.Providers, opts ...NewOpt) *commonService {
	s := &commonService{
		log:      servicelogger.EmptyLogger(),
		backends: backends,
	}

	for _, opt := range opts {
		opt(s)
	}

	return s
}

// Extract the JWT claims from the context, note that the JWT verification has happened in the middleware
func infoFromAuth(ctx context.Context) (*casJWT.Claims, error) {
	rawClaims, ok := jwt.FromContext(ctx)
	if !ok {
		return nil, kerrors.Unauthorized("cas", "missing authentication information")
	}

	claims, ok := rawClaims.(*casJWT.Claims)
	if !ok {
		return nil, kerrors.Unauthorized("cas", "invalid authentication information")
	}

	return claims, nil
}
