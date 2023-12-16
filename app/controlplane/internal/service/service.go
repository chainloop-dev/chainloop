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
	NewOrgInvitationService,
	NewReferrerService,
	NewAPITokenService,
	wire.Struct(new(NewWorkflowRunServiceOpts), "*"),
	wire.Struct(new(NewAttestationServiceOpts), "*"),
)

func requireCurrentUser(ctx context.Context) (*usercontext.User, error) {
	currentUser := usercontext.CurrentUser(ctx)
	if currentUser == nil {
		return nil, errors.NotFound("not found", "logged in user")
	}

	return currentUser, nil
}

func requireAPIToken(ctx context.Context) (*usercontext.APIToken, error) {
	token := usercontext.CurrentAPIToken(ctx)
	if token == nil {
		return nil, errors.NotFound("not found", "API token")
	}

	return token, nil
}

func requireCurrentUserOrAPIToken(ctx context.Context) (*usercontext.User, *usercontext.APIToken, error) {
	user, err := requireCurrentUser(ctx)
	if err != nil && !errors.IsNotFound(err) {
		return nil, nil, err
	}

	apiToken, err := requireAPIToken(ctx)
	if err != nil && !errors.IsNotFound(err) {
		return nil, nil, err
	}

	// NOTE: we shouldn't get to this point since the middleware should have already catched this
	// Adding the check here for defensivity and testing purposes
	if user == nil && apiToken == nil {
		return nil, nil, errors.Forbidden("authz required", "logged in user nor API token found")
	}

	return user, apiToken, nil
}

func requireCurrentOrg(ctx context.Context) (*usercontext.Org, error) {
	currentOrg := usercontext.CurrentOrg(ctx)
	if currentOrg == nil {
		return nil, errors.NotFound("not found", "current organization not set")
	}

	return currentOrg, nil
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
	case biz.IsErrValidation(err) || biz.IsErrInvalidUUID(err):
		return errors.BadRequest(fmt.Sprintf("invalid %s", entity), err.Error())
	case biz.IsNotFound(err):
		return errors.NotFound(fmt.Sprintf("%s not found", entity), err.Error())
	case biz.IsErrUnauthorized(err):
		return errors.Forbidden(fmt.Sprintf("unauthorized %s", entity), err.Error())
	default:
		return servicelogger.LogAndMaskErr(err, l)
	}
}
