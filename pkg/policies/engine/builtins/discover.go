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

	v1 "github.com/chainloop-dev/chainloop/app/controlplane/api/controlplane/v1"
	"google.golang.org/grpc"
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
	// Truncated is true when the page cap stopped the read before the last page of references
	Truncated bool `json:"truncated"`
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
