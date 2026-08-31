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

	"github.com/chainloop-dev/chainloop/app/cli/internal/trace"
	"github.com/chainloop-dev/chainloop/pkg/attestation/crafter/materials/aicodingsession"
	"github.com/stretchr/testify/assert"
)

func TestEnrich(t *testing.T) {
	t.Run("no AI data attributes all changes to human", func(t *testing.T) {
		changes := &aicodingsession.CodeChanges{
			LinesAdded:   10,
			LinesRemoved: 4,
			Files: []aicodingsession.FileChange{
				{Path: "a.go", LinesAdded: 10, LinesRemoved: 4},
			},
		}

		Enrich("sess", nil, changes)

		assert.Equal(t, 10, changes.HumanLinesAdded)
		assert.Equal(t, 4, changes.HumanLinesRemoved)
		assert.Equal(t, 0, changes.AILinesAdded)
		assert.Equal(t, 0, changes.AILinesRemoved)
		assert.Equal(t, trace.AttributionHuman, changes.Files[0].Attribution)
	})

	t.Run("shell-generated file recorded as AI is not counted as human (PFM-6684)", func(t *testing.T) {
		// Regression: a 177,764-line fixture the agent produced via a shell
		// command, once recorded in the AI-line map (whole-file range), must be
		// attributed to AI — not dumped into the human bucket where it drowns
		// the small edits and yields a misleading 0% AI.
		aiFiles := map[string][]aicodingsession.LineRange{
			"policies/foo.rego": {{Start: 1, End: 222}},
			"policies/integration-tests/materials/sbom.cyclonedx.json": {{Start: 1, End: 177764}},
		}
		changes := &aicodingsession.CodeChanges{
			LinesAdded: 177986,
			Files: []aicodingsession.FileChange{
				{Path: "policies/foo.rego", Status: "modified", LinesAdded: 222},
				{Path: "policies/integration-tests/materials/sbom.cyclonedx.json", Status: "created", LinesAdded: 177764},
			},
		}

		Enrich("sess", aiFiles, changes)

		assert.Equal(t, trace.AttributionAI, changes.Files[0].Attribution)
		assert.Equal(t, trace.AttributionAI, changes.Files[1].Attribution)
		assert.Equal(t, 177986, changes.AILinesAdded)
		assert.Equal(t, 0, changes.HumanLinesAdded)
	})

	t.Run("file with AI ranges is attributed to AI", func(t *testing.T) {
		aiFiles := map[string][]aicodingsession.LineRange{
			"a.go": {{Start: 1, End: 5}},
		}
		changes := &aicodingsession.CodeChanges{
			LinesAdded:   5,
			LinesRemoved: 2,
			Files: []aicodingsession.FileChange{
				{Path: "a.go", LinesAdded: 5, LinesRemoved: 2},
			},
		}

		Enrich("sess", aiFiles, changes)

		assert.Equal(t, trace.AttributionAI, changes.Files[0].Attribution)
		assert.Equal(t, []string{"sess"}, changes.Files[0].SessionIDs)
		assert.Equal(t, 5, changes.AILinesAdded)
		assert.Equal(t, 2, changes.AILinesRemoved)
		assert.Equal(t, 0, changes.HumanLinesAdded)
		assert.Equal(t, 0, changes.HumanLinesRemoved)
	})

	t.Run("deletion-only file with empty ranges is attributed to AI", func(t *testing.T) {
		// AI removed lines from a file — there are no "after" ranges, but
		// the file is still listed in the session's attribution map.
		aiFiles := map[string][]aicodingsession.LineRange{
			"a.go": nil,
		}
		changes := &aicodingsession.CodeChanges{
			LinesAdded:   0,
			LinesRemoved: 4,
			Files: []aicodingsession.FileChange{
				{Path: "a.go", LinesAdded: 0, LinesRemoved: 4},
			},
		}

		Enrich("sess", aiFiles, changes)

		assert.Equal(t, trace.AttributionAI, changes.Files[0].Attribution)
		assert.Equal(t, []string{"sess"}, changes.Files[0].SessionIDs)
		assert.Equal(t, 0, changes.AILinesAdded)
		assert.Equal(t, 4, changes.AILinesRemoved)
		assert.Equal(t, 0, changes.HumanLinesAdded)
		assert.Equal(t, 0, changes.HumanLinesRemoved)
	})

	t.Run("files not in aiFiles are attributed to human", func(t *testing.T) {
		aiFiles := map[string][]aicodingsession.LineRange{
			"a.go": {{Start: 1, End: 2}},
		}
		changes := &aicodingsession.CodeChanges{
			LinesAdded:   6,
			LinesRemoved: 3,
			Files: []aicodingsession.FileChange{
				{Path: "a.go", LinesAdded: 2, LinesRemoved: 1},
				{Path: "b.go", LinesAdded: 4, LinesRemoved: 2},
			},
		}

		Enrich("sess", aiFiles, changes)

		assert.Equal(t, trace.AttributionAI, changes.Files[0].Attribution)
		assert.Equal(t, trace.AttributionHuman, changes.Files[1].Attribution)
		assert.Equal(t, 2, changes.AILinesAdded)
		assert.Equal(t, 1, changes.AILinesRemoved)
		assert.Equal(t, 4, changes.HumanLinesAdded)
		assert.Equal(t, 2, changes.HumanLinesRemoved)
	})
}
