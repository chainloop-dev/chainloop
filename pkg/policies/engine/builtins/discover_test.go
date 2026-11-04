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
	"strings"
	"testing"

	v1 "github.com/chainloop-dev/chainloop/app/controlplane/api/controlplane/v1"
	"github.com/open-policy-agent/opa/v1/ast"
	"github.com/open-policy-agent/opa/v1/topdown"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
)

// testKind is the kind of the referrer that the tests discover
const testKind = "CONTAINER_IMAGE"

// fakeReferrerClient serves total references of one digest in pages of the requested limit.
// The cursor is the index of the first reference of the page.
type fakeReferrerClient struct {
	v1.ReferrerServiceClient
	total  int
	failAt int // fail the request with this page number (1-based), 0 means never
	// noPagination acts as a control plane without pagination: it ignores the
	// pagination request and returns all the references with no pagination response
	noPagination bool
	// ignoreCursor acts as a control plane that pages but ignores the cursor
	ignoreCursor bool
	requests     []*v1.ReferrerServiceDiscoverPrivateRequest
}

func (f *fakeReferrerClient) DiscoverPrivate(_ context.Context, req *v1.ReferrerServiceDiscoverPrivateRequest, _ ...grpc.CallOption) (*v1.ReferrerServiceDiscoverPrivateResponse, error) {
	f.requests = append(f.requests, req)
	if f.failAt == len(f.requests) {
		return nil, errors.New("boom")
	}

	if f.noPagination {
		return &v1.ReferrerServiceDiscoverPrivateResponse{
			Result: &v1.ReferrerItem{Digest: req.GetDigest(), Kind: testKind, References: f.refs(0, f.total)},
		}, nil
	}

	start := 0
	if c := req.GetPagination().GetCursor(); c != "" && !f.ignoreCursor {
		var err error
		if start, err = strconv.Atoi(c); err != nil {
			return nil, err
		}
	}
	end := min(start+int(req.GetPagination().GetLimit()), f.total)

	var next string
	if end < f.total {
		next = strconv.Itoa(end)
	}

	return &v1.ReferrerServiceDiscoverPrivateResponse{
		Result:     &v1.ReferrerItem{Digest: req.GetDigest(), Kind: testKind, References: f.refs(start, end)},
		Pagination: &v1.CursorPaginationResponse{NextCursor: next},
	}, nil
}

func (f *fakeReferrerClient) refs(start, end int) []*v1.ReferrerItem {
	refs := make([]*v1.ReferrerItem, 0, end-start)
	for i := start; i < end; i++ {
		refs = append(refs, &v1.ReferrerItem{Digest: fmt.Sprintf("sha256:%d", i), Kind: "ATTESTATION"})
	}
	return refs
}

func TestDiscoverReadsAllPages(t *testing.T) {
	const maxRefs = discoverMaxPages * discoverPageSize
	testCases := []struct {
		name          string
		total         int
		failAt        int
		noPagination  bool
		ignoreCursor  bool
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
		// a control plane without pagination returns everything in one response
		{name: "control plane without pagination", total: maxRefs + 500, noPagination: true, wantRequests: 1, wantRefs: maxRefs + 500},
		// a control plane that ignores the cursor repeats the first page: keep it once and stop
		{name: "control plane ignores the cursor", total: 2*discoverPageSize + 50, ignoreCursor: true, wantRequests: 2, wantRefs: discoverPageSize},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			client := &fakeReferrerClient{total: tc.total, failAt: tc.failAt, noPagination: tc.noPagination, ignoreCursor: tc.ignoreCursor}
			svc := &DiscoverService{client: client}

			got, err := svc.Discover(context.Background(), "sha256:deadbeef", testKind)
			assert.Len(t, client.requests, tc.wantRequests)
			if tc.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)

			// every request asks for the largest page and keeps the digest and kind
			for i, req := range client.requests {
				assert.Equal(t, "sha256:deadbeef", req.GetDigest())
				assert.Equal(t, testKind, req.GetKind())
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

func TestDiscoverRegoImpl(t *testing.T) {
	testCases := []struct {
		name string
		svc  *DiscoverService
		want string
	}{
		{
			name: "merged references and truncated field",
			svc:  &DiscoverService{client: &fakeReferrerClient{total: discoverPageSize + 1}},
			want: fmt.Sprintf(`{"digest": "sha256:deadbeef", "kind": "CONTAINER_IMAGE", "truncated": false, "references": [%s]}`, refsJSON(discoverPageSize+1)),
		},
		{
			name: "no connection",
			svc:  NewDiscoverService(nil),
			want: "null",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var got *ast.Term
			err := getDiscoverRegoImpl(tc.svc)(
				topdown.BuiltinContext{Context: context.Background()},
				[]*ast.Term{ast.StringTerm("sha256:deadbeef"), ast.StringTerm("")},
				func(term *ast.Term) error { got = term; return nil },
			)
			require.NoError(t, err)
			require.NotNil(t, got)
			assert.JSONEq(t, tc.want, got.String())
		})
	}
}

// refsJSON returns the JSON of the first n references that fakeReferrerClient serves
func refsJSON(n int) string {
	refs := make([]string, n)
	for i := range refs {
		refs[i] = fmt.Sprintf(`{"digest": "sha256:%d", "kind": "ATTESTATION"}`, i)
	}
	return strings.Join(refs, ",")
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
			Kind:       testKind,
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
