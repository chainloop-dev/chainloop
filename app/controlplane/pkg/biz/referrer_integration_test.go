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
	"bytes"
	"context"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/biz"
	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/biz/testhelpers"
	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/pagination"
	"github.com/chainloop-dev/chainloop/pkg/credentials"
	creds "github.com/chainloop-dev/chainloop/pkg/credentials/mocks"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

func (s *referrerIntegrationTestSuite) TestExtractAndPersistsDependentAttestation() {
	envelope, attJSON := testEnvelope(s.T(), "testdata/attestations/with-dependent-attestation.json")
	h, _, err := v1.SHA256(bytes.NewReader(attJSON))
	require.NoError(s.T(), err)
	ctx := context.Background()

	var (
		wantReferrerAtt, _  = v1.NewHash("sha256:950c7b4c65447a3b86b6f769515005e7c44a67c8193bff790750eadf13207fbb")
		wantDependentAtt, _ = v1.NewHash("sha256:2dc17f7c933d20e06b49250a582a3d19bdfbadba9c4e5f3f856af6f261db79d4")
	)

	s.Run("creation fails because the dependent attestation doesn't exist yet", func() {
		err := s.Referrer.ExtractAndPersist(ctx, envelope, h, s.workflow1.ID.String())
		s.ErrorContains(err, "attestation material does not exist")
	})

	s.Run("if the dependent attestation exists we ingest it", func() {
		// We store the dependent attestation
		dependentAtt, _ := testEnvelope(s.T(), "testdata/attestations/dependent-attestation.json")
		err := s.Referrer.ExtractAndPersist(ctx, dependentAtt, wantDependentAtt, s.workflow1.ID.String())
		require.NoError(s.T(), err)

		err = s.Referrer.ExtractAndPersist(ctx, envelope, wantReferrerAtt, s.workflow1.ID.String())
		s.NoError(err)
		got, _, err := s.Referrer.GetFromRootUser(ctx, wantReferrerAtt.String(), "ATTESTATION", s.user.ID, nil)
		s.NoError(err)
		// It has a commit and an attestation
		require.Len(s.T(), got.References, 2)
		digests := []string{got.References[0].Digest, got.References[1].Digest}
		s.Contains(digests, wantDependentAtt.String())
	})
}

func (s *referrerIntegrationTestSuite) TestExtractAndPersistsConcurrency() {
	envelope, attJSON := testEnvelope(s.T(), "testdata/attestations/with-git-subject.json")
	ctx := context.Background()
	h, _, err := v1.SHA256(bytes.NewReader(attJSON))
	require.NoError(s.T(), err)

	s.T().Run("and works with concurrency of the same thing", func(_ *testing.T) {
		var wg sync.WaitGroup
		for i := 0; i < 5; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				err := s.Referrer.ExtractAndPersist(ctx, envelope, h, s.workflow1.ID.String())
				s.NoError(err)
				err = s.Referrer.ExtractAndPersist(ctx, envelope, h, s.workflow2.ID.String())
				s.NoError(err)
			}()
		}
		wg.Wait()

		got, _, err := s.Referrer.GetFromRootUser(ctx, h.String(), "", s.user.ID, nil)
		s.NoError(err)
		s.Len(got.WorkflowIDs, 2)
		s.Equal([]uuid.UUID{s.workflow2.ID, s.workflow1.ID}, got.WorkflowIDs)
	})
}

