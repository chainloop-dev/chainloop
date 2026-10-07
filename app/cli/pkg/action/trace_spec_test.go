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
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/chainloop-dev/chainloop/app/cli/internal/trace/spec"
	"github.com/chainloop-dev/chainloop/app/cli/internal/trace/state"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestSpecCaptureInstruction pins the properties of the wording that decide
// whether an agent acts on it. The prose itself is free to change; these are
// the parts that carry meaning.
func TestSpecCaptureInstruction(t *testing.T) {
	const (
		specDir = "/repo/.chainloop/specs/abc-123"
		// whyVocabulary is shared by the four kind cases: each exists for the
		// same reason, and spelling it out four times says nothing extra.
		whyVocabulary = "the vocabulary has to be spelled out"
	)

	got := specCaptureInstruction(specDir)

	testCases := []struct {
		name string
		want string
		why  string
	}{
		{name: "the destination", want: specDir, why: "the agent has to be told where to write"},
		{name: "the deadline", want: "in your next tool call", why: "an instruction read as background gets deferred past the work"},
		{name: "the frontmatter fence", want: "---", why: "the parser splits on it"},
		{name: "the kind key", want: "kind:", why: "the parser reads it"},
		{name: "the uri key", want: "uri:", why: "the parser reads it"},
		{name: "a ticket kind", want: "ticket", why: whyVocabulary},
		{name: "a document kind", want: "document", why: whyVocabulary},
		{name: "an image kind", want: "image", why: whyVocabulary},
		{name: "a text kind", want: "text", why: whyVocabulary},
		{name: "the role key", want: "role:", why: "the parser reads it"},
		{name: "the title key", want: "title:", why: "the parser reads it"},
		{name: "the description key", want: "description:", why: "the parser reads it"},
		{name: "a task role", want: "task:", why: whyVocabulary},
		{name: "a spec role", want: "spec:", why: whyVocabulary},
		{name: "a plan role", want: "plan:", why: whyVocabulary},
		{name: "a reference role", want: "reference:", why: whyVocabulary},
		{name: "the main purpose", want: "pick the role that tells why it is in this session", why: "a ticket can also hold the full spec, and the agent must pick one role"},
		{name: "the companion file", want: ".meta.yaml", why: "a binary file has no header, so its values go into a file next to it"},
		{name: "the text, not a link", want: "Not a link to it, not your summary of it", why: "a link is worthless as evidence once the source moves"},
		{name: "one file per source", want: "One file per source", why: "several sources become several entries, not one blob"},
		{name: "an image file is copied", want: "copy the file itself into the folder", why: "an image the agent can reach as a file is stored as the image, not as a description"},
		{name: "a pasted image is not captured", want: "Do not capture an image pasted into this conversation", why: "the agent cannot copy the bytes of a pasted image, and a description of it is not the image"},
		{name: "an approved plan", want: "a plan approved in this session", why: "a plan the session wrote and the user approved is what the work ran against"},
		{name: "no request prompt", want: "the user's request prompt is not a spec", why: "the transcript already holds the prompt, so a copy of it adds nothing"},
		{name: "a pasted spec still counts", want: "Spec text that the user pastes", why: "a ticket or design document pasted into the chat is still a spec"},
		{name: "a changed spec", want: "overwrite its file with the current content", why: "the push records what is on disk, so the final version replaces its drafts"},
		{name: "a spec the session writes", want: "a design note outside the repository", why: "a plan the session keeps outside the working tree is a spec that changes"},
		{name: "each push", want: "each time it is pushed", why: "the files stay for the whole session, and every push records them"},
		{name: "the way out", want: "write nothing at all", why: "without it a model invents a spec for a typo fix"},
		// A real session was lost exactly here: the agent resolved both sources
		// correctly and then wrote them with `mkdir -p X && cat > Y <<EOF`,
		// which a worktree-isolated sandbox refuses as "too complex to verify".
		// It never retried with its file-writing tool.
		{name: "the tool to use", want: "file-writing tool", why: "naming the tool is what stops the agent reaching for a shell heredoc"},
		{name: "the tool not to use", want: "not a shell redirect or heredoc", why: "compound shell writes are refused outright in a git worktree"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Contains(t, got, tc.want, tc.why)
		})
	}

	// It opens with the action rather than with a condition: a leading "if"
	// is an easy out to take.
	first, _, ok := strings.Cut(got, "\n")
	require.True(t, ok)
	assert.True(t, strings.HasPrefix(first, "Write the specification"), "the instruction must lead with the action")

	// The header example is copied as written. A list of alternatives in it
	// becomes the literal kind "ticket | document | image | text", which the
	// parser can only read as text.
	assert.NotContains(t, got, "kind: ticket | document", "the example header must hold one real kind")
	assert.Contains(t, got, "Set kind to one of ticket, document, image or text", "the choice of kind is stated outside the example")

	// The request prompt is no longer a source: listing it would ask the
	// agent to copy the first message of the transcript.
	assert.NotContains(t, got, "a written prompt")
	// A description of a pasted image is no longer asked for.
	assert.NotContains(t, got, "write a description of it")
}

