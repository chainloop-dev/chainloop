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
//
// It states the stakes, because an agent that reads the captures as
// bookkeeping skips the update after an edit (spec issue-3561). A local file
// gets the placeholder instead of its text: each push reads the file, so the
// evidence follows its edits with no help from the agent. Only a remote
// source still depends on the agent, so the rules to keep it current are
// about that one.
//
// It says that a conversation is not a plan, because an agent left to define
// "approved plan" itself took a chat agreement for one, wrote the plan itself
// and rewrote it at each follow-up (issue-3580). The stakes come with a way
// out for when the agent is not sure, and the agent tells the user what it
// captured, so that a capture is never hidden from the user.
func specCaptureInstruction(specDir string) string {
	return fmt.Sprintf(`Write the specification this session is working from into %[1]s now, in your next tool call, before you start the work.

These files are attested evidence of what this session was asked to build, like a commit. Each push of this session records them. A partial, outdated or summarized capture is a false record. When you are not sure, capture nothing.

Use your file-writing tool, one call per file — not a shell redirect or heredoc, which a sandboxed or worktree-isolated session refuses. It creates the directory for you. Each time you capture a file, tell the user in one line, for example "Captured ticket-eng-1234.md as the task."

Resolve whatever the task points at — a ticket, a design document, an image file, a plan that the user approved in the plan mode of your agent — and write its actual text. Not a link to it, not your summary of it. Use whatever tool reaches it: an issue tracker, a URL fetch, a local file. One file per source, named for what it is (ticket-eng-1234.md, design-proposal.md). Begin each file with a frontmatter block like this one, then the text:

---
kind: ticket
uri: https://tracker.example.com/issue/ENG-1234
role: task
title: "ENG-1234: Add an export button"
description: The ticket that this session implements.
---

Set kind to one of ticket, document, image or text. An approved plan, or spec text that came from outside this conversation, is kind text. Set uri to the source URL or the path of a local file, or omit the uri line when the text has no other source.

Set role to the purpose of the source. If a source has more than one purpose, pick the role that tells why it is in this session. Use one of these values:
- task: the item that states the work to do.
- spec: a document that defines what to build.
- plan: a plan for the work that comes from an external source (a ticket, a design document, a spec file) or that the user approved in the plan mode of your agent. A conversation is not a plan.
- reference: supporting material, such as a screenshot or a background document.
Set title to a short name for the source, such as the ticket title. Set description to one or two sentences that tell what the source holds.

A local text file is the exception to writing text. When the source is a file on disk — a spec in this repository, or a design note outside the repository — set uri to its path (absolute, ~/, or relative to the repository root) and write only this line below the frontmatter, not the text of the file:

%[2]s

Each push reads the file and records its content at that time, so a later edit of the file needs no new capture.

Capture only a source that exists outside the chat: a ticket, a document, a file, a web page, spec text that the user pastes, or a plan approved in plan mode. The transcript of this session already holds the conversation. So the user's request prompt is not a spec, and a conversation is not a plan: a change that you and the user agree on in the chat is not a source, also when the user approves it. Never make a capture from the conversation.

An image is the other exception to writing text. If you can reach the image as a file — on disk, or at a URL you can download — copy the file itself into the folder with a copy or download command (cp, curl -o), keeping its extension and adding no frontmatter. Then write the role, title and description lines in a second file next to it. Give that file the name of the image plus .meta.yaml, for example mockup.png.meta.yaml. Do not capture an image pasted into this conversation, also when you have a file path for it, such as a file in the folder where your agent keeps pasted images: the transcript holds it.

For any other source — a ticket, a web page, pasted text, an approved plan — write its full text. If the task changes, or such a source changes, overwrite its file with the current content in the same turn as the change, or add another. A reply in the chat does not change a source. Before you push, check that each of these files holds the current full text of its source. If there is nothing to capture — a one-line request, a typo fix, a question, a passing remark — write nothing at all.

The files are removed when the session ends. They are git-ignored and will never appear in a commit.`, specDir, spec.Placeholder)
}

// specCaptureReminder is the one place the wording of the reminder lives.
//
// It goes into the context at every turn, so it is a few lines and names four
// cases only: a broader rule would capture too much, and a long one would
// distract the agent from the task. It repeats the folder and the header
// format, because a resumed session may not hold the full instruction any
// more. Like the instruction, it states the stakes and gives a local file the
// placeholder instead of its text (spec issue-3561), and like the
// instruction it does not take a conversation for a plan (issue-3580).
func specCaptureReminder(specDir string) string {
	return fmt.Sprintf(`Chainloop spec capture. Folder: %s
These files are attested evidence: a partial or outdated capture is a false record. Capture only in these cases, one file per source. Write each text file with your file-writing tool, starting with frontmatter (kind: ticket, document, image or text; uri: the source, when there is one). Copy an image file as it is, with no frontmatter:
- The user pasted or gave a new spec: write its full text.
- The user gave an image as a file path or a URL: copy the file.
- The user approved a plan in plan mode: write it as kind text. A conversation is not a plan.
- A spec is a local text file, also one this session writes: set uri to its path. Each push reads the file. Write only this line below the frontmatter: %s
- A spec that is not a local file changed: overwrite its file with the current full text in the same turn; a reply in the chat is not a change. Check these files before a push.
Do not capture the user's request prompt or a pasted image, also one saved to a file. When you capture a file, tell the user in one line. If no case applies or you are not sure, capture nothing.`, specDir, spec.Placeholder)
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

// recordPushedSpecs keeps the names of the spec files that a push stored, for
// the next push of the session. The list replaces the one of the push before:
// that push put its files first, so each of them that is still on disk is in
// this list too. The list then never holds more than spec.MaxEntries names.
// A failure costs a later push its order, never this push its evidence.
func recordPushedSpecs(store *state.Store, sessionID string, fileNames []string, log zerolog.Logger) {
	if err := store.SetRecordedSpecFiles(sessionID, fileNames); err != nil {
		log.Debug().Err(err).Str("session", sessionID).Msg("could not keep the spec files this push recorded")
	}
}
