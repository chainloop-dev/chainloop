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
	"encoding/json"
	"os"
	"testing"

	"github.com/chainloop-dev/chainloop/app/controlplane/internal/biz"
	"github.com/chainloop-dev/chainloop/app/controlplane/internal/biz/testhelpers"
	"github.com/chainloop-dev/chainloop/app/controlplane/internal/conf"
	"github.com/chainloop-dev/chainloop/internal/credentials"
	creds "github.com/chainloop-dev/chainloop/internal/credentials/mocks"
	"github.com/google/uuid"
	"github.com/secure-systems-lab/go-securesystemslib/dsse"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

func (s *referrerIntegrationTestSuite) TestGetFromRootInPublicSharedIndex() {
	// Load attestation
	attJSON, err := os.ReadFile("testdata/attestations/with-git-subject.json")
	require.NoError(s.T(), err)
	var envelope *dsse.Envelope
	require.NoError(s.T(), json.Unmarshal(attJSON, &envelope))

	wantReferrerAtt := &biz.Referrer{
		Digest:       "sha256:de36d470d792499b1489fc0e6623300fc8822b8f0d2981bb5ec563f8dde723c7",
		Kind:         "ATTESTATION",
		Downloadable: true,
	}

	// We'll store the attestation in the private only index
	ctx := context.Background()
	s.T().Run("public endpoint fails if feature not enabled", func(t *testing.T) {
		_, err := s.Referrer.GetFromRootInPublicSharedIndex(ctx, wantReferrerAtt.Digest, "")
		s.ErrorContains(err, "not enabled")
	})

	s.T().Run("storing it associated with a private workflow keeps it private and not in the index", func(t *testing.T) {
		err = s.sharedEnabledUC.ExtractAndPersist(ctx, envelope, s.workflow1.ID.String())
		require.NoError(s.T(), err)
		ref, err := s.Referrer.GetFromRootUser(ctx, wantReferrerAtt.Digest, "", s.user.ID)
		s.NoError(err)
		s.False(ref.InPublicWorkflow)
		res, err := s.sharedEnabledUC.GetFromRootInPublicSharedIndex(ctx, wantReferrerAtt.Digest, "")
		s.True(biz.IsNotFound(err))
		s.Nil(res)
	})

	s.T().Run("storing it associated with a public workflow but not allowed org keeps it out of the index", func(t *testing.T) {
		// Make workflow2 public
		_, err := s.Workflow.Update(ctx, s.org2.ID, s.workflow2.ID.String(), &biz.WorkflowUpdateOpts{Public: toPtrBool(true)})
		require.NoError(t, err)

		err = s.sharedEnabledUC.ExtractAndPersist(ctx, envelope, s.workflow2.ID.String())
		require.NoError(s.T(), err)
		// It's marked as public in the internal index
		ref, err := s.sharedEnabledUC.GetFromRootUser(ctx, wantReferrerAtt.Digest, "", s.user.ID)
		s.NoError(err)
		s.True(ref.InPublicWorkflow)

		// But it's not in the public shared index because the org 2 is not whitelisted
		res, err := s.sharedEnabledUC.GetFromRootInPublicSharedIndex(ctx, wantReferrerAtt.Digest, "")
		s.True(biz.IsNotFound(err))
		s.Nil(res)
	})

	s.T().Run("it should appear if we whitelist org2", func(t *testing.T) {
		uc, err := biz.NewReferrerUseCase(s.Repos.Referrer, s.Repos.Workflow, s.Repos.Membership,
			&conf.ReferrerSharedIndex{
				Enabled:     true,
				AllowedOrgs: []string{s.org2.ID},
			}, nil)
		require.NoError(t, err)
		// Now it's public since org2 is whitelisted
		res, err := uc.GetFromRootInPublicSharedIndex(ctx, wantReferrerAtt.Digest, "")
		s.NoError(err)
		s.Equal(wantReferrerAtt.Digest, res.Digest)
	})

	s.T().Run("or we can make the workflow 1 public", func(t *testing.T) {
		// reset workflow2 to private
		_, err := s.Workflow.Update(ctx, s.org2.ID, s.workflow2.ID.String(), &biz.WorkflowUpdateOpts{Public: toPtrBool(false)})
		require.NoError(t, err)
		// Make workflow1 public
		_, err = s.Workflow.Update(ctx, s.org1.ID, s.workflow1.ID.String(), &biz.WorkflowUpdateOpts{Public: toPtrBool(true)})
		require.NoError(t, err)
		err = s.sharedEnabledUC.ExtractAndPersist(ctx, envelope, s.workflow2.ID.String())
		require.NoError(s.T(), err)
		// Now it's public since org1 is whitelisted
		res, err := s.sharedEnabledUC.GetFromRootInPublicSharedIndex(ctx, wantReferrerAtt.Digest, "")
		s.NoError(err)
		s.Equal(wantReferrerAtt.Digest, res.Digest)
	})
}

