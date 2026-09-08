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

package service

import (
	"context"
	"io"
	"testing"

	pb "github.com/chainloop-dev/chainloop/app/controlplane/api/controlplane/v1"
	conf "github.com/chainloop-dev/chainloop/app/controlplane/internal/conf/controlplane/config/v1"
	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/biz"
	bizMocks "github.com/chainloop-dev/chainloop/app/controlplane/pkg/biz/mocks"
	attestationpb "github.com/chainloop-dev/chainloop/pkg/attestation/crafter/api/attestation/v1"
	"github.com/chainloop-dev/chainloop/pkg/attestation/renderer/chainloop"
	"github.com/chainloop-dev/chainloop/pkg/cache/policyevalbundle"
	"github.com/google/uuid"
	intoto "github.com/in-toto/attestation/go/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/encoding/protojson"
)

const (
	sha256Alg           = "sha256"
	testBundleHexDigest = "cf4c9c8b7b1b4f4d0b4e3f4a5c6d7e8f9a0b1c2d3e4f5a6b7c8d9e0f1a2b3c4d"
	testBundleDigest    = sha256Alg + ":" + testBundleHexDigest
)

// policyEvaluationsBundle builds a valid protojson-encoded bundle carrying a
// single evaluation with one violation.
func policyEvaluationsBundle(t *testing.T) []byte {
	t.Helper()

	bundle := &attestationpb.PolicyEvaluationBundle{
		Evaluations: []*attestationpb.PolicyEvaluation{
			{
				Name:         "strong-acl",
				MaterialName: "registry-report",
				Violations: []*attestationpb.PolicyEvaluation_Violation{
					{Subject: "HKLM\\Software", Message: "weak ACL"},
				},
			},
		},
	}

	data, err := protojson.Marshal(bundle)
	require.NoError(t, err)

	return data
}

func testResourceDescriptor() *intoto.ResourceDescriptor {
	return &intoto.ResourceDescriptor{
		Name:      "policy-evaluations",
		Digest:    map[string]string{sha256Alg: testBundleHexDigest},
		MediaType: chainloop.PolicyEvaluationsBundleMediaType,
	}
}

