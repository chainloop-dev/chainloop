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

package action

import (
	"context"
	"crypto/sha256"
	"fmt"
	"testing"

	v1 "github.com/chainloop-dev/chainloop/pkg/attestation/crafter/api/attestation/v1"
	"github.com/chainloop-dev/chainloop/pkg/attestation/renderer/chainloop"
	"github.com/chainloop-dev/chainloop/pkg/casclient"
	casclientmock "github.com/chainloop-dev/chainloop/pkg/casclient/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/encoding/protojson"
)

func TestUploadPolicyEvaluationsBundle(t *testing.T) {
	testCases := []struct {
		name        string
		evaluations []*v1.PolicyEvaluation
		uploader    func(t *testing.T) casclient.Uploader
		wantRef     bool
		wantErr     bool
	}{
		{
			name:        "nil evaluations returns nil ref",
			evaluations: nil,
			wantRef:     false,
		},
		{
			name:        "empty evaluations returns nil ref",
			evaluations: []*v1.PolicyEvaluation{},
			wantRef:     false,
		},
		{
			name: "nil uploader returns nil ref",
			evaluations: []*v1.PolicyEvaluation{
				{Name: "test-policy"},
			},
			wantRef: false,
		},
		{
			name: "successful upload returns ref with correct digest and media type",
			evaluations: []*v1.PolicyEvaluation{
				{Name: "test-policy", MaterialName: "sbom"},
			},
			uploader: func(t *testing.T) casclient.Uploader {
				t.Helper()
				m := casclientmock.NewUploader(t)
				m.On("Upload", mock.Anything, mock.Anything, "policy-evaluations.json", mock.MatchedBy(func(digest string) bool {
					return len(digest) > 7 && digest[:7] == "sha256:"
				})).Return(&casclient.UpDownStatus{Filename: "policy-evaluations.json"}, nil)
				return m
			},
			wantRef: true,
		},
		{
			name: "upload failure returns error",
			evaluations: []*v1.PolicyEvaluation{
				{Name: "test-policy"},
			},
			uploader: func(t *testing.T) casclient.Uploader {
				t.Helper()
				m := casclientmock.NewUploader(t)
				m.On("Upload", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil, fmt.Errorf("upload failed"))
				return m
			},
			wantErr: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var uploader casclient.Uploader
			if tc.uploader != nil {
				uploader = tc.uploader(t)
			}

			ref, err := uploadPolicyEvaluationsBundle(context.Background(), tc.evaluations, uploader)
			if tc.wantErr {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)

			if !tc.wantRef {
				assert.Nil(t, ref)
				return
			}

			require.NotNil(t, ref)
			assert.Equal(t, "policy-evaluations", ref.Name)
			assert.Equal(t, chainloop.PolicyEvaluationsBundleMediaType, ref.MediaType)
			assert.NotEmpty(t, ref.Digest["sha256"])

			// Verify the digest matches what we'd expect from serializing the bundle
			bundle := &v1.PolicyEvaluationBundle{Evaluations: tc.evaluations}
			data, err := protojson.Marshal(bundle)
			require.NoError(t, err)
			expectedDigest := fmt.Sprintf("%x", sha256.Sum256(data))
			assert.Equal(t, expectedDigest, ref.Digest["sha256"])
		})
	}
}
