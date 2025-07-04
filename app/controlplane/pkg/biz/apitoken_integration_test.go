//
// Copyright 2024-2025 The Chainloop Authors.
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

	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/authz"
	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/biz"
	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/biz/testhelpers"
	"github.com/golang-jwt/jwt/v4"
	"github.com/google/uuid"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

func randomName() string {
	return fmt.Sprintf("name-%s", uuid.New().String())
}

func (s *apiTokenTestSuite) TestCreate() {
	ctx := context.Background()
	s.Run("invalid org ID", func() {
		token, err := s.APIToken.Create(ctx, randomName(), nil, nil, "deadbeef")
		s.Error(err)
		s.True(biz.IsErrInvalidUUID(err))
		s.Nil(token)
	})

	s.Run("happy path without expiration nor description", func() {
		token, err := s.APIToken.Create(ctx, randomName(), nil, nil, s.org.ID)
		s.NoError(err)
		s.NotNil(token.ID)
		s.Equal(s.org.ID, token.OrganizationID.String())
		s.Empty(token.Description)
		s.Nil(token.ExpiresAt)
		s.Nil(token.RevokedAt)
		s.NotNil(token.JWT)
	})

	s.Run("happy path with description and expiration", func() {
		token, err := s.APIToken.Create(ctx, randomName(), toPtrS("tokenStr"), toPtrDuration(24*time.Hour), s.org.ID)
		s.NoError(err)
		s.Equal(s.org.ID, token.OrganizationID.String())
		s.Equal("tokenStr", token.Description)
		s.NotNil(token.ExpiresAt)
		s.Nil(token.RevokedAt)
	})

	s.Run("happy path with project", func() {
		token, err := s.APIToken.Create(ctx, randomName(), nil, nil, s.org.ID, biz.APITokenWithProject(s.p1))
		s.NoError(err)
		s.Equal(s.org.ID, token.OrganizationID.String())
		s.Equal(s.p1.ID, *token.ProjectID)
		s.Equal(s.p1.Name, *token.ProjectName)
	})

	s.Run("testing name uniqueness", func() {
		testCases := []struct {
			name       string
			tokenName  string
			wantErrMsg string
			project    *biz.Project
		}{
			{
				name:       "name missing",
				tokenName:  "",
				wantErrMsg: "required",
			},
			{
				name:       "invalid name",
				tokenName:  "this/not/valid",
				wantErrMsg: "RFC 1123",
			},
			{
				name:       "another invalid name",
				tokenName:  "this-not Valid",
				wantErrMsg: "RFC 1123",
			},
			{
				name:      "can create it with just the name",
				tokenName: "my-name",
			},
			{
				name:       "handle duplicates",
				tokenName:  "my-name",
				wantErrMsg: "name already taken",
			},
			{
				name:      "tokens in projects can have the same name",
				tokenName: "my-name",
				project:   s.p1,
			},
			{
				name:      "tokens in different projects too",
				tokenName: "my-name",
				project:   s.p2,
			},
			{
				name:       "can't be duplicated in the same project",
				tokenName:  "my-name",
				project:    s.p1,
				wantErrMsg: "name already taken",
			},
		}

		for _, tc := range testCases {
			s.Run(tc.name, func() {
				var opts []biz.APITokenUseCaseOpt
				if tc.project != nil {
					opts = append(opts, biz.APITokenWithProject(tc.project))
				}

				token, err := s.APIToken.Create(ctx, tc.tokenName, nil, nil, s.org.ID, opts...)
				if tc.wantErrMsg != "" {
					s.Error(err)
					s.Contains(err.Error(), tc.wantErrMsg)
					s.Nil(token)
					return
				}

				s.NoError(err)
				s.NotNil(token)
			})
		}
	})
}

func (s *apiTokenTestSuite) TestAuthzPolicies() {
	// a new token has a new set of policies associated
	token, err := s.APIToken.Create(context.Background(), randomName(), nil, nil, s.org.ID)
	require.NoError(s.T(), err)

	subject := (&authz.SubjectAPIToken{ID: token.ID.String()}).String()
	// load the policies associated with the token from the global enforcer
	policies, err := s.Enforcer.GetFilteredPolicy(0, subject)
	s.Require().NoError(err)

	// Check that only default policies are loaded
	s.Len(policies, len(s.APIToken.DefaultAuthzPolicies))
	for _, p := range s.APIToken.DefaultAuthzPolicies {
		ok, err := s.Enforcer.HasPolicy(subject, p.Resource, p.Action)
		s.NoError(err)
		s.True(ok, fmt.Sprintf("policy %s:%s not found", p.Resource, p.Action))
	}
}

