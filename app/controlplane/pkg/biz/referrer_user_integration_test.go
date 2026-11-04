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
	"bytes"
	"context"

	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/authz"
	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/biz"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

// A user discovers only in the given organization, and only with the project visibility
// that the user has in that organization: a visible project in another organization must
// not make a referrer of the given organization visible.
func (s *referrerIntegrationTestSuite) TestGetFromRootUserSingleOrg() {
	ctx := context.Background()
	envelope, envBytes := testEnvelope(s.T(), "testdata/attestations/with-git-subject.json")
	attDigest, _, err := v1.SHA256(bytes.NewReader(envBytes))
	require.NoError(s.T(), err)
	// The same attestation lives in a project of org1 and in a project of org2
	require.NoError(s.T(), s.Referrer.ExtractAndPersist(ctx, envelope, attDigest, s.workflow1.ID.String()))
	require.NoError(s.T(), s.Referrer.ExtractAndPersist(ctx, envelope, attDigest, s.workflow2.ID.String()))
	digest := attDigest.String()

	// A member with RBAC in both organizations who sees only the org2 project
	rbacUser, err := s.User.UpsertByEmail(ctx, "rbac-user@test.com", nil)
	require.NoError(s.T(), err)
	_, err = s.Membership.Create(ctx, s.org1.ID, rbacUser.ID, biz.WithMembershipRole(authz.RoleOrgMember))
	require.NoError(s.T(), err)
	_, err = s.Membership.Create(ctx, s.org2.ID, rbacUser.ID, biz.WithMembershipRole(authz.RoleOrgMember), biz.WithCurrentMembership())
	require.NoError(s.T(), err)
	require.NoError(s.T(), s.Membership.SetProjectOwner(ctx, s.org2UUID, s.workflow2.ProjectID, uuid.MustParse(rbacUser.ID)))

	testCases := []struct {
		name      string
		userID    string
		orgID     uuid.UUID
		wantFound bool
	}{
		{
			name:      "a member of the organization",
			userID:    s.user.ID,
			orgID:     s.org1UUID,
			wantFound: true,
		},
		{
			name:   "an organization the user is not a member of",
			userID: s.user2.ID,
			orgID:  s.org1UUID,
		},
		{
			name:      "a visible project in the organization",
			userID:    rbacUser.ID,
			orgID:     s.org2UUID,
			wantFound: true,
		},
		{
			name:   "a visible project in another organization does not make it visible",
			userID: rbacUser.ID,
			orgID:  s.org1UUID,
		},
	}

	for _, tc := range testCases {
		s.Run(tc.name, func() {
			got, _, err := s.Referrer.GetFromRootUser(ctx, digest, "", tc.userID, tc.orgID, nil)
			if !tc.wantFound {
				s.True(biz.IsNotFound(err), "expected not found, got %v", err)
				s.Nil(got)
				return
			}

			s.Require().NoError(err)
			s.Equal(digest, got.Digest)
		})
	}
}
