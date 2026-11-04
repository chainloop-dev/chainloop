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

package builtins

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"testing"

	v1 "github.com/chainloop-dev/chainloop/app/controlplane/api/controlplane/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
)

// fakeReferrerClient serves total references of one digest in pages of the requested limit.
// The cursor is the index of the first reference of the page.
type fakeReferrerClient struct {
	v1.ReferrerServiceClient
	total    int
	failAt   int // fail the request with this page number (1-based), 0 means never
	requests []*v1.ReferrerServiceDiscoverPrivateRequest
}

func (f *fakeReferrerClient) DiscoverPrivate(_ context.Context, req *v1.ReferrerServiceDiscoverPrivateRequest, _ ...grpc.CallOption) (*v1.ReferrerServiceDiscoverPrivateResponse, error) {
	f.requests = append(f.requests, req)
	if f.failAt == len(f.requests) {
		return nil, errors.New("boom")
	}

	start := 0
	if c := req.GetPagination().GetCursor(); c != "" {
		var err error
		if start, err = strconv.Atoi(c); err != nil {
			return nil, err
		}
	}
	end := min(start+int(req.GetPagination().GetLimit()), f.total)

	refs := make([]*v1.ReferrerItem, 0, end-start)
	for i := start; i < end; i++ {
		refs = append(refs, &v1.ReferrerItem{Digest: fmt.Sprintf("sha256:%d", i), Kind: "ATTESTATION"})
	}

	var next string
	if end < f.total {
		next = strconv.Itoa(end)
	}

	return &v1.ReferrerServiceDiscoverPrivateResponse{
		Result:     &v1.ReferrerItem{Digest: req.GetDigest(), Kind: "CONTAINER_IMAGE", References: refs},
		Pagination: &v1.CursorPaginationResponse{NextCursor: next},
	}, nil
}

func TestDiscoverReadsAllPages(t *testing.T) {
	const maxRefs = discoverMaxPages * discoverPageSize
	testCases := []struct {
		name          string
		total         int
		failAt        int
		wantRequests  int
		wantRefs      int
		wantTruncated bool
		wantErr       bool
	}{
		{name: "no references", total: 0, wantRequests: 1},
		{name: "single page", total: 20, wantRequests: 1, wantRefs: 20},
		{name: "exactly one full page", total: discoverPageSize, wantRequests: 1, wantRefs: discoverPageSize},
		{name: "multiple pages", total: 2*discoverPageSize + 50, wantRequests: 3, wantRefs: 2*discoverPageSize + 50},
		{name: "last allowed page ends the read", total: maxRefs, wantRequests: discoverMaxPages, wantRefs: maxRefs},
		{name: "page cap stops the read", total: maxRefs + 1, wantRequests: discoverMaxPages, wantRefs: maxRefs, wantTruncated: true},
		{name: "error on a later page", total: 2*discoverPageSize + 50, failAt: 2, wantRequests: 2, wantErr: true},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			client := &fakeReferrerClient{total: tc.total, failAt: tc.failAt}
			svc := &DiscoverService{client: client}

			got, err := svc.Discover(context.Background(), "sha256:deadbeef", "CONTAINER_IMAGE")
			assert.Len(t, client.requests, tc.wantRequests)
			if tc.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)

			// every request asks for the largest page and keeps the digest and kind
			for i, req := range client.requests {
				assert.Equal(t, "sha256:deadbeef", req.GetDigest())
				assert.Equal(t, "CONTAINER_IMAGE", req.GetKind())
				assert.Equal(t, int32(discoverPageSize), req.GetPagination().GetLimit())
				if i == 0 {
					assert.Empty(t, req.GetPagination().GetCursor())
				}
			}

			assert.Equal(t, "sha256:deadbeef", got.GetDigest())
			assert.Equal(t, tc.wantTruncated, got.Truncated)
			require.Len(t, got.GetReferences(), tc.wantRefs)
			// the references keep the order of the pages
			for i, ref := range got.GetReferences() {
				assert.Equal(t, fmt.Sprintf("sha256:%d", i), ref.GetDigest())
			}
		})
	}
}

func TestDiscoverNoConnection(t *testing.T) {
	got, err := NewDiscoverService(nil).Discover(context.Background(), "sha256:deadbeef", "")
	require.NoError(t, err)
	assert.Nil(t, got)
}

func TestDiscoverResultJSONShape(t *testing.T) {
	res := &DiscoverResult{
		ReferrerItem: &v1.ReferrerItem{
			Digest:     "sha256:deadbeef",
			Kind:       "CONTAINER_IMAGE",
			References: []*v1.ReferrerItem{{Digest: "sha256:1", Kind: "ATTESTATION"}},
		},
	}

	raw, err := json.Marshal(res)
	require.NoError(t, err)
	assert.JSONEq(t, `{
		"digest": "sha256:deadbeef",
		"kind": "CONTAINER_IMAGE",
		"references": [{"digest": "sha256:1", "kind": "ATTESTATION"}],
		"truncated": false
	}`, string(raw))
}
