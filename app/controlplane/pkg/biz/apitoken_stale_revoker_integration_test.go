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
	"database/sql"
	"fmt"
	"testing"
	"time"

	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/biz"
	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/biz/testhelpers"
	"github.com/google/uuid"
	_ "github.com/lib/pq"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

func TestAPITokenStaleRevoker(t *testing.T) {
	suite.Run(t, new(staleRevokerTestSuite))
}

type staleRevokerTestSuite struct {
	testhelpers.UseCasesEachTestSuite
	revoker *biz.APITokenStaleRevoker
	user    *biz.User
}

func (s *staleRevokerTestSuite) SetupTest() {
	t := s.T()
	ctx := context.Background()

	s.TestingUseCases = testhelpers.NewTestingUseCases(t)
	s.revoker = biz.NewAPITokenStaleRevoker(s.Repos.OrganizationRepo, s.Repos.APITokenRepo, s.APIToken, s.L)

	var err error
	s.user, err = s.User.UpsertByEmail(ctx, "revoker-test@test.com", nil)
	require.NoError(t, err)
}

func (s *staleRevokerTestSuite) TestSweepNoOrgsWithThreshold() {
	ctx := context.Background()

	// Create an org without any threshold
	_, err := s.Organization.CreateWithRandomName(ctx)
	s.Require().NoError(err)

	err = s.revoker.Sweep(ctx)
	s.NoError(err)
}

func (s *staleRevokerTestSuite) TestSweepAllTokensRecentlyActive() {
	ctx := context.Background()

	org := s.createOrgWithThreshold(ctx, 30)

	// Create a token and mark it as recently used
	token := s.createToken(ctx, org.ID)
	s.setLastUsedAt(ctx, token.ID, time.Now().Add(-1*24*time.Hour)) // used 1 day ago

	err := s.revoker.Sweep(ctx)
	s.NoError(err)

	// Token should NOT be revoked
	s.assertTokenNotRevoked(ctx, token.ID)
}

func (s *staleRevokerTestSuite) TestSweepTokenNeverUsedCreatedBeforeCutoff() {
	ctx := context.Background()

	org := s.createOrgWithThreshold(ctx, 30)

	// Create a token and backdate its created_at to 40 days ago (never used)
	token := s.createToken(ctx, org.ID)
	s.backdateCreatedAt(ctx, token.ID, time.Now().Add(-40*24*time.Hour))

	err := s.revoker.Sweep(ctx)
	s.NoError(err)

	// Token SHOULD be revoked
	s.assertTokenRevoked(ctx, token.ID)
}

func (s *staleRevokerTestSuite) TestSweepTokenUsedButInactive() {
	ctx := context.Background()

	org := s.createOrgWithThreshold(ctx, 30)

	// Create a token, mark it as used 40 days ago
	token := s.createToken(ctx, org.ID)
	s.setLastUsedAt(ctx, token.ID, time.Now().Add(-40*24*time.Hour))

	err := s.revoker.Sweep(ctx)
	s.NoError(err)

	// Token SHOULD be revoked
	s.assertTokenRevoked(ctx, token.ID)
}

func (s *staleRevokerTestSuite) TestSweepMixedStaleAndActive() {
	ctx := context.Background()

	org := s.createOrgWithThreshold(ctx, 30)

	// Active token: used 5 days ago
	activeToken := s.createToken(ctx, org.ID)
	s.setLastUsedAt(ctx, activeToken.ID, time.Now().Add(-5*24*time.Hour))

	// Stale token: used 40 days ago
	staleToken := s.createToken(ctx, org.ID)
	s.setLastUsedAt(ctx, staleToken.ID, time.Now().Add(-40*24*time.Hour))

	// Stale token: never used, created 40 days ago
	neverUsedToken := s.createToken(ctx, org.ID)
	s.backdateCreatedAt(ctx, neverUsedToken.ID, time.Now().Add(-40*24*time.Hour))

	err := s.revoker.Sweep(ctx)
	s.NoError(err)

	s.assertTokenNotRevoked(ctx, activeToken.ID)
	s.assertTokenRevoked(ctx, staleToken.ID)
	s.assertTokenRevoked(ctx, neverUsedToken.ID)
}

