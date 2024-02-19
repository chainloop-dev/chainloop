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
	"fmt"
	"testing"
	"time"

	"github.com/chainloop-dev/chainloop/app/controlplane/internal/authz"
	"github.com/chainloop-dev/chainloop/app/controlplane/internal/biz"
	"github.com/chainloop-dev/chainloop/app/controlplane/internal/biz/testhelpers"
	"github.com/golang-jwt/jwt/v4"
	"github.com/google/uuid"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

func (s *apiTokenTestSuite) TestCreate() {
	ctx := context.Background()
	s.T().Run("invalid org ID", func(t *testing.T) {
		token, err := s.APIToken.Create(ctx, nil, nil, "deadbeef")
		s.Error(err)
		s.True(biz.IsErrInvalidUUID(err))
		s.Nil(token)
	})

	s.T().Run("happy path without expiration nor description", func(t *testing.T) {
		token, err := s.APIToken.Create(ctx, nil, nil, s.org.ID)
		s.NoError(err)
		s.NotNil(token.ID)
		s.Equal(s.org.ID, token.OrganizationID.String())
		s.Empty(token.Description)
		s.Nil(token.ExpiresAt)
		s.Nil(token.RevokedAt)
		s.NotNil(token.JWT)
	})

	s.T().Run("happy path with description and expiration", func(t *testing.T) {
		token, err := s.APIToken.Create(ctx, toPtrS("tokenStr"), toPtrDuration(24*time.Hour), s.org.ID)
		s.NoError(err)
		s.Equal(s.org.ID, token.OrganizationID.String())
		s.Equal("tokenStr", token.Description)
		s.NotNil(token.ExpiresAt)
		s.Nil(token.RevokedAt)
	})
}

func (s *apiTokenTestSuite) TestAuthzPolicies() {
	// a new token has a new set of policies associated
	token, err := s.APIToken.Create(context.Background(), nil, nil, s.org.ID)
	require.NoError(s.T(), err)

	subject := (&authz.SubjectAPIToken{ID: token.ID.String()}).String()
	// load the policies associated with the token from the global enforcer
	policies := s.Enforcer.GetFilteredPolicy(0, subject)

	// Check that only default policies are loaded
	s.Len(policies, len(s.APIToken.DefaultAuthzPolicies))
	for _, p := range s.APIToken.DefaultAuthzPolicies {
		ok := s.Enforcer.HasPolicy(subject, p.Resource, p.Action)
		s.True(ok, fmt.Sprintf("policy %s:%s not found", p.Resource, p.Action))
	}
}

func (s *apiTokenTestSuite) TestRevoke() {
	ctx := context.Background()

	s.T().Run("invalid org ID", func(t *testing.T) {
		err := s.APIToken.Revoke(ctx, "deadbeef", s.t1.ID.String())
		s.Error(err)
		s.True(biz.IsErrInvalidUUID(err))
	})

	s.T().Run("invalid token ID", func(t *testing.T) {
		err := s.APIToken.Revoke(ctx, s.org.ID, "deadbeef")
		s.Error(err)
		s.True(biz.IsErrInvalidUUID(err))
	})

	s.T().Run("token not found in org", func(t *testing.T) {
		err := s.APIToken.Revoke(ctx, s.org.ID, s.t3.ID.String())
		s.Error(err)
		s.True(biz.IsNotFound(err))
	})

	s.T().Run("the revoked token also get its policies cleared", func(t *testing.T) {
		sub := (&authz.SubjectAPIToken{ID: s.t2.ID.String()}).String()
		// It has the default policies
		gotPolicies := s.Enforcer.GetFilteredPolicy(0, sub)
		assert.Len(t, gotPolicies, len(s.APIToken.DefaultAuthzPolicies))
		err := s.APIToken.Revoke(ctx, s.org.ID, s.t2.ID.String())
		s.NoError(err)
		// once revoked, the policies are cleared
		gotPolicies = s.Enforcer.GetFilteredPolicy(0, sub)
		assert.Len(t, gotPolicies, 0)
	})

	s.T().Run("token can be revoked once", func(t *testing.T) {
		err := s.APIToken.Revoke(ctx, s.org.ID, s.t1.ID.String())
		s.NoError(err)
		tokens, err := s.APIToken.List(ctx, s.org.ID, true)
		s.NoError(err)
		s.Equal(s.t1.ID, tokens[0].ID)
		// It's revoked
		s.NotNil(tokens[0].RevokedAt)

		// Can't be revoked twice
		err = s.APIToken.Revoke(ctx, s.org.ID, s.t1.ID.String())
		s.Error(err)
		s.True(biz.IsNotFound(err))
	})
}

