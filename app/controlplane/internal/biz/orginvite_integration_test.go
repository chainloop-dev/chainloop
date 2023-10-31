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
	"github.com/google/uuid"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

const receiverEmail = "sarah@cyberdyne.io"

func (s *OrgInviteIntegrationTestSuite) TestCreate() {
	s.T().Run("invalid org ID", func(t *testing.T) {
		invite, err := s.OrgInvite.Create(context.Background(), "deadbeef", s.user.ID, receiverEmail)
		s.Error(err)
		s.True(biz.IsErrInvalidUUID(err))
		s.Nil(invite)
	})

	s.T().Run("invalid user ID", func(t *testing.T) {
		invite, err := s.OrgInvite.Create(context.Background(), s.org1.ID, "deadbeef", receiverEmail)
		s.Error(err)
		s.True(biz.IsErrInvalidUUID(err))
		s.Nil(invite)
	})

	s.T().Run("missing receiver email", func(t *testing.T) {
		invite, err := s.OrgInvite.Create(context.Background(), s.org1.ID, s.user.ID, "")
		s.Error(err)
		s.True(biz.IsErrValidation(err))
		s.Nil(invite)
	})

	s.T().Run("receiver email same than sender", func(t *testing.T) {
		invite, err := s.OrgInvite.Create(context.Background(), s.org1.ID, s.user.ID, s.user.Email)
		s.Error(err)
		s.ErrorContains(err, "sender and receiver emails cannot be the same")
		s.True(biz.IsErrValidation(err))
		s.Nil(invite)
	})

	s.T().Run("receiver is already a member", func(t *testing.T) {
		invite, err := s.OrgInvite.Create(context.Background(), s.org1.ID, s.user.ID, s.user2.Email)
		s.Error(err)
		s.ErrorContains(err, "user already exists in the org")
		s.True(biz.IsErrValidation(err))
		s.Nil(invite)
	})

	s.T().Run("org not found", func(t *testing.T) {
		invite, err := s.OrgInvite.Create(context.Background(), s.org1.ID, uuid.NewString(), receiverEmail)
		s.Error(err)
		s.True(biz.IsNotFound(err))
		s.Nil(invite)
	})

	s.T().Run("user is not member of that org", func(t *testing.T) {
		invite, err := s.OrgInvite.Create(context.Background(), s.org3.ID, s.user.ID, receiverEmail)
		s.Error(err)
		s.True(biz.IsNotFound(err))
		s.Nil(invite)
	})

	s.T().Run("can create invites for org1 and 2", func(t *testing.T) {
		for _, org := range []*biz.Organization{s.org1, s.org2} {
			invite, err := s.OrgInvite.Create(context.Background(), org.ID, s.user.ID, receiverEmail)
			s.NoError(err)
			s.Equal(org, invite.Org)
			s.Equal(s.user, invite.Sender)
			s.Equal(receiverEmail, invite.ReceiverEmail)
			s.Equal(biz.OrgInviteStatusPending, invite.Status)
			s.NotNil(invite.CreatedAt)
		}
	})

	s.T().Run("but can't create if there is one pending", func(t *testing.T) {
		invite, err := s.OrgInvite.Create(context.Background(), s.org1.ID, s.user.ID, receiverEmail)
		s.Error(err)
		s.ErrorContains(err, "already exists")
		s.True(biz.IsErrValidation(err))
		s.Nil(invite)
	})

	s.T().Run("but it can if it's another email", func(t *testing.T) {
		invite, err := s.OrgInvite.Create(context.Background(), s.org1.ID, s.user.ID, "anotheremail@cyberdyne.io")
		s.Equal("anotheremail@cyberdyne.io", invite.ReceiverEmail)
		s.Equal(s.org1, invite.Org)
		s.NoError(err)
	})
}

func (s *OrgInviteIntegrationTestSuite) TestAcceptPendingInvites() {
	ctx := context.Background()
	receiver, err := s.User.FindOrCreateByEmail(ctx, receiverEmail)
	require.NoError(s.T(), err)

	s.T().Run("user doesn't exist", func(t *testing.T) {
		err := s.OrgInvite.AcceptPendingInvites(ctx, "non-existant@cyberdyne.io")
		s.ErrorContains(err, "not found")
	})

	s.T().Run("no invites for user", func(t *testing.T) {
		err = s.OrgInvite.AcceptPendingInvites(ctx, receiverEmail)
		s.NoError(err)

		memberships, err := s.Membership.ByUser(ctx, receiver.ID)
		s.NoError(err)
		s.Len(memberships, 0)
	})

	s.T().Run("user is invited to org 1", func(t *testing.T) {
		invite, err := s.OrgInvite.Create(ctx, s.org1.ID, s.user.ID, receiverEmail)
		require.NoError(s.T(), err)
		err = s.OrgInvite.AcceptPendingInvites(ctx, receiverEmail)
		s.NoError(err)

		memberships, err := s.Membership.ByUser(ctx, receiver.ID)
		s.NoError(err)
		s.Len(memberships, 1)
		assert.Equal(s.T(), s.org1.ID, memberships[0].OrganizationID.String())

		// the invite is now accepted
		invite, err = s.OrgInvite.FindByID(ctx, invite.ID.String())
		s.NoError(err)
		s.Equal(biz.OrgInviteStatusAccepted, invite.Status)
	})
}

