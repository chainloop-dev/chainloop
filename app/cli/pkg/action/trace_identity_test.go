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
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// writable builds a page of projects the caller can add a workflow to.
func writable(names ...string) []*TraceProject {
	page := make([]*TraceProject, 0, len(names))
	for _, n := range names {
		page = append(page, &TraceProject{Name: n, CanCreateWorkflow: true})
	}

	return page
}

// readOnly builds a page of projects the caller can see but not write to.
func readOnly(names ...string) []*TraceProject {
	page := make([]*TraceProject, 0, len(names))
	for _, n := range names {
		page = append(page, &TraceProject{Name: n})
	}

	return page
}

// fakeProjectPager serves canned pages, recording what it was asked for.
type fakeProjectPager struct {
	pages [][]*TraceProject
	// totalPages is what the server reports; it defaults to len(pages)
	totalPages int32
	err        error

	requested []int32
}

func (f *fakeProjectPager) listProjectsPage(_ context.Context, page, _ int32) ([]*TraceProject, int32, error) {
	f.requested = append(f.requested, page)
	if f.err != nil {
		return nil, 0, f.err
	}

	total := f.totalPages
	if total == 0 {
		total = int32(len(f.pages))
	}

	idx := int(page) - 1
	if idx < 0 || idx >= len(f.pages) {
		return nil, total, nil
	}

	return f.pages[idx], total, nil
}

// Project names the table cases share.
const (
	projAlpha = "alpha"
	projBeta  = "beta"
	projGamma = "gamma"
)

func TestListAllTraceProjects(t *testing.T) {
	testCases := []struct {
		name          string
		pages         [][]*TraceProject
		totalPages    int32
		want          []*TraceProject
		wantRequested []int32
	}{
		{
			name:          "no projects",
			pages:         nil,
			want:          []*TraceProject{},
			wantRequested: []int32{1},
		},
		{
			name:          "a single page is returned as is",
			pages:         [][]*TraceProject{writable(projAlpha, projBeta)},
			want:          writable(projAlpha, projBeta),
			wantRequested: []int32{1},
		},
		{
			name:          "every page is fetched and concatenated",
			pages:         [][]*TraceProject{writable(projAlpha, projBeta), writable(projGamma)},
			want:          writable(projAlpha, projBeta, projGamma),
			wantRequested: []int32{1, 2},
		},
		{
			name:          "a project repeated across pages appears once",
			pages:         [][]*TraceProject{writable(projAlpha, projBeta), writable(projBeta, projGamma)},
			want:          writable(projAlpha, projBeta, projGamma),
			wantRequested: []int32{1, 2},
		},
		{
			name: "an empty page stops the walk even when the server claims more",
			// A server that reports more pages than it serves must not spin here.
			pages:         [][]*TraceProject{writable(projAlpha), {}},
			totalPages:    50,
			want:          writable(projAlpha),
			wantRequested: []int32{1, 2},
		},
		{
			// A project the caller cannot write to is reported, not dropped. The
			// caller needs it to tell a read-only project apart from a missing
			// one, which are offered differently.
			name:          "a project the caller cannot write to is still reported",
			pages:         [][]*TraceProject{append(writable(projAlpha), readOnly(projBeta)...)},
			want:          append(writable(projAlpha), readOnly(projBeta)...),
			wantRequested: []int32{1},
		},
		{
			name:          "a page of read-only projects does not stop the walk",
			pages:         [][]*TraceProject{readOnly(projBeta), writable(projGamma)},
			want:          append(readOnly(projBeta), writable(projGamma)...),
			wantRequested: []int32{1, 2},
		},
		{
			name:          "projects are sorted by name across pages",
			pages:         [][]*TraceProject{writable(projGamma), writable(projAlpha)},
			want:          writable(projAlpha, projGamma),
			wantRequested: []int32{1, 2},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			pager := &fakeProjectPager{pages: tc.pages, totalPages: tc.totalPages}
			got, err := listAllTraceProjects(context.Background(), pager)
			require.NoError(t, err)
			assert.Equal(t, tc.want, got)
			assert.Equal(t, tc.wantRequested, pager.requested)
		})
	}
}

func TestListAllTraceProjectsError(t *testing.T) {
	_, err := listAllTraceProjects(context.Background(), &fakeProjectPager{err: errors.New("boom")})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "boom")
}
