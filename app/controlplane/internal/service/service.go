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
	"io"

	"github.com/chainloop-dev/chainloop/app/controlplane/internal/biz"
	"github.com/chainloop-dev/chainloop/app/controlplane/internal/usercontext"
	"github.com/chainloop-dev/chainloop/internal/servicelogger"
	errors "github.com/go-kratos/kratos/v2/errors"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/google/wire"
)

// ProviderSet is service providers.
var ProviderSet = wire.NewSet(
	NewWorkflowService,
	NewAuthService,
	NewRobotAccountService,
	NewWorkflowRunService,
	NewAttestationService,
	NewWorkflowSchemaService,
	NewCASCredentialsService,
	NewContextService,
	NewOrgMetricsService,
	NewIntegrationsService,
	NewCASBackendService,
	NewCASRedirectService,
	NewOrganizationService,
	NewOrgInviteService,
	wire.Struct(new(NewWorkflowRunServiceOpts), "*"),
	wire.Struct(new(NewAttestationServiceOpts), "*"),
)

func loadCurrentUserAndOrg(ctx context.Context) (*usercontext.User, *usercontext.Org, error) {
	currentUser, currentOrg := usercontext.CurrentUser(ctx), usercontext.CurrentOrg(ctx)
	if currentUser == nil || currentOrg == nil {
		return nil, nil, errors.NotFound("not found", "logged in user and org not found")
	}

	return currentUser, currentOrg, nil
}

func newService(opts ...NewOpt) *service {
	s := &service{
		log: log.NewHelper(log.NewStdLogger(io.Discard)),
	}

	for _, opt := range opts {
		opt(s)
	}

	return s
}

type service struct {
	log *log.Helper
}

type NewOpt func(s *service)

func WithLogger(logger log.Logger) NewOpt {
	return func(s *service) {
		s.log = servicelogger.ScopedHelper(logger, "service")
	}
}

func handleUseCaseErr(entity string, err error, l *log.Helper) error {
	switch {
	case biz.IsErrValidation(err):
		return errors.BadRequest(fmt.Sprintf("invalid %s", entity), err.Error())
	case biz.IsNotFound(err):
		return errors.NotFound(fmt.Sprintf("%s not found", entity), err.Error())
	case biz.IsErrUnauthorized(err):
		return errors.Forbidden(fmt.Sprintf("unauthorized %s", entity), err.Error())
	default:
		return servicelogger.LogAndMaskErr(err, l)
	}
}
