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
	"github.com/rs/zerolog"
)

// sessionSpecInstruction is what the session-start hook asks the agent to do
// before it starts work: resolve whatever set the task and write the text down,
// so that the intent the work gets judged against travels with the evidence
// instead of sitting behind a link.
//
// It returns "" once anything has been captured for this session — the common
// case after the first turn. Repeating the instruction on every resume would
// spend context tokens on something already done.
//
// A directory we cannot create costs this session's spec and nothing else, so
// the failure is logged and swallowed rather than surfaced: capture is the one
// thing here worth less than the session it would interrupt.
func sessionSpecInstruction(repoRoot, sessionID string, log zerolog.Logger) string {
	if err := spec.EnsureDir(repoRoot); err != nil {
		log.Debug().Err(err).Msg("session-start: could not prepare the spec directory")

		return ""
	}

	if spec.Exists(repoRoot, sessionID) {
		return ""
	}

	return specCaptureInstruction(spec.SessionDir(repoRoot, sessionID))
}

// specCaptureInstruction is the one place this wording lives.
//
// It leads with the action and a deadline, because an instruction read as
// background gets deferred past the work it is meant to precede. It insists on
// the text rather than a link, because a link is what a model would rather
// write and is worthless as evidence once the source moves or access is
// revoked. And it says plainly that some sessions have nothing to capture,
// because a model handed a mandatory-sounding instruction will otherwise
// manufacture a spec for a typo fix.
func specCaptureInstruction(specDir string) string {
	return fmt.Sprintf(`Write the specification this session is working from into %s now, in your next tool call, before you start the work.

Use your file-writing tool, one call per file — not a shell redirect or heredoc, which a sandboxed or worktree-isolated session refuses. It creates the directory for you.

Resolve whatever the task points at — a ticket, a design document, an image, a written prompt, a plan approved in this session — and write its actual text. Not a link to it, not your summary of it. Use whatever tool reaches it: an issue tracker, a URL fetch, a local file. One file per source, named for what it is (ticket-eng-1234.md, design-proposal.md). Begin each file with a frontmatter block like this one, then the text:

---
kind: ticket
uri: https://tracker.example.com/issue/ENG-1234
---

Set kind to one of ticket, document, image or text. A written prompt or an approved plan is kind text. Set uri to the source URL, or omit the uri line when the task was stated in this conversation.

If the task changes, overwrite the file it changed or add another. If there is nothing to capture — a typo fix, a question, a passing remark — write nothing at all.

These files are recorded as part of this session's evidence each time it is pushed, and removed when the session ends. They are git-ignored and will never appear in a commit. There is no need to mention any of this to the user.`, specDir)
}
