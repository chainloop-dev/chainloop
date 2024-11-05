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

package biz_test

import (
	"context"
	"testing"

	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/biz"
	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/biz/testhelpers"

	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

// Utility struct to hold the test suite
type ProjectVersionIntegrationTestSuite struct {
	testhelpers.UseCasesEachTestSuite
	org     *biz.Organization
	project *biz.Project
}

func (s *ProjectVersionIntegrationTestSuite) TestUpdateReleaseStatus() {
	t := s.T()
	ctx := context.Background()

	version, err := s.ProjectVersion.Create(ctx, s.project.ID.String(), "1.0.0", true)
	require.NoError(t, err)

	// Test updating to release status
	updatedVersion, err := s.ProjectVersion.UpdateReleaseStatus(ctx, version.ID.String(), true)
	require.NoError(t, err)
	require.NotNil(t, updatedVersion)
	require.False(t, updatedVersion.Prerelease)

	// Test updating back to prerelease status
	updatedVersion, err = s.ProjectVersion.UpdateReleaseStatus(ctx, version.ID.String(), false)
	require.NoError(t, err)
	require.NotNil(t, updatedVersion)
	require.True(t, updatedVersion.Prerelease)

	// Test with invalid UUID
	_, err = s.ProjectVersion.UpdateReleaseStatus(ctx, "invalid-uuid", true)
	require.Error(t, err)
}

// 3 orgs, user belongs to org1 and org2 but not org3
func (s *ProjectVersionIntegrationTestSuite) SetupTest() {
	t := s.T()
	var err error
	ctx := context.Background()

	s.TestingUseCases = testhelpers.NewTestingUseCases(t)
	s.org, err = s.Organization.Create(ctx, "org1")
	require.NoError(t, err)
	s.project, err = s.Project.Create(ctx, s.org.ID, "project1")
	require.NoError(t, err)
}

func TestProjectVersionUseCase(t *testing.T) {
	suite.Run(t, new(ProjectVersionIntegrationTestSuite))
}
