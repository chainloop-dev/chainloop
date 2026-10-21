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
	"strings"
	"testing"

	"github.com/chainloop-dev/chainloop/app/cli/internal/trace"
	tracegit "github.com/chainloop-dev/chainloop/app/cli/internal/trace/git"
	"github.com/chainloop-dev/chainloop/app/cli/internal/trace/state"
	"github.com/chainloop-dev/chainloop/pkg/attestation/crafter/materials/aicodingsession"
	"github.com/stretchr/testify/assert"
)

const (
	testSession  = "sess"
	otherSession = "other"
	testFile     = "a.go"
)

// legacyLedger is the ledger of a session whose edits were recorded before
// line fingerprints existed: ranges only.
func legacyLedger(sessionID string, files map[string][]aicodingsession.LineRange) *state.AILineAttribution {
	attr := &state.AILineAttribution{SessionID: sessionID, Files: files, Legacy: map[string]bool{}}
	for f := range files {
		attr.Legacy[f] = true
	}

	return attr
}

// ledger is the ledger of a session that wrote and erased the given lines
// of one file.
func ledger(sessionID, file string, written, erased []string) *state.AILineAttribution {
	w, e := state.LineCounts{}, state.LineCounts{}
	w.Add(fps(written...)...)
	e.Add(fps(erased...)...)

	return &state.AILineAttribution{
		SessionID: sessionID,
		Files:     map[string][]aicodingsession.LineRange{file: nil},
		Written:   map[string]state.LineCounts{file: w},
		Erased:    map[string]state.LineCounts{file: e},
	}
}

// oldFile is the path of testFile before a rename.
const oldFile = "old.go"

// counts holds the fingerprints of lines.
func counts(lines ...string) state.LineCounts {
	c := state.LineCounts{}
	c.Add(fps(lines...)...)

	return c
}

