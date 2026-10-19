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
	"github.com/chainloop-dev/chainloop/app/cli/internal/trace"
	tracegit "github.com/chainloop-dev/chainloop/app/cli/internal/trace/git"
	"github.com/chainloop-dev/chainloop/app/cli/internal/trace/state"
	"github.com/chainloop-dev/chainloop/pkg/attestation/crafter/materials/aicodingsession"
)

// CommitLines is one commit of a session: the lines it changed and the
// sessions that its record names.
type CommitLines struct {
	SHA        string
	SessionIDs []string
	Files      []tracegit.FileLines
	// Joined is set on a commit in no session that replaced lines of the
	// session and so joined it (R-010). It holds, for each file, the
	// fingerprints of the lines that its replacing hunks added. Only these
	// lines count for the session, and they are human.
	Joined map[string][]string
}

// Enrich adds AI/human attribution to the code changes of a session.
//
// ledgers holds the AI line ledger of each session by ID. commits holds the
// session's commits, oldest first. Each line that a commit added is AI when its
// content matches a line that an edit of the session wrote (R-001), and each
// removed line is AI when an edit of the session removed it (R-005). Each
// ledger line matches one commit line at most.
//
// The session's own ledger is matched first. When it and another session of
// the commit both hold a line, the evidence of each session counts the line as
// its own: each evidence is the view of one session. A line that matches only
// another session of the commit counts for neither side: it is that session's
// work. All other lines are human.
//
// tip holds, for each file, its lines at the pushed commit with the commit
// that last wrote each one. The added lines of a file in tip are the net
// contribution of the session (R-003): the lines that its commits added and
// that no later commit replaced, each one counted once. A file that tip holds
// with no lines is gone, and adds nothing. A file that tip does not hold falls
// back to the sum of the added lines of each commit. Removed lines are always
// the sum over the session's own commits.
//
// The line ranges of a file in tip are the positions of the session's net
// added lines in the pushed commit, each range AI or human (spec issue-3639,
// line ranges R-001, R-002). They hold as many lines as the net counts
// (R-003). A file that tip does not hold keeps the AI ranges at edit time,
// with no attribution, as older CLIs write them (R-005).
//
// A file with an edit recorded before line fingerprints existed keeps the
// earlier rule: AI lines are the recorded ranges, capped at the file's diff.
func Enrich(sessionID string, ledgers map[string]*state.AILineAttribution, commits []CommitLines, tip map[string][]tracegit.BlameLine, changes *aicodingsession.CodeChanges) {
	self := ledgers[sessionID]
	if self == nil {
		self = &state.AILineAttribution{SessionID: sessionID}
	}

	byPath := splitByPath(sessionID, self, ledgers, commits)
	positioned := make(map[string][]aicodingsession.LineRange, len(tip))
	for path, net := range netAdded(sessionID, self, ledgers, commits, tip) {
		s, ok := byPath[path]
		if !ok {
			s = &fileSplit{}
			byPath[path] = s
		}
		s.aiAdded, s.humanAdded = net.aiAdded, net.humanAdded
		positioned[path] = net.ranges
	}

	changes.AILinesAdded, changes.AILinesRemoved = 0, 0
	changes.HumanLinesAdded, changes.HumanLinesRemoved = 0, 0
	for i := range changes.Files {
		f := &changes.Files[i]

		// The marker keeps its earlier meaning, a file that the session
		// edited, for platforms that do not read the split (R-009). An empty
		// ranges slice is valid: the agent only deleted lines.
		if aiRanges, ok := self.Files[f.Path]; ok {
			f.Attribution = trace.AttributionAI
			f.LineRanges = aiRanges
			f.SessionIDs = []string{sessionID}
		} else {
			f.Attribution = trace.AttributionHuman
		}
		if ranges, ok := positioned[f.Path]; ok {
			f.LineRanges = ranges
		}

		switch s, ok := byPath[f.Path]; {
		case self.Legacy[f.Path]:
			legacySplit(f, self.Files[f.Path])
		case ok:
			f.AILinesAdded, f.AILinesRemoved = s.aiAdded, s.aiRemoved
			f.HumanLinesAdded, f.HumanLinesRemoved = s.humanAdded, s.humanRemoved
		case f.Attribution == trace.AttributionHuman:
			// No line content for the file (a binary file, or a commit that
			// could not be read): the diff totals are human, as before.
			f.HumanLinesAdded, f.HumanLinesRemoved = f.LinesAdded, f.LinesRemoved
		default:
			legacySplit(f, self.Files[f.Path])
		}

		changes.AILinesAdded += f.AILinesAdded
		changes.AILinesRemoved += f.AILinesRemoved
		changes.HumanLinesAdded += f.HumanLinesAdded
		changes.HumanLinesRemoved += f.HumanLinesRemoved
	}
}

