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

package e2e

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func cell(feature, model string, expected Support, status Status) Cell {
	return Cell{Feature: feature, Harness: HarnessClaudeCode, Model: model, Expected: expected, Status: status}
}

func TestCompare(t *testing.T) {
	testCases := []struct {
		name             string
		prev             []Cell
		cur              []Cell
		wantRegressions  []string
		wantImprovements []string
	}{
		{
			name:            "pass to fail is a regression",
			prev:            []Cell{cell(featTranscript, testModel, SupportYes, StatusPass)},
			cur:             []Cell{cell(featTranscript, testModel, SupportYes, StatusFail)},
			wantRegressions: []string{featTranscript},
		},
		{
			name: "fail twice in a row is not reported again",
			prev: []Cell{cell(featTranscript, testModel, SupportYes, StatusFail)},
			cur:  []Cell{cell(featTranscript, testModel, SupportYes, StatusFail)},
		},
		{
			name: "same feature, other model, is another cell",
			prev: []Cell{cell(featTranscript, testModel, SupportYes, StatusPass)},
			cur:  []Cell{cell(featTranscript, "sonnet", SupportYes, StatusFail)},
		},
		{
			name:             "unknown cell that passes is an improvement",
			prev:             []Cell{cell("Media", testModel, SupportUnknown, StatusFail)},
			cur:              []Cell{cell("Media", testModel, SupportUnknown, StatusPass)},
			wantImprovements: []string{"Media"},
		},
		{
			name:             "unknown cell that passes with no previous run is an improvement",
			cur:              []Cell{cell("Media", testModel, SupportUnknown, StatusPass)},
			wantImprovements: []string{"Media"},
		},
		{
			name: "unknown cell that passed before is not reported again",
			prev: []Cell{cell("Media", testModel, SupportUnknown, StatusPass)},
			cur:  []Cell{cell("Media", testModel, SupportUnknown, StatusPass)},
		},
		{
			name: "pass to not covered is not a regression",
			prev: []Cell{cell("Welcome message", testModel, SupportYes, StatusPass)},
			cur:  []Cell{cell("Welcome message", testModel, SupportYes, StatusNotCovered)},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var prev *Result
			if tc.prev != nil {
				prev = &Result{Cells: tc.prev}
			}
			changes := Compare(prev, &Result{Cells: tc.cur})

			assert.ElementsMatch(t, tc.wantRegressions, changeFeatures(changes.Regressions))
			assert.ElementsMatch(t, tc.wantImprovements, changeFeatures(changes.Improvements))
		})
	}
}

func changeFeatures(changes []Change) []string {
	out := make([]string, 0, len(changes))
	for _, c := range changes {
		out = append(out, c.Cell.Feature)
	}
	return out
}

func TestResultStore(t *testing.T) {
	started := time.Date(2026, 10, 16, 3, 0, 0, 0, time.UTC)

	testCases := []struct {
		name         string
		full         bool
		wantPrevious bool
	}{
		{name: "full run becomes the previous result", full: true, wantPrevious: true},
		{name: "filtered run does not change the previous result", full: false, wantPrevious: false},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			store := ResultStore{Dir: t.TempDir()}

			prev, err := store.Previous()
			require.NoError(t, err)
			assert.Nil(t, prev, "an empty store has no previous result")

			res := &Result{
				StartedAt: started,
				Full:      tc.full,
				Cells:     []Cell{cell(featTranscript, testModel, SupportYes, StatusPass)},
			}
			path, err := store.Save(res)
			require.NoError(t, err)
			assert.FileExists(t, path, "every run is kept in the history")
			assert.Equal(t, filepath.Join(store.Dir, "runs", "20261016T030000Z.json"), path)

			prev, err = store.Previous()
			require.NoError(t, err)
			if !tc.wantPrevious {
				assert.Nil(t, prev)
				return
			}
			require.NotNil(t, prev)
			assert.Equal(t, resultSchemaVersion, prev.SchemaVersion)
			assert.Equal(t, res.Cells, prev.Cells)
		})
	}

	t.Run("unreadable previous result is an error", func(t *testing.T) {
		store := ResultStore{Dir: t.TempDir()}
		require.NoError(t, os.WriteFile(filepath.Join(store.Dir, latestResultFile), []byte("{"), 0o600))
		_, err := store.Previous()
		assert.Error(t, err)
	})
}

func TestMarkdown(t *testing.T) {
	res := &Result{
		CLIVersion: "v1.2.3",
		Cells: []Cell{
			{Feature: featTranscript, Harness: HarnessClaudeCode, HarnessVersion: testHarnessVersion, Model: testModel, Status: StatusPass, Passes: 1, Runs: 1},
			{Feature: "Spec capture instruction", Harness: HarnessClaudeCode, HarnessVersion: testHarnessVersion, Model: testModel, Status: StatusFail, Passes: 1, Runs: 3},
			{Feature: "Session link after push", Harness: HarnessClaudeCode, HarnessVersion: testHarnessVersion, Model: testModel, Status: StatusNotCovered},
			{Feature: featTranscript, Harness: colCursor, Model: modelAuto, Status: StatusSkipped},
		},
	}

	want := `| Feature | Claude Code 2.1.0 / haiku | Cursor / auto |
|---|---|---|
| Transcript | pass | skipped |
| Spec capture instruction | fail 1/3 |  |
| Session link after push | not covered |  |
`
	assert.Equal(t, want, Markdown(res))
}

func TestProblems(t *testing.T) {
	res := &Result{Cells: []Cell{
		cell(featTranscript, testModel, SupportYes, StatusPass),
		cell("Subagents", testModel, SupportYes, StatusNoCheck),
		cell("Skills tracking", testModel, SupportYes, StatusError),
	}}
	got := res.Problems()
	require.Len(t, got, 2)
	assert.Equal(t, "Subagents", got[0].Feature)
	assert.Equal(t, "Skills tracking", got[1].Feature)
}