func TestEnrich_Legacy(t *testing.T) {
	t.Run("no AI data attributes all changes to human", func(t *testing.T) {
		changes := &aicodingsession.CodeChanges{
			LinesAdded:   10,
			LinesRemoved: 4,
			Files: []aicodingsession.FileChange{
				{Path: testFile, LinesAdded: 10, LinesRemoved: 4},
			},
		}

		Enrich(testSession, nil, nil, nil, changes)

		assert.Equal(t, 10, changes.HumanLinesAdded)
		assert.Equal(t, 4, changes.HumanLinesRemoved)
		assert.Equal(t, 0, changes.AILinesAdded)
		assert.Equal(t, 0, changes.AILinesRemoved)
		assert.Equal(t, trace.AttributionHuman, changes.Files[0].Attribution)
		assert.Equal(t, 10, changes.Files[0].HumanLinesAdded)
		assert.Equal(t, 4, changes.Files[0].HumanLinesRemoved)
	})

	t.Run("shell-generated file recorded as AI is not counted as human (PFM-6684)", func(t *testing.T) {
		// Regression: a 177,764-line fixture the agent produced via a shell
		// command, once recorded in the AI-line map (whole-file range), must be
		// attributed to AI — not dumped into the human bucket where it drowns
		// the small edits and yields a misleading 0% AI.
		ledgers := map[string]*state.AILineAttribution{testSession: legacyLedger(testSession, map[string][]aicodingsession.LineRange{
			"policies/foo.rego": {{Start: 1, End: 222}},
			"policies/integration-tests/materials/sbom.cyclonedx.json": {{Start: 1, End: 177764}},
		})}
		changes := &aicodingsession.CodeChanges{
			LinesAdded: 177986,
			Files: []aicodingsession.FileChange{
				{Path: "policies/foo.rego", Status: "modified", LinesAdded: 222},
				{Path: "policies/integration-tests/materials/sbom.cyclonedx.json", Status: "created", LinesAdded: 177764},
			},
		}

		Enrich(testSession, ledgers, nil, nil, changes)

		assert.Equal(t, trace.AttributionAI, changes.Files[0].Attribution)
		assert.Equal(t, trace.AttributionAI, changes.Files[1].Attribution)
		assert.Equal(t, 177986, changes.AILinesAdded)
		assert.Equal(t, 0, changes.HumanLinesAdded)
	})

	t.Run("file with AI ranges is attributed to AI", func(t *testing.T) {
		ledgers := map[string]*state.AILineAttribution{testSession: legacyLedger(testSession, map[string][]aicodingsession.LineRange{
			testFile: {{Start: 1, End: 5}},
		})}
		changes := &aicodingsession.CodeChanges{
			LinesAdded:   5,
			LinesRemoved: 2,
			Files: []aicodingsession.FileChange{
				{Path: testFile, LinesAdded: 5, LinesRemoved: 2},
			},
		}

		Enrich(testSession, ledgers, nil, nil, changes)

		assert.Equal(t, trace.AttributionAI, changes.Files[0].Attribution)
		assert.Equal(t, []string{testSession}, changes.Files[0].SessionIDs)
		assert.Equal(t, []aicodingsession.LineRange{{Start: 1, End: 5}}, changes.Files[0].LineRanges,
			"a file with no blame keeps the ranges at edit time, with no attribution (R-005)")
		assert.Equal(t, 5, changes.AILinesAdded)
		assert.Equal(t, 2, changes.AILinesRemoved)
		assert.Equal(t, 0, changes.HumanLinesAdded)
		assert.Equal(t, 0, changes.HumanLinesRemoved)
		assert.Equal(t, 5, changes.Files[0].AILinesAdded)
		assert.Equal(t, 2, changes.Files[0].AILinesRemoved)
	})

	t.Run("deletion-only file with empty ranges is attributed to AI", func(t *testing.T) {
		// AI removed lines from a file — there are no "after" ranges, but
		// the file is still listed in the session's attribution map.
		ledgers := map[string]*state.AILineAttribution{testSession: legacyLedger(testSession, map[string][]aicodingsession.LineRange{
			testFile: nil,
		})}
		changes := &aicodingsession.CodeChanges{
			LinesAdded:   0,
			LinesRemoved: 4,
			Files: []aicodingsession.FileChange{
				{Path: testFile, LinesAdded: 0, LinesRemoved: 4},
			},
		}

		Enrich(testSession, ledgers, nil, nil, changes)

		assert.Equal(t, trace.AttributionAI, changes.Files[0].Attribution)
		assert.Equal(t, []string{testSession}, changes.Files[0].SessionIDs)
		assert.Equal(t, 0, changes.AILinesAdded)
		assert.Equal(t, 4, changes.AILinesRemoved)
		assert.Equal(t, 0, changes.HumanLinesAdded)
		assert.Equal(t, 0, changes.HumanLinesRemoved)
	})

	t.Run("files not in the ledger are attributed to human", func(t *testing.T) {
		ledgers := map[string]*state.AILineAttribution{testSession: legacyLedger(testSession, map[string][]aicodingsession.LineRange{
			testFile: {{Start: 1, End: 2}},
		})}
		changes := &aicodingsession.CodeChanges{
			LinesAdded:   6,
			LinesRemoved: 3,
			Files: []aicodingsession.FileChange{
				{Path: testFile, LinesAdded: 2, LinesRemoved: 1},
				{Path: "b.go", LinesAdded: 4, LinesRemoved: 2},
			},
		}

		Enrich(testSession, ledgers, nil, nil, changes)

		assert.Equal(t, trace.AttributionAI, changes.Files[0].Attribution)
		assert.Equal(t, trace.AttributionHuman, changes.Files[1].Attribution)
		assert.Equal(t, 2, changes.AILinesAdded)
		assert.Equal(t, 1, changes.AILinesRemoved)
		assert.Equal(t, 4, changes.HumanLinesAdded)
		assert.Equal(t, 2, changes.HumanLinesRemoved)
	})
}

