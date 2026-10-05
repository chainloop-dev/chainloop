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

package action

import (
	"fmt"

	"github.com/chainloop-dev/chainloop/app/cli/internal/trace/spec"
	"github.com/chainloop-dev/chainloop/app/cli/internal/trace/state"
	"github.com/rs/zerolog"
)

// sessionSpecInstruction is what the session-start hook asks the agent to do
// before it starts work: resolve whatever set the task and write the text down,
// so that the intent the work gets judged against travels with the evidence
// instead of sitting behind a link.
//
// remindsEachTurn tells whether the agent also gets the reminder of
// sessionSpecReminder at each user prompt. When it does, the instruction is
// "" once anything has been captured for this session: the reminder at the
// first turn of a resumed session covers it, and repeating the full
// instruction would spend context tokens on something already done. An agent
// without that channel has only this instruction, so it always gets it.
//
// A directory we cannot create costs this session's spec and nothing else, so
// the failure is logged and swallowed rather than surfaced: capture is the one
// thing here worth less than the session it would interrupt.
func sessionSpecInstruction(repoRoot, sessionID string, remindsEachTurn bool, log zerolog.Logger) string {
	if err := spec.EnsureDir(repoRoot); err != nil {
		log.Debug().Err(err).Msg("session-start: could not prepare the spec directory")

		return ""
	}

	if remindsEachTurn && spec.Exists(repoRoot, sessionID) {
		return ""
	}

	return specCaptureInstruction(spec.SessionDir(repoRoot, sessionID))
}

// sessionSpecReminder is what the prompt-submit hook adds to the context at
// each user prompt, so that a spec given or changed after the session start
// also reaches the evidence. It is given also when the folder already holds
// files: a spec can still change or be added.
//
// As for the instruction, a directory we cannot create costs the spec only.
func sessionSpecReminder(repoRoot, sessionID string, log zerolog.Logger) string {
	if err := spec.EnsureDir(repoRoot); err != nil {
		log.Debug().Err(err).Msg("prompt-submit: could not prepare the spec directory")

		return ""
	}

	return specCaptureReminder(spec.SessionDir(repoRoot, sessionID))
}

// specCaptureInstruction is the one place this wording lives.
//
// It leads with the action and a deadline, because an instruction read as
// background gets deferred past the work it is meant to precede. It insists on
// the text rather than a link, because a link is what a model would rather
// write and is worthless as evidence once the source moves or access is
// revoked. It says that the request prompt is not a spec, because the
// transcript already holds it. And it says plainly that some sessions have
// nothing to capture, because a model handed a mandatory-sounding instruction
// will otherwise manufacture a spec for a typo fix.
func specCaptureInstruction(specDir string) string {
	return fmt.Sprintf(`Write the specification this session is working from into %s now, in your next tool call, before you start the work.

Use your file-writing tool, one call per file — not a shell redirect or heredoc, which a sandboxed or worktree-isolated session refuses. It creates the directory for you.

Resolve whatever the task points at — a ticket, a design document, an image file, a plan approved in this session — and write its actual text. Not a link to it, not your summary of it. Use whatever tool reaches it: an issue tracker, a URL fetch, a local file. One file per source, named for what it is (ticket-eng-1234.md, design-proposal.md). Begin each file with a frontmatter block like this one, then the text:

---
kind: ticket
uri: https://tracker.example.com/issue/ENG-1234
---

Set kind to one of ticket, document, image or text. An approved plan, or spec text that came from outside this conversation, is kind text. Set uri to the source URL or the path of a local file, or omit the uri line when the text has no other source.

The transcript of this session already holds the conversation, so the user's request prompt is not a spec: do not capture it. Spec text that the user pastes, such as a ticket or a design document, is a spec.

An image is the one exception to writing text. If you can reach the image as a file — on disk, or at a URL you can download — copy the file itself into the folder with a copy or download command (cp, curl -o), keeping its extension and adding no frontmatter. Do not capture an image pasted into this conversation: you have no file for it.

If the task changes, or a spec changes — also one that this session writes, such as a design note outside the repository — overwrite its file with the current content, or add another. If there is nothing to capture — a one-line request, a typo fix, a question, a passing remark — write nothing at all.

These files are recorded as part of this session's evidence each time it is pushed, and removed when the session ends. They are git-ignored and will never appear in a commit. There is no need to mention any of this to the user.`, specDir)
}

// specCaptureReminder is the one place the wording of the reminder lives.
//
// It goes into the context at every turn, so it is a few lines and names four
// cases only: a broader rule would capture too much, and a long one would
// distract the agent from the task. It repeats the folder and the header
// format, because a resumed session may not hold the full instruction any
// more.
func specCaptureReminder(specDir string) string {
	return fmt.Sprintf(`Chainloop spec capture. Folder: %s
Capture only in these cases, with your file-writing tool, one file per source, each starting with frontmatter (kind: ticket, document, image or text; uri: the source, when there is one):
- The user pasted or gave a new spec: write its actual text.
- The user gave an image as a file path or a URL: copy the file.
- The user approved a plan: write it as kind text.
- A spec changed, also one this session edits: overwrite its file with the current content. A local file has its path as uri.
Do not capture the user's request prompt or a pasted image. If no case applies, do nothing and say nothing about it.`, specDir)
}

// readSessionSpecs reads the spec files of a session for a push. Most
// sessions have none, and a failure to read them is never a reason to lose
// the session: evidence without a spec is still evidence. The files an
// earlier push recorded come first, so the limit never drops them.
func readSessionSpecs(store *state.Store, repoRoot, sessionID string, log zerolog.Logger) ([]spec.Capture, []string) {
	recorded, err := store.RecordedSpecFiles(sessionID)
	if err != nil {
		// Without the list the push still records the oldest files first.
		log.Debug().Err(err).Str("session", sessionID).Msg("could not read the spec files earlier pushes recorded")
	}

	captures, warnings, err := spec.ReadAll(repoRoot, sessionID, recorded)
	if err != nil {
		// The error stays in the local log: its text carries local paths.
		log.Warn().Err(err).Str("session", sessionID).Msg("could not read the session spec; the session is attested without it")
		warnings = append(warnings, "the session spec was not recorded: the spec folder could not be read")
	}

	return captures, warnings
}

// recordPushedSpecs keeps the names of the spec files that a push recorded,
// for the later pushes of the session. A failure costs a later push its
// order, never this push its evidence.
func recordPushedSpecs(store *state.Store, sessionID string, captures []spec.Capture, log zerolog.Logger) {
	if len(captures) == 0 {
		return
	}

	names := make([]string, 0, len(captures))
	for _, c := range captures {
		names = append(names, c.FileName)
	}

	if err := store.AddRecordedSpecFiles(sessionID, names); err != nil {
		log.Debug().Err(err).Str("session", sessionID).Msg("could not keep the spec files this push recorded")
	}
}
