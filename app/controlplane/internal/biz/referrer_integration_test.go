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
	"encoding/json"
	"os"
	"testing"

	"github.com/chainloop-dev/chainloop/app/controlplane/internal/biz"
	"github.com/chainloop-dev/chainloop/app/controlplane/internal/biz/testhelpers"
	"github.com/google/uuid"
	"github.com/secure-systems-lab/go-securesystemslib/dsse"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

func (s *referrerIntegrationTestSuite) TestExtractAndPersists() {
	// Load attestation
	attJSON, err := os.ReadFile("testdata/attestations/with-git-subject.json")
	require.NoError(s.T(), err)
	var envelope *dsse.Envelope
	require.NoError(s.T(), json.Unmarshal(attJSON, &envelope))

	wantReferrerAtt := &biz.StoredReferrer{
		Digest:       "sha256:ad704d286bcad6e155e71c33d48247931231338396acbcd9769087530085b2a2",
		Kind:         "ATTESTATION",
		Downloadable: true,
	}

	wantReferrerCommit := &biz.StoredReferrer{
		Digest: "sha1:78ac366c9e8a300d51808d581422ca61f7b5b721",
		Kind:   "GIT_HEAD_COMMIT",
	}

	wantReferrerSBOM := &biz.StoredReferrer{
		Digest:       "sha256:16159bb881eb4ab7eb5d8afc5350b0feeed1e31c0a268e355e74f9ccbe885e0c",
		Kind:         "SBOM_CYCLONEDX_JSON",
		Downloadable: true,
	}

	wantReferrerArtifact := &biz.StoredReferrer{
		Digest:       "sha256:385c4188b9c080499413f2e0fa0b3951ed107b5f0cb35c2f2b1f07a7be9a7512",
		Kind:         "ARTIFACT",
		Downloadable: true,
	}

	wantReferrerOpenVEX := &biz.StoredReferrer{
		Digest:       "sha256:b4bd86d5855f94bcac0a92d3100ae7b85d050bd2e5fb9037a200e5f5f0b073a2",
		Kind:         "OPENVEX",
		Downloadable: true,
	}

	wantReferrerSarif := &biz.StoredReferrer{
		Digest:       "sha256:c4a63494f9289dd9fd44f841efb4f5b52765c2de6332f2d86e5f6c0340b40a95",
		Kind:         "SARIF",
		Downloadable: true,
	}

	wantReferrerContainerImage := &biz.StoredReferrer{
		Digest: "sha256:fbd9335f55d83d8aaf9ab1a539b0f2a87b444e8c54f34c9a1ca9d7df15605db4",
		Kind:   "CONTAINER_IMAGE",
	}

	ctx := context.Background()
	s.T().Run("creation fails if the org doesn't exist", func(t *testing.T) {
		err := s.Referrer.ExtractAndPersist(ctx, envelope, uuid.NewString())
		s.True(biz.IsNotFound(err))
	})

	var prevStoredRef *biz.StoredReferrer
	s.T().Run("it can store properly the first time", func(t *testing.T) {
		err := s.Referrer.ExtractAndPersist(ctx, envelope, s.org1.ID)
		s.NoError(err)
		prevStoredRef, err = s.Referrer.GetFromRoot(ctx, wantReferrerAtt.Digest, s.user.ID)
		s.NoError(err)
	})

	s.T().Run("and it's idempotent", func(t *testing.T) {
		err := s.Referrer.ExtractAndPersist(ctx, envelope, s.org1.ID)
		s.NoError(err)
		ref, err := s.Referrer.GetFromRoot(ctx, wantReferrerAtt.Digest, s.user.ID)
		s.NoError(err)
		// Check it's the same referrer than previously retrieved, including timestamps
		s.Equal(prevStoredRef, ref)
	})

	s.T().Run("contains all the info", func(t *testing.T) {
		got, err := s.Referrer.GetFromRoot(ctx, wantReferrerAtt.Digest, s.user.ID)
		s.NoError(err)
		// parent i.e attestation
		s.Equal(wantReferrerAtt.Digest, got.Digest)
		s.Equal(wantReferrerAtt.Downloadable, got.Downloadable)
		s.Equal(wantReferrerAtt.Kind, got.Kind)
		// it has all the references
		require.Len(t, got.References, 6)

		for i, want := range []*biz.StoredReferrer{
			wantReferrerCommit, wantReferrerSBOM, wantReferrerArtifact, wantReferrerOpenVEX, wantReferrerSarif, wantReferrerContainerImage} {
			gotR := got.References[i]
			s.Equal(want.Digest, gotR.Digest)
			s.Equal(want.Kind, gotR.Kind)
			s.Equal(want.Downloadable, gotR.Downloadable)
		}
		s.Equal([]uuid.UUID{s.org1UUID}, got.OrgIDs)
	})

	s.T().Run("can't be accessed by a second user in another org", func(t *testing.T) {
		// the user2 has not access to org1
		got, err := s.Referrer.GetFromRoot(ctx, wantReferrerAtt.Digest, s.user2.ID)
		s.True(biz.IsNotFound(err))
		s.Nil(got)
	})

	s.T().Run("but another org can be attached", func(t *testing.T) {
		err = s.Referrer.ExtractAndPersist(ctx, envelope, s.org2.ID)
		s.NoError(err)
		got, err := s.Referrer.GetFromRoot(ctx, wantReferrerAtt.Digest, s.user.ID)
		s.NoError(err)
		require.Len(t, got.OrgIDs, 2)
		s.Contains(got.OrgIDs, s.org1UUID)
		s.Contains(got.OrgIDs, s.org2UUID)

		// and it's idempotent (no new orgs added)
		err = s.Referrer.ExtractAndPersist(ctx, envelope, s.org2.ID)
		s.NoError(err)
		got, err = s.Referrer.GetFromRoot(ctx, wantReferrerAtt.Digest, s.user.ID)
		s.NoError(err)
		require.Len(t, got.OrgIDs, 2)
	})

	s.T().Run("and now user2 has access to it since it has access to org2", func(t *testing.T) {
		err = s.Referrer.ExtractAndPersist(ctx, envelope, s.org2.ID)
		s.NoError(err)
		got, err := s.Referrer.GetFromRoot(ctx, wantReferrerAtt.Digest, s.user2.ID)
		s.NoError(err)
		require.Len(t, got.OrgIDs, 2)
	})

	s.T().Run("you can ask for info about materials that are subjects", func(t *testing.T) {
		got, err := s.Referrer.GetFromRoot(ctx, wantReferrerContainerImage.Digest, s.user.ID)
		s.NoError(err)
		// parent i.e attestation
		s.Equal(wantReferrerContainerImage.Digest, got.Digest)
		s.Equal(wantReferrerContainerImage.Downloadable, got.Downloadable)
		s.Equal(wantReferrerContainerImage.Kind, got.Kind)
		// it's connected to the attestation
		require.Len(t, got.References, 1)
		s.Equal(wantReferrerAtt.Digest, got.References[0].Digest)
		s.Equal(wantReferrerAtt.Kind, got.References[0].Kind)
		s.Equal(wantReferrerAtt.Downloadable, got.References[0].Downloadable)
	})

	s.T().Run("it might not have references", func(t *testing.T) {
		got, err := s.Referrer.GetFromRoot(ctx, wantReferrerSarif.Digest, s.user.ID)
		s.NoError(err)
		// parent i.e attestation
		s.Equal(wantReferrerSarif.Digest, got.Digest)
		s.Equal(wantReferrerSarif.Downloadable, got.Downloadable)
		s.Equal(wantReferrerSarif.Kind, got.Kind)
		require.Len(t, got.References, 0)
	})

	s.T().Run("or not to exist", func(t *testing.T) {
		got, err := s.Referrer.GetFromRoot(ctx, "sha256:deadbeef", s.user.ID)
		s.True(biz.IsNotFound(err))
		s.Nil(got)
	})

	s.T().Run("it should fail if the attestation has the same material twice with different types", func(t *testing.T) {
		attJSON, err = os.ReadFile("testdata/attestations/with-duplicated-sha.json")
		require.NoError(s.T(), err)
		require.NoError(s.T(), json.Unmarshal(attJSON, &envelope))

		err := s.Referrer.ExtractAndPersist(ctx, envelope, s.org1.ID)
		s.ErrorContains(err, "has different types")
	})

	s.T().Run("it should fail on retrieval if we have stored two referrers with same digest (for two different types)", func(t *testing.T) {
		// this attestation contains a material with same digest than the container image from git-subject.json
		attJSON, err = os.ReadFile("testdata/attestations/same-digest-than-git-subject.json")
		require.NoError(s.T(), err)
		require.NoError(s.T(), json.Unmarshal(attJSON, &envelope))

		// storing will not fail since it's the a different artifact type
		err := s.Referrer.ExtractAndPersist(ctx, envelope, s.org1.ID)
		s.NoError(err)

		// but retrieval should fail. In the future we will ask the user to provide the artifact type in these cases of ambiguity
		got, err := s.Referrer.GetFromRoot(ctx, wantReferrerSarif.Digest, s.user.ID)
		s.Nil(got)
		s.ErrorContains(err, "found more than one referrer with digest")
	})

	s.T().Run("now there should a container image pointing to two attestations", func(t *testing.T) {
		// but retrieval should fail. In the future we will ask the user to provide the artifact type in these cases of ambiguity
		got, err := s.Referrer.GetFromRoot(ctx, wantReferrerContainerImage.Digest, s.user.ID)
		s.NoError(err)
		// it should be referenced by two attestations since it's subject of both
		require.Len(t, got.References, 2)
		s.Equal("ATTESTATION", got.References[0].Kind)
		s.Equal(wantReferrerAtt.Digest, got.References[0].Digest)
		s.Equal("ATTESTATION", got.References[1].Kind)
		s.Equal("sha256:c90ccaab0b2cfda9980836aef407f62d747680ea9793ddc6ad2e2d7ab615933d", got.References[1].Digest)
	})
}