func (s *apiTokenTestSuite) TestFindByID() {
	ctx := context.Background()

	s.T().Run("invalid ID", func(t *testing.T) {
		_, err := s.APIToken.FindByID(ctx, "deadbeef")
		s.Error(err)
		s.True(biz.IsErrInvalidUUID(err))
	})

	s.T().Run("token not found", func(t *testing.T) {
		token, err := s.APIToken.FindByID(ctx, uuid.NewString())
		s.Error(err)
		s.True(biz.IsNotFound(err))
		s.Nil(token)
	})

	s.T().Run("token is found", func(t *testing.T) {
		token, err := s.APIToken.FindByID(ctx, s.t1.ID.String())
		s.NoError(err)
		s.Equal(s.t1.ID, token.ID)
	})
}

func (s *apiTokenTestSuite) TestList() {
	ctx := context.Background()
	s.T().Run("invalid org ID", func(t *testing.T) {
		tokens, err := s.APIToken.List(ctx, "deadbeef", false)
		s.Error(err)
		s.True(biz.IsErrInvalidUUID(err))
		s.Nil(tokens)
	})

	s.T().Run("returns empty list", func(t *testing.T) {
		emptyOrg, err := s.Organization.CreateWithRandomName(ctx)
		require.NoError(s.T(), err)
		tokens, err := s.APIToken.List(ctx, emptyOrg.ID, false)
		s.NoError(err)
		s.Len(tokens, 0)
	})

	s.T().Run("returns the tokens for that org", func(t *testing.T) {
		var err error
		tokens, err := s.APIToken.List(ctx, s.org.ID, false)
		s.NoError(err)
		require.Len(s.T(), tokens, 2)
		s.Equal(s.t1.ID, tokens[0].ID)
		s.Equal(s.t2.ID, tokens[1].ID)

		tokens, err = s.APIToken.List(ctx, s.org2.ID, false)
		s.NoError(err)
		require.Len(s.T(), tokens, 1)
		s.Equal(s.t3.ID, tokens[0].ID)
	})

	s.T().Run("doesn't return revoked by default", func(t *testing.T) {
		// revoke one token
		err := s.APIToken.Revoke(ctx, s.org.ID, s.t1.ID.String())
		require.NoError(s.T(), err)
		tokens, err := s.APIToken.List(ctx, s.org.ID, false)
		s.NoError(err)
		require.Len(s.T(), tokens, 1)
		s.Equal(s.t2.ID, tokens[0].ID)
	})

	s.T().Run("doesn't return revoked unless requested", func(t *testing.T) {
		// revoke one token
		tokens, err := s.APIToken.List(ctx, s.org.ID, true)
		s.NoError(err)
		require.Len(s.T(), tokens, 2)
		s.Equal(s.t1.ID, tokens[0].ID)
		s.Equal(s.t2.ID, tokens[1].ID)
	})
}

func (s *apiTokenTestSuite) TestGeneratedJWT() {
	token, err := s.APIToken.Create(context.Background(), nil, toPtrDuration(24*time.Hour), s.org.ID)
	s.NoError(err)
	require.NotNil(s.T(), token)

	claims := &jwt.RegisteredClaims{}
	tokenInfo, err := jwt.ParseWithClaims(token.JWT, claims, func(_ *jwt.Token) (interface{}, error) {
		return []byte("test"), nil
	})

	require.NoError(s.T(), err)
	s.True(tokenInfo.Valid)
	// The resulting JWT should have the same org, token ID and expiration time than
	// the reference in the DB
	s.Equal(token.OrganizationID.String(), s.org.ID)
	s.Equal(token.ID.String(), claims.ID)
	s.Equal(token.ExpiresAt.Truncate(time.Second), claims.ExpiresAt.Truncate(time.Second))
}

// Run the tests
func TestAPITokenUseCase(t *testing.T) {
	suite.Run(t, new(apiTokenTestSuite))
}

// Utility struct to hold the test suite
type apiTokenTestSuite struct {
	testhelpers.UseCasesEachTestSuite
	org, org2  *biz.Organization
	t1, t2, t3 *biz.APIToken
}

func (s *apiTokenTestSuite) SetupTest() {
	t := s.T()
	var err error
	assert := assert.New(s.T())
	ctx := context.Background()

	s.TestingUseCases = testhelpers.NewTestingUseCases(t)
	s.org, err = s.Organization.CreateWithRandomName(ctx)
	assert.NoError(err)
	s.org2, err = s.Organization.CreateWithRandomName(ctx)
	assert.NoError(err)

	// Create 2 tokens for org 1
	s.t1, err = s.APIToken.Create(ctx, nil, nil, s.org.ID)
	require.NoError(s.T(), err)
	s.t2, err = s.APIToken.Create(ctx, nil, nil, s.org.ID)
	require.NoError(s.T(), err)
	// and 1 token for org 2
	s.t3, err = s.APIToken.Create(ctx, nil, nil, s.org2.ID)
	require.NoError(s.T(), err)
}