func TestEnrich_GeneratedLines(t *testing.T) {
	withGenerated := func(attr *state.AILineAttribution, added, removed int) *state.AILineAttribution {
		attr.GeneratedAdded, attr.GeneratedRemoved = added, removed

		return attr
	}

	testCases := []struct {
		name                   string
		ledgers                map[string]*state.AILineAttribution
		wantAdded, wantRemoved int
	}{
		{
			// The agent wrote 10 lines and removed 3. Only 2 lines reached
			// the commit; the session still reports all of them (spec
			// issue-3639, D-011).
			name: "reports all the lines of the agent edits, also the ones that no commit holds",
			ledgers: map[string]*state.AILineAttribution{
				testSession: withGenerated(ledger(testSession, testFile, []string{"a", "b"}, nil), 10, 3),
			},
			wantAdded:   10,
			wantRemoved: 3,
		},
		{
			name: "does not add the lines of another session",
			ledgers: map[string]*state.AILineAttribution{
				testSession:  withGenerated(ledger(testSession, testFile, []string{"a", "b"}, nil), 2, 0),
				otherSession: withGenerated(ledger(otherSession, testFile, []string{"c"}, nil), 7, 1),
			},
			wantAdded: 2,
		},
		{
			name:    "a session with no ledger reports no lines",
			ledgers: nil,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			changes := &aicodingsession.CodeChanges{
				LinesAdded: 2,
				Files:      []aicodingsession.FileChange{{Path: testFile, LinesAdded: 2}},
			}
			commits := []CommitLines{{SessionIDs: []string{testSession}, Files: []tracegit.FileLines{
				{Path: testFile, Added: []string{"a", "b"}},
			}}}

			Enrich(testSession, tc.ledgers, commits, nil, changes)

			assert.Equal(t, tc.wantAdded, changes.AIGeneratedLinesAdded)
			assert.Equal(t, tc.wantRemoved, changes.AIGeneratedLinesRemoved)
		})
	}
}

