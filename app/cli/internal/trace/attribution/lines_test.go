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

func TestFingerprint(t *testing.T) {
	const returnNil = "return nil"

	testCases := []struct {
		name  string
		a, b  string
		equal bool
	}{
		{name: "same content", a: returnNil, b: returnNil, equal: true},
		{name: "re-indented", a: "\treturn nil", b: "        return nil", equal: true},
		{name: "inner whitespace", a: "x := 1", b: "x:=1", equal: true},
		{name: "carriage return", a: "x := 1\r", b: "x := 1", equal: true},
		{name: "different content", a: returnNil, b: "return err", equal: false},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.equal, Fingerprint(tc.a) == Fingerprint(tc.b))
		})
	}
}

func fps(lines ...string) []string {
	out := make([]string, 0, len(lines))
	for _, l := range lines {
		out = append(out, Fingerprint(l))
	}

	return out
}

func TestComputeEdit(t *testing.T) {
	testCases := []struct {
		name         string
		before       []byte
		after        []byte
		changed      bool
		ranges       []aicodingsession.LineRange
		added        []string
		removed      []string
		addedCount   int
		removedCount int
	}{
		{
			name:  "identical content",
			after: []byte("a\n"), before: []byte("a\n"),
		},
		{
			name:       "new file",
			after:      []byte("a\nb\nc"),
			changed:    true,
			ranges:     []aicodingsession.LineRange{{Start: 1, End: 3}},
			added:      fps("a", "b", "c"),
			addedCount: 3,
		},
		{
			name:         "modified line",
			before:       []byte("a\nb\nc\n"),
			after:        []byte("a\nB\nc\n"),
			changed:      true,
			ranges:       []aicodingsession.LineRange{{Start: 2, End: 2}},
			added:        fps("B"),
			removed:      fps("b"),
			addedCount:   1,
			removedCount: 1,
		},
		{
			name:         "deleted file",
			before:       []byte("a\nb\n"),
			changed:      true,
			removed:      fps("a", "b"),
			removedCount: 2,
		},
		{
			// A re-indent writes no new code, so the line keeps its owner
			// (D-008). The raw counts still record what the edit touched.
			name:         "whitespace-only change",
			before:       []byte("if x {\nreturn\n}\n"),
			after:        []byte("if x {\n\treturn\n}\n"),
			changed:      true,
			ranges:       []aicodingsession.LineRange{{Start: 2, End: 2}},
			addedCount:   1,
			removedCount: 1,
		},
		{
			name:         "re-indent next to a real change",
			before:       []byte("a\nb\n"),
			after:        []byte("  a\nc\n"),
			changed:      true,
			ranges:       []aicodingsession.LineRange{{Start: 1, End: 2}},
			added:        fps("c"),
			removed:      fps("b"),
			addedCount:   2,
			removedCount: 2,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			got := ComputeEdit(tc.before, tc.after)
			assert.Equal(t, tc.changed, got.Changed)
			assert.Equal(t, tc.ranges, got.Ranges)
			assert.ElementsMatch(t, tc.added, got.Added)
			assert.ElementsMatch(t, tc.removed, got.Removed)
			assert.Equal(t, tc.addedCount, got.AddedCount)
			assert.Equal(t, tc.removedCount, got.RemovedCount)
		})
	}
}

func TestFingerprintLines(t *testing.T) {
	assert.Equal(t, fps("a", "b"), FingerprintLines([]byte("a\nb\n")))
	assert.Equal(t, fps("a", "b"), FingerprintLines([]byte("a\nb")))
	assert.Empty(t, FingerprintLines(nil))
}
