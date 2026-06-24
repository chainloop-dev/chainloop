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

func (s *ProjectVersionIntegrationTestSuite) TestMarkAsLatest() {
	t := s.T()
	ctx := context.Background()

	// Create two pre-release versions — the second one becomes latest by default
	v1, err := s.ProjectVersion.Create(ctx, s.project.ID.String(), "1.0.0", true)
	require.NoError(t, err)
	require.True(t, v1.Latest)

	v2, err := s.ProjectVersion.Create(ctx, s.project.ID.String(), "2.0.0", true)
	require.NoError(t, err)
	require.True(t, v2.Latest)

	// v1 should no longer be latest after v2 was created
	v1After, err := s.ProjectVersion.FindByProjectAndVersion(ctx, s.project.ID.String(), "1.0.0")
	require.NoError(t, err)
	require.False(t, v1After.Latest)

	// Promote v1 back to latest
	err = s.ProjectVersion.MarkAsLatest(ctx, s.project.ID.String(), v1.ID.String())
	require.NoError(t, err)

	// Verify v1 is now latest
	v1Promoted, err := s.ProjectVersion.FindByProjectAndVersion(ctx, s.project.ID.String(), "1.0.0")
	require.NoError(t, err)
	require.True(t, v1Promoted.Latest)

	// Verify v2 was demoted
	v2Demoted, err := s.ProjectVersion.FindByProjectAndVersion(ctx, s.project.ID.String(), "2.0.0")
	require.NoError(t, err)
	require.False(t, v2Demoted.Latest)
}

func (s *ProjectVersionIntegrationTestSuite) TestMarkAsLatestReleasedVersionError() {
	t := s.T()
	ctx := context.Background()

	// Create a version and release it
	v, err := s.ProjectVersion.Create(ctx, s.project.ID.String(), "1.0.0", true)
	require.NoError(t, err)

	_, err = s.ProjectVersion.UpdateReleaseStatus(ctx, v.ID.String(), true)
	require.NoError(t, err)

	// Attempting to mark a released version as latest should fail
	err = s.ProjectVersion.MarkAsLatest(ctx, s.project.ID.String(), v.ID.String())
	require.Error(t, err)
	require.True(t, biz.IsErrValidation(err))
}

func (s *ProjectVersionIntegrationTestSuite) TestMarkAsLatestIdempotent() {
	t := s.T()
	ctx := context.Background()

	v, err := s.ProjectVersion.Create(ctx, s.project.ID.String(), "1.0.0", true)
	require.NoError(t, err)
	require.True(t, v.Latest)

	// Promoting a version that is already latest should succeed (idempotent)
	err = s.ProjectVersion.MarkAsLatest(ctx, s.project.ID.String(), v.ID.String())
	require.NoError(t, err)

	reloaded, err := s.ProjectVersion.FindByProjectAndVersion(ctx, s.project.ID.String(), "1.0.0")
	require.NoError(t, err)
	require.True(t, reloaded.Latest)
}

func (s *ProjectVersionIntegrationTestSuite) TestMarkAsLatestNonExistentVersion() {
	t := s.T()
	ctx := context.Background()

	nonExistentID := "00000000-0000-0000-0000-000000000099"
	err := s.ProjectVersion.MarkAsLatest(ctx, s.project.ID.String(), nonExistentID)
	require.Error(t, err)
	require.True(t, biz.IsNotFound(err))
}

func (s *ProjectVersionIntegrationTestSuite) TestMarkAsLatestInvalidUUID() {
	t := s.T()
	ctx := context.Background()

	err := s.ProjectVersion.MarkAsLatest(ctx, "invalid", "invalid")
	require.Error(t, err)
}

// Regression test for PFM-6470: a version that was deleted and recreated with the
// same name leaves a soft-deleted row alongside the active one. Looking the version
// up by name must return the active row instead of crashing because the query matched
// more than one row.
func (s *ProjectVersionIntegrationTestSuite) TestFindByProjectAndVersionIgnoresSoftDeleted() {
	t := s.T()
	ctx := context.Background()

	// Create a version and soft-delete it, simulating a delete from the UI/API.
	deleted, err := s.ProjectVersion.Create(ctx, s.project.ID.String(), "v17", true)
	require.NoError(t, err)

	_, err = s.Data.DB.ProjectVersion.UpdateOneID(deleted.ID).SetDeletedAt(time.Now()).Save(ctx)
	require.NoError(t, err)

	// Recreate a version with the same name; now two rows share version "v17"
	// (one soft-deleted, one active).
	recreated, err := s.ProjectVersion.Create(ctx, s.project.ID.String(), "v17", true)
	require.NoError(t, err)
	require.NotEqual(t, deleted.ID, recreated.ID)

	// Looking up "v17" must return the active row, not error out because the query
	// matched both the soft-deleted and the active row.
	found, err := s.ProjectVersion.FindByProjectAndVersion(ctx, s.project.ID.String(), "v17")
	require.NoError(t, err)
	require.NotNil(t, found)
	require.Equal(t, recreated.ID, found.ID)
}

func TestProjectVersionUseCase(t *testing.T) {
	suite.Run(t, new(ProjectVersionIntegrationTestSuite))
}