// fileSplit is the AI/human split of the changed lines of one file.
type fileSplit struct {
	aiAdded, aiRemoved, humanAdded, humanRemoved int
	// ranges are the positions of the net added lines in the pushed commit.
	ranges []aicodingsession.LineRange
}

// addLine adds line number n, owned by owner, to the ranges. A line next to
// the last range with the same owner extends it.
func (s *fileSplit) addLine(n int, owner string) {
	if last := len(s.ranges) - 1; last >= 0 && s.ranges[last].End == n-1 && s.ranges[last].Attribution == owner {
		s.ranges[last].End = n
		return
	}

	s.ranges = append(s.ranges, aicodingsession.LineRange{Start: n, End: n, Attribution: owner})
}

// splitByPath matches the lines of each commit against the ledgers, and
// returns the split of each file that the commits changed.
func splitByPath(sessionID string, self *state.AILineAttribution, ledgers map[string]*state.AILineAttribution, commits []CommitLines) map[string]*fileSplit {
	written := linePool{pick: func(a *state.AILineAttribution) map[string]state.LineCounts { return a.Written }}
	erased := linePool{pick: func(a *state.AILineAttribution) map[string]state.LineCounts { return a.Erased }}

	out := make(map[string]*fileSplit)
	for _, commit := range commits {
		// A joined commit counts only through the lines that are still in
		// the pushed commit (netAdded).
		if commit.Joined != nil {
			continue
		}
		others := otherLedgers(sessionID, ledgers, commit.SessionIDs)

		for _, fl := range commit.Files {
			s, ok := out[fl.Path]
			if !ok {
				s = &fileSplit{}
				out[fl.Path] = s
			}

			for _, line := range fl.Added {
				fp := Fingerprint(line)
				switch {
				case written.take(self, fl.Path, fp):
					s.aiAdded++
				case written.takeAny(others, fl.Path, fp):
				default:
					s.humanAdded++
				}
			}
			for _, line := range fl.Removed {
				fp := Fingerprint(line)
				switch {
				case erased.take(self, fl.Path, fp):
					s.aiRemoved++
				case erased.takeAny(others, fl.Path, fp):
				default:
					s.humanRemoved++
				}
			}
		}
	}

	return out
}

// netAdded splits the lines of each file in tip that the session's commits
// wrote, and records the position of each one. A line from a commit outside
// the session is not the session's. A line from a joined commit counts as
// human when its replacing hunks added it, and not at all otherwise.
func netAdded(sessionID string, self *state.AILineAttribution, ledgers map[string]*state.AILineAttribution, commits []CommitLines, tip map[string][]tracegit.BlameLine) map[string]*fileSplit {
	bySHA := make(map[string]CommitLines, len(commits))
	for _, c := range commits {
		bySHA[c.SHA] = c
	}

	written := linePool{pick: func(a *state.AILineAttribution) map[string]state.LineCounts { return a.Written }}
	joined := make(map[[2]string]state.LineCounts)

	out := make(map[string]*fileSplit, len(tip))
	for path, lines := range tip {
		s := &fileSplit{}
		out[path] = s

		for i, l := range lines {
			commit, ok := bySHA[l.SHA]
			if !ok {
				continue
			}
			fp := Fingerprint(l.Text)

			if commit.Joined != nil {
				key := [2]string{commit.SHA, path}
				pool, ok := joined[key]
				if !ok {
					pool = state.LineCounts{}
					pool.Add(commit.Joined[path]...)
					joined[key] = pool
				}
				if pool.Take(fp) {
					s.humanAdded++
					s.addLine(i+1, trace.AttributionHuman)
				}

				continue
			}

			switch {
			case written.take(self, path, fp):
				s.aiAdded++
				s.addLine(i+1, trace.AttributionAI)
			case written.takeAny(otherLedgers(sessionID, ledgers, commit.SessionIDs), path, fp):
			default:
				s.humanAdded++
				s.addLine(i+1, trace.AttributionHuman)
			}
		}
	}

	return out
}

// otherLedgers returns the ledgers of the sessions in ids other than
// sessionID.
func otherLedgers(sessionID string, ledgers map[string]*state.AILineAttribution, ids []string) []*state.AILineAttribution {
	var out []*state.AILineAttribution
	for _, id := range ids {
		if l := ledgers[id]; id != sessionID && l != nil {
			out = append(out, l)
		}
	}

	return out
}