func (s *referrerIntegrationTestSuite) TestExtractAndPersistsDependentAttestation() {
	envelope := testEnvelope(s.T(), "testdata/attestations/with-dependent-attestation.json")
	ctx := context.Background()

	const (
		wantReferrerAtt  = "sha256:950c7b4c65447a3b86b6f769515005e7c44a67c8193bff790750eadf13207fbb"
		wantDependentAtt = "sha256:2dc17f7c933d20e06b49250a582a3d19bdfbadba9c4e5f3f856af6f261db79d4"
	)

	s.Run("creation fails because the dependent attestation doesn't exist yet", func() {
		err := s.Referrer.ExtractAndPersist(ctx, envelope, s.workflow1.ID.String())
		s.ErrorContains(err, "attestation material does not exist")
	})

	s.Run("if the dependent attestation exists we ingest it", func() {
		// We store the dependent attestation
		dependentAtt := testEnvelope(s.T(), "testdata/attestations/dependent-attestation.json")
		err := s.Referrer.ExtractAndPersist(ctx, dependentAtt, s.workflow1.ID.String())
		require.NoError(s.T(), err)

		err = s.Referrer.ExtractAndPersist(ctx, envelope, s.workflow1.ID.String())
		s.NoError(err)
		got, err := s.Referrer.GetFromRootUser(ctx, wantReferrerAtt, "ATTESTATION", s.user.ID)
		s.NoError(err)
		// It has a commit and an attestation
		require.Len(s.T(), got.References, 2)
		s.Equal(wantDependentAtt, got.References[0].Digest)
	})
}

