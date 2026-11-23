//
// Copyright 2025-2026 The Chainloop Authors.
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

	v1 "github.com/chainloop-dev/chainloop/app/controlplane/api/controlplane/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const (
	// discoverPageSize is the number of references requested per page, the maximum that the API accepts
	discoverPageSize = 100
	// discoverMaxPages caps the number of requests of one discover call
	discoverMaxPages = 10
)

// DiscoverResult is the referrer with the references of all the pages read.
// It serializes to the same shape as a ReferrerItem, plus the truncated field.
type DiscoverResult struct {
	*v1.ReferrerItem
	// Truncated is true when the read stopped before the last page of references: either the
	// page cap was reached, or the control plane repeated its next cursor (it ignores the cursor)
	Truncated bool `json:"truncated"`
}

// MarshalJSON always writes references as an array, an empty one when there are no references.
// The proto field omits an empty list, and then a policy that counts the references gets an
// undefined value instead of 0. Rego converts the result through this JSON too.
func (r DiscoverResult) MarshalJSON() ([]byte, error) {
	refs := r.GetReferences()
	if refs == nil {
		refs = []*v1.ReferrerItem{}
	}

	// the shallower references field hides the one of the embedded item
	return json.Marshal(struct {
		*v1.ReferrerItem
		References []*v1.ReferrerItem `json:"references"`
		Truncated  bool               `json:"truncated"`
	}{r.ReferrerItem, refs, r.Truncated})
}

// DiscoverService wraps the gRPC discover functionality to be shared across engines
type DiscoverService struct {
	client v1.ReferrerServiceClient
}

// NewDiscoverService creates a new discover service
func NewDiscoverService(conn *grpc.ClientConn) *DiscoverService {
	if conn == nil {
		return &DiscoverService{}
	}

	return &DiscoverService{client: v1.NewReferrerServiceClient(conn)}
}

// Discover calls the DiscoverPrivate gRPC endpoint to get artifact graph data.
// It follows the pagination cursor and merges the references of all pages,
// up to discoverMaxPages requests.
//
// A control plane without pagination support ignores the pagination request and
// returns all the references in one response with no next cursor, so the read
// ends after the first page. A next cursor that does not advance also ends the
// read, so the same page is never merged twice, and marks the result as truncated.
//
// A digest with no node in the graph gives a result with no references. A NotFound on
// a later page is still an error, because the node existed for the first page.
func (s *DiscoverService) Discover(ctx context.Context, digest, kind string) (*DiscoverResult, error) {
	if s.client == nil {
		return nil, nil
	}

	readPage := func(cursor string) (*v1.ReferrerServiceDiscoverPrivateResponse, error) {
		return s.client.DiscoverPrivate(ctx, &v1.ReferrerServiceDiscoverPrivateRequest{
			Digest:     digest,
			Kind:       kind,
			Pagination: &v1.CursorPaginationRequest{Cursor: cursor, Limit: discoverPageSize},
		})
	}

	resp, err := readPage("")
	if err != nil {
		// NotFound means that no node of the graph has this digest yet, for example a material
		// that no workflow run has attested. Return an empty result with no references, so a
		// policy can report it, instead of failing the whole evaluation. A nil result keeps
		// meaning "no connection to the control plane".
		if status.Code(err) == codes.NotFound {
			return &DiscoverResult{ReferrerItem: &v1.ReferrerItem{Digest: digest, Kind: kind}}, nil
		}

		return nil, err
	}

	result := &DiscoverResult{ReferrerItem: resp.GetResult()}
	for pages := 1; result.ReferrerItem != nil; pages++ {
		cursor := resp.GetPagination().GetNextCursor()
		if cursor == "" {
			break
		}

		if pages == discoverMaxPages {
			result.Truncated = true
			break
		}

		if resp, err = readPage(cursor); err != nil {
			return nil, err
		}

		// a server that ignores the cursor returns the same page and the same next cursor again,
		// so the rest of the references cannot be read
		if resp.GetPagination().GetNextCursor() == cursor {
			result.Truncated = true
			break
		}

		result.References = append(result.References, resp.GetResult().GetReferences()...)
	}

	return result, nil
}