func (s *apiTokenTestSuite) TestRevoke() {
	ctx := context.Background()

	s.Run("invalid org ID", func() {
		err := s.APIToken.Revoke(ctx, "deadbeef", s.t1.ID.String())
		s.Error(err)
		s.True(biz.IsErrInvalidUUID(err))
	})

	s.Run("invalid token ID", func() {
		err := s.APIToken.Revoke(ctx, s.org.ID, "deadbeef")
		s.Error(err)
		s.True(biz.IsErrInvalidUUID(err))
	})

	s.Run("token not found in org", func() {
		err := s.APIToken.Revoke(ctx, s.org.ID, s.t3.ID.String())
		s.Error(err)
		s.True(biz.IsNotFound(err))
	})

	s.Run("the revoked token also get its policies cleared", func() {
		sub := (&authz.SubjectAPIToken{ID: s.t2.ID.String()}).String()
		// It has the default policies
		gotPolicies, err := s.Enforcer.GetFilteredPolicy(0, sub)
		s.NoError(err)
		s.Len(gotPolicies, len(s.APIToken.DefaultAuthzPolicies))
		err = s.APIToken.Revoke(ctx, s.org.ID, s.t2.ID.String())
		s.NoError(err)
		// once revoked, the policies are cleared
		gotPolicies, err = s.Enforcer.GetFilteredPolicy(0, sub)
		s.NoError(err)
		s.Len(gotPolicies, 0)
	})

	s.Run("token can be revoked once", func() {
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

	s.Run("invalid ID", func() {
		_, err := s.APIToken.FindByID(ctx, "deadbeef")
		s.Error(err)
		s.True(biz.IsErrInvalidUUID(err))
	})

	s.Run("token not found", func() {
		token, err := s.APIToken.FindByID(ctx, uuid.NewString())
		s.Error(err)
		s.True(biz.IsNotFound(err))
		s.Nil(token)
	})

	s.Run("token is found", func() {
		token, err := s.APIToken.FindByID(ctx, s.t1.ID.String())
		s.NoError(err)
		s.Equal(s.t1.ID, token.ID)
	})
}

func (s *apiTokenTestSuite) TestList() {
	ctx := context.Background()
	s.Run("invalid org ID", func() {
		tokens, err := s.APIToken.List(ctx, "deadbeef", false)
		s.Error(err)
		s.True(biz.IsErrInvalidUUID(err))
		s.Nil(tokens)
	})

	s.Run("returns empty list", func() {
		emptyOrg, err := s.Organization.CreateWithRandomName(ctx)
		require.NoError(s.T(), err)
		tokens, err := s.APIToken.List(ctx, emptyOrg.ID, false)
		s.NoError(err)
		s.Len(tokens, 0)
	})

	s.Run("returns all tokens for that org both system and project scoped", func() {
		var err error
		tokens, err := s.APIToken.List(ctx, s.org.ID, false)
		s.NoError(err)
		require.Len(s.T(), tokens, 4)
		s.Equal(s.t1.ID, tokens[0].ID)
		s.Equal(s.t2.ID, tokens[1].ID)
		// It has a name set
		s.NotEmpty(tokens[1].Name)
		s.Equal(s.t2.Name, tokens[1].Name)

		tokens, err = s.APIToken.List(ctx, s.org2.ID, false)
		s.NoError(err)
		require.Len(s.T(), tokens, 1)
		s.Equal(s.t3.ID, tokens[0].ID)
	})

	s.Run("can return only for a specific project", func() {
		var err error
		tokens, err := s.APIToken.List(ctx, s.org.ID, false, biz.APITokenWithProject(s.p1))
		s.NoError(err)
		require.Len(s.T(), tokens, 2)
		s.Equal(s.t4.ID, tokens[0].ID)
		s.Equal(s.t5.ID, tokens[1].ID)
	})

	s.Run("or just the system tokens", func() {
		var err error
		tokens, err := s.APIToken.List(ctx, s.org.ID, false, biz.APITokenShowOnlySystemTokens(true))
		s.NoError(err)
		require.Len(s.T(), tokens, 2)
		s.Equal(s.t1.ID, tokens[0].ID)
		s.Equal(s.t2.ID, tokens[1].ID)
	})

	s.Run("doesn't return revoked by default", func() {
		// revoke one token
		err := s.APIToken.Revoke(ctx, s.org.ID, s.t1.ID.String())
		require.NoError(s.T(), err)
		tokens, err := s.APIToken.List(ctx, s.org.ID, false)
		s.NoError(err)
		require.Len(s.T(), tokens, 3)
		s.Equal(s.t2.ID, tokens[0].ID)
	})

	s.Run("doesn't return revoked unless requested", func() {
		// revoke one token
		tokens, err := s.APIToken.List(ctx, s.org.ID, true)
		s.NoError(err)
		require.Len(s.T(), tokens, 4)
		s.Equal(s.t1.ID, tokens[0].ID)
		s.Equal(s.t2.ID, tokens[1].ID)
	})
}

func (s *apiTokenTestSuite) TestGeneratedJWT() {
	token, err := s.APIToken.Create(context.Background(), randomName(), nil, toPtrDuration(24*time.Hour), s.org.ID)
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
	org, org2          *biz.Organization
	t1, t2, t3, t4, t5 *biz.APIToken
	p1, p2             *biz.Project
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

	s.p1, err = s.Project.Create(ctx, s.org.ID, "project1")
	require.NoError(s.T(), err)

	s.p2, err = s.Project.Create(ctx, s.org.ID, "project2")
	require.NoError(s.T(), err)

	// Create 2 tokens for org 1
	s.t1, err = s.APIToken.Create(ctx, randomName(), nil, nil, s.org.ID)
	require.NoError(s.T(), err)
	s.t2, err = s.APIToken.Create(ctx, randomName(), nil, nil, s.org.ID)
	require.NoError(s.T(), err)
	// and 1 token for org 2
	s.t3, err = s.APIToken.Create(ctx, randomName(), nil, nil, s.org2.ID)
	require.NoError(s.T(), err)

	// Create 2 tokens for project 1
	s.t4, err = s.APIToken.Create(ctx, randomName(), nil, nil, s.org.ID, biz.APITokenWithProject(s.p1))
	require.NoError(s.T(), err)
	s.t5, err = s.APIToken.Create(ctx, randomName(), nil, nil, s.org.ID, biz.APITokenWithProject(s.p1))
	require.NoError(s.T(), err)
}

func (s *apiTokenTestSuite) TestUpdateLastUsedAt() {
	ctx := context.Background()

	s.Run("update last used at", func() {
		token, err := s.APIToken.Create(ctx, randomName(), nil, nil, s.org.ID)
		s.NoError(err)
		s.Nil(token.LastUsedAt)

		err = s.APIToken.UpdateLastUsedAt(ctx, token.ID.String())
		s.NoError(err)

		updatedToken, err := s.APIToken.FindByID(ctx, token.ID.String())
		s.NoError(err)
		s.NotNil(updatedToken.LastUsedAt)
		s.WithinDuration(time.Now(), *updatedToken.LastUsedAt, time.Second)
	})

	s.Run("invalid token ID", func() {
		err := s.APIToken.UpdateLastUsedAt(ctx, "invalid-uuid")
		s.Error(err)
		s.True(biz.IsErrInvalidUUID(err))
	})

	s.Run("token not found", func() {
		err := s.APIToken.UpdateLastUsedAt(ctx, uuid.NewString())
		s.Error(err)
		s.True(biz.IsNotFound(err))
	})

	s.Run("token is revoked", func() {
		token, err := s.APIToken.Create(ctx, randomName(), nil, nil, s.org.ID)
		s.NoError(err)
		err = s.APIToken.Revoke(ctx, s.org.ID, token.ID.String())
		s.NoError(err)

		err = s.APIToken.UpdateLastUsedAt(ctx, token.ID.String())

		s.Error(err)
		s.True(biz.IsNotFound(err))
	})
}
