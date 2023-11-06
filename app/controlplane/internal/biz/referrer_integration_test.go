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
	"github.com/secure-systems-lab/go-securesystemslib/dsse"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

func (s *referrerIntegrationTestSuite) TestExtractAndPersists() {
	// Load attestation
	attJSON, err := os.ReadFile("testdata/attestations/with-git-subject.json")
	const attDigest = "sha256:ad704d286bcad6e155e71c33d48247931231338396acbcd9769087530085b2a2"
	require.NoError(s.T(), err)
	var envelope *dsse.Envelope
	require.NoError(s.T(), json.Unmarshal(attJSON, &envelope))

	wantReferrerAtt := &biz.StoredReferrer{
		Digest:       attDigest,
		ArtifactType: "ATTESTATION",
		Downloadable: true,
	}

	wantReferrerCommit := &biz.StoredReferrer{
		Digest:       "sha1:78ac366c9e8a300d51808d581422ca61f7b5b721",
		ArtifactType: "GIT_HEAD_COMMIT",
	}

	wantReferrerSBOM := &biz.StoredReferrer{
		Digest:       "sha256:16159bb881eb4ab7eb5d8afc5350b0feeed1e31c0a268e355e74f9ccbe885e0c",
		ArtifactType: "SBOM_CYCLONEDX_JSON",
		Downloadable: true,
	}

	wantReferrerArtifact := &biz.StoredReferrer{
		Digest:       "sha256:385c4188b9c080499413f2e0fa0b3951ed107b5f0cb35c2f2b1f07a7be9a7512",
		ArtifactType: "ARTIFACT",
		Downloadable: true,
	}

	wantReferrerOpenVEX := &biz.StoredReferrer{
		Digest:       "sha256:b4bd86d5855f94bcac0a92d3100ae7b85d050bd2e5fb9037a200e5f5f0b073a2",
		ArtifactType: "OPENVEX",
		Downloadable: true,
	}

	wantReferrerSarif := &biz.StoredReferrer{
		Digest:       "sha256:c4a63494f9289dd9fd44f841efb4f5b52765c2de6332f2d86e5f6c0340b40a95",
		ArtifactType: "SARIF",
		Downloadable: true,
	}

	wantReferrerContainerImage := &biz.StoredReferrer{
		Digest:       "sha256:fbd9335f55d83d8aaf9ab1a539b0f2a87b444e8c54f34c9a1ca9d7df15605db4",
		ArtifactType: "CONTAINER_IMAGE",
	}

	var prevStoredRef *biz.StoredReferrer
	s.T().Run("it can store properly the first time", func(t *testing.T) {
		err := s.Referrer.ExtractAndPersist(context.Background(), envelope)
		assert.NoError(t, err)
		prevStoredRef, err = s.Referrer.GetFromRoot(context.Background(), attDigest)
		assert.NoError(t, err)
	})

	s.T().Run("and it's idempotent", func(t *testing.T) {
		err := s.Referrer.ExtractAndPersist(context.Background(), envelope)
		assert.NoError(t, err)
		ref, err := s.Referrer.GetFromRoot(context.Background(), attDigest)
		assert.NoError(t, err)
		// Check it's the same referrer than previously retrieved, including timestamps
		assert.Equal(t, prevStoredRef, ref)
	})

	s.T().Run("contains all the info", func(t *testing.T) {
		got, err := s.Referrer.GetFromRoot(context.Background(), attDigest)
		assert.NoError(t, err)
		// parent i.e attestation
		assert.Equal(t, wantReferrerAtt.Digest, got.Digest)
		assert.Equal(t, wantReferrerAtt.Downloadable, got.Downloadable)
		assert.Equal(t, wantReferrerAtt.ArtifactType, got.ArtifactType)
		// it has all the references
		require.Len(t, got.References, 6)

		for i, want := range []*biz.StoredReferrer{
			wantReferrerCommit, wantReferrerSBOM, wantReferrerArtifact, wantReferrerOpenVEX, wantReferrerSarif, wantReferrerContainerImage} {
			gotR := got.References[i]
			assert.Equal(t, want.Digest, gotR.Digest)
			assert.Equal(t, want.ArtifactType, gotR.ArtifactType)
			assert.Equal(t, want.Downloadable, gotR.Downloadable)
		}
	})

	s.T().Run("you can ask for info about materials that are subjects", func(t *testing.T) {
		got, err := s.Referrer.GetFromRoot(context.Background(), wantReferrerContainerImage.Digest)
		assert.NoError(t, err)
		// parent i.e attestation
		assert.Equal(t, wantReferrerContainerImage.Digest, got.Digest)
		assert.Equal(t, wantReferrerContainerImage.Downloadable, got.Downloadable)
		assert.Equal(t, wantReferrerContainerImage.ArtifactType, got.ArtifactType)
		// it's connected to the attestation
		require.Len(t, got.References, 1)
		assert.Equal(t, wantReferrerAtt.Digest, got.References[0].Digest)
		assert.Equal(t, wantReferrerAtt.ArtifactType, got.References[0].ArtifactType)
		assert.Equal(t, wantReferrerAtt.Downloadable, got.References[0].Downloadable)
	})

	s.T().Run("it might not have references", func(t *testing.T) {
		got, err := s.Referrer.GetFromRoot(context.Background(), wantReferrerSarif.Digest)
		assert.NoError(t, err)
		// parent i.e attestation
		assert.Equal(t, wantReferrerSarif.Digest, got.Digest)
		assert.Equal(t, wantReferrerSarif.Downloadable, got.Downloadable)
		assert.Equal(t, wantReferrerSarif.ArtifactType, got.ArtifactType)
		require.Len(t, got.References, 0)
	})

	s.T().Run("or not to exist", func(t *testing.T) {
		got, err := s.Referrer.GetFromRoot(context.Background(), "sha256:deadbeef")
		s.True(biz.IsNotFound(err))
		s.Nil(got)
	})
}

type referrerIntegrationTestSuite struct {
	testhelpers.UseCasesEachTestSuite
}

func (s *referrerIntegrationTestSuite) SetupTest() {
	s.TestingUseCases = testhelpers.NewTestingUseCases(s.T())
}

func TestReferrerIntegration(t *testing.T) {
	suite.Run(t, new(referrerIntegrationTestSuite))
}
