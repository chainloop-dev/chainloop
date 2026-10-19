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
	"bytes"

	"github.com/chainloop-dev/chainloop/pkg/attestation/crafter/materials/aicodingsession"
	"github.com/sergi/go-diff/diffmatchpatch"
)

// ComputeLineRanges computes which line ranges in "after" are new or modified
// compared to "before". Returns 1-indexed, inclusive line ranges plus a
// `changed` flag that is true whenever before and after differ — including
// deletion-only changes where no "after" ranges exist.
// If before is nil, the entire file is considered new.
func ComputeLineRanges(before, after []byte) ([]aicodingsession.LineRange, bool) {
	edit := ComputeEdit(before, after)

	return edit.Ranges, edit.Changed
}

// Edit is what one change did to a file: the line ranges it wrote in the new
// content, and the lines it added and removed.
type Edit struct {
	// Changed is true whenever the content differs, including a change that
	// only removes lines.
	Changed bool
	// Ranges are the 1-indexed, inclusive line ranges of the new content that
	// the change wrote.
	Ranges []aicodingsession.LineRange
	// AddedCount and RemovedCount are the lines the change added and removed,
	// counted as the line diff shows them.
	AddedCount, RemovedCount int
	// Added and Removed are the fingerprints of the lines the change added and
	// removed. A line that the change only re-indented or moved shows as one
	// removal and one addition with the same fingerprint; such pairs are left
	// out, because the change wrote no new code on that line (D-008).
	Added, Removed []string
}

// ComputeEdit diffs the content of a file before and after a change. A nil
// before means the file is new, and a nil or empty after means the change
// removed all of its lines.
func ComputeEdit(before, after []byte) Edit {
	if bytes.Equal(before, after) {
		return Edit{}
	}

	edit := Edit{Changed: true}

	dmp := diffmatchpatch.New()
	beforeChars, afterChars, lines := dmp.DiffLinesToChars(string(before), string(after))
	diffs := dmp.DiffMain(beforeChars, afterChars, false)
	// Clean up while each character still stands for a whole line. On the
	// expanded text the cleanup factors out common prefixes and suffixes, and
	// a diff entry could then hold part of a line.
	diffs = dmp.DiffCleanupSemantic(diffs)
	diffs = dmp.DiffCharsToLines(diffs, lines)

	var ranges []aicodingsession.LineRange
	afterLine := 1

	for _, d := range diffs {
		text := splitLines(d.Text)

		switch d.Type {
		case diffmatchpatch.DiffEqual:
			afterLine += len(text)
		case diffmatchpatch.DiffInsert:
			if len(text) > 0 {
				ranges = append(ranges, aicodingsession.LineRange{
					Start: afterLine,
					End:   afterLine + len(text) - 1,
				})
				afterLine += len(text)
			}
			edit.AddedCount += len(text)
			for _, l := range text {
				edit.Added = append(edit.Added, Fingerprint(l))
			}
		case diffmatchpatch.DiffDelete:
			// Lines removed from "before" — don't advance afterLine.
			edit.RemovedCount += len(text)
			for _, l := range text {
				edit.Removed = append(edit.Removed, Fingerprint(l))
			}
		}
	}

	edit.Ranges = mergeAdjacentRanges(ranges)
	edit.Added, edit.Removed = dropCommon(edit.Added, edit.Removed)

	return edit
}

// dropCommon removes from a and b each fingerprint that both hold, one
// occurrence for one occurrence.
func dropCommon(a, b []string) ([]string, []string) {
	if len(a) == 0 || len(b) == 0 {
		return a, b
	}

	inB := make(map[string]int, len(b))
	for _, fp := range b {
		inB[fp]++
	}

	common := make(map[string]int)
	keptA := a[:0]
	for _, fp := range a {
		if inB[fp] > 0 {
			inB[fp]--
			common[fp]++
			continue
		}
		keptA = append(keptA, fp)
	}

	keptB := b[:0]
	for _, fp := range b {
		if common[fp] > 0 {
			common[fp]--
			continue
		}
		keptB = append(keptB, fp)
	}

	return keptA, keptB
}

// mergeAdjacentRanges merges overlapping or adjacent line ranges.
func mergeAdjacentRanges(ranges []aicodingsession.LineRange) []aicodingsession.LineRange {
	if len(ranges) <= 1 {
		return ranges
	}

	merged := []aicodingsession.LineRange{ranges[0]}
	for _, r := range ranges[1:] {
		last := &merged[len(merged)-1]
		if r.Start <= last.End+1 {
			if r.End > last.End {
				last.End = r.End
			}
		} else {
			merged = append(merged, r)
		}
	}

	return merged
}