func (s *staleRevokerTestSuite) TestSweepMultipleOrgsWithDifferentThresholds() {
	ctx := context.Background()

	// Org1: 30-day threshold
	org1 := s.createOrgWithThreshold(ctx, 30)
	// Org2: 90-day threshold
	org2 := s.createOrgWithThreshold(ctx, 90)

	// Token in org1: used 40 days ago (stale for org1's 30-day threshold)
	token1 := s.createToken(ctx, org1.ID)
	s.setLastUsedAt(ctx, token1.ID, time.Now().Add(-40*24*time.Hour))

	// Token in org2: used 40 days ago (NOT stale for org2's 90-day threshold)
	token2 := s.createToken(ctx, org2.ID)
	s.setLastUsedAt(ctx, token2.ID, time.Now().Add(-40*24*time.Hour))

	err := s.revoker.Sweep(ctx)
	s.NoError(err)

	s.assertTokenRevoked(ctx, token1.ID)
	s.assertTokenNotRevoked(ctx, token2.ID)
}

func (s *staleRevokerTestSuite) TestSweepAlreadyRevokedTokensNotAffected() {
	ctx := context.Background()

	org := s.createOrgWithThreshold(ctx, 30)

	// Create and manually revoke a token
	token := s.createToken(ctx, org.ID)
	err := s.APIToken.Revoke(ctx, org.ID, token.ID.String())
	s.Require().NoError(err)

	// Verify it's revoked
	revokedToken, err := s.APIToken.FindByID(ctx, token.ID.String())
	s.Require().NoError(err)
	s.Require().NotNil(revokedToken.RevokedAt)
	revokedAt := *revokedToken.RevokedAt

	// Run sweep
	err = s.revoker.Sweep(ctx)
	s.NoError(err)

	// Token's revoked_at should remain unchanged
	afterSweep, err := s.APIToken.FindByID(ctx, token.ID.String())
	s.Require().NoError(err)
	s.Require().NotNil(afterSweep.RevokedAt)
	s.Equal(revokedAt.Truncate(time.Millisecond), afterSweep.RevokedAt.Truncate(time.Millisecond))
}

// --- helpers ---

func (s *staleRevokerTestSuite) createOrgWithThreshold(ctx context.Context, days int) *biz.Organization {
	s.T().Helper()

	org, err := s.Organization.CreateWithRandomName(ctx)
	require.NoError(s.T(), err)

	// Need a membership so Update works
	_, err = s.Membership.Create(ctx, org.ID, s.user.ID, biz.WithCurrentMembership())
	require.NoError(s.T(), err)

	org, err = s.Organization.Update(ctx, s.user.ID, org.Name, nil, nil, nil, nil, &days)
	require.NoError(s.T(), err)
	require.NotNil(s.T(), org.APITokenInactivityThresholdDays)
	assert.Equal(s.T(), days, *org.APITokenInactivityThresholdDays)

	return org
}

func (s *staleRevokerTestSuite) createToken(ctx context.Context, orgID string) *biz.APIToken {
	s.T().Helper()
	name := fmt.Sprintf("token-%s", uuid.New().String())
	token, err := s.APIToken.Create(ctx, name, nil, nil, &orgID)
	require.NoError(s.T(), err)
	return token
}

func (s *staleRevokerTestSuite) setLastUsedAt(ctx context.Context, tokenID uuid.UUID, t time.Time) {
	s.T().Helper()
	err := s.Repos.APITokenRepo.UpdateLastUsedAt(ctx, tokenID, t)
	require.NoError(s.T(), err)
}

func (s *staleRevokerTestSuite) backdateCreatedAt(ctx context.Context, tokenID uuid.UUID, t time.Time) {
	s.T().Helper()
	// created_at is immutable in Ent, so we use a raw SQL update via a direct DB connection
	db, err := sql.Open("postgres", s.DB.ConnectionString(s.T())+"?sslmode=disable")
	require.NoError(s.T(), err)
	defer db.Close()

	_, err = db.ExecContext(ctx, "UPDATE api_tokens SET created_at = $1 WHERE id = $2", t, tokenID)
	require.NoError(s.T(), err)
}

func (s *staleRevokerTestSuite) assertTokenRevoked(ctx context.Context, tokenID uuid.UUID) {
	s.T().Helper()
	token, err := s.APIToken.FindByID(ctx, tokenID.String())
	s.Require().NoError(err)
	s.NotNil(token.RevokedAt, "expected token %s to be revoked", tokenID)
}

func (s *staleRevokerTestSuite) assertTokenNotRevoked(ctx context.Context, tokenID uuid.UUID) {
	s.T().Helper()
	token, err := s.APIToken.FindByID(ctx, tokenID.String())
	s.Require().NoError(err)
	s.Nil(token.RevokedAt, "expected token %s to NOT be revoked", tokenID)
}
