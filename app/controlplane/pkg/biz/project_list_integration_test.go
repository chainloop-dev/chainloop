//
// Copyright 2026 The Chainloop Authors.
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

	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/biz"
	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/biz/testhelpers"
	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/pagination"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

// Project names the table cases share.
const (
	projAlphaService = "alpha-service"
	projBetaService  = "beta-service"
	projGammaAPI     = "gamma-api"
)

func TestProjectList(t *testing.T) {
	suite.Run(t, new(projectListIntegrationTestSuite))
}

type projectListIntegrationTestSuite struct {
	testhelpers.UseCasesEachTestSuite
	org      *biz.Organization
	otherOrg *biz.Organization
	// projects in org, created in a deliberately unsorted order
	alpha, beta, gamma *biz.Project
}

func (s *projectListIntegrationTestSuite) SetupTest() {
	var err error
	ctx := context.Background()
	s.TestingUseCases = testhelpers.NewTestingUseCases(s.T())

	s.org, err = s.Organization.CreateWithRandomName(ctx)
	require.NoError(s.T(), err)
	s.otherOrg, err = s.Organization.CreateWithRandomName(ctx)
	require.NoError(s.T(), err)

	s.gamma, err = s.Project.Create(ctx, s.org.ID, projGammaAPI)
	require.NoError(s.T(), err)
	s.alpha, err = s.Project.Create(ctx, s.org.ID, projAlphaService)
	require.NoError(s.T(), err)
	s.beta, err = s.Project.Create(ctx, s.org.ID, projBetaService)
	require.NoError(s.T(), err)

	_, err = s.Project.Create(ctx, s.otherOrg.ID, "other-org-project")
	require.NoError(s.T(), err)
}

func (s *projectListIntegrationTestSuite) TearDownTest() {
	ctx := context.Background()
	_, _ = s.Data.DB.Project.Delete().Exec(ctx)
}

func (s *projectListIntegrationTestSuite) TestList() {
	ctx := context.Background()
	orgID := uuid.MustParse(s.org.ID)
	name := func(v string) *string { return &v }

	testCases := []struct {
		name string
		opts *biz.ProjectListOpts
		// wantNames is the expected result, in order
		wantNames []string
		// wantCount is the total number of matches, before pagination
		wantCount int
	}{
		{
			name:      "nil visible projects lists every project in the organization, sorted by name",
			opts:      &biz.ProjectListOpts{OrganizationID: orgID},
			wantNames: []string{projAlphaService, projBetaService, projGammaAPI},
			wantCount: 3,
		},
		{
			name:      "visible projects restricts the result to that set",
			opts:      &biz.ProjectListOpts{OrganizationID: orgID, VisibleProjects: []uuid.UUID{s.gamma.ID, s.alpha.ID}},
			wantNames: []string{projAlphaService, projGammaAPI},
			wantCount: 2,
		},
		{
			name:      "an empty visible projects set means nothing is visible, not everything",
			opts:      &biz.ProjectListOpts{OrganizationID: orgID, VisibleProjects: []uuid.UUID{}},
			wantNames: []string{},
			wantCount: 0,
		},
		{
			name:      "name filter matches a substring",
			opts:      &biz.ProjectListOpts{OrganizationID: orgID, Name: name("service")},
			wantNames: []string{projAlphaService, projBetaService},
			wantCount: 2,
		},
		{
			name:      "name filter is case insensitive",
			opts:      &biz.ProjectListOpts{OrganizationID: orgID, Name: name("SERVICE")},
			wantNames: []string{projAlphaService, projBetaService},
			wantCount: 2,
		},
		{
			name:      "name filter and visible projects are combined",
			opts:      &biz.ProjectListOpts{OrganizationID: orgID, Name: name("service"), VisibleProjects: []uuid.UUID{s.beta.ID}},
			wantNames: []string{projBetaService},
			wantCount: 1,
		},
		{
			name:      "another organization's projects are never returned",
			opts:      &biz.ProjectListOpts{OrganizationID: uuid.MustParse(s.otherOrg.ID)},
			wantNames: []string{"other-org-project"},
			wantCount: 1,
		},
	}

	for _, tc := range testCases {
		s.Run(tc.name, func() {
			got, count, err := s.Project.List(ctx, tc.opts, nil)
			s.NoError(err)
			s.Equal(tc.wantCount, count)

			gotNames := make([]string, 0, len(got))
			for _, p := range got {
				gotNames = append(gotNames, p.Name)
			}
			s.Equal(tc.wantNames, gotNames)
		})
	}
}

func (s *projectListIntegrationTestSuite) TestListPagination() {
	ctx := context.Background()
	opts := &biz.ProjectListOpts{OrganizationID: uuid.MustParse(s.org.ID)}

	s.Run("the count is the total number of matches, not the page size", func() {
		paginationOpts, err := pagination.NewOffsetPaginationOpts(1, 2)
		require.NoError(s.T(), err)

		got, count, err := s.Project.List(ctx, opts, paginationOpts)
		s.NoError(err)
		s.Equal(3, count)
		s.Len(got, 2)
		s.Equal(projAlphaService, got[0].Name)
		s.Equal(projBetaService, got[1].Name)
	})

	s.Run("the second page continues where the first ended", func() {
		paginationOpts, err := pagination.NewOffsetPaginationOpts(2, 2)
		require.NoError(s.T(), err)

		got, count, err := s.Project.List(ctx, opts, paginationOpts)
		s.NoError(err)
		s.Equal(3, count)
		s.Len(got, 1)
		s.Equal(projGammaAPI, got[0].Name)
	})
}

func (s *projectListIntegrationTestSuite) TestListValidation() {
	ctx := context.Background()

	s.Run("the organization is required", func() {
		_, _, err := s.Project.List(ctx, &biz.ProjectListOpts{}, nil)
		s.Error(err)
		s.True(biz.IsErrValidation(err))
	})

	s.Run("the options are required", func() {
		_, _, err := s.Project.List(ctx, nil, nil)
		s.Error(err)
		s.True(biz.IsErrValidation(err))
	})
}