// TestSpecCaptureReminder pins the reminder given at each user prompt. It goes
// into the context at every turn, so it must stay short and cover the four
// cases only.
func TestSpecCaptureReminder(t *testing.T) {
	const specDir = "/repo/.chainloop/specs/abc-123"

	got := specCaptureReminder(specDir)

	testCases := []struct {
		name string
		want string
		why  string
	}{
		{name: "the destination", want: specDir, why: "a resumed session may not hold the session-start instruction any more"},
		{name: "a new spec", want: "pasted or gave a new spec", why: "a spec given after the start must reach the evidence"},
		{name: "an image file", want: "image as a file path or a URL", why: "an image the agent can reach is copied as it is"},
		{name: "an approved plan", want: "approved a plan", why: "the plan is what the work ran against"},
		{name: "kind text for a plan", want: "kind text", why: "an approved plan has no other kind"},
		{name: "a changed spec", want: "overwrite its file with the current content", why: "the push records what is on disk"},
		{name: "a local path as the source", want: "its path as uri", why: "a local document has its path as the source address"},
		{name: "no request prompt", want: "Do not capture the user's request prompt", why: "the transcript already holds it"},
		{name: "the way out", want: "do nothing", why: "most turns have nothing to capture"},
		{name: "the tool to use", want: "file-writing tool", why: "a shell heredoc is refused in a worktree-isolated session"},
		{name: "an image keeps no header", want: "Copy an image file as it is, with no frontmatter", why: "a header written into an image file breaks the image"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Contains(t, got, tc.want, tc.why)
		})
	}

	// The reminder costs tokens at every turn, so it stays far shorter than
	// the full instruction.
	assert.Less(t, len(got), len(specCaptureInstruction(specDir))/2, "the reminder must stay short")
	assert.LessOrEqual(t, strings.Count(got, "\n"), 8, "the reminder must stay a few lines")
}

func TestSessionSpecReminder(t *testing.T) {
	const sessionID = "abc-123"

	t.Run("names the session folder and prepares the ignore file", func(t *testing.T) {
		root := t.TempDir()

		got := sessionSpecReminder(root, sessionID, zerolog.Nop())

		assert.Contains(t, got, spec.SessionDir(root, sessionID))
		assert.FileExists(t, filepath.Join(spec.Dir(root), ".gitignore"))
		assert.NoDirExists(t, spec.SessionDir(root, sessionID))
	})

	t.Run("still reminds once something is captured", func(t *testing.T) {
		// A resumed session gets the reminder also when its folder holds
		// files: a spec can still change or be added.
		root := t.TempDir()
		require.NoError(t, spec.EnsureDir(root))
		require.NoError(t, os.MkdirAll(spec.SessionDir(root, sessionID), 0755))
		require.NoError(t, os.WriteFile(filepath.Join(spec.SessionDir(root, sessionID), "ticket.md"), []byte("the ticket"), 0600))

		assert.Contains(t, sessionSpecReminder(root, sessionID, zerolog.Nop()), spec.SessionDir(root, sessionID))
	})

	t.Run("says nothing when the directory cannot be prepared", func(t *testing.T) {
		root := t.TempDir()
		require.NoError(t, os.WriteFile(filepath.Join(root, ".chainloop"), []byte("not a directory"), 0600))

		assert.Empty(t, sessionSpecReminder(root, sessionID, zerolog.Nop()))
	})
}

