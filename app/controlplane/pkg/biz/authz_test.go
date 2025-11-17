//
// Copyright 2025 The Chainloop Authors.
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

package biz_test

import (
	"context"
	"errors"
	"io"
	"testing"

	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/authz"
	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/biz"
	bizMocks "github.com/chainloop-dev/chainloop/app/controlplane/pkg/biz/mocks"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
)

type authzTestSuite struct {
	suite.Suite
	useCase      *biz.AuthzUseCase
	apiTokenRepo *bizMocks.APITokenRepo
	enforcer     *authz.CasbinEnforcer
	logger       log.Logger
}

func (s *authzTestSuite) SetupTest() {
	s.apiTokenRepo = bizMocks.NewAPITokenRepo(s.T())
	s.logger = log.NewStdLogger(io.Discard)

	// Create a real enforcer for testing
	var err error
	s.enforcer, err = authz.NewCasbinEnforcer(&authz.Config{
		RolesMap: authz.RolesMap,
	})
	s.Require().NoError(err)

	s.useCase = biz.NewAuthzUseCase(&biz.AuthzUseCaseConfig{
		CasbinEnforcer:      s.enforcer,
		APITokenRepo:        s.apiTokenRepo,
		RestrictOrgCreation: false,
		Logger:              s.logger,
	})
}

func TestAuthzUseCase(t *testing.T) {
	suite.Run(t, new(authzTestSuite))
}

func (s *authzTestSuite) TestEnforce_RegularUser_APITokenSubjectReturnsError() {
	// The enforcer itself should reject API token subjects
	subject := "api-token:some-id"
	policy := &authz.Policy{
		Resource: authz.ResourceWorkflow,
		Action:   authz.ActionRead,
	}

	// The enforcer.Enforce() method directly rejects API token subjects
	ok, err := s.enforcer.Enforce(subject, policy)

	s.Error(err)
	s.False(ok)
	s.Contains(err.Error(), "API token subjects not supported")
}