// linePool hands out the lines of the ledgers, one copy per match. Each
// session's lines of a file are copied on first use and drawn down across
// the commits, so a line matches at most as often as the ledger holds it.
type linePool struct {
	// pick selects the lines of a ledger that the pool draws from.
	pick   func(*state.AILineAttribution) map[string]state.LineCounts
	counts map[[2]string]state.LineCounts
}

// take takes one copy of fp from the lines of ledger l for path.
func (p *linePool) take(l *state.AILineAttribution, path, fp string) bool {
	if p.counts == nil {
		p.counts = make(map[[2]string]state.LineCounts)
	}

	key := [2]string{l.SessionID, path}
	c, ok := p.counts[key]
	if !ok {
		c = p.pick(l)[path].Clone()
		p.counts[key] = c
	}

	return c.Take(fp)
}

// takeAny takes fp from the first ledger that holds it.
func (p *linePool) takeAny(ledgers []*state.AILineAttribution, path, fp string) bool {
	for _, l := range ledgers {
		if p.take(l, path, fp) {
			return true
		}
	}

	return false
}

// legacySplit is the split of a file from its recorded ranges: the lines that
// the ranges cover are AI, capped at the file's diff, and every removal is AI.
func legacySplit(f *aicodingsession.FileChange, aiRanges []aicodingsession.LineRange) {
	aiLineCount := 0
	for _, r := range aiRanges {
		aiLineCount += r.End - r.Start + 1
	}
	f.AILinesAdded = min(aiLineCount, f.LinesAdded)
	f.HumanLinesAdded = f.LinesAdded - f.AILinesAdded
	f.AILinesRemoved = f.LinesRemoved
	f.HumanLinesRemoved = 0
}

// SettleCommit returns the open lines of one file that a commit closes: the
// open lines that the commit added or removed, and the open added lines that
// the worktree no longer holds, because something other than the session
// changed them (R-002). The other open lines stay open for a later commit
// (R-006). When clean is true, the worktree holds the file as the commit
// does, so nothing of the file is left to commit and all of it closes.
func SettleCommit(open *state.OpenLines, committed tracegit.FileLines, worktree []byte, clean bool) state.LineSet {
	var out state.LineSet
	if open == nil {
		return out
	}

	added, removed := open.Added.Clone(), open.Removed.Clone()
	for _, line := range committed.Added {
		if fp := Fingerprint(line); added.Take(fp) {
			out.Added = append(out.Added, fp)
		}
	}
	for _, line := range committed.Removed {
		if fp := Fingerprint(line); removed.Take(fp) {
			out.Removed = append(out.Removed, fp)
		}
	}

	if clean {
		out.Added = append(out.Added, added.Missing(nil)...)
		out.Removed = append(out.Removed, removed.Missing(nil)...)

		return out
	}

	present := state.LineCounts{}
	present.Add(FingerprintLines(worktree)...)
	out.Added = append(out.Added, added.Missing(present)...)

	return out
}

// DropOpen removes from an edit the lines that are already open for the
// session, and lowers its counts by the same amount. A shell command is diffed
// against HEAD, not against the session's last edit, so its diff holds again
// the uncommitted lines that earlier edits recorded. A command that writes one
// more copy of an open line loses that copy too; the ledger cannot tell the
// two apart.
func DropOpen(edit Edit, open *state.OpenLines) Edit {
	if open == nil {
		return edit
	}

	keep := func(fps []string, openCounts state.LineCounts) []string {
		left := openCounts.Clone()
		var out []string
		for _, fp := range fps {
			if !left.Take(fp) {
				out = append(out, fp)
			}
		}

		return out
	}

	added, removed := keep(edit.Added, open.Added), keep(edit.Removed, open.Removed)
	edit.AddedCount -= len(edit.Added) - len(added)
	edit.RemovedCount -= len(edit.Removed) - len(removed)
	edit.Added, edit.Removed = added, removed

	return edit
}

// ReplacedLines returns the open added lines of a file that its current
// content no longer holds. Before an agent edit, these are the lines that a
// human or another session changed since the session's last edit (R-002).
func ReplacedLines(open *state.OpenLines, current []byte) state.LineSet {
	if open == nil || len(open.Added) == 0 {
		return state.LineSet{}
	}

	present := state.LineCounts{}
	present.Add(FingerprintLines(current)...)

	return state.LineSet{Added: open.Added.Missing(present)}
}