type referrerIntegrationTestSuite struct {
	testhelpers.UseCasesEachTestSuite
	org1, org2         *biz.Organization
	org1UUID, org2UUID uuid.UUID
	user, user2        *biz.User
}

func (s *referrerIntegrationTestSuite) SetupTest() {
	s.TestingUseCases = testhelpers.NewTestingUseCases(s.T())
	ctx := context.Background()

	var err error
	s.org1, err = s.Organization.Create(ctx, "testing org")
	require.NoError(s.T(), err)
	s.org2, err = s.Organization.Create(ctx, "testing org 2")
	require.NoError(s.T(), err)

	s.org1UUID, err = uuid.Parse(s.org1.ID)
	require.NoError(s.T(), err)
	s.org2UUID, err = uuid.Parse(s.org2.ID)
	require.NoError(s.T(), err)

	// user 1 has access to org 1 and 2
	s.user, err = s.User.FindOrCreateByEmail(ctx, "user-1@test.com")
	require.NoError(s.T(), err)
	_, err = s.Membership.Create(ctx, s.org1.ID, s.user.ID, true)
	require.NoError(s.T(), err)
	_, err = s.Membership.Create(ctx, s.org2.ID, s.user.ID, true)
	require.NoError(s.T(), err)

	// user 2 has access to only org 2
	s.user2, err = s.User.FindOrCreateByEmail(ctx, "user-2@test.com")
	require.NoError(s.T(), err)
	_, err = s.Membership.Create(ctx, s.org2.ID, s.user2.ID, true)
	require.NoError(s.T(), err)
}

func TestReferrerIntegration(t *testing.T) {
	suite.Run(t, new(referrerIntegrationTestSuite))
}