func TestEnrich_Lines(t *testing.T) {
	type split struct{ aiAdded, aiRemoved, humanAdded, humanRemoved int }

	testCases := []struct {
		name    string
		ledgers map[string]*state.AILineAttribution
		commits []CommitLines
		want    split
	}{
		{
			// R-001: the AI writes five lines, and a human changes one of them
			// before the commit.
			name:    "a human change to an AI line is human",
			ledgers: map[string]*state.AILineAttribution{testSession: ledger(testSession, testFile, []string{"a", "b", "c", "d", "e"}, nil)},
			commits: []CommitLines{{SessionIDs: []string{testSession}, Files: []tracegit.FileLines{
				{Path: testFile, Added: []string{"a", "b", "C", "d", "e"}},
			}}},
			want: split{aiAdded: 4, humanAdded: 1},
		},
		{
			name:    "a re-indented AI line stays AI",
			ledgers: map[string]*state.AILineAttribution{testSession: ledger(testSession, testFile, []string{"\tfoo()"}, nil)},
			commits: []CommitLines{{SessionIDs: []string{testSession}, Files: []tracegit.FileLines{
				{Path: testFile, Added: []string{"    foo()"}},
			}}},
			want: split{aiAdded: 1},
		},
		{
			name:    "a repeated line counts as AI only as often as the AI wrote it",
			ledgers: map[string]*state.AILineAttribution{testSession: ledger(testSession, testFile, []string{"}", "}"}, nil)},
			commits: []CommitLines{{SessionIDs: []string{testSession}, Files: []tracegit.FileLines{
				{Path: testFile, Added: []string{"}", "}", "}"}},
			}}},
			want: split{aiAdded: 2, humanAdded: 1},
		},
		{
			// R-005: only the lines that an agent edit removed are AI removals.
			name:    "removals count for the remover",
			ledgers: map[string]*state.AILineAttribution{testSession: ledger(testSession, testFile, nil, []string{"x"})},
			commits: []CommitLines{{SessionIDs: []string{testSession}, Files: []tracegit.FileLines{
				{Path: testFile, Removed: []string{"x", "y"}},
			}}},
			want: split{aiRemoved: 1, humanRemoved: 1},
		},
		{
			name: "lines of another session of the commit count for neither",
			ledgers: map[string]*state.AILineAttribution{
				testSession:  ledger(testSession, testFile, []string{"a"}, nil),
				otherSession: ledger(otherSession, testFile, []string{"b"}, []string{"x"}),
			},
			commits: []CommitLines{{SessionIDs: []string{testSession, otherSession}, Files: []tracegit.FileLines{
				{Path: testFile, Added: []string{"a", "b", "c"}, Removed: []string{"x"}},
			}}},
			want: split{aiAdded: 1, humanAdded: 1},
		},
		{
			name: "a session not named by the commit does not own its lines",
			ledgers: map[string]*state.AILineAttribution{
				testSession:  ledger(testSession, testFile, []string{"a"}, nil),
				otherSession: ledger(otherSession, testFile, []string{"b"}, nil),
			},
			commits: []CommitLines{{SessionIDs: []string{testSession}, Files: []tracegit.FileLines{
				{Path: testFile, Added: []string{"a", "b"}},
			}}},
			want: split{aiAdded: 1, humanAdded: 1},
		},
		{
			// Each commit of the session adds to the split. Milestone 2 of the
			// spec replaces this sum with the net contribution (R-003).
			name:    "commits of the session add up",
			ledgers: map[string]*state.AILineAttribution{testSession: ledger(testSession, testFile, []string{"a", "b"}, nil)},
			commits: []CommitLines{
				{SessionIDs: []string{testSession}, Files: []tracegit.FileLines{{Path: testFile, Added: []string{"a", "b"}}}},
				{SessionIDs: []string{testSession}, Files: []tracegit.FileLines{{Path: testFile, Added: []string{"B"}, Removed: []string{"b"}}}},
			},
			want: split{aiAdded: 2, humanAdded: 1, humanRemoved: 1},
		},
		{
			// R-007: the agent edited the file under its old name, and the
			// commit renamed it.
			name:    "a renamed file matches the ledger of its old path",
			ledgers: map[string]*state.AILineAttribution{testSession: ledger(testSession, oldFile, []string{"a"}, []string{"x"})},
			commits: []CommitLines{{SessionIDs: []string{testSession}, Files: []tracegit.FileLines{
				{Path: testFile, OldPath: oldFile, Added: []string{"a", "b"}, Removed: []string{"x", "y"}},
			}}},
			want: split{aiAdded: 1, aiRemoved: 1, humanAdded: 1, humanRemoved: 1},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			changes := &aicodingsession.CodeChanges{Files: []aicodingsession.FileChange{{Path: testFile}}}

			Enrich(testSession, tc.ledgers, tc.commits, nil, changes)

			f := changes.Files[0]
			assert.Equal(t, tc.want, split{f.AILinesAdded, f.AILinesRemoved, f.HumanLinesAdded, f.HumanLinesRemoved}, "file split")
			assert.Equal(t, tc.want, split{changes.AILinesAdded, changes.AILinesRemoved, changes.HumanLinesAdded, changes.HumanLinesRemoved},
				"session totals are the sum of the files")
			assert.Equal(t, trace.AttributionAI, f.Attribution, "the marker for older platforms still names the files that the session edited")
			assert.Equal(t, []string{testSession}, f.SessionIDs)
		})
	}
}