func (s *referrerIntegrationTestSuite) TestExtractAndPersists() {
	// Load attestation
	envelope, envBytes := testEnvelope(s.T(), "testdata/attestations/with-git-subject.json")
	h, _, err := v1.SHA256(bytes.NewReader(envBytes))
	require.NoError(s.T(), err)

	wantReferrerAtt := &biz.Referrer{
		Digest:       h.String(),
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
		err := s.Referrer.ExtractAndPersist(ctx, envelope, h, uuid.NewString())
		s.True(biz.IsNotFound(err))
	})

	var prevStoredRef *biz.StoredReferrer
	s.T().Run("it can store properly the first time", func(t *testing.T) {
		err := s.Referrer.ExtractAndPersist(ctx, envelope, h, s.workflow1.ID.String())
		s.NoError(err)
		prevStoredRef, _, err = s.Referrer.GetFromRootUser(ctx, h.String(), "", s.user.ID, nil)
		s.NoError(err)
	})

	s.T().Run("and it's idempotent", func(t *testing.T) {
		err := s.Referrer.ExtractAndPersist(ctx, envelope, h, s.workflow1.ID.String())
		s.NoError(err)
		ref, _, err := s.Referrer.GetFromRootUser(ctx, h.String(), "", s.user.ID, nil)
		s.NoError(err)
		// Check it's the same referrer than previously retrieved, including timestamps
		s.Equal(prevStoredRef, ref)
	})

	s.T().Run("contains all the info", func(t *testing.T) {
		got, _, err := s.Referrer.GetFromRootUser(ctx, wantReferrerAtt.Digest, "", s.user.ID, nil)
		s.NoError(err)
		// parent i.e attestation
		s.Equal(wantReferrerAtt.Digest, got.Digest)
		s.Equal(wantReferrerAtt.Downloadable, got.Downloadable)
		s.Equal(wantReferrerAtt.Kind, got.Kind)
		// It has metadata
		s.Equal(map[string]string{
			"contractName":             "",
			"contractVersion":          "",
			"name":                     "test-new-types",
			"project":                  "test",
			"team":                     "my-team",
			"organization":             "my-org",
			"hasGatedPolicyViolations": "false",
			"hasPolicyViolations":      "false",
			"projectVersion":           "",
			"projectVersionPrerelease": "false",
		}, got.Metadata)
		// it has all the references
		require.Len(t, got.References, 6)

		wantRefs := []*biz.Referrer{
			wantReferrerArtifact, wantReferrerContainerImage, wantReferrerCommit, wantReferrerOpenVEX, wantReferrerSarif, wantReferrerSBOM,
		}
		for _, want := range wantRefs {
			found := false
			for _, gotR := range got.References {
				if gotR.Digest == want.Digest {
					s.Equal(want, gotR.Referrer)
					found = true
					break
				}
			}
			s.True(found, "expected referrer with digest %s not found", want.Digest)
		}
		s.Equal([]uuid.UUID{s.org1UUID}, got.OrgIDs)
		s.Equal([]uuid.UUID{s.workflow1.ID}, got.WorkflowIDs)
	})

	s.T().Run("can get sha1 digests too", func(t *testing.T) {
		got, _, err := s.Referrer.GetFromRootUser(ctx, wantReferrerCommit.Digest, "", s.user.ID, nil)
		s.NoError(err)
		s.Equal(wantReferrerCommit.Digest, got.Digest)
	})

	s.T().Run("can't be accessed by a second user in another org", func(t *testing.T) {
		// the user2 has not access to org1
		got, _, err := s.Referrer.GetFromRootUser(ctx, wantReferrerAtt.Digest, "", s.user2.ID, nil)
		s.True(biz.IsNotFound(err))
		s.Nil(got)
	})

	s.T().Run("but another workflow can be attached", func(t *testing.T) {
		err := s.Referrer.ExtractAndPersist(ctx, envelope, h, s.workflow2.ID.String())
		s.NoError(err)
		got, _, err := s.Referrer.GetFromRootUser(ctx, h.String(), "", s.user.ID, nil)
		s.NoError(err)
		require.Len(t, got.OrgIDs, 2)
		s.Contains(got.OrgIDs, s.org1UUID)
		s.Contains(got.OrgIDs, s.org2UUID)

		// and it's idempotent (no new orgs added)
		err = s.Referrer.ExtractAndPersist(ctx, envelope, h, s.workflow2.ID.String())
		s.NoError(err)
		got, _, err = s.Referrer.GetFromRootUser(ctx, h.String(), "", s.user.ID, nil)
		s.NoError(err)
		require.Len(t, got.OrgIDs, 2)
		s.Equal([]uuid.UUID{s.org2UUID, s.org1UUID}, got.OrgIDs)
		s.Equal([]uuid.UUID{s.workflow2.ID, s.workflow1.ID}, got.WorkflowIDs)
	})

	s.T().Run("and now user2 has access to it since it has access to workflow2 in org2", func(t *testing.T) {
		err := s.Referrer.ExtractAndPersist(ctx, envelope, h, s.workflow2.ID.String())
		s.NoError(err)
		got, _, err := s.Referrer.GetFromRootUser(ctx, h.String(), "", s.user2.ID, nil)
		s.NoError(err)
		require.Len(t, got.OrgIDs, 2)
	})

	s.T().Run("subject materials are returned connected to the attestation", func(t *testing.T) {
		got, _, err := s.Referrer.GetFromRootUser(ctx, wantReferrerContainerImage.Digest, "", s.user.ID, nil)
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
		got, _, err := s.Referrer.GetFromRootUser(ctx, wantReferrerSarif.Digest, "", s.user.ID, nil)
		s.NoError(err)
		require.Len(t, got.References, 1)
		s.Equal(wantReferrerAtt.Digest, got.References[0].Digest)
		s.Equal(wantReferrerAtt.Kind, got.References[0].Kind)
		s.Equal(wantReferrerAtt.Downloadable, got.References[0].Downloadable)
	})

	s.T().Run("or it does not exist", func(t *testing.T) {
		got, _, err := s.Referrer.GetFromRootUser(ctx, "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", "", s.user.ID, nil)
		s.True(biz.IsNotFound(err))
		s.Nil(got)
	})

	s.T().Run("it should NOT fail storing the attestation with the same material twice with different types", func(t *testing.T) {
		envelope, _ := testEnvelope(s.T(), "testdata/attestations/with-duplicated-sha.json")

		err := s.Referrer.ExtractAndPersist(ctx, envelope, h, s.workflow1.ID.String())
		s.NoError(err)
	})

	s.T().Run("it should fail on retrieval if we have stored two referrers with same digest (for two different types)", func(t *testing.T) {
		envelope, attJSON := testEnvelope(s.T(), "testdata/attestations/same-digest-than-git-subject.json")
		h, _, err := v1.SHA256(bytes.NewReader(attJSON))
		require.NoError(s.T(), err)

		// storing will not fail since it's the a different artifact type
		err = s.Referrer.ExtractAndPersist(ctx, envelope, h, s.workflow1.ID.String())
		s.NoError(err)

		// but retrieval should fail. In the future we will ask the user to provide the artifact type in these cases of ambiguity
		got, _, err := s.Referrer.GetFromRootUser(ctx, wantReferrerSarif.Digest, "", s.user.ID, nil)
		s.Nil(got)
		s.ErrorContains(err, "present in 2 kinds")
	})

	s.T().Run("it should not fail on retrieval if we filter out by one kind", func(t *testing.T) {
		// but retrieval should fail. In the future we will ask the user to provide the artifact type in these cases of ambiguity
		got, _, err := s.Referrer.GetFromRootUser(ctx, wantReferrerSarif.Digest, "SARIF", s.user.ID, nil)
		s.NoError(err)
		s.Equal(wantReferrerSarif.Digest, got.Digest)
		s.Equal(true, got.Downloadable)
		s.Equal("SARIF", got.Kind)

		got, _, err = s.Referrer.GetFromRootUser(ctx, wantReferrerSarif.Digest, "ARTIFACT", s.user.ID, nil)
		s.NoError(err)
		s.Equal(wantReferrerSarif.Digest, got.Digest)
		s.Equal(true, got.Downloadable)
		s.Equal("ARTIFACT", got.Kind)
	})

	s.T().Run("now there should a container image pointing to two attestations", func(t *testing.T) {
		// but retrieval should fail. In the future we will ask the user to provide the artifact type in these cases of ambiguity
		got, _, err := s.Referrer.GetFromRootUser(ctx, wantReferrerContainerImage.Digest, "", s.user.ID, nil)
		s.NoError(err)
		// it should be referenced by two attestations since it's subject of both
		require.Len(t, got.References, 2)
		gotDigests := []string{got.References[0].Digest, got.References[1].Digest}
		for _, ref := range got.References {
			s.Equal("ATTESTATION", ref.Kind)
		}
		s.Contains(gotDigests, "sha256:2e9bf8e13acd112eff355787b2b72eb8af4ee51fc22c7e65611939f2225e1dc5")
		s.Contains(gotDigests, "sha256:5f4d1baadaf3e439f769f11c7ba0c5f77dad27d00689144d1311b48e65818bbd")
	})

	s.T().Run("returns the associated workflow IDs", func(_ *testing.T) {
		got, _, err := s.Referrer.GetFromRootUser(ctx, wantReferrerAtt.Digest, "", s.user.ID, nil)
		s.NoError(err)
		s.Equal([]uuid.UUID{s.workflow2.ID, s.workflow1.ID}, got.WorkflowIDs)
	})
}