func (s *OrgInviteIntegrationTestSuite) TestRevoke() {
	s.T().Run("invalid ID", func(t *testing.T) {
		err := s.OrgInvite.Revoke(context.Background(), s.user.ID, "deadbeef")
		s.Error(err)
		s.True(biz.IsErrInvalidUUID(err))
	})

	s.T().Run("invalid user ID", func(t *testing.T) {
		err := s.OrgInvite.Revoke(context.Background(), "deadbeef", uuid.NewString())
		s.Error(err)
		s.True(biz.IsErrInvalidUUID(err))
	})

	s.T().Run("invitation not found", func(t *testing.T) {
		err := s.OrgInvite.Revoke(context.Background(), s.user.ID, uuid.NewString())
		s.Error(err)
		s.True(biz.IsNotFound(err))
	})

	s.T().Run("invitation not created by this user", func(t *testing.T) {
		invite, err := s.OrgInvite.Create(context.Background(), s.org1.ID, s.user2.ID, "anotheremail@cyberdyne.io")
		require.NoError(s.T(), err)
		err = s.OrgInvite.Revoke(context.Background(), s.user.ID, invite.ID.String())
		s.Error(err)
		s.True(biz.IsNotFound(err))
	})

	s.T().Run("invitation not in pending state", func(t *testing.T) {
		invite, err := s.OrgInvite.Create(context.Background(), s.org1.ID, s.user.ID, receiverEmail)
		require.NoError(s.T(), err)
		err = s.OrgInvite.AcceptInvite(context.Background(), invite.ID.String())
		require.NoError(s.T(), err)

		// It's in accepted state now
		err = s.OrgInvite.Revoke(context.Background(), s.user.ID, invite.ID.String())
		s.Error(err)
		s.ErrorContains(err, "not in pending state")
		s.True(biz.IsErrValidation(err))
	})

	s.T().Run("happy path", func(t *testing.T) {
		invite, err := s.OrgInvite.Create(context.Background(), s.org1.ID, s.user.ID, receiverEmail)
		require.NoError(s.T(), err)
		err = s.OrgInvite.Revoke(context.Background(), s.user.ID, invite.ID.String())
		s.NoError(err)
		err = s.OrgInvite.Revoke(context.Background(), s.user.ID, invite.ID.String())
		s.Error(err)
		s.True(biz.IsNotFound(err))
	})
}

// Run the tests
func TestOrgInviteUseCase(t *testing.T) {
	suite.Run(t, new(OrgInviteIntegrationTestSuite))
}

// Utility struct to hold the test suite
type OrgInviteIntegrationTestSuite struct {
	testhelpers.UseCasesEachTestSuite
	org1, org2, org3 *biz.Organization
	user, user2      *biz.User
}

// 3 orgs, user belongs to org1 and org2 but not org3
func (s *OrgInviteIntegrationTestSuite) SetupTest() {
	t := s.T()
	var err error
	assert := assert.New(s.T())
	ctx := context.Background()

	s.TestingUseCases = testhelpers.NewTestingUseCases(t)
	s.org1, err = s.Organization.Create(ctx, "org1")
	assert.NoError(err)
	s.org2, err = s.Organization.Create(ctx, "org2")
	assert.NoError(err)
	s.org3, err = s.Organization.Create(ctx, "org3")
	assert.NoError(err)

	// Create User 1
	s.user, err = s.User.FindOrCreateByEmail(ctx, "user-1@test.com")
	assert.NoError(err)
	// Attach both orgs
	_, err = s.Membership.Create(ctx, s.org1.ID, s.user.ID, true)
	assert.NoError(err)
	_, err = s.Membership.Create(ctx, s.org2.ID, s.user.ID, true)
	assert.NoError(err)

	s.user2, err = s.User.FindOrCreateByEmail(ctx, "user-2@test.com")
	assert.NoError(err)
	_, err = s.Membership.Create(ctx, s.org1.ID, s.user2.ID, true)
	assert.NoError(err)
}
