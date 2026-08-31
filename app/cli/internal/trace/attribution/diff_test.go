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

package attribution

import (
	"testing"

	"github.com/chainloop-dev/chainloop/pkg/attestation/crafter/materials/aicodingsession"
	"github.com/stretchr/testify/assert"
)

func TestComputeLineRanges(t *testing.T) {
	t.Run("nil before means entire file is new", func(t *testing.T) {
		after := []byte("line1\nline2\nline3\n")
		ranges, changed := ComputeLineRanges(nil, after)
		assert.True(t, changed)
		assert.Equal(t, []aicodingsession.LineRange{{Start: 1, End: 3}}, ranges)
	})

	t.Run("both nil is unchanged", func(t *testing.T) {
		ranges, changed := ComputeLineRanges(nil, nil)
		assert.False(t, changed)
		assert.Nil(t, ranges)
	})

	t.Run("empty after with content before is deletion-only", func(t *testing.T) {
		ranges, changed := ComputeLineRanges([]byte("old\n"), []byte(""))
		assert.True(t, changed)
		assert.Nil(t, ranges)
	})

	t.Run("identical content is unchanged", func(t *testing.T) {
		content := []byte("line1\nline2\n")
		ranges, changed := ComputeLineRanges(content, content)
		assert.False(t, changed)
		assert.Nil(t, ranges)
	})

	t.Run("deletion-only edit is changed with no ranges", func(t *testing.T) {
		before := []byte("line1\nline2\nline3\n")
		after := []byte("line1\nline3\n")
		ranges, changed := ComputeLineRanges(before, after)
		assert.True(t, changed)
		assert.Empty(t, ranges)
	})

	t.Run("appended lines detected", func(t *testing.T) {
		before := []byte("line1\nline2\n")
		after := []byte("line1\nline2\nline3\nline4\n")
		ranges, changed := ComputeLineRanges(before, after)
		assert.True(t, changed)
		assert.Equal(t, []aicodingsession.LineRange{{Start: 3, End: 4}}, ranges)
	})

	t.Run("inserted lines in middle", func(t *testing.T) {
		before := []byte("line1\nline3\n")
		after := []byte("line1\nline2\nline3\n")
		ranges, changed := ComputeLineRanges(before, after)
		assert.True(t, changed)
		assert.Equal(t, []aicodingsession.LineRange{{Start: 2, End: 2}}, ranges)
	})

	t.Run("replaced lines detected", func(t *testing.T) {
		before := []byte("line1\nold\nline3\n")
		after := []byte("line1\nnew\nline3\n")
		ranges, changed := ComputeLineRanges(before, after)
		assert.True(t, changed)
		assert.Equal(t, []aicodingsession.LineRange{{Start: 2, End: 2}}, ranges)
	})

	t.Run("new file without trailing newline", func(t *testing.T) {
		ranges, changed := ComputeLineRanges(nil, []byte("single line"))
		assert.True(t, changed)
		assert.Equal(t, []aicodingsession.LineRange{{Start: 1, End: 1}}, ranges)
	})

	t.Run("multiple separate changes", func(t *testing.T) {
		before := []byte("a\nb\nc\nd\ne\n")
		after := []byte("a\nX\nc\nY\ne\n")
		ranges, changed := ComputeLineRanges(before, after)
		assert.True(t, changed)
		// The diff library may merge nearby changes; verify lines 2 and 4 are covered
		assert.NotEmpty(t, ranges)
		assert.Equal(t, 2, ranges[0].Start)
		assert.Equal(t, 4, ranges[len(ranges)-1].End)
	})
}

func TestMergeAdjacentRanges(t *testing.T) {
	t.Run("empty returns empty", func(t *testing.T) {
		assert.Nil(t, mergeAdjacentRanges(nil))
	})

	t.Run("single range unchanged", func(t *testing.T) {
		ranges := []aicodingsession.LineRange{{Start: 1, End: 3}}
		assert.Equal(t, ranges, mergeAdjacentRanges(ranges))
	})

	t.Run("adjacent ranges merged", func(t *testing.T) {
		ranges := []aicodingsession.LineRange{
			{Start: 1, End: 3},
			{Start: 4, End: 6},
		}
		assert.Equal(t, []aicodingsession.LineRange{{Start: 1, End: 6}}, mergeAdjacentRanges(ranges))
	})

	t.Run("overlapping ranges merged", func(t *testing.T) {
		ranges := []aicodingsession.LineRange{
			{Start: 1, End: 5},
			{Start: 3, End: 8},
		}
		assert.Equal(t, []aicodingsession.LineRange{{Start: 1, End: 8}}, mergeAdjacentRanges(ranges))
	})

	t.Run("non-adjacent ranges kept separate", func(t *testing.T) {
		ranges := []aicodingsession.LineRange{
			{Start: 1, End: 3},
			{Start: 5, End: 7},
		}
		assert.Equal(t, ranges, mergeAdjacentRanges(ranges))
	})
}