func TestEnrich_Net(t *testing.T) {
	type split struct{ aiAdded, aiRemoved, humanAdded, humanRemoved int }

	const c1, c2, c3 = "c1", "c2", "c3"
	tipOf := func(lines ...string) []tracegit.BlameLine {
		// Each entry is "sha:text".
		out := make([]tracegit.BlameLine, 0, len(lines))
		for _, l := range lines {
			sha, text, _ := strings.Cut(l, ":")
			out = append(out, tracegit.BlameLine{SHA: sha, Text: text})
		}

		return out
	}
	five := []string{"a", "b", "c", "d", "e"}
	ai := func(start, end int) aicodingsession.LineRange {
		return aicodingsession.LineRange{Start: start, End: end, Attribution: trace.AttributionAI}
	}
	human := func(start, end int) aicodingsession.LineRange {
		return aicodingsession.LineRange{Start: start, End: end, Attribution: trace.AttributionHuman}
	}
	ownCommit := func(sha string, added, removed []string) CommitLines {
		return CommitLines{SHA: sha, SessionIDs: []string{testSession}, Files: []tracegit.FileLines{{Path: testFile, Added: added, Removed: removed}}}
	}

	testCases := []struct {
		name    string
		ledgers map[string]*state.AILineAttribution
		commits []CommitLines
		tip     map[string][]tracegit.BlameLine
		want    split
		// ranges are the positions of the session's lines at the pushed
		// commit (line ranges spec R-001, R-002).
		ranges []aicodingsession.LineRange
	}{
		{
			// Case 3 of the spec: a later commit of the session replaces an
			// AI line by hand. Each line counts once (R-003). The replaced
			// line is the session's own, so c2 does not hold it as removed
			// (sessionCommitLines leaves it out).
			name:    "a replaced line of the session counts once",
			ledgers: map[string]*state.AILineAttribution{testSession: ledger(testSession, testFile, five, nil)},
			commits: []CommitLines{
				ownCommit(c1, five, nil),
				ownCommit(c2, []string{"C"}, nil),
			},
			tip:    map[string][]tracegit.BlameLine{testFile: tipOf(c1+":a", c1+":b", c2+":C", c1+":d", c1+":e")},
			want:   split{aiAdded: 4, humanAdded: 1},
			ranges: []aicodingsession.LineRange{ai(1, 2), human(3, 3), ai(4, 5)},
		},
		{
			// Case 2 of the spec: a commit in no session replaces an AI line
			// and joins the session (R-010). Its replacing lines are human,
			// and its removals of the session's lines do not count again.
			name:    "a joined commit adds human lines of its replacing hunks",
			ledgers: map[string]*state.AILineAttribution{testSession: ledger(testSession, testFile, five, nil)},
			commits: []CommitLines{
				ownCommit(c1, five, nil),
				{SHA: c2, Files: []tracegit.FileLines{{Path: testFile, Added: []string{"C", "x"}, Removed: []string{"c"}}},
					Joined: map[string][]string{testFile: fps("C")}},
			},
			tip:    map[string][]tracegit.BlameLine{testFile: tipOf(c1+":a", c1+":b", c2+":C", c1+":d", c1+":e", c2+":x")},
			want:   split{aiAdded: 4, humanAdded: 1},
			ranges: []aicodingsession.LineRange{ai(1, 2), human(3, 3), ai(4, 5)},
		},
		{
			// Case 4 of the spec: a commit of another session replaces the
			// line. The session keeps only what is still there.
			name:    "lines that a later commit outside the session replaced are gone",
			ledgers: map[string]*state.AILineAttribution{testSession: ledger(testSession, testFile, five, nil)},
			commits: []CommitLines{ownCommit(c1, five, nil)},
			tip:     map[string][]tracegit.BlameLine{testFile: tipOf(c1+":a", c1+":b", c3+":C", c1+":d", c1+":e")},
			want:    split{aiAdded: 4},
			ranges:  []aicodingsession.LineRange{ai(1, 2), ai(4, 5)},
		},
		{
			name: "a line of another session of the commit counts for neither",
			ledgers: map[string]*state.AILineAttribution{
				testSession:  ledger(testSession, testFile, []string{"a"}, nil),
				otherSession: ledger(otherSession, testFile, []string{"b"}, nil),
			},
			commits: []CommitLines{{SHA: c1, SessionIDs: []string{testSession, otherSession},
				Files: []tracegit.FileLines{{Path: testFile, Added: []string{"a", "b", "h"}}}}},
			tip:    map[string][]tracegit.BlameLine{testFile: tipOf(c1+":a", c1+":b", c1+":h")},
			want:   split{aiAdded: 1, humanAdded: 1},
			ranges: []aicodingsession.LineRange{ai(1, 1), human(3, 3)},
		},
		{
			name:    "a file that the pushed commit does not hold adds nothing",
			ledgers: map[string]*state.AILineAttribution{testSession: ledger(testSession, testFile, five, []string{"z"})},
			commits: []CommitLines{
				ownCommit(c1, five, nil),
				ownCommit(c2, nil, append([]string{"z"}, five...)),
			},
			tip:  map[string][]tracegit.BlameLine{testFile: nil},
			want: split{aiRemoved: 1, humanRemoved: 5},
		},
		{
			// D-003: the human lines of the session get ranges too, also in a
			// file that no agent edit of the session touched.
			name:    "a file that only a human changed in the session has human ranges",
			ledgers: map[string]*state.AILineAttribution{testSession: ledger(testSession, "b.go", nil, nil)},
			commits: []CommitLines{ownCommit(c1, []string{"x", "y"}, nil)},
			tip:     map[string][]tracegit.BlameLine{testFile: tipOf(c3+":k", c1+":x", c1+":y")},
			want:    split{humanAdded: 2},
			ranges:  []aicodingsession.LineRange{human(2, 3)},
		},
		{
			// R-007: the session wrote old.go, a commit renamed it to the
			// file, and a later commit of the session changed the file. The
			// tip of both paths is the file at HEAD, and each line counts for
			// the path that its commit changed.
			name: "a line counts for the path that its commit changed",
			ledgers: map[string]*state.AILineAttribution{testSession: {
				SessionID: testSession,
				Written: map[string]state.LineCounts{
					oldFile:  counts(five...),
					testFile: counts("f"),
				},
			}},
			commits: []CommitLines{
				{SHA: c1, SessionIDs: []string{testSession}, Files: []tracegit.FileLines{{Path: oldFile, Added: five}}},
				ownCommit(c2, []string{"f"}, nil),
			},
			tip: map[string][]tracegit.BlameLine{
				oldFile:  tipOf(c1+":a", c1+":b", c1+":c", c1+":d", c1+":e", c2+":f"),
				testFile: tipOf(c1+":a", c1+":b", c1+":c", c1+":d", c1+":e", c2+":f"),
			},
			want:   split{aiAdded: 1},
			ranges: []aicodingsession.LineRange{ai(6, 6)},
		},
		{
			// R-007: a commit in no session renamed the file and replaced a
			// line of the session in it.
			name:    "a joined commit counts in a renamed file",
			ledgers: map[string]*state.AILineAttribution{testSession: ledger(testSession, testFile, five, nil)},
			commits: []CommitLines{
				ownCommit(c1, five, nil),
				{SHA: c2, Files: []tracegit.FileLines{{Path: "new.go", OldPath: testFile, Added: []string{"C"}, Removed: []string{"c"}}},
					Joined: map[string][]string{"new.go": fps("C")}},
			},
			tip:    map[string][]tracegit.BlameLine{testFile: tipOf(c1+":a", c1+":b", c2+":C", c1+":d", c1+":e")},
			want:   split{aiAdded: 4, humanAdded: 1},
			ranges: []aicodingsession.LineRange{ai(1, 2), human(3, 3), ai(4, 5)},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			changes := &aicodingsession.CodeChanges{Files: []aicodingsession.FileChange{{Path: testFile}}}

			Enrich(testSession, tc.ledgers, tc.commits, tc.tip, changes)

			f := changes.Files[0]
			assert.Equal(t, tc.want, split{f.AILinesAdded, f.AILinesRemoved, f.HumanLinesAdded, f.HumanLinesRemoved})
			assert.Equal(t, tc.want, split{changes.AILinesAdded, changes.AILinesRemoved, changes.HumanLinesAdded, changes.HumanLinesRemoved})
			assert.Equal(t, tc.ranges, f.LineRanges)

			// The line counts of the file and of the session are the sum of
			// the split (issue 3674).
			assert.Equal(t, tc.want.aiAdded+tc.want.humanAdded, f.LinesAdded, "lines added of the file")
			assert.Equal(t, tc.want.aiRemoved+tc.want.humanRemoved, f.LinesRemoved, "lines removed of the file")
			assert.Equal(t, f.LinesAdded, changes.LinesAdded, "lines added of the session")
			assert.Equal(t, f.LinesRemoved, changes.LinesRemoved, "lines removed of the session")

			// R-003: the ranges hold as many lines as the net added lines.
			covered := map[string]int{}
			for _, r := range f.LineRanges {
				covered[r.Attribution] += r.End - r.Start + 1
			}
			assert.Equal(t, f.AILinesAdded, covered[trace.AttributionAI], "AI ranges agree with the AI count")
			assert.Equal(t, f.HumanLinesAdded, covered[trace.AttributionHuman], "human ranges agree with the human count")
		})
	}
}