func (s *referrerIntegrationTestSuite) TestPagination() {
	// Load attestation — it has 6 references (ARTIFACT, CONTAINER_IMAGE, GIT_HEAD_COMMIT, OPENVEX, SARIF, SBOM_CYCLONEDX_JSON)
	envelope, envBytes := testEnvelope(s.T(), "testdata/attestations/with-git-subject.json")
	h, _, err := v1.SHA256(bytes.NewReader(envBytes))
	require.NoError(s.T(), err)
	ctx := context.Background()

	err = s.Referrer.ExtractAndPersist(ctx, envelope, h, s.workflow1.ID.String())
	require.NoError(s.T(), err)

	tests := []struct {
		name           string
		limit          int
		cursor         string
		wantCount      int
		wantNextCursor bool
		description    string
	}{
		{
			name:           "nil pagination returns all references (no limit)",
			wantCount:      6,
			wantNextCursor: false,
			description:    "backward compat: nil pagination returns everything",
		},
		{
			name:           "limit larger than total returns all",
			limit:          100,
			wantCount:      6,
			wantNextCursor: false,
		},
		{
			name:           "limit equal to total returns all with no next cursor",
			limit:          6,
			wantCount:      6,
			wantNextCursor: false,
		},
		{
			name:           "limit less than total returns partial with next cursor",
			limit:          3,
			wantCount:      3,
			wantNextCursor: true,
		},
		{
			name:           "limit of 1 returns single reference with next cursor",
			limit:          1,
			wantCount:      1,
			wantNextCursor: true,
		},
	}

	for _, tc := range tests {
		s.T().Run(tc.name, func(t *testing.T) {
			var p *pagination.CursorOptions
			if tc.limit > 0 {
				p, err = pagination.NewCursor(tc.cursor, tc.limit)
				require.NoError(t, err)
			}

			got, nextCursor, err := s.Referrer.GetFromRootUser(ctx, h.String(), "", s.user.ID, p)
			require.NoError(t, err)
			require.Len(t, got.References, tc.wantCount)

			if tc.wantNextCursor {
				s.NotEmpty(nextCursor, "expected a next cursor")
			} else {
				s.Empty(nextCursor, "expected no next cursor")
			}
		})
	}

	// Test cursor traversal: page through all references one at a time
	s.T().Run("cursor traversal visits all references", func(t *testing.T) {
		var allDigests []string
		cursor := ""

		for i := 0; i < 10; i++ { // safety limit
			p, err := pagination.NewCursor(cursor, 1)
			require.NoError(t, err)

			got, nextCursor, err := s.Referrer.GetFromRootUser(ctx, h.String(), "", s.user.ID, p)
			require.NoError(t, err)
			require.Len(t, got.References, 1)

			allDigests = append(allDigests, got.References[0].Digest)

			if nextCursor == "" {
				break
			}
			cursor = nextCursor
		}

		// Should have traversed all 6 references
		s.Len(allDigests, 6)
		// All digests should be unique
		seen := make(map[string]bool)
		for _, d := range allDigests {
			s.False(seen[d], "duplicate digest found: %s", d)
			seen[d] = true
		}
	})
}

