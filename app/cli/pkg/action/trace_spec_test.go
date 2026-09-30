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
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/chainloop-dev/chainloop/app/cli/internal/trace/spec"
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
		{name: "the text, not a link", want: "Not a link to it, not your summary of it", why: "a link is worthless as evidence once the source moves"},
		{name: "one file per source", want: "One file per source", why: "several sources become several entries, not one blob"},
		{name: "an approved plan", want: "a plan approved in this session", why: "a plan the session wrote and the user approved is what the work ran against"},
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
}

func TestSessionSpecInstruction(t *testing.T) {
	const sessionID = "abc-123"

	t.Run("asks for a spec when the session has none", func(t *testing.T) {
		root := t.TempDir()

		got := sessionSpecInstruction(root, sessionID, zerolog.Nop())

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
		assert.Empty(t, sessionSpecInstruction(root, sessionID, zerolog.Nop()))
	})

	t.Run("says nothing when the directory cannot be prepared", func(t *testing.T) {
		root := t.TempDir()
		// A file where the directory needs to go: MkdirAll fails, and the
		// session carries on without a spec rather than failing.
		require.NoError(t, os.WriteFile(filepath.Join(root, ".chainloop"), []byte("not a directory"), 0600))

		assert.Empty(t, sessionSpecInstruction(root, sessionID, zerolog.Nop()))
	})
}