// TestSessionSpecsAcrossPushes covers R-004 of Spec 003: a file that an
// earlier push recorded stays in the evidence of later pushes, also when the
// agent overwrote it and the session went over the limit.
func TestSessionSpecsAcrossPushes(t *testing.T) {
	const sessionID = "abc-123"

	root := t.TempDir()
	store := state.NewGitStore(filepath.Join(root, ".git"))
	require.NoError(t, store.InitTraceDir())

	dir := spec.SessionDir(root, sessionID)
	require.NoError(t, spec.EnsureDir(root))
	require.NoError(t, os.MkdirAll(dir, 0755))

	write := func(name, body string, at time.Time) {
		path := filepath.Join(dir, name)
		require.NoError(t, os.WriteFile(path, []byte(body), 0600))
		require.NoError(t, os.Chtimes(path, at, at))
	}

	base := time.Date(2026, 9, 16, 10, 0, 0, 0, time.UTC)
	write("ticket.md", "---\nkind: ticket\n---\nthe ticket", base)

	// The first push records the starting ticket.
	captures, warnings := readSessionSpecs(store, root, sessionID, zerolog.Nop())
	require.Len(t, captures, 1)
	assert.Empty(t, warnings)
	recordPushedSpecs(store, sessionID, fileNames(captures), zerolog.Nop())

	// The session then adds more files than the limit allows, and overwrites
	// the ticket last.
	for i := range spec.MaxEntries {
		write(fmt.Sprintf("note-%02d.md", i), fmt.Sprintf("note %d", i), base.Add(time.Duration(i+1)*time.Minute))
	}
	write("ticket.md", "---\nkind: ticket\n---\nthe updated ticket", base.Add(time.Hour))

	captures, warnings = readSessionSpecs(store, root, sessionID, zerolog.Nop())
	require.Len(t, captures, spec.MaxEntries)
	require.Len(t, warnings, 1)
	assert.Equal(t, "ticket.md", captures[0].FileName, "the starting ticket must stay")
	assert.Contains(t, string(captures[0].Raw), "the updated ticket", "the push records the content at push time")

	// The second push stores these files. The agent then deletes a note and
	// adds one. The next push keeps every file the second push stored that
	// is still on disk. The free place goes to the oldest new file: the note
	// that the second push dropped, not the file added last.
	recordPushedSpecs(store, sessionID, fileNames(captures), zerolog.Nop())
	require.NoError(t, os.Remove(filepath.Join(dir, "note-00.md")))
	write("late.md", "a late spec", base.Add(2*time.Hour))

	captures, warnings = readSessionSpecs(store, root, sessionID, zerolog.Nop())
	require.Len(t, captures, spec.MaxEntries)
	require.Len(t, warnings, 1)
	got := fileNames(captures)
	assert.Equal(t, "ticket.md", got[0])
	assert.Contains(t, got, fmt.Sprintf("note-%02d.md", spec.MaxEntries-1))
	assert.NotContains(t, got, "late.md")

	// The list holds one push only, so it never grows past the limit.
	recorded, err := store.RecordedSpecFiles(sessionID)
	require.NoError(t, err)
	assert.LessOrEqual(t, len(recorded), spec.MaxEntries)
}

func TestSessionSpecInstructionOnResume(t *testing.T) {
	const sessionID = "abc-123"

	root := t.TempDir()
	require.NoError(t, spec.EnsureDir(root))
	require.NoError(t, os.MkdirAll(spec.SessionDir(root, sessionID), 0755))
	require.NoError(t, os.WriteFile(filepath.Join(spec.SessionDir(root, sessionID), "ticket.md"), []byte("the ticket"), 0600))

	// An agent with a reminder at each prompt gets it at its first turn, so a
	// resumed session needs no full instruction again.
	assert.Empty(t, sessionSpecInstruction(root, sessionID, true, zerolog.Nop()))
	// An agent without that channel has only the session-start instruction.
	assert.Contains(t, sessionSpecInstruction(root, sessionID, false, zerolog.Nop()), spec.SessionDir(root, sessionID))
}

func TestSessionSpecInstruction(t *testing.T) {
	const sessionID = "abc-123"

	t.Run("asks for a spec when the session has none", func(t *testing.T) {
		root := t.TempDir()

		got := sessionSpecInstruction(root, sessionID, true, zerolog.Nop())

		assert.Contains(t, got, spec.SessionDir(root, sessionID))
		// The ignore file has to be in place before the agent writes, or the
		// specs show up in git status.
		assert.FileExists(t, filepath.Join(spec.Dir(root), ".gitignore"))
		// The session directory is deliberately NOT created: the agent's
		// file-writing tool makes parents, and creating it here would leave an
		// empty directory in the working tree for every session that captures
		// nothing — which is most of them.
		assert.NoDirExists(t, spec.SessionDir(root, sessionID))
	})

	t.Run("goes silent once something is captured", func(t *testing.T) {
		root := t.TempDir()
		require.NoError(t, spec.EnsureDir(root))
		require.NoError(t, os.MkdirAll(spec.SessionDir(root, sessionID), 0755))
		require.NoError(t, os.WriteFile(filepath.Join(spec.SessionDir(root, sessionID), "ticket.md"), []byte("the ticket"), 0600))

		// Repeating it on every resume would spend context tokens on
		// something already done.
		assert.Empty(t, sessionSpecInstruction(root, sessionID, true, zerolog.Nop()))
	})

	t.Run("says nothing when the directory cannot be prepared", func(t *testing.T) {
		root := t.TempDir()
		// A file where the directory needs to go: MkdirAll fails, and the
		// session carries on without a spec rather than failing.
		require.NoError(t, os.WriteFile(filepath.Join(root, ".chainloop"), []byte("not a directory"), 0600))

		assert.Empty(t, sessionSpecInstruction(root, sessionID, true, zerolog.Nop()))
	})
}

// fileNames returns the file names of captures, in order.
func fileNames(captures []spec.Capture) []string {
	names := make([]string, 0, len(captures))
	for _, c := range captures {
		names = append(names, c.FileName)
	}

	return names
}