func TestResolvePolicyEvaluations(t *testing.T) {
	orgID := uuid.New()
	bundle := policyEvaluationsBundle(t)

	testCases := []struct {
		name string
		// descriptor defaults to a valid one when nil and useNilDescriptor is false
		descriptor       *intoto.ResourceDescriptor
		useNilDescriptor bool
		// bundleSize is what the attestation records; zero means it records none
		bundleSize int64
		// maxInlineBytes 0 selects the built-in default
		maxInlineBytes int64
		// seedCache pre-populates the bundle cache with these bytes
		seedCache []byte
		// mappingErr makes the CAS mapping lookup fail
		mappingErr error
		// downloadBody is what the CAS download writes out
		downloadBody []byte

		wantNilResolution bool
		wantEvaluations   bool
		wantRefReason     pb.PolicyEvaluationsRef_Reason
		wantRefSize       int64
		// the reference carries no digest when the descriptor had no sha256
		wantEmptyRefDigest bool
		wantMappingLookup  bool
		wantDownloadCall   bool
	}{
		{
			name:              "no descriptor resolves to nothing",
			useNilDescriptor:  true,
			wantNilResolution: true,
		},
		{
			name:              "a recorded size under the cap is downloaded and inlined",
			bundleSize:        int64(len(bundle)),
			downloadBody:      bundle,
			wantEvaluations:   true,
			wantMappingLookup: true,
			wantDownloadCall:  true,
		},
		{
			name:           "a recorded size over the cap never reaches the CAS",
			maxInlineBytes: 16,
			bundleSize:     64 * 1024 * 1024,
			wantRefReason:  pb.PolicyEvaluationsRef_REASON_TOO_LARGE,
			wantRefSize:    64 * 1024 * 1024,
		},
		{
			name:              "a recorded size exactly at the cap is inlined",
			maxInlineBytes:    int64(len(bundle)),
			bundleSize:        int64(len(bundle)),
			downloadBody:      bundle,
			wantEvaluations:   true,
			wantMappingLookup: true,
			wantDownloadCall:  true,
		},
		{
			// An unrecorded size leaves nothing to enforce the cap against, and
			// sizing the bundle would cost a CAS round trip on every view.
			name:              "an attestation with no recorded size is inlined whatever the cap",
			maxInlineBytes:    4,
			downloadBody:      bundle,
			wantEvaluations:   true,
			wantMappingLookup: true,
			wantDownloadCall:  true,
		},
		{
			name:              "missing CAS mapping is not downloaded",
			bundleSize:        int64(len(bundle)),
			mappingErr:        biz.NewErrNotFound("digest"),
			wantRefReason:     pb.PolicyEvaluationsRef_REASON_UNAVAILABLE,
			wantRefSize:       int64(len(bundle)),
			wantMappingLookup: true,
		},
		{
			name:            "cached bundle skips the CAS entirely",
			bundleSize:      int64(len(bundle)),
			seedCache:       bundle,
			wantEvaluations: true,
		},
		{
			name:           "a cached bundle is still capped by the recorded size",
			seedCache:      bundle,
			bundleSize:     int64(len(bundle)),
			maxInlineBytes: 4,
			wantRefReason:  pb.PolicyEvaluationsRef_REASON_TOO_LARGE,
			wantRefSize:    int64(len(bundle)),
		},
		{
			name:              "undecodable bundle reports unavailable",
			bundleSize:        16,
			downloadBody:      []byte("this is not protojson"),
			wantRefReason:     pb.PolicyEvaluationsRef_REASON_UNAVAILABLE,
			wantRefSize:       int64(len("this is not protojson")),
			wantMappingLookup: true,
			wantDownloadCall:  true,
		},
		{
			name: "descriptor without a sha256 digest reports unavailable",
			descriptor: &intoto.ResourceDescriptor{
				Name:   "policy-evaluations",
				Digest: map[string]string{"sha512": "abc"},
			},
			wantRefReason:      pb.PolicyEvaluationsRef_REASON_UNAVAILABLE,
			wantEmptyRefDigest: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()

			casClient := bizMocks.NewCASClient(t)
			if tc.wantDownloadCall {
				casClient.On("Download", mock.Anything, mock.Anything, mock.Anything, orgID, mock.Anything, testBundleDigest).
					Run(func(args mock.Arguments) {
						w, ok := args.Get(4).(io.Writer)
						require.True(t, ok)
						_, err := w.Write(tc.downloadBody)
						require.NoError(t, err)
					}).Return(nil)
			}

			mappingRepo := bizMocks.NewCASMappingRepo(t)
			if tc.wantMappingLookup {
				mapping := &biz.CASMapping{CASBackend: &biz.CASBackend{
					Provider:       "OCI_REPOSITORY",
					SecretName:     "secret-name",
					OrganizationID: orgID,
				}}
				if tc.mappingErr != nil {
					mapping = nil
				}
				mappingRepo.On("FindByDigestInOrgs", mock.Anything, testBundleDigest, mock.Anything, mock.Anything).
					Return(mapping, tc.mappingErr)
			}

			cache, err := policyevalbundle.New(ctx, nil, nil)
			require.NoError(t, err)
			if tc.seedCache != nil {
				require.NoError(t, cache.Set(ctx, testBundleDigest, tc.seedCache))
			}

			svc := NewWorkflowRunService(&NewWorkflowRunServiceOpts{
				CASClient:       casClient,
				CASMappingUC:    biz.NewCASMappingUseCase(mappingRepo, nil, nil),
				PolicyEvalCache: cache,
				BootstrapConfig: &conf.Bootstrap{
					Attestations: &conf.Attestations{
						PolicyEvaluationsMaxInlineBytes: tc.maxInlineBytes,
					},
				},
			})

			descriptor := tc.descriptor
			if !tc.useNilDescriptor && descriptor == nil {
				descriptor = testResourceDescriptor()
			}

			got := svc.resolvePolicyEvaluations(ctx, descriptor, tc.bundleSize, orgID)

			if tc.wantNilResolution {
				assert.Nil(t, got)
				return
			}

			require.NotNil(t, got)

			if tc.wantEvaluations {
				require.NotEmpty(t, got.evaluations)
				assert.Len(t, got.evaluations["registry-report"], 1)

				// The bundle is referenced alongside the evaluations so callers
				// always know where the full set lives.
				require.NotNil(t, got.ref)
				assert.True(t, got.ref.GetInlined())
				assert.Equal(t, pb.PolicyEvaluationsRef_REASON_UNSPECIFIED, got.ref.GetReason())
				assert.Equal(t, testBundleDigest, got.ref.GetDigest())
				assert.Equal(t, chainloop.PolicyEvaluationsBundleMediaType, got.ref.GetMediaType())
				// The decoded byte count, not the size the attestation recorded.
				assert.Equal(t, int64(len(bundle)), got.ref.GetSizeBytes())

				return
			}

			assert.Empty(t, got.evaluations)
			require.NotNil(t, got.ref)
			assert.False(t, got.ref.GetInlined())
			assert.Equal(t, tc.wantRefReason, got.ref.GetReason())
			assert.Equal(t, tc.wantRefSize, got.ref.GetSizeBytes())

			wantRefDigest := testBundleDigest
			if tc.wantEmptyRefDigest {
				wantRefDigest = ""
			}
			assert.Equal(t, wantRefDigest, got.ref.GetDigest())

			if !tc.wantDownloadCall {
				casClient.AssertNotCalled(t, "Download", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything)
			}
		})
	}
}