func TestSettleCommit(t *testing.T) {
	open := func(added, removed []string) *state.OpenLines {
		o := &state.OpenLines{Added: state.LineCounts{}, Removed: state.LineCounts{}}
		o.Added.Add(fps(added...)...)
		o.Removed.Add(fps(removed...)...)

		return o
	}

	testCases := []struct {
		name      string
		open      *state.OpenLines
		committed tracegit.FileLines
		worktree  string
		clean     bool
		want      state.LineSet
	}{
		{
			name:      "the commit takes the open lines it holds (R-006)",
			open:      open([]string{"a", "b"}, []string{"x", "y"}),
			committed: tracegit.FileLines{Added: []string{"a", "h"}, Removed: []string{"x"}},
			worktree:  "a\nb\n",
			want:      state.LineSet{Added: fps("a"), Removed: fps("x")},
		},
		{
			// R-002: a human changed b before the commit. The line is gone, so
			// it cannot reach a later commit.
			name:      "open lines gone from the worktree close",
			open:      open([]string{"a", "b"}, nil),
			committed: tracegit.FileLines{Added: []string{"a", "B"}},
			worktree:  "a\nB\n",
			want:      state.LineSet{Added: fps("a", "b")},
		},
		{
			name:      "a fully committed file closes all its open lines",
			open:      open([]string{"a", "b"}, []string{"x"}),
			committed: tracegit.FileLines{Added: []string{"a"}},
			worktree:  "a\nb\n",
			clean:     true,
			want:      state.LineSet{Added: fps("a", "b"), Removed: fps("x")},
		},
		{
			name:      "nothing open",
			committed: tracegit.FileLines{Added: []string{"a"}},
			worktree:  "a\n",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			got := SettleCommit(tc.open, tc.committed, []byte(tc.worktree), tc.clean)
			assert.ElementsMatch(t, tc.want.Added, got.Added)
			assert.ElementsMatch(t, tc.want.Removed, got.Removed)
		})
	}
}