func (s *referrerIntegrationTestSuite) TestExtractAndPersists() {
	// Load attestation
	envelope := testEnvelope(s.T(), "testdata/attestations/with-git-subject.json")

	wantReferrerAtt := &biz.Referrer{
		Digest:       "sha256:de36d470d792499b1489fc0e6623300fc8822b8f0d2981bb5ec563f8dde723c7",
		Kind:         "ATTESTATION",
		Downloadable: true,
	}

	wantReferrerCommit := &biz.Referrer{
		Digest: "sha1:78ac366c9e8a300d51808d581422ca61f7b5b721",
		Kind:   "GIT_HEAD_COMMIT",
	}

	wantReferrerSBOM := &biz.Referrer{
		Digest:       "sha256:16159bb881eb4ab7eb5d8afc5350b0feeed1e31c0a268e355e74f9ccbe885e0c",
		Kind:         "SBOM_CYCLONEDX_JSON",
		Downloadable: true,
	}

	wantReferrerArtifact := &biz.Referrer{
		Digest:       "sha256:385c4188b9c080499413f2e0fa0b3951ed107b5f0cb35c2f2b1f07a7be9a7512",
		Kind:         "ARTIFACT",
		Downloadable: true,
	}

	wantReferrerOpenVEX := &biz.Referrer{
		Digest:       "sha256:b4bd86d5855f94bcac0a92d3100ae7b85d050bd2e5fb9037a200e5f5f0b073a2",
		Kind:         "OPENVEX",
		Downloadable: true,
	}

	wantReferrerSarif := &biz.Referrer{
		Digest:       "sha256:c4a63494f9289dd9fd44f841efb4f5b52765c2de6332f2d86e5f6c0340b40a95",
		Kind:         "SARIF",
		Downloadable: true,
	}

	wantReferrerContainerImage := &biz.Referrer{
		Digest: "sha256:fbd9335f55d83d8aaf9ab1a539b0f2a87b444e8c54f34c9a1ca9d7df15605db4",
		Kind:   "CONTAINER_IMAGE",
	}

	ctx := context.Background()
	s.T().Run("creation fails if the workflow doesn't exist", func(t *testing.T) {
		err := s.Referrer.ExtractAndPersist(ctx, envelope, uuid.NewString())
		s.True(biz.IsNotFound(err))
	})

	var prevStoredRef *biz.StoredReferrer
	s.T().Run("it can store properly the first time", func(t *testing.T) {
		err := s.Referrer.ExtractAndPersist(ctx, envelope, s.workflow1.ID.String())
		s.NoError(err)
		prevStoredRef, err = s.Referrer.GetFromRootUser(ctx, wantReferrerAtt.Digest, "", s.user.ID)
		s.NoError(err)
	})

	s.T().Run("and it's idempotent", func(t *testing.T) {
		err := s.Referrer.ExtractAndPersist(ctx, envelope, s.workflow1.ID.String())
		s.NoError(err)
		ref, err := s.Referrer.GetFromRootUser(ctx, wantReferrerAtt.Digest, "", s.user.ID)
		s.NoError(err)
		// Check it's the same referrer than previously retrieved, including timestamps
		s.Equal(prevStoredRef, ref)
	})

	s.T().Run("contains all the info", func(t *testing.T) {
		got, err := s.Referrer.GetFromRootUser(ctx, wantReferrerAtt.Digest, "", s.user.ID)
		s.NoError(err)
		// parent i.e attestation
		s.Equal(wantReferrerAtt.Digest, got.Digest)
		s.Equal(wantReferrerAtt.Downloadable, got.Downloadable)
		s.Equal(wantReferrerAtt.Kind, got.Kind)
		// It has metadata
		s.Equal(map[string]string{
			"name":         "test-new-types",
			"project":      "test",
			"team":         "my-team",
			"organization": "my-org",
		}, got.Metadata)
		// it has all the references
		require.Len(t, got.References, 6)

		for i, want := range []*biz.Referrer{
			wantReferrerArtifact, wantReferrerContainerImage, wantReferrerCommit, wantReferrerOpenVEX, wantReferrerSarif, wantReferrerSBOM} {
			gotR := got.References[i]
			s.Equal(want, gotR.Referrer)
		}
		s.Equal([]uuid.UUID{s.org1UUID}, got.OrgIDs)
		s.Equal([]uuid.UUID{s.workflow1.ID}, got.WorkflowIDs)
	})

	s.T().Run("can get sha1 digests too", func(t *testing.T) {
		got, err := s.Referrer.GetFromRootUser(ctx, wantReferrerCommit.Digest, "", s.user.ID)
		s.NoError(err)
		s.Equal(wantReferrerCommit.Digest, got.Digest)
	})

	s.T().Run("can't be accessed by a second user in another org", func(t *testing.T) {
		// the user2 has not access to org1
		got, err := s.Referrer.GetFromRootUser(ctx, wantReferrerAtt.Digest, "", s.user2.ID)
		s.True(biz.IsNotFound(err))
		s.Nil(got)
	})

	s.T().Run("but another workflow can be attached", func(t *testing.T) {
		err := s.Referrer.ExtractAndPersist(ctx, envelope, s.workflow2.ID.String())
		s.NoError(err)
		got, err := s.Referrer.GetFromRootUser(ctx, wantReferrerAtt.Digest, "", s.user.ID)
		s.NoError(err)
		require.Len(t, got.OrgIDs, 2)
		s.Contains(got.OrgIDs, s.org1UUID)
		s.Contains(got.OrgIDs, s.org2UUID)

		// and it's idempotent (no new orgs added)
		err = s.Referrer.ExtractAndPersist(ctx, envelope, s.workflow2.ID.String())
		s.NoError(err)
		got, err = s.Referrer.GetFromRootUser(ctx, wantReferrerAtt.Digest, "", s.user.ID)
		s.NoError(err)
		require.Len(t, got.OrgIDs, 2)
		s.Equal([]uuid.UUID{s.org2UUID, s.org1UUID}, got.OrgIDs)
		s.Equal([]uuid.UUID{s.workflow2.ID, s.workflow1.ID}, got.WorkflowIDs)
	})

	s.T().Run("and now user2 has access to it since it has access to workflow2 in org2", func(t *testing.T) {
		err := s.Referrer.ExtractAndPersist(ctx, envelope, s.workflow2.ID.String())
		s.NoError(err)
		got, err := s.Referrer.GetFromRootUser(ctx, wantReferrerAtt.Digest, "", s.user2.ID)
		s.NoError(err)
		require.Len(t, got.OrgIDs, 2)
	})

	s.T().Run("subject materials are returned connected to the attestation", func(t *testing.T) {
		got, err := s.Referrer.GetFromRootUser(ctx, wantReferrerContainerImage.Digest, "", s.user.ID)
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

	s.T().Run("non-subject materials also are connected to the attestation", func(t *testing.T) {
		got, err := s.Referrer.GetFromRootUser(ctx, wantReferrerSarif.Digest, "", s.user.ID)
		s.NoError(err)
		require.Len(t, got.References, 1)
		s.Equal(wantReferrerAtt.Digest, got.References[0].Digest)
		s.Equal(wantReferrerAtt.Kind, got.References[0].Kind)
		s.Equal(wantReferrerAtt.Downloadable, got.References[0].Downloadable)
	})

	s.T().Run("or it does not exist", func(t *testing.T) {
		got, err := s.Referrer.GetFromRootUser(ctx, "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", "", s.user.ID)
		s.True(biz.IsNotFound(err))
		s.Nil(got)
	})

	s.T().Run("it should NOT fail storing the attestation with the same material twice with different types", func(t *testing.T) {
		envelope := testEnvelope(s.T(), "testdata/attestations/with-duplicated-sha.json")

		err := s.Referrer.ExtractAndPersist(ctx, envelope, s.workflow1.ID.String())
		s.NoError(err)
	})

	s.T().Run("it should fail on retrieval if we have stored two referrers with same digest (for two different types)", func(t *testing.T) {
		envelope := testEnvelope(s.T(), "testdata/attestations/same-digest-than-git-subject.json")

		// storing will not fail since it's the a different artifact type
		err := s.Referrer.ExtractAndPersist(ctx, envelope, s.workflow1.ID.String())
		s.NoError(err)

		// but retrieval should fail. In the future we will ask the user to provide the artifact type in these cases of ambiguity
		got, err := s.Referrer.GetFromRootUser(ctx, wantReferrerSarif.Digest, "", s.user.ID)
		s.Nil(got)
		s.ErrorContains(err, "present in 2 kinds")
	})

	s.T().Run("it should not fail on retrieval if we filter out by one kind", func(t *testing.T) {
		// but retrieval should fail. In the future we will ask the user to provide the artifact type in these cases of ambiguity
		got, err := s.Referrer.GetFromRootUser(ctx, wantReferrerSarif.Digest, "SARIF", s.user.ID)
		s.NoError(err)
		s.Equal(wantReferrerSarif.Digest, got.Digest)
		s.Equal(true, got.Downloadable)
		s.Equal("SARIF", got.Kind)

		got, err = s.Referrer.GetFromRootUser(ctx, wantReferrerSarif.Digest, "ARTIFACT", s.user.ID)
		s.NoError(err)
		s.Equal(wantReferrerSarif.Digest, got.Digest)
		s.Equal(true, got.Downloadable)
		s.Equal("ARTIFACT", got.Kind)
	})

	s.T().Run("now there should a container image pointing to two attestations", func(t *testing.T) {
		// but retrieval should fail. In the future we will ask the user to provide the artifact type in these cases of ambiguity
		got, err := s.Referrer.GetFromRootUser(ctx, wantReferrerContainerImage.Digest, "", s.user.ID)
		s.NoError(err)
		// it should be referenced by two attestations since it's subject of both
		require.Len(t, got.References, 2)
		s.Equal("ATTESTATION", got.References[0].Kind)
		s.Equal("sha256:de36d470d792499b1489fc0e6623300fc8822b8f0d2981bb5ec563f8dde723c7", got.References[0].Digest)
		s.Equal("ATTESTATION", got.References[1].Kind)
		s.Equal("sha256:c90ccaab0b2cfda9980836aef407f62d747680ea9793ddc6ad2e2d7ab615933d", got.References[1].Digest)
	})

	s.T().Run("if all associated workflows are private, the referrer is private", func(t *testing.T) {
		got, err := s.Referrer.GetFromRootUser(ctx, wantReferrerAtt.Digest, "", s.user.ID)
		s.NoError(err)
		s.False(got.InPublicWorkflow)
		s.Equal([]uuid.UUID{s.workflow2.ID, s.workflow1.ID}, got.WorkflowIDs)
		for _, r := range got.References {
			s.False(r.InPublicWorkflow)
		}
	})

	s.T().Run("the referrer will be public if one associated workflow is public", func(t *testing.T) {
		// Make workflow1 public
		_, err := s.Workflow.Update(ctx, s.org1.ID, s.workflow1.ID.String(), &biz.WorkflowUpdateOpts{Public: toPtrBool(true)})
		require.NoError(t, err)

		got, err := s.Referrer.GetFromRootUser(ctx, wantReferrerAtt.Digest, "", s.user.ID)
		s.NoError(err)
		s.True(got.InPublicWorkflow)
		for _, r := range got.References {
			s.True(r.InPublicWorkflow)
		}
	})
}

type referrerIntegrationTestSuite struct {
	testhelpers.UseCasesEachTestSuite
	org1, org2           *biz.Organization
	workflow1, workflow2 *biz.Workflow
	org1UUID, org2UUID   uuid.UUID
	user, user2          *biz.User
	sharedEnabledUC      *biz.ReferrerUseCase
	run                  *biz.WorkflowRun
}

func (s *referrerIntegrationTestSuite) SetupTest() {
	credsWriter := creds.NewReaderWriter(s.T())
	ctx := context.Background()
	credsWriter.On("SaveCredentials", ctx, mock.Anything, &credentials.OCIKeypair{Repo: "repo", Username: "username", Password: "pass"}).Return("stored-OCI-secret", nil)

	s.TestingUseCases = testhelpers.NewTestingUseCases(s.T(), testhelpers.WithCredsReaderWriter(credsWriter))

	var err error
	s.org1, err = s.Organization.CreateWithRandomName(ctx)
	require.NoError(s.T(), err)
	s.org2, err = s.Organization.CreateWithRandomName(ctx)
	require.NoError(s.T(), err)

	s.org1UUID, err = uuid.Parse(s.org1.ID)
	require.NoError(s.T(), err)
	s.org2UUID, err = uuid.Parse(s.org2.ID)
	require.NoError(s.T(), err)

	s.workflow1, err = s.Workflow.Create(ctx, &biz.WorkflowCreateOpts{Name: "wf", Team: "team", OrgID: s.org1.ID})
	require.NoError(s.T(), err)
	s.workflow2, err = s.Workflow.Create(ctx, &biz.WorkflowCreateOpts{Name: "wf-from-org-2", Team: "team", OrgID: s.org2.ID})
	require.NoError(s.T(), err)

	// user 1 has access to org 1 and 2
	s.user, err = s.User.FindOrCreateByEmail(ctx, "user-1@test.com")
	require.NoError(s.T(), err)
	_, err = s.Membership.Create(ctx, s.org1.ID, s.user.ID)
	require.NoError(s.T(), err)
	_, err = s.Membership.Create(ctx, s.org2.ID, s.user.ID, biz.WithCurrentMembership())
	require.NoError(s.T(), err)

	// user 2 has access to only org 2
	s.user2, err = s.User.FindOrCreateByEmail(ctx, "user-2@test.com")
	require.NoError(s.T(), err)
	_, err = s.Membership.Create(ctx, s.org2.ID, s.user2.ID, biz.WithCurrentMembership())
	require.NoError(s.T(), err)

	s.sharedEnabledUC, err = biz.NewReferrerUseCase(s.Repos.Referrer, s.Repos.Workflow, s.Repos.Membership,
		&conf.ReferrerSharedIndex{
			Enabled:     true,
			AllowedOrgs: []string{s.org1.ID},
		}, nil)
	require.NoError(s.T(), err)

	// Find contract revision
	contractVersion, err := s.WorkflowContract.Describe(ctx, s.org1.ID, s.workflow1.ContractID.String(), 0)
	require.NoError(s.T(), err)

	casBackend, err := s.CASBackend.CreateOrUpdate(ctx, s.org1.ID, "repo", "username", "pass", backendType, true)
	require.NoError(s.T(), err)

	s.run, err = s.WorkflowRun.Create(ctx,
		&biz.WorkflowRunCreateOpts{
			WorkflowID: s.workflow1.ID.String(), ContractRevision: contractVersion, CASBackendID: casBackend.ID,
		})

	require.NoError(s.T(), err)
}

func TestReferrerIntegration(t *testing.T) {
	suite.Run(t, new(referrerIntegrationTestSuite))
}