func (s *authzTestSuite) TestEnforce_APIToken() {
	assert := assert.New(s.T())

	s.Run("InvalidUUID", func() {
		ctx := context.Background()
		subject := "api-token:invalid-uuid"
		policy := &authz.Policy{
			Resource: authz.ResourceWorkflow,
			Action:   authz.ActionRead,
		}

		ok, err := s.useCase.Enforce(ctx, subject, policy)

		assert.Error(err)
		assert.False(ok)
	})

	s.Run("TokenNotFound", func() {
		ctx := context.Background()
		tokenID := uuid.New()
		subject := "api-token:" + tokenID.String()
		policy := &authz.Policy{
			Resource: authz.ResourceWorkflow,
			Action:   authz.ActionRead,
		}

		s.apiTokenRepo.On("FindByID", ctx, tokenID).Return(nil, nil)

		ok, err := s.useCase.Enforce(ctx, subject, policy)

		assert.Error(err)
		assert.True(biz.IsNotFound(err))
		assert.False(ok)
	})

	s.Run("RepoError", func() {
		ctx := context.Background()
		tokenID := uuid.New()
		subject := "api-token:" + tokenID.String()
		policy := &authz.Policy{
			Resource: authz.ResourceWorkflow,
			Action:   authz.ActionRead,
		}

		expectedErr := errors.New("database error")
		s.apiTokenRepo.On("FindByID", ctx, tokenID).Return(nil, expectedErr)

		ok, err := s.useCase.Enforce(ctx, subject, policy)

		assert.Error(err)
		assert.Equal(expectedErr, err)
		assert.False(ok)
	})

	s.Run("PolicyAllowed", func() {
		ctx := context.Background()
		tokenID := uuid.New()
		subject := "api-token:" + tokenID.String()
		policy := &authz.Policy{
			Resource: authz.ResourceWorkflow,
			Action:   authz.ActionRead,
		}

		token := &biz.APIToken{
			ID: tokenID,
			Policies: []*authz.Policy{
				{Resource: authz.ResourceWorkflow, Action: authz.ActionRead},
				{Resource: authz.ResourceWorkflowRun, Action: authz.ActionList},
			},
		}

		s.apiTokenRepo.On("FindByID", ctx, tokenID).Return(token, nil)

		ok, err := s.useCase.Enforce(ctx, subject, policy)

		assert.NoError(err)
		assert.True(ok)
	})

	s.Run("PolicyDenied", func() {
		ctx := context.Background()
		tokenID := uuid.New()
		subject := "api-token:" + tokenID.String()
		policy := &authz.Policy{
			Resource: authz.ResourceWorkflow,
			Action:   authz.ActionDelete,
		}

		token := &biz.APIToken{
			ID: tokenID,
			Policies: []*authz.Policy{
				{Resource: authz.ResourceWorkflow, Action: authz.ActionRead},
				{Resource: authz.ResourceWorkflow, Action: authz.ActionList},
			},
		}

		s.apiTokenRepo.On("FindByID", ctx, tokenID).Return(token, nil)

		ok, err := s.useCase.Enforce(ctx, subject, policy)

		assert.NoError(err)
		assert.False(ok)
	})

	s.Run("EmptyPolicies", func() {
		ctx := context.Background()
		tokenID := uuid.New()
		subject := "api-token:" + tokenID.String()
		policy := &authz.Policy{
			Resource: authz.ResourceWorkflow,
			Action:   authz.ActionRead,
		}

		token := &biz.APIToken{
			ID:       tokenID,
			Policies: []*authz.Policy{},
		}

		s.apiTokenRepo.On("FindByID", ctx, tokenID).Return(token, nil)

		ok, err := s.useCase.Enforce(ctx, subject, policy)

		assert.NoError(err)
		assert.False(ok)
	})

	s.Run("NilPolicies", func() {
		ctx := context.Background()
		tokenID := uuid.New()
		subject := "api-token:" + tokenID.String()
		policy := &authz.Policy{
			Resource: authz.ResourceWorkflow,
			Action:   authz.ActionRead,
		}

		token := &biz.APIToken{
			ID:       tokenID,
			Policies: nil,
		}

		s.apiTokenRepo.On("FindByID", ctx, tokenID).Return(token, nil)

		ok, err := s.useCase.Enforce(ctx, subject, policy)

		assert.NoError(err)
		assert.False(ok)
	})

	s.Run("PartialMatchResource", func() {
		ctx := context.Background()
		tokenID := uuid.New()
		subject := "api-token:" + tokenID.String()
		policy := &authz.Policy{
			Resource: authz.ResourceWorkflow,
			Action:   authz.ActionRead,
		}

		// Token has matching resource but different action
		token := &biz.APIToken{
			ID: tokenID,
			Policies: []*authz.Policy{
				{Resource: authz.ResourceWorkflow, Action: authz.ActionCreate},
			},
		}

		s.apiTokenRepo.On("FindByID", ctx, tokenID).Return(token, nil)

		ok, err := s.useCase.Enforce(ctx, subject, policy)

		assert.NoError(err)
		assert.False(ok)
	})

	s.Run("PartialMatchAction", func() {
		ctx := context.Background()
		tokenID := uuid.New()
		subject := "api-token:" + tokenID.String()
		policy := &authz.Policy{
			Resource: authz.ResourceWorkflow,
			Action:   authz.ActionRead,
		}

		// Token has matching action but different resource
		token := &biz.APIToken{
			ID: tokenID,
			Policies: []*authz.Policy{
				{Resource: authz.ResourceWorkflowRun, Action: authz.ActionRead},
			},
		}

		s.apiTokenRepo.On("FindByID", ctx, tokenID).Return(token, nil)

		ok, err := s.useCase.Enforce(ctx, subject, policy)

		assert.NoError(err)
		assert.False(ok)
	})

	s.Run("MultiplePoliciesWithMatch", func() {
		ctx := context.Background()
		tokenID := uuid.New()
		subject := "api-token:" + tokenID.String()
		policy := &authz.Policy{
			Resource: authz.ResourceWorkflowContract,
			Action:   authz.ActionUpdate,
		}

		// Token has multiple policies, one of them matches
		token := &biz.APIToken{
			ID: tokenID,
			Policies: []*authz.Policy{
				{Resource: authz.ResourceWorkflow, Action: authz.ActionRead},
				{Resource: authz.ResourceWorkflow, Action: authz.ActionCreate},
				{Resource: authz.ResourceWorkflowContract, Action: authz.ActionUpdate},
				{Resource: authz.ResourceCASArtifact, Action: authz.ActionRead},
			},
		}

		s.apiTokenRepo.On("FindByID", ctx, tokenID).Return(token, nil)

		ok, err := s.useCase.Enforce(ctx, subject, policy)

		assert.NoError(err)
		assert.True(ok)
	})
}
