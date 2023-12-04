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

package biz_test

import (
	"context"
	"testing"

	"github.com/chainloop-dev/chainloop/app/controlplane/internal/biz"
	"github.com/chainloop-dev/chainloop/app/controlplane/internal/biz/testhelpers"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
)

func (s *apiTokenTestSuite) TestCreate() {
	ctx := context.Background()
	s.T().Run("invalid org ID", func(t *testing.T) {
		token, err := s.APIToken.Create(ctx, nil, nil, s.user.ID, "deadbeef")
		s.Error(err)
		s.True(biz.IsErrInvalidUUID(err))
		s.Nil(token)
	})

	s.T().Run("invalid user ID", func(t *testing.T) {
		token, err := s.APIToken.Create(ctx, nil, nil, "deadbeef", s.org.ID)
		s.Error(err)
		s.True(biz.IsErrInvalidUUID(err))
		s.Nil(token)
	})

	s.T().Run("user2 has no access to org", func(t *testing.T) {
		token, err := s.APIToken.Create(ctx, nil, nil, s.user2.ID, s.org.ID)
		s.Error(err)
		s.True(biz.IsNotFound(err))
		s.Nil(token)
	})

	s.T().Run("invalid expiration format expiration", func(t *testing.T) {
		token, err := s.APIToken.Create(ctx, nil, toPtrS("wrong"), s.user.ID, s.org.ID)
		s.Error(err)
		s.True(biz.IsErrValidation(err))
		s.ErrorContains(err, "invalid expiration format")
		s.Nil(token)
	})

	s.T().Run("expiration below threshold", func(t *testing.T) {
		token, err := s.APIToken.Create(ctx, nil, toPtrS("1h"), s.user.ID, s.org.ID)
		s.Error(err)
		s.True(biz.IsErrValidation(err))
		s.ErrorContains(err, "expiration must be at least")
		s.Nil(token)
	})

	s.T().Run("happy path without expiration nor description", func(t *testing.T) {
		token, err := s.APIToken.Create(ctx, nil, nil, s.user.ID, s.org.ID)
		s.NoError(err)
		s.NotNil(token.ID)
		s.Equal(s.org.ID, token.OrganizationID.String())
		s.Empty(token.Description)
		s.Nil(token.ExpiresAt)
		s.Nil(token.RevokedAt)
	})

	s.T().Run("happy path with description and expiration", func(t *testing.T) {
		token, err := s.APIToken.Create(ctx, toPtrS("tokenStr"), toPtrS("24h"), s.user.ID, s.org.ID)
		s.NoError(err)
		s.Equal(s.org.ID, token.OrganizationID.String())
		s.Equal("tokenStr", token.Description)
		s.NotNil(token.ExpiresAt)
		s.Nil(token.RevokedAt)
	})
}

// Run the tests
func TestAPITokenUseCase(t *testing.T) {
	suite.Run(t, new(apiTokenTestSuite))
}

// Utility struct to hold the test suite
type apiTokenTestSuite struct {
	testhelpers.UseCasesEachTestSuite
	org         *biz.Organization
	user, user2 *biz.User
}

func (s *apiTokenTestSuite) SetupTest() {
	t := s.T()
	var err error
	assert := assert.New(s.T())
	ctx := context.Background()

	s.TestingUseCases = testhelpers.NewTestingUseCases(t)
	s.org, err = s.Organization.Create(ctx, "org1")
	assert.NoError(err)

	// Create User 1
	s.user, err = s.User.FindOrCreateByEmail(ctx, "user-1@test.com")
	assert.NoError(err)
	// Attach org 1
	_, err = s.Membership.Create(ctx, s.org.ID, s.user.ID, true)
	assert.NoError(err)

	// Create user 2 with no orgs
	s.user2, err = s.User.FindOrCreateByEmail(ctx, "user-2@test.com")
	assert.NoError(err)
}
