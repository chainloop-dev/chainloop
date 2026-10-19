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

package state

import (
	"sort"

	"github.com/chainloop-dev/chainloop/pkg/attestation/crafter/materials/aicodingsession"
)

// LineCounts is a multiset of line fingerprints: how many lines with each
// content key. Line attribution matches lines by content, and a file can hold
// the same line more than once, so each match takes one copy.
type LineCounts map[string]int

// Add adds one copy of each fingerprint.
func (c LineCounts) Add(fps ...string) {
	for _, fp := range fps {
		c[fp]++
	}
}

// Take removes one copy of fp, and reports whether there was one.
func (c LineCounts) Take(fp string) bool {
	if c[fp] == 0 {
		return false
	}
	c[fp]--
	if c[fp] == 0 {
		delete(c, fp)
	}

	return true
}

// Len returns the number of lines, counting each copy.
func (c LineCounts) Len() int {
	n := 0
	for _, k := range c {
		n += k
	}

	return n
}

// Clone returns a copy that does not share counts with c.
func (c LineCounts) Clone() LineCounts {
	out := make(LineCounts, len(c))
	for fp, k := range c {
		out[fp] = k
	}

	return out
}

// Missing returns the lines of c that present does not hold, one entry per
// copy, sorted. present is not changed.
func (c LineCounts) Missing(present LineCounts) []string {
	var out []string
	for fp, k := range c {
		for range k - present[fp] {
			out = append(out, fp)
		}
	}
	sort.Strings(out)

	return out
}

// LineSet lists the fingerprints of lines that a change added and removed.
type LineSet struct {
	Added   []string `json:"added,omitempty"`
	Removed []string `json:"removed,omitempty"`
}

// IsEmpty reports whether the set holds no line.
func (s LineSet) IsEmpty() bool {
	return len(s.Added) == 0 && len(s.Removed) == 0
}

// OpenLines are the lines of one file that a session's edits added or
// removed and that no commit has taken yet. While a file has open lines, a
// commit of the file can be credited to the session.
type OpenLines struct {
	Added   LineCounts
	Removed LineCounts
}

// isEmpty reports whether no line is open.
func (o *OpenLines) isEmpty() bool {
	return o == nil || (len(o.Added) == 0 && len(o.Removed) == 0)
}

// EditRecord is one agent edit of a file, as the ledger records it.
type EditRecord struct {
	// Ranges are the line ranges that the edit wrote in the new content.
	Ranges []aicodingsession.LineRange
	// Lines are the fingerprints of the lines that the edit added and
	// removed, without the lines that it only re-indented or moved.
	Lines LineSet
	// AddedCount and RemovedCount are the lines that the edit added and
	// removed, as the line diff counts them (R-011).
	AddedCount, RemovedCount int
}