func TestDropOpen(t *testing.T) {
	o := &state.OpenLines{Added: state.LineCounts{}, Removed: state.LineCounts{}}
	o.Added.Add(fps("a", "b")...)
	o.Removed.Add(fps("x")...)

	got := DropOpen(Edit{
		Changed:      true,
		Added:        fps("a", "b", "c"),
		Removed:      fps("x", "y"),
		AddedCount:   3,
		RemovedCount: 2,
	}, o)

	assert.ElementsMatch(t, fps("c"), got.Added)
	assert.ElementsMatch(t, fps("y"), got.Removed)
	assert.Equal(t, 1, got.AddedCount, "lines recorded before are not generated again")
	assert.Equal(t, 1, got.RemovedCount)
	assert.True(t, got.Changed)
	assert.Equal(t, 2, o.Added.Len(), "the open lines are not changed")

	assert.Equal(t, fps("a"), DropOpen(Edit{Added: fps("a")}, nil).Added)
}

func TestReplacedLines(t *testing.T) {
	o := &state.OpenLines{Added: state.LineCounts{}, Removed: state.LineCounts{}}
	o.Added.Add(fps("a", "b", "b")...)

	assert.ElementsMatch(t, fps("b"), ReplacedLines(o, []byte("a\nb\nB\n")).Added)
	assert.Empty(t, ReplacedLines(o, []byte("b\nb\na\n")).Added)
	assert.True(t, ReplacedLines(nil, []byte("a\n")).IsEmpty())
}
