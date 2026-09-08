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

// fakeProjectPager serves canned pages, recording what it was asked for.
type fakeProjectPager struct {
	pages [][]string
	// totalPages is what the server reports; it defaults to len(pages)
	totalPages int32
	err        error

	requested []int32
}

func (f *fakeProjectPager) listProjectsPage(_ context.Context, page, _ int32) ([]string, int32, error) {
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
		pages         [][]string
		totalPages    int32
		want          []string
		wantRequested []int32
	}{
		{
			name:          "no projects",
			pages:         nil,
			want:          []string{},
			wantRequested: []int32{1},
		},
		{
			name:          "a single page is returned as is",
			pages:         [][]string{{projAlpha, projBeta}},
			want:          []string{projAlpha, projBeta},
			wantRequested: []int32{1},
		},
		{
			name:          "every page is fetched and concatenated",
			pages:         [][]string{{projAlpha, projBeta}, {projGamma}},
			want:          []string{projAlpha, projBeta, projGamma},
			wantRequested: []int32{1, 2},
		},
		{
			name:          "a project repeated across pages appears once",
			pages:         [][]string{{projAlpha, projBeta}, {projBeta, projGamma}},
			want:          []string{projAlpha, projBeta, projGamma},
			wantRequested: []int32{1, 2},
		},
		{
			name: "an empty page stops the walk even when the server claims more",
			// A server that reports more pages than it serves must not spin here.
			pages:         [][]string{{projAlpha}, {}},
			totalPages:    50,
			want:          []string{projAlpha},
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
