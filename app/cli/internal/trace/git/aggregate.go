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

package git

import (
	"sort"

	"github.com/chainloop-dev/chainloop/pkg/attestation/crafter/materials/aicodingsession"
)

// fileAgg is the running per-path total used while accumulating diffs from a
// set of commits. Backends share it so the aggregation rule (sum lines,
// last-wins status) lives in one place.
type fileAgg struct {
	linesAdded   int
	linesRemoved int
	status       string
}

// addCommitStats folds one commit's diff into the running totals.
func addCommitStats(out *aicodingsession.CodeChanges, byPath map[string]*fileAgg, perCommit *aicodingsession.CodeChanges) {
	out.LinesAdded += perCommit.LinesAdded
	out.LinesRemoved += perCommit.LinesRemoved

	for _, f := range perCommit.Files {
		agg, ok := byPath[f.Path]
		if !ok {
			agg = &fileAgg{}
			byPath[f.Path] = agg
		}
		agg.linesAdded += f.LinesAdded
		agg.linesRemoved += f.LinesRemoved
		if f.Status != "" {
			agg.status = f.Status
		}
	}
}

// finalizeAggregatedFiles converts the running per-path map into a sorted
// Files slice and per-status counters on out.
func finalizeAggregatedFiles(out *aicodingsession.CodeChanges, byPath map[string]*fileAgg) {
	paths := make([]string, 0, len(byPath))
	for p := range byPath {
		paths = append(paths, p)
	}
	sort.Strings(paths)

	for _, p := range paths {
		agg := byPath[p]
		out.Files = append(out.Files, aicodingsession.FileChange{
			Path:         p,
			Status:       agg.status,
			LinesAdded:   agg.linesAdded,
			LinesRemoved: agg.linesRemoved,
		})
		switch agg.status {
		case StatusCreated:
			out.FilesCreated++
		case StatusDeleted:
			out.FilesDeleted++
		default:
			out.FilesModified++
		}
	}
}