func (s *referrerIntegrationTestSuite) TestGetFromRootProjectVersionFilter() {
	// Load attestation and persist its referrers under workflow1 (project "test")
	envelope, envBytes := testEnvelope(s.T(), "testdata/attestations/with-git-subject.json")
	h, _, err := v1.SHA256(bytes.NewReader(envBytes))
	require.NoError(s.T(), err)
	ctx := context.Background()

	err = s.Referrer.ExtractAndPersist(ctx, envelope, h, s.workflow1.ID.String())
	require.NoError(s.T(), err)

	// The SBOM is one of the materials referenced by the attestation
	const sbomDigest = "sha256:16159bb881eb4ab7eb5d8afc5350b0feeed1e31c0a268e355e74f9ccbe885e0c"

	// Create a workflow run for project version "v1.0.0" on workflow1 and link the attestation digest to it
	contractVersion, err := s.WorkflowContract.Describe(ctx, s.org1.ID, s.workflow1.ContractID.String(), 0)
	require.NoError(s.T(), err)
	casBackend, err := s.CASBackend.CreateOrUpdate(ctx, s.org1.ID, "repo", "username", "pass", backendType, true)
	require.NoError(s.T(), err)
	run, err := s.WorkflowRun.Create(ctx, &biz.WorkflowRunCreateOpts{
		WorkflowID: s.workflow1.ID.String(), ContractRevision: contractVersion, CASBackendID: casBackend.ID,
		ProjectVersion: "v1.0.0",
	})
	require.NoError(s.T(), err)
	require.NoError(s.T(), s.Repos.WorkflowRunRepo.SaveAttestationDigest(ctx, run.ID, h.String(), false))

	s.Run("attestation root is returned when project+version match", func() {
		got, _, err := s.Referrer.GetFromRootUser(ctx, h.String(), "", s.user.ID, nil, biz.WithProjectScope("test", "v1.0.0"))
		s.NoError(err)
		s.Require().NotNil(got)
		s.Equal(h.String(), got.Digest)
		// children (materials) are still returned
		s.NotEmpty(got.References)
	})

	s.Run("attestation root is not found for a different version", func() {
		got, _, err := s.Referrer.GetFromRootUser(ctx, h.String(), "", s.user.ID, nil, biz.WithProjectScope("test", "v9.9.9"))
		s.True(biz.IsNotFound(err))
		s.Nil(got)
	})

	s.Run("attestation root is not found for a different project", func() {
		got, _, err := s.Referrer.GetFromRootUser(ctx, h.String(), "", s.user.ID, nil, biz.WithProjectScope("does-not-exist", "v1.0.0"))
		s.True(biz.IsNotFound(err))
		s.Nil(got)
	})

	s.Run("material root is returned when reachable from an in-version attestation", func() {
		got, _, err := s.Referrer.GetFromRootUser(ctx, sbomDigest, "", s.user.ID, nil, biz.WithProjectScope("test", "v1.0.0"))
		s.NoError(err)
		s.Require().NotNil(got)
		s.Equal(sbomDigest, got.Digest)
		// its parent attestation (in version) is returned as a reference
		require.Len(s.T(), got.References, 1)
		s.Equal(h.String(), got.References[0].Digest)
	})

	s.Run("material root is not found for a different version", func() {
		got, _, err := s.Referrer.GetFromRootUser(ctx, sbomDigest, "", s.user.ID, nil, biz.WithProjectScope("test", "v9.9.9"))
		s.True(biz.IsNotFound(err))
		s.Nil(got)
	})

	s.Run("without the filter the referrer is returned regardless of version", func() {
		got, _, err := s.Referrer.GetFromRootUser(ctx, h.String(), "", s.user.ID, nil)
		s.NoError(err)
		s.Require().NotNil(got)
		s.Equal(h.String(), got.Digest)
	})

	s.Run("project_name alone returns the referrer across all versions of that project", func() {
		// A second run on workflow1 at v3.0.0 with a fresh attestation digest. With a project-only
		// filter, the SBOM must be reachable through either v1.0.0 (via h) or v3.0.0 (via newH).
		const newH = "sha256:" + "b" + "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
		runV3, err := s.WorkflowRun.Create(ctx, &biz.WorkflowRunCreateOpts{
			WorkflowID: s.workflow1.ID.String(), ContractRevision: contractVersion, CASBackendID: casBackend.ID,
			ProjectVersion: "v3.0.0",
		})
		require.NoError(s.T(), err)
		require.NoError(s.T(), s.Repos.WorkflowRunRepo.SaveAttestationDigest(ctx, runV3.ID, newH, false))

		// project_name only (empty version) → SBOM returned via its in-project attestation.
		got, _, err := s.Referrer.GetFromRootUser(ctx, sbomDigest, "", s.user.ID, nil, biz.WithProjectScope("test", ""))
		s.NoError(err)
		s.Require().NotNil(got)
		s.Equal(sbomDigest, got.Digest)

		// Attestation root: returned when its digest belongs to any version of the project.
		got, _, err = s.Referrer.GetFromRootUser(ctx, h.String(), "", s.user.ID, nil, biz.WithProjectScope("test", ""))
		s.NoError(err)
		s.Require().NotNil(got)
		s.Equal(h.String(), got.Digest)

		// Unknown project still NotFound.
		got, _, err = s.Referrer.GetFromRootUser(ctx, sbomDigest, "", s.user.ID, nil, biz.WithProjectScope("does-not-exist", ""))
		s.True(biz.IsNotFound(err))
		s.Nil(got)
	})

	s.Run("RBAC: project filter must respect the caller's visible projects", func() {
		// A second workflow in org1 under a different project. Persisting the same envelope on
		// it links the existing materials (including the SBOM) to that project too — so the
		// material is technically visible to the user via the original "test" project even when
		// the second project is not in their visible set.
		wfOther, err := s.Workflow.Create(ctx, &biz.WorkflowCreateOpts{
			Name: "wf-other", Team: "team", OrgID: s.org1.ID, Project: "other-proj",
		})
		require.NoError(s.T(), err)
		require.NoError(s.T(), s.Referrer.ExtractAndPersist(ctx, envelope, h, wfOther.ID.String()))

		// A v1.0.0 run on the second workflow whose attestation digest points at the same
		// attestation, so "other-proj" v1.0.0 contains h.
		contractOther, err := s.WorkflowContract.Describe(ctx, s.org1.ID, wfOther.ContractID.String(), 0)
		require.NoError(s.T(), err)
		runOther, err := s.WorkflowRun.Create(ctx, &biz.WorkflowRunCreateOpts{
			WorkflowID: wfOther.ID.String(), ContractRevision: contractOther, CASBackendID: casBackend.ID,
			ProjectVersion: "v1.0.0",
		})
		require.NoError(s.T(), err)
		require.NoError(s.T(), s.Repos.WorkflowRunRepo.SaveAttestationDigest(ctx, runOther.ID, h.String(), false))

		// RBAC: caller can see project "test" in org1 only — NOT "other-proj".
		rbac := map[biz.OrgID][]biz.ProjectID{s.org1UUID: {s.workflow1.ProjectID}}

		got, _, err := s.Referrer.GetFromRoot(ctx, sbomDigest, "", []uuid.UUID{s.org1UUID}, rbac, nil, biz.WithProjectScope("other-proj", "v1.0.0"))
		s.True(biz.IsNotFound(err), "expected NotFound when filtering by a project the caller cannot see")
		s.Nil(got)

		// Sanity: with the visible project, the same material is returned scoped to its version,
		// proving the test setup is sound and the fix isn't over-blocking.
		got, _, err = s.Referrer.GetFromRoot(ctx, sbomDigest, "", []uuid.UUID{s.org1UUID}, rbac, nil, biz.WithProjectScope("test", "v1.0.0"))
		s.NoError(err)
		s.Require().NotNil(got)
		s.Equal(sbomDigest, got.Digest)
	})

	s.Run("material root cannot bypass version scoping by supplying a cursor", func() {
		// A second project version whose run points to an unrelated attestation digest, so the
		// SBOM (only referenced by the v1.0.0 attestation) does not belong to it.
		run2, err := s.WorkflowRun.Create(ctx, &biz.WorkflowRunCreateOpts{
			WorkflowID: s.workflow1.ID.String(), ContractRevision: contractVersion, CASBackendID: casBackend.ID,
			ProjectVersion: "v2.0.0",
		})
		require.NoError(s.T(), err)
		require.NoError(s.T(), s.Repos.WorkflowRunRepo.SaveAttestationDigest(ctx, run2.ID, "sha256:"+strings.Repeat("a", 64), false))

		// Paging past the first page must not skip the membership check (regression for the
		// firstPage gate that allowed a cursor to bypass version scoping).
		cursor, err := pagination.NewCursor(pagination.EncodeCursor(time.Now(), uuid.New()), 10)
		require.NoError(s.T(), err)
		got, _, err := s.Referrer.GetFromRootUser(ctx, sbomDigest, "", s.user.ID, cursor, biz.WithProjectScope("test", "v2.0.0"))
		s.True(biz.IsNotFound(err))
		s.Nil(got)
	})
}

