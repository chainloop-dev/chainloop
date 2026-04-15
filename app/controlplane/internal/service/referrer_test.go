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
	"testing"

	pb "github.com/chainloop-dev/chainloop/app/controlplane/api/controlplane/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestReferrerPaginationOptsFromProto(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		in        *pb.CursorPaginationRequest
		wantLimit int
	}{
		{
			name:      "nil request applies default page size",
			in:        nil,
			wantLimit: defaultReferrerPageSize,
		},
		{
			name:      "empty request applies default page size",
			in:        &pb.CursorPaginationRequest{},
			wantLimit: defaultReferrerPageSize,
		},
		{
			name:      "zero limit applies default page size",
			in:        &pb.CursorPaginationRequest{Limit: 0},
			wantLimit: defaultReferrerPageSize,
		},
		{
			name:      "explicit limit is honored",
			in:        &pb.CursorPaginationRequest{Limit: 50},
			wantLimit: 50,
		},
		{
			name:      "limit of 1 is honored",
			in:        &pb.CursorPaginationRequest{Limit: 1},
			wantLimit: 1,
		},
		{
			name:      "negative limit falls through to default page size",
			in:        &pb.CursorPaginationRequest{Limit: -5},
			wantLimit: defaultReferrerPageSize,
		},
		{
			name:      "proto max limit of 100 is honored",
			in:        &pb.CursorPaginationRequest{Limit: 100},
			wantLimit: 100,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			opts, err := referrerPaginationOptsFromProto(tc.in)
			require.NoError(t, err)
			require.NotNil(t, opts, "pagination options must always be returned so the response is bounded")
			assert.Equal(t, tc.wantLimit, opts.Limit)
		})
	}
}
