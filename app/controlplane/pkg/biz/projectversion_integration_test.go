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
	"testing"
	"time"

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

func (s *ProjectVersionIntegrationTestSuite) TestReleasedAtTimestampPreserved() {
	t := s.T()
	ctx := context.Background()

	// Create a prerelease version
	version, err := s.ProjectVersion.Create(ctx, s.project.ID.String(), "2.0.0", true)
	require.NoError(t, err)
	require.True(t, version.Prerelease)
	require.Nil(t, version.ReleasedAt, "Prerelease version should not have released_at set")

	// Update to release status for the first time
	releasedVersion, err := s.ProjectVersion.UpdateReleaseStatus(ctx, version.ID.String(), true)
	require.NoError(t, err)
	require.False(t, releasedVersion.Prerelease)
	require.NotNil(t, releasedVersion.ReleasedAt, "Released version should have released_at set")
	firstReleasedAt := releasedVersion.ReleasedAt

	// Wait a bit to ensure timestamps would differ if reset
	time.Sleep(100 * time.Millisecond)

	// Update to release status again (should preserve original timestamp)
	reReleasedVersion, err := s.ProjectVersion.UpdateReleaseStatus(ctx, version.ID.String(), true)
	require.NoError(t, err)
	require.False(t, reReleasedVersion.Prerelease)
	require.NotNil(t, reReleasedVersion.ReleasedAt, "Released version should still have released_at set")
	require.Equal(t, firstReleasedAt, reReleasedVersion.ReleasedAt, "released_at timestamp should be preserved on subsequent release updates")

	// Update back to prerelease (should clear released_at)
	preReleaseVersion, err := s.ProjectVersion.UpdateReleaseStatus(ctx, version.ID.String(), false)
	require.NoError(t, err)
	require.True(t, preReleaseVersion.Prerelease)
	require.Nil(t, preReleaseVersion.ReleasedAt, "Prerelease version should have released_at cleared")

	// Update to release status again (should set a new timestamp)
	time.Sleep(100 * time.Millisecond)
	newReleasedVersion, err := s.ProjectVersion.UpdateReleaseStatus(ctx, version.ID.String(), true)
	require.NoError(t, err)
	require.False(t, newReleasedVersion.Prerelease)
	require.NotNil(t, newReleasedVersion.ReleasedAt, "Re-released version should have released_at set")
	require.NotEqual(t, firstReleasedAt, newReleasedVersion.ReleasedAt, "released_at should be a new timestamp after toggling through prerelease")
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
