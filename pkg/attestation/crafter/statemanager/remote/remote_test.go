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

package remote

import (
	"context"
	"testing"

	pb "github.com/chainloop-dev/chainloop/app/controlplane/api/controlplane/v1"
	pbmocks "github.com/chainloop-dev/chainloop/app/controlplane/api/controlplane/v1/mocks"
	v1 "github.com/chainloop-dev/chainloop/pkg/attestation/crafter/api/attestation/v1"

	"github.com/chainloop-dev/chainloop/pkg/attestation/crafter"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestWriteUpdatesChecksum(t *testing.T) {
	testCases := []struct {
		name             string
		initialChecksum  string
		responseDigest   string
		expectedChecksum string
	}{
		{
			name:             "updates checksum from response digest",
			initialChecksum:  "old-digest",
			responseDigest:   "new-digest-from-server",
			expectedChecksum: "new-digest-from-server",
		},
		{
			name:             "keeps checksum when response digest is empty",
			initialChecksum:  "old-digest",
			responseDigest:   "",
			expectedChecksum: "old-digest",
		},
		{
			name:             "first write with empty initial checksum",
			initialChecksum:  "",
			responseDigest:   "first-digest",
			expectedChecksum: "first-digest",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			client := pbmocks.NewAttestationStateServiceClient(t)
			client.On("Save", mock.Anything, mock.Anything, mock.Anything).
				Return(&pb.AttestationStateServiceSaveResponse{Digest: tc.responseDigest}, nil)

			r, err := New(client, nil)
			require.NoError(t, err)

			state := &crafter.VersionedCraftingState{
				CraftingState:  &v1.CraftingState{},
				UpdateCheckSum: tc.initialChecksum,
			}

			err = r.Write(context.Background(), "run-123", state)
			require.NoError(t, err)
			assert.Equal(t, tc.expectedChecksum, state.UpdateCheckSum)
		})
	}
}

func TestWriteSendsBaseDigest(t *testing.T) {
	client := pbmocks.NewAttestationStateServiceClient(t)
	client.On("Save", mock.Anything, mock.MatchedBy(func(req *pb.AttestationStateServiceSaveRequest) bool {
		return req.BaseDigest == "current-digest" && req.WorkflowRunId == "run-456"
	}), mock.Anything).Return(&pb.AttestationStateServiceSaveResponse{Digest: "next-digest"}, nil)

	r, err := New(client, nil)
	require.NoError(t, err)

	state := &crafter.VersionedCraftingState{
		CraftingState:  &v1.CraftingState{},
		UpdateCheckSum: "current-digest",
	}

	err = r.Write(context.Background(), "run-456", state)
	require.NoError(t, err)
	assert.Equal(t, "next-digest", state.UpdateCheckSum)
}

func TestConsecutiveWritesUseUpdatedDigest(t *testing.T) {
	client := pbmocks.NewAttestationStateServiceClient(t)

	// First write: sends empty digest, gets "digest-1" back
	client.On("Save", mock.Anything, mock.MatchedBy(func(req *pb.AttestationStateServiceSaveRequest) bool {
		return req.BaseDigest == ""
	}), mock.Anything).Return(&pb.AttestationStateServiceSaveResponse{Digest: "digest-1"}, nil).Once()

	// Second write: sends "digest-1", gets "digest-2" back
	client.On("Save", mock.Anything, mock.MatchedBy(func(req *pb.AttestationStateServiceSaveRequest) bool {
		return req.BaseDigest == "digest-1"
	}), mock.Anything).Return(&pb.AttestationStateServiceSaveResponse{Digest: "digest-2"}, nil).Once()

	r, err := New(client, nil)
	require.NoError(t, err)

	state := &crafter.VersionedCraftingState{
		CraftingState:  &v1.CraftingState{},
		UpdateCheckSum: "",
	}

	// First write
	err = r.Write(context.Background(), "run-789", state)
	require.NoError(t, err)
	assert.Equal(t, "digest-1", state.UpdateCheckSum)

	// Second write uses the updated digest
	err = r.Write(context.Background(), "run-789", state)
	require.NoError(t, err)
	assert.Equal(t, "digest-2", state.UpdateCheckSum)
}
