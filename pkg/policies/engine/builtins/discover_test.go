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
	"github.com/open-policy-agent/opa/v1/rego"
	"github.com/open-policy-agent/opa/v1/topdown"
	"github.com/open-policy-agent/opa/v1/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// testKind is the kind of the referrer that the tests discover
const testKind = "CONTAINER_IMAGE"

// fakeReferrerClient serves total references of one digest in pages of the requested limit.
// The cursor is the index of the first reference of the page.
type fakeReferrerClient struct {
	v1.ReferrerServiceClient
	total  int
	failAt int // fail the request with this page number (1-based), 0 means never
	// failErr is the error of the failed request, a generic error when nil
	failErr error
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
		if f.failErr != nil {
			return nil, f.failErr
		}
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
		// a control plane that ignores the cursor repeats the first page: keep it once, stop and report truncated
		{name: "control plane ignores the cursor", total: 2*discoverPageSize + 50, ignoreCursor: true, wantRequests: 2, wantRefs: discoverPageSize, wantTruncated: true},
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

func TestDiscoverErrors(t *testing.T) {
	notFound := status.Error(codes.NotFound, "artifact or piece of evidence with digest sha256:deadbeef not found")
	testCases := []struct {
		name    string
		failAt  int
		failErr error
		wantErr bool
	}{
		// the digest has no node in the graph yet: no references, not an error
		{name: "not found", failAt: 1, failErr: notFound},
		{name: "unavailable", failAt: 1, failErr: status.Error(codes.Unavailable, "down"), wantErr: true},
		{name: "permission denied", failAt: 1, failErr: status.Error(codes.PermissionDenied, "denied"), wantErr: true},
		{name: "unauthenticated", failAt: 1, failErr: status.Error(codes.Unauthenticated, "no token"), wantErr: true},
		{name: "internal", failAt: 1, failErr: status.Error(codes.Internal, "boom"), wantErr: true},
		{name: "not a gRPC status", failAt: 1, failErr: errors.New("boom"), wantErr: true},
		// the node existed for the first page, so a later NotFound is not "no node yet"
		{name: "not found on a later page", failAt: 2, failErr: notFound, wantErr: true},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			svc := &DiscoverService{client: &fakeReferrerClient{total: 2 * discoverPageSize, failAt: tc.failAt, failErr: tc.failErr}}

			got, err := svc.Discover(context.Background(), "sha256:deadbeef", "")
			if tc.wantErr {
				require.Error(t, err)
				assert.Equal(t, status.Code(tc.failErr), status.Code(err))
				assert.Nil(t, got)
				return
			}

			require.NoError(t, err)
			// an empty result, not nil: nil means no connection to the control plane
			require.NotNil(t, got)
			assert.Empty(t, got.GetReferences())
			assert.False(t, got.Truncated)

			raw, err := json.Marshal(got)
			require.NoError(t, err)
			assert.JSONEq(t, `{"digest": "sha256:deadbeef", "truncated": false}`, string(raw))
		})
	}
}

// TestDiscoverBuiltinInRego evaluates a policy that calls the builtin with strict builtin errors,
// as the policy engine does, so a builtin error fails the evaluation
func TestDiscoverBuiltinInRego(t *testing.T) {
	const query = `count([ref | some ref in chainloop.discover("sha256:deadbeef", "").references])`
	testCases := []struct {
		name    string
		client  *fakeReferrerClient
		want    int
		wantErr bool
	}{
		{name: "references", client: &fakeReferrerClient{total: 3}, want: 3},
		{name: "digest not found", client: &fakeReferrerClient{failAt: 1, failErr: status.Error(codes.NotFound, "not found")}, want: 0},
		{name: "control plane unavailable", client: &fakeReferrerClient{failAt: 1, failErr: status.Error(codes.Unavailable, "down")}, wantErr: true},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			impl := getDiscoverRegoImpl(&DiscoverService{client: tc.client})
			discover := rego.Function2(
				&rego.Function{Name: discoverBuiltinName, Decl: types.NewFunction(types.Args(types.S, types.S), types.A), Nondeterministic: true},
				func(bctx rego.BuiltinContext, digest, kind *ast.Term) (*ast.Term, error) {
					var out *ast.Term
					err := impl(bctx, []*ast.Term{digest, kind}, func(term *ast.Term) error { out = term; return nil })
					return out, err
				},
			)

			rs, err := rego.New(rego.Query(query), discover, rego.StrictBuiltinErrors(true)).Eval(context.Background())
			if tc.wantErr {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
			require.Len(t, rs, 1)
			assert.EqualValues(t, json.Number(strconv.Itoa(tc.want)), rs[0].Expressions[0].Value)
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