type referrerIntegrationTestSuite struct {
	testhelpers.UseCasesEachTestSuite
	org1, org2           *biz.Organization
	workflow1, workflow2 *biz.Workflow
	org1UUID, org2UUID   uuid.UUID
	user, user2          *biz.User
	run                  *biz.WorkflowRun
}

func (s *referrerIntegrationTestSuite) SetupTest() {
	credsWriter := creds.NewReaderWriter(s.T())
	ctx := context.Background()
	credsWriter.On("SaveCredentials", mock.Anything, mock.Anything, &credentials.OCIKeypair{Repo: "repo", Username: "username", Password: "pass"}).Return("stored-OCI-secret", nil)

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

	s.workflow1, err = s.Workflow.Create(ctx, &biz.WorkflowCreateOpts{Name: "wf", Team: "team", OrgID: s.org1.ID, Project: "test"})
	require.NoError(s.T(), err)
	s.workflow2, err = s.Workflow.Create(ctx, &biz.WorkflowCreateOpts{Name: "wf-from-org-2", Team: "team", OrgID: s.org2.ID, Project: "test"})
	require.NoError(s.T(), err)

	// user 1 has access to org 1 and 2
	s.user, err = s.User.UpsertByEmail(ctx, "user-1@test.com", nil)
	require.NoError(s.T(), err)
	_, err = s.Membership.Create(ctx, s.org1.ID, s.user.ID)
	require.NoError(s.T(), err)
	_, err = s.Membership.Create(ctx, s.org2.ID, s.user.ID, biz.WithCurrentMembership())
	require.NoError(s.T(), err)

	// user 2 has access to only org 2
	s.user2, err = s.User.UpsertByEmail(ctx, "user-2@test.com", nil)
	require.NoError(s.T(), err)
	_, err = s.Membership.Create(ctx, s.org2.ID, s.user2.ID, biz.WithCurrentMembership())
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

// TestEdgesAmong covers the endpoint a client uses once it already holds a set of referrers and
// only needs to know how they connect.
func (s *referrerIntegrationTestSuite) TestEdgesAmong() {
	ctx := context.Background()
	envelope, envBytes := testEnvelope(s.T(), "testdata/attestations/with-git-subject.json")
	attDigest, _, err := v1.SHA256(bytes.NewReader(envBytes))
	require.NoError(s.T(), err)
	require.NoError(s.T(), s.Referrer.ExtractAndPersist(ctx, envelope, attDigest, s.workflow1.ID.String()))

	// Read back what the attestation is connected to, so the test does not hardcode the fixture.
	root, _, err := s.Referrer.GetFromRootUser(ctx, attDigest.String(), "", s.user.ID, nil)
	require.NoError(s.T(), err)
	require.NotEmpty(s.T(), root.References)

	attestation := &biz.ReferrerRef{Digest: root.Digest, Kind: root.Kind}
	materials := make([]*biz.ReferrerRef, 0, len(root.References))
	for _, ref := range root.References {
		materials = append(materials, &biz.ReferrerRef{Digest: ref.Digest, Kind: ref.Kind})
	}

	s.Run("every material is reported as connected to the attestation, once", func() {
		nodes := append([]*biz.ReferrerRef{attestation}, materials...)
		edges, err := s.Referrer.EdgesAmongUser(ctx, nodes, s.user.ID)
		s.NoError(err)
		s.Len(edges, len(materials), "one edge per material, not one per stored direction")

		for _, e := range edges {
			s.Less(e.From, e.To, "edges are normalised so the lower index comes first")
			s.Equal(0, e.From, "every edge in this fixture hangs off the attestation at index 0")
		}
	})

	// The caller's role-based project visibility is not an optional filter: a member who cannot
	// reach the project must not learn that two of its referrers are connected, whether or not
	// they name a project in the request.
	s.Run("a caller restricted to no project in the org gets no edges", func() {
		nodes := append([]*biz.ReferrerRef{attestation}, materials...)
		// RBAC on for the org, nothing granted in it.
		noProjects := map[biz.OrgID][]biz.ProjectID{s.org1UUID: {}}
		edges, err := s.Referrer.EdgesAmong(ctx, nodes, []uuid.UUID{s.org1UUID}, noProjects)
		s.NoError(err)
		s.Empty(edges, "org membership alone must not reveal connections")

		// And the established endpoint agrees for the same caller.
		_, _, err = s.Referrer.GetFromRoot(ctx, attestation.Digest, attestation.Kind, []uuid.UUID{s.org1UUID}, noProjects, nil)
		s.Error(err, "the reference endpoint denies this caller, so the edges endpoint must too")
	})

	s.Run("a caller granted the project still gets every edge", func() {
		nodes := append([]*biz.ReferrerRef{attestation}, materials...)
		granted := map[biz.OrgID][]biz.ProjectID{s.org1UUID: {s.workflow1.ProjectID}}
		edges, err := s.Referrer.EdgesAmong(ctx, nodes, []uuid.UUID{s.org1UUID}, granted)
		s.NoError(err)
		s.Len(edges, len(materials), "a granted project must not lose any connection")
	})

	s.Run("the order the caller gives is the order the indexes refer to", func() {
		// Same set, attestation last: the indexes must follow.
		nodes := append(append([]*biz.ReferrerRef{}, materials...), attestation)
		attestationIndex := len(nodes) - 1
		edges, err := s.Referrer.EdgesAmongUser(ctx, nodes, s.user.ID)
		s.NoError(err)
		s.Len(edges, len(materials))
		for _, e := range edges {
			s.Equal(attestationIndex, e.To, "the attestation is now the higher index of every pair")
		}
	})

	s.Run("materials alone have no edges between them", func() {
		if len(materials) < 2 {
			s.T().Skip("fixture has a single material")
		}
		edges, err := s.Referrer.EdgesAmongUser(ctx, materials, s.user.ID)
		s.NoError(err)
		s.Empty(edges, "materials are connected through their attestation, not to each other")
	})

	s.Run("a referrer the caller cannot see contributes no edges", func() {
		nodes := append([]*biz.ReferrerRef{attestation}, materials...)
		// user2 belongs to org2 only, where none of this was ingested.
		edges, err := s.Referrer.EdgesAmongUser(ctx, nodes, s.user2.ID)
		s.NoError(err)
		s.Empty(edges)
	})

	s.Run("an unknown referrer is ignored rather than failing", func() {
		nodes := []*biz.ReferrerRef{
			attestation,
			{Digest: "sha256:0000000000000000000000000000000000000000000000000000000000000000", Kind: "ATTESTATION"},
		}
		edges, err := s.Referrer.EdgesAmongUser(ctx, nodes, s.user.ID)
		s.NoError(err)
		s.Empty(edges)
	})

	s.Run("the kind is part of the identity", func() {
		// The same digest under a kind it was not stored as must not resolve.
		nodes := []*biz.ReferrerRef{
			{Digest: attestation.Digest, Kind: "CONTAINER_IMAGE"},
			materials[0],
		}
		edges, err := s.Referrer.EdgesAmongUser(ctx, nodes, s.user.ID)
		s.NoError(err)
		s.Empty(edges)
	})

	s.Run("fewer than two referrers is refused without touching the store", func() {
		_, err := s.Referrer.EdgesAmongUser(ctx, []*biz.ReferrerRef{attestation}, s.user.ID)
		s.Error(err)
		s.True(biz.IsErrValidation(err), "one referrer describes no connection")
	})

	// A client that collected referrers from more than one attestation will list a shared
	// material twice. That is accepted, and the answer names the position it first appeared in.
	s.Run("a referrer listed twice is answered at its first position", func() {
		nodes := []*biz.ReferrerRef{attestation, materials[0], materials[0]}
		edges, err := s.Referrer.EdgesAmongUser(ctx, nodes, s.user.ID)
		s.NoError(err)
		s.Equal([]biz.ReferrerEdge{{From: 0, To: 1}}, edges, "index 2 repeats index 1, which is the one named")
	})

	// Identity is the pair, not the two joined together: kind is whatever the caller sent, so a
	// separator inside it must not make two different referrers count as one.
	s.Run("referrers that differ only in where the kind ends are two referrers", func() {
		nodes := []*biz.ReferrerRef{{Kind: "A", Digest: "B-C"}, {Kind: "A-B", Digest: "C"}}
		edges, err := s.Referrer.EdgesAmongUser(ctx, nodes, s.user.ID)
		s.NoError(err, "these are two distinct referrers and must not be refused as one")
		s.Empty(edges, "neither exists, so neither contributes an edge")
	})

	s.Run("the same referrer twice is not two referrers", func() {
		_, err := s.Referrer.EdgesAmongUser(ctx, []*biz.ReferrerRef{attestation, attestation}, s.user.ID)
		s.Error(err, "two positions naming one referrer describe no connection")
		s.True(biz.IsErrValidation(err))
	})
}

// TestEveryConnectionIsWrittenFromTheAttestation pins the property EdgesAmong depends on to stay
// bounded: it reads connections from the attestation side, so ingest must always write that row.
// A future change that only recorded the material's side would make edges disappear from the
// graph rather than fail loudly.
func (s *referrerIntegrationTestSuite) TestEveryConnectionIsWrittenFromTheAttestation() {
	ctx := context.Background()

	// Cover the shapes ingest produces: materials, a git subject, and a dependent attestation.
	dependent, _ := testEnvelope(s.T(), "testdata/attestations/dependent-attestation.json")
	dependentDigest, _ := v1.NewHash("sha256:2dc17f7c933d20e06b49250a582a3d19bdfbadba9c4e5f3f856af6f261db79d4")
	require.NoError(s.T(), s.Referrer.ExtractAndPersist(ctx, dependent, dependentDigest, s.workflow1.ID.String()))

	withDependent, _ := testEnvelope(s.T(), "testdata/attestations/with-dependent-attestation.json")
	withDependentDigest, _ := v1.NewHash("sha256:950c7b4c65447a3b86b6f769515005e7c44a67c8193bff790750eadf13207fbb")
	require.NoError(s.T(), s.Referrer.ExtractAndPersist(ctx, withDependent, withDependentDigest, s.workflow1.ID.String()))

	gitSubject, gitBytes := testEnvelope(s.T(), "testdata/attestations/with-git-subject.json")
	gitDigest, _, err := v1.SHA256(bytes.NewReader(gitBytes))
	require.NoError(s.T(), err)
	require.NoError(s.T(), s.Referrer.ExtractAndPersist(ctx, gitSubject, gitDigest, s.workflow1.ID.String()))

	// Every stored connection must have a row whose owning side is an attestation.
	var orphaned int
	row := s.Data.SQLDB.QueryRowContext(ctx, `
		SELECT count(*) FROM referrer_references rr
		JOIN referrers owner ON owner.id = rr.referrer_id
		WHERE owner.kind <> 'ATTESTATION'
		  AND NOT EXISTS (
		    SELECT 1 FROM referrer_references mirror
		    JOIN referrers mirror_owner ON mirror_owner.id = mirror.referrer_id
		    WHERE mirror.referrer_id = rr.referred_by_id
		      AND mirror.referred_by_id = rr.referrer_id
		      AND mirror_owner.kind = 'ATTESTATION'
		  )`)
	require.NoError(s.T(), row.Scan(&orphaned))
	s.Zero(orphaned, "every connection must be reachable from the attestation that created it")

	var total int
	require.NoError(s.T(), s.Data.SQLDB.QueryRowContext(ctx, `SELECT count(*) FROM referrer_references`).Scan(&total))
	s.NotZero(total, "the fixture must actually have written connections")
}
