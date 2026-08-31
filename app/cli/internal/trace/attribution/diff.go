package attribution

import (
	"bytes"
	"strings"

	"github.com/chainloop-dev/chainloop/pkg/attestation/crafter/materials/aicodingsession"
	"github.com/sergi/go-diff/diffmatchpatch"
)

// ComputeLineRanges computes which line ranges in "after" are new or modified
// compared to "before". Returns 1-indexed, inclusive line ranges plus a
// `changed` flag that is true whenever before and after differ — including
// deletion-only changes where no "after" ranges exist.
// If before is nil, the entire file is considered new.
func ComputeLineRanges(before, after []byte) ([]aicodingsession.LineRange, bool) {
	if bytes.Equal(before, after) {
		return nil, false
	}

	afterStr := string(after)
	if afterStr == "" {
		// Deletion-only: before had content, after is empty.
		return nil, true
	}

	if before == nil {
		lineCount := strings.Count(afterStr, "\n")
		if !strings.HasSuffix(afterStr, "\n") {
			lineCount++
		}
		return []aicodingsession.LineRange{{Start: 1, End: lineCount}}, true
	}

	beforeStr := string(before)
	dmp := diffmatchpatch.New()
	beforeChars, afterChars, lines := dmp.DiffLinesToChars(beforeStr, afterStr)
	diffs := dmp.DiffMain(beforeChars, afterChars, false)
	diffs = dmp.DiffCharsToLines(diffs, lines)
	diffs = dmp.DiffCleanupSemantic(diffs)

	var ranges []aicodingsession.LineRange
	afterLine := 1

	for _, d := range diffs {
		lineCount := strings.Count(d.Text, "\n")
		if len(d.Text) > 0 && !strings.HasSuffix(d.Text, "\n") {
			lineCount++
		}

		switch d.Type {
		case diffmatchpatch.DiffEqual:
			afterLine += lineCount
		case diffmatchpatch.DiffInsert:
			if lineCount > 0 {
				ranges = append(ranges, aicodingsession.LineRange{
					Start: afterLine,
					End:   afterLine + lineCount - 1,
				})
				afterLine += lineCount
			}
		case diffmatchpatch.DiffDelete:
			// Lines removed from "before" — don't advance afterLine.
		}
	}

	return mergeAdjacentRanges(ranges), true
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
