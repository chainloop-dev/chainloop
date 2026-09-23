//
// Copyright 2024-2026 The Chainloop Authors.
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
	apitokenjwt "github.com/chainloop-dev/chainloop/app/controlplane/pkg/jwt/apitoken"
	"github.com/golang-jwt/jwt/v5"
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
		token, err := s.APIToken.Create(ctx, randomName(), nil, nil, toPtrS("deadbeef"))
		s.Error(err)
		s.True(biz.IsErrInvalidUUID(err))
		s.Nil(token)
	})

	s.Run("happy path without expiration nor description", func() {
		token, err := s.APIToken.Create(ctx, randomName(), nil, nil, &s.org.ID)
		s.NoError(err)
		s.NotNil(token.ID)
		s.Equal(s.org.ID, token.OrganizationID.String())
		s.Empty(token.Description)
		s.Nil(token.ExpiresAt)
		s.Nil(token.RevokedAt)
		s.NotNil(token.JWT)
	})

	s.Run("happy path with description and expiration", func() {
		token, err := s.APIToken.Create(ctx, randomName(), toPtrS("tokenStr"), toPtrDuration(24*time.Hour), &s.org.ID)
		s.NoError(err)
		s.Equal(s.org.ID, token.OrganizationID.String())
		s.Equal("tokenStr", token.Description)
		s.NotNil(token.ExpiresAt)
		s.Nil(token.RevokedAt)
	})

	s.Run("happy path with project", func() {
		token, err := s.APIToken.Create(ctx, randomName(), nil, nil, &s.org.ID, biz.APITokenWithProject(s.p1))
		s.NoError(err)
		s.Equal(s.org.ID, token.OrganizationID.String())
		s.Equal(s.p1.ID, *token.ProjectID)
		s.Equal(s.p1.Name, *token.ProjectName)
	})

	s.Run("happy path with workflow", func() {
		wf, err := s.Workflow.Create(ctx, &biz.WorkflowCreateOpts{Name: randomName(), OrgID: s.org.ID, Project: s.p1.Name})
		require.NoError(s.T(), err)

		token, err := s.APIToken.Create(ctx, randomName(), nil, nil, &s.org.ID,
			biz.APITokenWithProject(s.p1),
			biz.APITokenWithWorkflow(wf),
		)
		s.NoError(err)
		s.Equal(s.p1.ID, *token.ProjectID)
		s.Equal(wf.ID, *token.WorkflowID)
		s.Equal(wf.Name, *token.WorkflowName)
	})

	s.Run("workflow without project is rejected", func() {
		wf, err := s.Workflow.Create(ctx, &biz.WorkflowCreateOpts{Name: randomName(), OrgID: s.org.ID, Project: s.p1.Name})
		require.NoError(s.T(), err)

		token, err := s.APIToken.Create(ctx, randomName(), nil, nil, &s.org.ID, biz.APITokenWithWorkflow(wf))
		s.Error(err)
		s.True(biz.IsErrValidation(err))
		s.Nil(token)
	})

	s.Run("workflow whose project does not match is rejected", func() {
		wfInP2, err := s.Workflow.Create(ctx, &biz.WorkflowCreateOpts{Name: randomName(), OrgID: s.org.ID, Project: s.p2.Name})
		require.NoError(s.T(), err)

		token, err := s.APIToken.Create(ctx, randomName(), nil, nil, &s.org.ID,
			biz.APITokenWithProject(s.p1),
			biz.APITokenWithWorkflow(wfInP2),
		)
		s.Error(err)
		s.True(biz.IsErrValidation(err))
		s.Nil(token)
	})

	s.Run("system token is persisted with the flag", func() {
		token, err := s.APIToken.Create(ctx, randomName(), nil, nil, &s.org.ID, biz.APITokenAsSystem())
		s.NoError(err)
		s.True(token.IsSystem)
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
				var opts []biz.APITokenCreateOpt
				if tc.project != nil {
					opts = append(opts, biz.APITokenWithProject(tc.project))
				}

				token, err := s.APIToken.Create(ctx, tc.tokenName, nil, nil, &s.org.ID, opts...)
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
	ctx := context.Background()

	s.Run("project-level token gets default policies", func() {
		token, err := s.APIToken.Create(ctx, randomName(), nil, nil, &s.org.ID, biz.APITokenWithProject(s.p1))
		require.NoError(s.T(), err)
		s.Require().NotNil(token.Policies)
		s.Len(token.Policies, len(s.APIToken.DefaultAuthzPolicies))

		for _, p := range s.APIToken.DefaultAuthzPolicies {
			found := false
			for _, actual := range token.Policies {
				if actual.Resource == p.Resource && actual.Action == p.Action {
					found = true
					break
				}
			}
			s.True(found, fmt.Sprintf("policy %s:%s not found", p.Resource, p.Action))
		}
	})

	s.Run("org-level token gets default plus token management policies", func() {
		token, err := s.APIToken.Create(ctx, randomName(), nil, nil, &s.org.ID)
		require.NoError(s.T(), err)
		s.Require().NotNil(token.Policies)

		// All default policies must be present
		for _, p := range s.APIToken.DefaultAuthzPolicies {
			found := false
			for _, actual := range token.Policies {
				if actual.Resource == p.Resource && actual.Action == p.Action {
					found = true
					break
				}
			}
			s.True(found, fmt.Sprintf("default policy %s:%s not found", p.Resource, p.Action))
		}

		// Additional org-level token management policies must be present
		for _, p := range []*authz.Policy{authz.PolicyAPITokenCreate, authz.PolicyAPITokenList, authz.PolicyAPITokenRevoke} {
			found := false
			for _, actual := range token.Policies {
				if actual.Resource == p.Resource && actual.Action == p.Action {
					found = true
					break
				}
			}
			s.True(found, fmt.Sprintf("org policy %s:%s not found", p.Resource, p.Action))
		}
	})
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

	s.Run("token can be revoked once", func() {
		err := s.APIToken.Revoke(ctx, s.org.ID, s.t1.ID.String())
		s.NoError(err)
		tokens, err := s.APIToken.List(ctx, s.org.ID, biz.WithAPITokenStatusFilter(biz.APITokenStatusFilterAll))
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
		tokens, err := s.APIToken.List(ctx, "deadbeef")
		s.Error(err)
		s.True(biz.IsErrInvalidUUID(err))
		s.Nil(tokens)
	})

	s.Run("returns empty list", func() {
		emptyOrg, err := s.Organization.CreateWithRandomName(ctx)
		require.NoError(s.T(), err)
		tokens, err := s.APIToken.List(ctx, emptyOrg.ID)
		s.NoError(err)
		s.Len(tokens, 0)
	})

	s.Run("returns all tokens for that org both system and project scoped", func() {
		tokens, err := s.APIToken.List(ctx, s.org.ID)
		s.NoError(err)
		require.Len(s.T(), tokens, 5)
		s.Equal(s.t1.ID, tokens[0].ID)
		s.Equal(s.t2.ID, tokens[1].ID)
		// It has a name set
		s.NotEmpty(tokens[1].Name)

		tokens, err = s.APIToken.List(ctx, s.org2.ID)
		s.NoError(err)
		require.Len(s.T(), tokens, 1)
		s.Equal(s.t3.ID, tokens[0].ID)
	})

	s.Run("can return only for a specific project", func() {
		tokens, err := s.APIToken.List(ctx, s.org.ID, biz.WithAPITokenProjectFilter([]uuid.UUID{s.p1.ID}))
		s.NoError(err)
		require.Len(s.T(), tokens, 2)
		s.Equal(s.t4.ID, tokens[0].ID)
		s.Equal(s.t5.ID, tokens[1].ID)
	})

	s.Run("can return scoped to a project", func() {
		tokens, err := s.APIToken.List(ctx, s.org.ID, biz.WithAPITokenScope(authz.ResourceTypeProject))
		s.NoError(err)
		require.Len(s.T(), tokens, 3)
	})

	s.Run("can return scoped to a global", func() {
		tokens, err := s.APIToken.List(ctx, s.org.ID, biz.WithAPITokenScope(authz.ResourceTypeOrganization))
		s.NoError(err)
		s.Len(tokens, 2)
	})

	s.Run("they are org scoped", func() {
		tokens, err := s.APIToken.List(ctx, s.org.ID)
		s.NoError(err)
		s.Len(tokens, 5)

		tokens, err = s.APIToken.List(ctx, s.org2.ID)
		s.NoError(err)
		s.Len(tokens, 1)
	})

	s.Run("doesn't return revoked by default", func() {
		// revoke one token
		err := s.APIToken.Revoke(ctx, s.org.ID, s.t1.ID.String())
		require.NoError(s.T(), err)
		tokens, err := s.APIToken.List(ctx, s.org.ID)
		s.NoError(err)
		require.Len(s.T(), tokens, 4)
		s.Equal(s.t2.ID, tokens[0].ID)
	})

	s.Run("status filter all returns revoked and active tokens", func() {
		tokens, err := s.APIToken.List(ctx, s.org.ID, biz.WithAPITokenStatusFilter(biz.APITokenStatusFilterAll))
		s.NoError(err)
		require.Len(s.T(), tokens, 5)
		s.Equal(s.t1.ID, tokens[0].ID)
		s.Equal(s.t2.ID, tokens[1].ID)
	})

	s.Run("status filter active returns only active tokens", func() {
		tokens, err := s.APIToken.List(ctx, s.org.ID, biz.WithAPITokenStatusFilter(biz.APITokenStatusFilterActive))
		s.NoError(err)
		require.Len(s.T(), tokens, 4)
		for _, t := range tokens {
			s.Nil(t.RevokedAt)
		}
	})

	s.Run("status filter revoked returns only revoked tokens", func() {
		tokens, err := s.APIToken.List(ctx, s.org.ID, biz.WithAPITokenStatusFilter(biz.APITokenStatusFilterRevoked))
		s.NoError(err)
		require.Len(s.T(), tokens, 1)
		s.Equal(s.t1.ID, tokens[0].ID)
		s.NotNil(tokens[0].RevokedAt)
	})

	s.Run("system tokens are hidden by default and returned when opted in", func() {
		emptyOrg, err := s.Organization.CreateWithRandomName(ctx)
		require.NoError(s.T(), err)

		regular, err := s.APIToken.Create(ctx, randomName(), nil, nil, &emptyOrg.ID)
		require.NoError(s.T(), err)
		system, err := s.APIToken.Create(ctx, randomName(), nil, nil, &emptyOrg.ID, biz.APITokenAsSystem())
		require.NoError(s.T(), err)

		visible, err := s.APIToken.List(ctx, emptyOrg.ID)
		s.NoError(err)
		require.Len(s.T(), visible, 1)
		s.Equal(regular.ID, visible[0].ID)
		s.False(visible[0].IsSystem)

		all, err := s.APIToken.List(ctx, emptyOrg.ID, biz.WithIncludeSystemTokens())
		s.NoError(err)
		require.Len(s.T(), all, 2)
		ids := []uuid.UUID{all[0].ID, all[1].ID}
		s.Contains(ids, regular.ID)
		s.Contains(ids, system.ID)
	})
}

func (s *apiTokenTestSuite) TestGeneratedJWT() {
	token, err := s.APIToken.Create(context.Background(), randomName(), nil, toPtrDuration(24*time.Hour), &s.org.ID)
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
	org, org2              *biz.Organization
	t1, t2, t3, t4, t5, t6 *biz.APIToken
	p1, p2                 *biz.Project
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
	s.t1, err = s.APIToken.Create(ctx, randomName(), nil, nil, &s.org.ID)
	require.NoError(s.T(), err)
	s.t2, err = s.APIToken.Create(ctx, randomName(), nil, nil, &s.org.ID)
	require.NoError(s.T(), err)
	// and 1 token for org 2
	s.t3, err = s.APIToken.Create(ctx, randomName(), nil, nil, &s.org2.ID)
	require.NoError(s.T(), err)

	// Create 2 tokens for project 1
	s.t4, err = s.APIToken.Create(ctx, randomName(), nil, nil, &s.org.ID, biz.APITokenWithProject(s.p1))
	require.NoError(s.T(), err)
	s.t5, err = s.APIToken.Create(ctx, randomName(), nil, nil, &s.org.ID, biz.APITokenWithProject(s.p1))
	require.NoError(s.T(), err)

	// Create 1 token for project 2
	s.t6, err = s.APIToken.Create(ctx, randomName(), nil, nil, &s.org.ID, biz.APITokenWithProject(s.p2))
	require.NoError(s.T(), err)
}

func (s *apiTokenTestSuite) TestUpdateLastUsedAt() {
	ctx := context.Background()

	s.Run("update last used at", func() {
		token, err := s.APIToken.Create(ctx, randomName(), nil, nil, &s.org.ID)
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
		token, err := s.APIToken.Create(ctx, randomName(), nil, nil, &s.org.ID)
		s.NoError(err)
		err = s.APIToken.Revoke(ctx, s.org.ID, token.ID.String())
		s.NoError(err)

		err = s.APIToken.UpdateLastUsedAt(ctx, token.ID.String())

		s.Error(err)
		s.True(biz.IsNotFound(err))
	})
}

// A product scope is written by the platform and read back as stored.
func (s *apiTokenTestSuite) TestRepoPersistsAndReadsTheResourceScope() {
	ctx := context.Background()
	productID := uuid.New()
	orgUUID := uuid.MustParse(s.org.ID)

	created, err := s.Repos.APITokenRepo.Create(ctx, &biz.APITokenCreateOpts{
		Name:           randomName(),
		OrganizationID: &orgUUID,
		Scope:          biz.ToPtr(authz.ResourceTypeProduct),
		ScopeID:        &productID,
		Policies:       []*authz.Policy{},
	})
	s.Require().NoError(err)

	s.Require().NotNil(created.Scope)
	s.Equal(authz.ResourceTypeProduct, *created.Scope)
	s.Require().NotNil(created.ScopeID)
	s.Equal(productID, *created.ScopeID)
	// A scoped token is confined to neither a project nor a workflow.
	s.Nil(created.ProjectID)
	s.Nil(created.WorkflowID)

	reloaded, err := s.Repos.APITokenRepo.FindByID(ctx, created.ID)
	s.Require().NoError(err)
	s.Require().NotNil(reloaded.ScopeID)
	s.Equal(productID, *reloaded.ScopeID)
}

// A caller such as the platform may name the scope itself. It must agree with the
// organization and project the token is created for, and is refused otherwise.
func (s *apiTokenTestSuite) TestCreateWithAnExplicitScope() {
	ctx := context.Background()
	orgUUID := uuid.MustParse(s.org.ID)
	otherOrg := uuid.MustParse(s.org2.ID)
	productID := uuid.New()
	opts := func(o ...biz.APITokenCreateOpt) []biz.APITokenCreateOpt { return o }

	testCases := []struct {
		name        string
		org         *string
		opts        []biz.APITokenCreateOpt
		wantScope   authz.ResourceType
		wantScopeID *uuid.UUID
		wantErr     bool
	}{
		{
			name: "an organization scope naming its organization", org: &s.org.ID,
			opts:      opts(biz.APITokenWithScope(authz.ResourceTypeOrganization, &orgUUID)),
			wantScope: authz.ResourceTypeOrganization, wantScopeID: &orgUUID,
		},
		{
			name: "a project scope naming its project", org: &s.org.ID,
			opts:      opts(biz.APITokenWithProject(s.p1), biz.APITokenWithScope(authz.ResourceTypeProject, &s.p1.ID)),
			wantScope: authz.ResourceTypeProject, wantScopeID: &s.p1.ID,
		},
		{
			name:      "an instance scope has no id",
			opts:      opts(biz.APITokenWithScope(authz.ResourceTypeInstance, nil)),
			wantScope: authz.ResourceTypeInstance,
		},

		{name: "an organization scope naming another organization", org: &s.org.ID, opts: opts(biz.APITokenWithScope(authz.ResourceTypeOrganization, &otherOrg)), wantErr: true},
		{name: "an organization scope with no id", org: &s.org.ID, opts: opts(biz.APITokenWithScope(authz.ResourceTypeOrganization, nil)), wantErr: true},
		{name: "an organization scope on a project token", org: &s.org.ID, opts: opts(biz.APITokenWithProject(s.p1), biz.APITokenWithScope(authz.ResourceTypeOrganization, &orgUUID)), wantErr: true},
		{name: "an organization scope on an instance-level token", opts: opts(biz.APITokenWithScope(authz.ResourceTypeOrganization, &orgUUID)), wantErr: true},
		{name: "a project scope without its project", org: &s.org.ID, opts: opts(biz.APITokenWithScope(authz.ResourceTypeProject, &s.p1.ID)), wantErr: true},
		{name: "a project scope naming another project", org: &s.org.ID, opts: opts(biz.APITokenWithProject(s.p1), biz.APITokenWithScope(authz.ResourceTypeProject, &s.p2.ID)), wantErr: true},
		{name: "an instance scope with an id", opts: opts(biz.APITokenWithScope(authz.ResourceTypeInstance, &productID)), wantErr: true},
		{name: "an instance scope on an organization token", org: &s.org.ID, opts: opts(biz.APITokenWithScope(authz.ResourceTypeInstance, nil)), wantErr: true},
		{
			name: "a product scope naming its product", org: &s.org.ID,
			opts:      opts(biz.APITokenWithScope(authz.ResourceTypeProduct, &productID)),
			wantScope: authz.ResourceTypeProduct, wantScopeID: &productID,
		},
		{name: "a product scope with no id", org: &s.org.ID, opts: opts(biz.APITokenWithScope(authz.ResourceTypeProduct, nil)), wantErr: true},
		{name: "a kind tokens are never scoped to", org: &s.org.ID, opts: opts(biz.APITokenWithScope(authz.ResourceTypeGroup, &productID)), wantErr: true},
	}

	for _, tc := range testCases {
		s.Run(tc.name, func() {
			created, err := s.APIToken.Create(ctx, randomName(), nil, nil, tc.org, tc.opts...)
			if tc.wantErr {
				s.Require().Error(err)
				s.True(biz.IsErrValidation(err), "want a validation error, got %v", err)
				s.Nil(created)
				return
			}

			s.Require().NoError(err)
			stored, err := s.Repos.APITokenRepo.FindByID(ctx, created.ID)
			s.Require().NoError(err)
			for _, got := range []*biz.APIToken{created, stored} {
				s.Require().NotNil(got.Scope)
				s.Equal(tc.wantScope, *got.Scope)
				s.Equal(tc.wantScopeID, got.ScopeID)
			}
		})
	}
}

// Naming the organization scope explicitly mints the same token as leaving it implied,
// organization-level policies included.
func (s *apiTokenTestSuite) TestAnExplicitOrganizationScopeKeepsTheOrganizationPolicies() {
	ctx := context.Background()
	orgUUID := uuid.MustParse(s.org.ID)

	implied, err := s.APIToken.Create(ctx, randomName(), nil, nil, &s.org.ID)
	s.Require().NoError(err)
	explicit, err := s.APIToken.Create(ctx, randomName(), nil, nil, &s.org.ID,
		biz.APITokenWithScope(authz.ResourceTypeOrganization, &orgUUID))
	s.Require().NoError(err)

	s.ElementsMatch(implied.Policies, explicit.Policies)
}

// Listing scopes are resource kinds; anything tokens are not listed by is refused.
func (s *apiTokenTestSuite) TestListRejectsAScopeTokensAreNotListedBy() {
	ctx := context.Background()
	for _, scope := range []authz.ResourceType{authz.ResourceTypeGroup, "global", "nonsense"} {
		_, err := s.APIToken.List(ctx, s.org.ID, biz.WithAPITokenScope(scope))
		s.Require().Error(err, scope)
		s.True(biz.IsErrValidation(err), "scope %q: want a validation error, got %v", scope, err)
	}
}

// Every token minted from now on records what it is scoped to, so the columns can later back
// a single implementation. Only a product scope drives any logic for now: for the other kinds
// they mirror project_id and organization_id, which stay the fields the control plane reads.
func (s *apiTokenTestSuite) TestCreateRecordsTheScopeOfEveryNewToken() {
	ctx := context.Background()
	orgUUID := uuid.MustParse(s.org.ID)

	wf, err := s.Workflow.Create(ctx, &biz.WorkflowCreateOpts{Name: randomName(), OrgID: s.org.ID, Project: s.p1.Name})
	s.Require().NoError(err)

	testCases := []struct {
		name        string
		org         *string
		opts        []biz.APITokenCreateOpt
		wantScope   authz.ResourceType
		wantScopeID *uuid.UUID
		wantProject *uuid.UUID
	}{
		{name: "an organization-level token", org: &s.org.ID, wantScope: authz.ResourceTypeOrganization, wantScopeID: &orgUUID},
		{name: "a project token", org: &s.org.ID, opts: []biz.APITokenCreateOpt{biz.APITokenWithProject(s.p1)}, wantScope: authz.ResourceTypeProject, wantScopeID: &s.p1.ID, wantProject: &s.p1.ID},
		{name: "a workflow-pinned token is scoped to its project", org: &s.org.ID, opts: []biz.APITokenCreateOpt{biz.APITokenWithProject(s.p1), biz.APITokenWithWorkflow(wf)}, wantScope: authz.ResourceTypeProject, wantScopeID: &s.p1.ID, wantProject: &s.p1.ID},
		{name: "an instance-level token has a kind but no id", wantScope: authz.ResourceTypeInstance},
	}

	for _, tc := range testCases {
		s.Run(tc.name, func() {
			created, err := s.APIToken.Create(ctx, randomName(), nil, nil, tc.org, tc.opts...)
			s.Require().NoError(err)

			stored, err := s.Repos.APITokenRepo.FindByID(ctx, created.ID)
			s.Require().NoError(err)
			for _, got := range []*biz.APIToken{created, stored} {
				s.Require().NotNil(got.Scope)
				s.Equal(tc.wantScope, *got.Scope)
				s.Equal(tc.wantScopeID, got.ScopeID)
				// The fields the existing logic reads are unchanged.
				s.Equal(tc.wantProject, got.ProjectID)
			}
		})
	}
}

// The scope must agree with the row it is on. The database holds that because the platform
// writes these rows from outside this module; a refused row is malformed, not a name clash.
// Rows from before this change carry no scope at all and are untouched.
func (s *apiTokenTestSuite) TestRepoScopeMustAgreeWithTheToken() {
	ctx := context.Background()
	orgUUID := uuid.MustParse(s.org.ID)
	otherOrg := uuid.MustParse(s.org2.ID)
	productID := uuid.New()

	testCases := []struct {
		name      string
		org       *uuid.UUID
		projectID *uuid.UUID
		scope     *authz.ResourceType
		scopeID   *uuid.UUID
		wantErr   bool
	}{
		{name: "a token from before this change carries no scope", org: &orgUUID},
		{name: "an organization scope naming its organization", org: &orgUUID, scope: biz.ToPtr(authz.ResourceTypeOrganization), scopeID: &orgUUID},
		{name: "a project scope naming its project", org: &orgUUID, projectID: &s.p1.ID, scope: biz.ToPtr(authz.ResourceTypeProject), scopeID: &s.p1.ID},
		{name: "an instance scope with no id", scope: biz.ToPtr(authz.ResourceTypeInstance)},
		{name: "a product scope", org: &orgUUID, scope: biz.ToPtr(authz.ResourceTypeProduct), scopeID: &productID},

		{name: "an organization scope naming another organization", org: &orgUUID, scope: biz.ToPtr(authz.ResourceTypeOrganization), scopeID: &otherOrg, wantErr: true},
		{name: "an organization scope on a project token", org: &orgUUID, projectID: &s.p1.ID, scope: biz.ToPtr(authz.ResourceTypeOrganization), scopeID: &orgUUID, wantErr: true},
		{name: "an organization scope with no id", org: &orgUUID, scope: biz.ToPtr(authz.ResourceTypeOrganization), wantErr: true},
		{name: "a project scope naming another project", org: &orgUUID, projectID: &s.p1.ID, scope: biz.ToPtr(authz.ResourceTypeProject), scopeID: &s.p2.ID, wantErr: true},
		{name: "a project scope on a token with no project", org: &orgUUID, scope: biz.ToPtr(authz.ResourceTypeProject), scopeID: &s.p1.ID, wantErr: true},
		{name: "an instance scope with an id", scope: biz.ToPtr(authz.ResourceTypeInstance), scopeID: &productID, wantErr: true},
		{name: "an instance scope on an organization token", org: &orgUUID, scope: biz.ToPtr(authz.ResourceTypeInstance), wantErr: true},
		{name: "a product scope with no id", org: &orgUUID, scope: biz.ToPtr(authz.ResourceTypeProduct), wantErr: true},
		{name: "a product scope alongside a project", org: &orgUUID, projectID: &s.p1.ID, scope: biz.ToPtr(authz.ResourceTypeProduct), scopeID: &productID, wantErr: true},
		{name: "a product scope with no organization", scope: biz.ToPtr(authz.ResourceTypeProduct), scopeID: &productID, wantErr: true},
		{name: "a kind tokens are never scoped to", org: &orgUUID, scope: biz.ToPtr(authz.ResourceTypeGroup), scopeID: &productID, wantErr: true},
		{name: "a scope id without a kind", org: &orgUUID, scopeID: &productID, wantErr: true},
	}

	for _, tc := range testCases {
		s.Run(tc.name, func() {
			_, err := s.Repos.APITokenRepo.Create(ctx, &biz.APITokenCreateOpts{
				Name: randomName(), OrganizationID: tc.org, ProjectID: tc.projectID,
				Scope: tc.scope, ScopeID: tc.scopeID, Policies: []*authz.Policy{},
			})
			if !tc.wantErr {
				s.NoError(err)
				return
			}

			s.Require().Error(err)
			s.True(biz.IsErrValidation(err), "want a validation error, got %v", err)
			s.False(biz.IsErrAlreadyExists(err), "a malformed scope is not a name clash")
		})
	}
}

// Names live in one namespace per product, apart from the organization's own.
func (s *apiTokenTestSuite) TestRepoScopedTokenNameUniqueness() {
	ctx := context.Background()
	orgUUID := uuid.MustParse(s.org.ID)
	productA, productB := uuid.New(), uuid.New()

	scoped := func(name string, productID uuid.UUID) error {
		_, err := s.Repos.APITokenRepo.Create(ctx, &biz.APITokenCreateOpts{
			Name: name, OrganizationID: &orgUUID,
			Scope: biz.ToPtr(authz.ResourceTypeProduct), ScopeID: &productID,
			Policies: []*authz.Policy{},
		})
		return err
	}

	s.Require().NoError(scoped("ci", productA))
	s.Require().NoError(scoped("ci", productB), "the same name in a different product is allowed")
	s.Error(scoped("ci", productA), "the same name in the same product is refused")

	// An organization-level token may still take that name, and stays unique among its own.
	_, err := s.APIToken.Create(ctx, "ci", nil, nil, &s.org.ID)
	s.Require().NoError(err)
	_, err = s.APIToken.Create(ctx, "ci", nil, nil, &s.org.ID)
	s.Error(err)
	s.True(biz.IsErrAlreadyExists(err))
}

// Organization tokens from before this change carry no scope, new ones carry an organization
// scope. They are the same kind of token, so they share one name namespace.
func (s *apiTokenTestSuite) TestOrgTokenNamesStayUniqueAcrossOldAndNewRows() {
	ctx := context.Background()
	orgUUID := uuid.MustParse(s.org.ID)

	preChange := func(name string) error {
		_, err := s.Repos.APITokenRepo.Create(ctx, &biz.APITokenCreateOpts{
			Name: name, OrganizationID: &orgUUID, Policies: []*authz.Policy{},
		})
		return err
	}

	s.Require().NoError(preChange("deploy"))
	_, err := s.APIToken.Create(ctx, "deploy", nil, nil, &s.org.ID)
	s.Require().Error(err, "a new token cannot take the name of an existing one")
	s.True(biz.IsErrAlreadyExists(err))

	_, err = s.APIToken.Create(ctx, "release", nil, nil, &s.org.ID)
	s.Require().NoError(err)
	err = preChange("release")
	s.Require().Error(err, "a token written the old way cannot take the name of a new one")
	s.True(biz.IsErrAlreadyExists(err))
}

// Global means confined to neither a project nor a product: organization tokens from before
// and after this change appear under it, product tokens never do.
func (s *apiTokenTestSuite) TestListByScopeSeparatesProductFromGlobal() {
	ctx := context.Background()
	orgUUID := uuid.MustParse(s.org.ID)
	productID := uuid.New()

	productTokenName := randomName()
	_, err := s.Repos.APITokenRepo.Create(ctx, &biz.APITokenCreateOpts{
		Name: productTokenName, OrganizationID: &orgUUID,
		Scope: biz.ToPtr(authz.ResourceTypeProduct), ScopeID: &productID,
		Policies: []*authz.Policy{},
	})
	s.Require().NoError(err)

	preChangeName := randomName()
	_, err = s.Repos.APITokenRepo.Create(ctx, &biz.APITokenCreateOpts{
		Name: preChangeName, OrganizationID: &orgUUID, Policies: []*authz.Policy{},
	})
	s.Require().NoError(err)

	global, err := s.APIToken.List(ctx, s.org.ID, biz.WithAPITokenScope(authz.ResourceTypeOrganization))
	s.Require().NoError(err)

	names := make([]string, 0, len(global))
	for _, t := range global {
		names = append(names, t.Name)
		s.Nil(t.ProjectID)
		if t.Scope != nil {
			s.NotEqual(authz.ResourceTypeProduct, *t.Scope, "a product token must not appear under the global scope")
		}
	}
	s.Contains(names, preChangeName, "an organization token from before this change")
	s.Contains(names, s.t1.Name, "an organization token minted with a scope")
	s.NotContains(names, productTokenName)

	products, err := s.APIToken.List(ctx, s.org.ID, biz.WithAPITokenScope(authz.ResourceTypeProduct))
	s.Require().NoError(err)
	s.Require().Len(products, 1)
	s.Equal(productTokenName, products[0].Name)

	projects, err := s.APIToken.List(ctx, s.org.ID, biz.WithAPITokenScope(authz.ResourceTypeProject))
	s.Require().NoError(err)
	s.Len(projects, 3, "the project listing is unchanged")
}

// TestCreateWithProductScope covers a token confined to a resource that does not live in this
// database. The scope is persisted on the row, because authorization reads the row and never
// the JWT claim.
func (s *apiTokenTestSuite) TestCreateWithProductScope() {
	ctx := context.Background()
	productID := uuid.New()

	token, err := s.APIToken.Create(ctx, randomName(), nil, nil, &s.org.ID,
		biz.APITokenWithScope(authz.ResourceTypeProduct, &productID))
	s.Require().NoError(err)

	s.Require().NotNil(token.Scope)
	s.Equal(authz.ResourceTypeProduct, *token.Scope)
	s.Require().NotNil(token.ScopeID)
	s.Equal(productID, *token.ScopeID)

	// A scoped token is confined to neither a project nor a workflow.
	s.Nil(token.ProjectID)
	s.Nil(token.WorkflowID)

	// It survives a round trip through the database.
	reloaded, err := s.APIToken.FindByID(ctx, token.ID.String())
	s.Require().NoError(err)
	s.Require().NotNil(reloaded.Scope)
	s.Equal(authz.ResourceTypeProduct, *reloaded.Scope)
	s.Require().NotNil(reloaded.ScopeID)
	s.Equal(productID, *reloaded.ScopeID)
}

// A revoked scoped token releases its name, matching the project and organization indexes.
func (s *apiTokenTestSuite) TestProductTokenNameFreedOnRevocation() {
	ctx := context.Background()
	productID := uuid.New()

	token, err := s.APIToken.Create(ctx, "ci", nil, nil, &s.org.ID,
		biz.APITokenWithScope(authz.ResourceTypeProduct, &productID))
	s.Require().NoError(err)

	s.Require().NoError(s.APIToken.Revoke(ctx, s.org.ID, token.ID.String()))

	_, err = s.APIToken.Create(ctx, "ci", nil, nil, &s.org.ID,
		biz.APITokenWithScope(authz.ResourceTypeProduct, &productID))
	s.NoError(err)
}

// A scoped token carries exactly what a project-scoped token carries. The organization-level
// policies include minting further tokens and reading every registered integration in the
// organization, so a confined credential must never receive them.
func (s *apiTokenTestSuite) TestScopedTokenPoliciesMatchProjectToken() {
	ctx := context.Background()

	projectToken, err := s.APIToken.Create(ctx, randomName(), nil, nil, &s.org.ID, biz.APITokenWithProject(s.p1))
	s.Require().NoError(err)
	productToken, err := s.APIToken.Create(ctx, randomName(), nil, nil, &s.org.ID,
		biz.APITokenWithScope(authz.ResourceTypeProduct, biz.ToPtr(uuid.New())))
	s.Require().NoError(err)

	s.ElementsMatch(projectToken.Policies, productToken.Policies,
		"a scoped token is not organization-wide: its permission ceiling is the project token's")

	for _, forbidden := range []*authz.Policy{
		authz.PolicyAPITokenCreate, authz.PolicyAPITokenList, authz.PolicyAPITokenRevoke,
		authz.PolicyRegisteredIntegrationAdd, authz.PolicyRegisteredIntegrationList,
		authz.PolicyRegisteredIntegrationRead,
	} {
		s.NotContains(productToken.Policies, forbidden)
	}

	// The organization-level token still gets them, which is what makes the guard a narrowing
	// rather than a removal.
	orgToken, err := s.APIToken.Create(ctx, randomName(), nil, nil, &s.org.ID)
	s.Require().NoError(err)
	s.Contains(orgToken.Policies, authz.PolicyAPITokenCreate)
}

// Explicit policies survive untouched whatever the scope: a caller asking for a narrow policy
// set must not receive the organization-level ones on top.
func (s *apiTokenTestSuite) TestExplicitPoliciesAreNotWidenedForScopedTokens() {
	ctx := context.Background()
	explicit := []*authz.Policy{authz.PolicyWorkflowRunRead}

	scoped, err := s.APIToken.Create(ctx, randomName(), nil, nil, &s.org.ID,
		biz.APITokenWithPolicies(explicit),
		biz.APITokenWithScope(authz.ResourceTypeProduct, biz.ToPtr(uuid.New())))
	s.Require().NoError(err)
	s.ElementsMatch(explicit, scoped.Policies)
}

// Global means confined to neither a project nor a scoped resource. Getting this wrong lists a
// product token as an organization-wide one, in the listing whose whole job is to show what a
// credential reaches.
func (s *apiTokenTestSuite) TestListByScope() {
	ctx := context.Background()
	productID := uuid.New()

	orgToken, err := s.APIToken.Create(ctx, randomName(), nil, nil, &s.org.ID)
	s.Require().NoError(err)
	productTokenName := randomName()
	_, err = s.APIToken.Create(ctx, productTokenName, nil, nil, &s.org.ID,
		biz.APITokenWithScope(authz.ResourceTypeProduct, &productID))
	s.Require().NoError(err)

	s.Run("the global scope excludes product-scoped tokens", func() {
		global, err := s.APIToken.List(ctx, s.org.ID, biz.WithAPITokenScope(authz.ResourceTypeOrganization))
		s.Require().NoError(err)
		s.NotEmpty(global)

		names := make([]string, 0, len(global))
		for _, t := range global {
			s.False(t.IsResourceScoped(), "a product-scoped token must not appear under the global scope")
			s.Nil(t.ProjectID)
			names = append(names, t.Name)
		}
		s.Contains(names, orgToken.Name)
		s.NotContains(names, productTokenName)
	})

	s.Run("the product scope returns exactly the product tokens", func() {
		products, err := s.APIToken.List(ctx, s.org.ID, biz.WithAPITokenScope(authz.ResourceTypeProduct))
		s.Require().NoError(err)
		s.Require().Len(products, 1)
		s.Equal(productTokenName, products[0].Name)
		s.Require().NotNil(products[0].ScopeID)
		s.Equal(productID, *products[0].ScopeID)
	})

	s.Run("the project scope is unaffected", func() {
		projects, err := s.APIToken.List(ctx, s.org.ID, biz.WithAPITokenScope(authz.ResourceTypeProject))
		s.Require().NoError(err)
		s.NotEmpty(projects)
		for _, t := range projects {
			s.NotNil(t.ProjectID)
			s.False(t.IsResourceScoped())
		}
	})

	s.Run("an unknown scope is rejected", func() {
		_, err := s.APIToken.List(ctx, s.org.ID, biz.WithAPITokenScope("nonsense"))
		s.Error(err)
		s.True(biz.IsErrValidation(err))
	})
}

// The product claim must be minted on creation and survive a JWT regeneration, so a
// regenerated credential keeps cross-checking against the same row it always did.
func (s *apiTokenTestSuite) TestGeneratedJWTCarriesTheProductScope() {
	ctx := context.Background()
	productID := uuid.New()

	parseClaims := func(raw string) *apitokenjwt.CustomClaims {
		claims := &apitokenjwt.CustomClaims{}
		info, err := jwt.ParseWithClaims(raw, claims, func(_ *jwt.Token) (interface{}, error) {
			return []byte("test"), nil
		})
		s.Require().NoError(err)
		s.True(info.Valid)

		return claims
	}

	token, err := s.APIToken.Create(ctx, randomName(), nil, toPtrDuration(24*time.Hour), &s.org.ID,
		biz.APITokenWithScope(authz.ResourceTypeProduct, &productID))
	s.Require().NoError(err)
	s.Equal(productID.String(), parseClaims(token.JWT).ProductID)

	regenerated, err := s.APIToken.RegenerateJWT(ctx, token.ID, 48*time.Hour)
	s.Require().NoError(err)
	s.Equal(productID.String(), parseClaims(regenerated.JWT).ProductID,
		"a regenerated JWT must keep mirroring the row's scope")

	// A token with no scope carries no claim, so nothing changes for the tokens in existence.
	unscoped, err := s.APIToken.Create(ctx, randomName(), nil, toPtrDuration(24*time.Hour), &s.org.ID)
	s.Require().NoError(err)
	s.Empty(parseClaims(unscoped.JWT).ProductID)
}

// A name identifies at most one token per project and per scoped resource, never one per
// organization, so FindByNameInOrg can match several rows: two project tokens sharing a name,
// or an organization-level token and a product token sharing one. An ambiguous match must
// surface as such rather than as a raw ent NotSingularError the service layer masks into a
// 500.
func (s *apiTokenTestSuite) TestFindByNameInOrgIsAmbiguousNotInternal() {
	ctx := context.Background()

	testCases := []struct {
		name  string
		setup func(name string)
	}{
		{
			name: "two project tokens sharing a name",
			setup: func(n string) {
				_, err := s.APIToken.Create(ctx, n, nil, nil, &s.org.ID, biz.APITokenWithProject(s.p1))
				s.Require().NoError(err)
				_, err = s.APIToken.Create(ctx, n, nil, nil, &s.org.ID, biz.APITokenWithProject(s.p2))
				s.Require().NoError(err)
			},
		},
		{
			name: "an organization-level token and a product token sharing a name",
			setup: func(n string) {
				_, err := s.APIToken.Create(ctx, n, nil, nil, &s.org.ID)
				s.Require().NoError(err)
				_, err = s.APIToken.Create(ctx, n, nil, nil, &s.org.ID,
					biz.APITokenWithScope(authz.ResourceTypeProduct, biz.ToPtr(uuid.New())))
				s.Require().NoError(err)
			},
		},
	}

	for _, tc := range testCases {
		s.Run(tc.name, func() {
			name := randomName()
			tc.setup(name)

			_, err := s.APIToken.FindByNameInOrg(ctx, s.org.ID, name)
			s.Require().Error(err)
			// Whatever the resolution, an ambiguous lookup is the caller's problem to see,
			// not an opaque internal error.
			s.False(biz.IsNotFound(err), "an ambiguous match is not a missing token")
			s.True(biz.IsErrValidation(err), "expected a validation error, got %v", err)
		})
	}
}

// Create must refuse the scope combinations that would discard a confinement, because each of
// them silently widens what the token reaches.
func (s *apiTokenTestSuite) TestCreateRejectsIncoherentScopes() {
	ctx := context.Background()
	productID := uuid.New()

	testCases := []struct {
		name string
		opts []biz.APITokenCreateOpt
		org  *string
	}{
		{
			// IsResourceScoped keys on scope_id, so every project check would be
			// short-circuited and the project confinement enforced nowhere.
			name: "a project scope together with a resource scope",
			opts: []biz.APITokenCreateOpt{
				biz.APITokenWithProject(s.p1),
				biz.APITokenWithScope(authz.ResourceTypeProduct, &productID),
			},
			org: &s.org.ID,
		},
		{
			// Only the product kind is mirrored into the JWT and selected by the product
			// listing, so any other kind is confined but invisible and uncross-checked.
			name: "a scope kind other than product",
			opts: []biz.APITokenCreateOpt{
				biz.APITokenWithScope(authz.ResourceTypeGroup, &productID),
			},
			org: &s.org.ID,
		},
		{
			// An instance-level token has no organization; scoping it to a resource inside
			// one makes it both instance-admin and RBAC-confined.
			name: "a resource scope on an instance-level token",
			opts: []biz.APITokenCreateOpt{
				biz.APITokenWithScope(authz.ResourceTypeProduct, &productID),
			},
			org: nil,
		},
	}

	for _, tc := range testCases {
		s.Run(tc.name, func() {
			_, err := s.APIToken.Create(ctx, randomName(), nil, nil, tc.org, tc.opts...)
			s.Require().Error(err)
			s.True(biz.IsErrValidation(err), "expected a validation error, got %v", err)
		})
	}
}
