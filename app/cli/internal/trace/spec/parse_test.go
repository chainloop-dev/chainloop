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

package spec

import (
	"strings"
	"testing"
	"time"

	"github.com/chainloop-dev/chainloop/pkg/attestation/crafter/materials/aicodingsession"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var capturedAt = time.Date(2026, 9, 16, 10, 12, 3, 0, time.UTC)

// specBody is the stand-in spec text wherever the case is about the frontmatter
// rather than about what the body holds.
const specBody = "body"

func TestParse(t *testing.T) {
	testCases := []struct {
		name string
		doc  string
		// wantNil says the document carried nothing worth attesting.
		wantNil     bool
		wantKind    string
		wantURI     string
		wantContent string
		wantRole    string
		wantTitle   string
		wantDesc    string
	}{
		{
			name:        "a full frontmatter block",
			doc:         "---\nkind: ticket\nuri: https://linear.app/chainloop/issue/PFM-7289\n---\n# PFM-7289\n\nCapture the spec.",
			wantKind:    aicodingsession.SpecKindTicket,
			wantURI:     "https://linear.app/chainloop/issue/PFM-7289",
			wantContent: "# PFM-7289\n\nCapture the spec.",
		},
		{
			// The agent was told to write frontmatter and didn't. The text it
			// did write is the whole point, so it is kept.
			name:        "no frontmatter at all",
			doc:         "Make the retry path give up after three attempts.",
			wantKind:    aicodingsession.SpecKindText,
			wantContent: "Make the retry path give up after three attempts.",
		},
		{
			name:        "a kind outside the vocabulary",
			doc:         "---\nkind: jira-ticket\nuri: https://example.atlassian.net/browse/ENG-1\n---\n" + specBody,
			wantKind:    aicodingsession.SpecKindText,
			wantURI:     "https://example.atlassian.net/browse/ENG-1",
			wantContent: specBody,
		},
		{
			name:        "a kind with case and padding",
			doc:         "---\nkind: \"  Document \"\n---\n" + specBody,
			wantKind:    aicodingsession.SpecKindDocument,
			wantContent: specBody,
		},
		{
			name:        "a uri with no kind",
			doc:         "---\nuri: https://example.com/spec\n---\n" + specBody,
			wantKind:    aicodingsession.SpecKindText,
			wantURI:     "https://example.com/spec",
			wantContent: specBody,
		},
		{
			// Frontmatter is used precisely because it degrades. A value
			// that is not a kind costs the kind, never the spec.
			name:        "a kind that is not valid YAML",
			doc:         "---\nkind: [unclosed\n---\n" + specBody,
			wantKind:    aicodingsession.SpecKindText,
			wantContent: specBody,
		},
		{
			// A line that YAML rejects must not cost the other lines.
			name:        "a value that is not valid YAML after a valid kind",
			doc:         "---\nkind: ticket\nuri: [unclosed\n---\n" + specBody,
			wantKind:    aicodingsession.SpecKindTicket,
			wantURI:     "[unclosed",
			wantContent: specBody,
		},
		{
			// YAML reads " #" as the start of a comment and cuts the value.
			name:        "a description that holds a hash",
			doc:         "---\nkind: document\ndescription: The merged design spec (PR #3544) that defines R-001\n---\n" + specBody,
			wantKind:    aicodingsession.SpecKindDocument,
			wantDesc:    "The merged design spec (PR #3544) that defines R-001",
			wantContent: specBody,
		},
		{
			// YAML rejects ": " in a plain value, which used to cost every key.
			name:        "an unquoted title that holds colons",
			doc:         "---\nkind: ticket\nuri: https://linear.app/chainloop/issue/PFM-1\nrole: task\ntitle: PFM-1: fix: the thing\ndescription: The ticket for the fix.\n---\n" + specBody,
			wantKind:    aicodingsession.SpecKindTicket,
			wantURI:     "https://linear.app/chainloop/issue/PFM-1",
			wantRole:    aicodingsession.SpecRoleTask,
			wantTitle:   "PFM-1: fix: the thing",
			wantDesc:    "The ticket for the fix.",
			wantContent: specBody,
		},
		{
			// YAML reads a leading "[" as the start of a sequence.
			name:        "an unquoted title that starts with a bracket",
			doc:         "---\nkind: ticket\nuri: https://linear.app/chainloop/issue/PFM-2\nrole: task\ntitle: [WIP] something\ndescription: Work in progress.\n---\n" + specBody,
			wantKind:    aicodingsession.SpecKindTicket,
			wantURI:     "https://linear.app/chainloop/issue/PFM-2",
			wantRole:    aicodingsession.SpecRoleTask,
			wantTitle:   "[WIP] something",
			wantDesc:    "Work in progress.",
			wantContent: specBody,
		},
		{
			name:        "a description with backticks around a colon",
			doc:         "---\nkind: document\ndescription: uses `foo: bar` inline\n---\n" + specBody,
			wantKind:    aicodingsession.SpecKindDocument,
			wantDesc:    "uses `foo: bar` inline",
			wantContent: specBody,
		},
		{
			// A header that is already quoted gives the same values as before.
			name:        "quoted values with escapes",
			doc:         "---\nkind: \"ticket\"\nuri: 'https://example.com/a'\nrole: \"spec\"\ntitle: 'it''s: quoted'\ndescription: \"line one\\nPR \\\"#1\\\"\"\n---\n" + specBody,
			wantKind:    aicodingsession.SpecKindTicket,
			wantURI:     "https://example.com/a",
			wantRole:    aicodingsession.SpecRoleSpec,
			wantTitle:   "it's: quoted",
			wantDesc:    "line one\nPR \"#1\"",
			wantContent: specBody,
		},
		{
			// Text after the closing quote makes it a plain value.
			name:        "a value that only starts with a quote",
			doc:         "---\ntitle: \"Export\" button: v2\n---\n" + specBody,
			wantKind:    aicodingsession.SpecKindText,
			wantTitle:   "\"Export\" button: v2",
			wantContent: specBody,
		},
		{
			name:        "comment lines, unknown keys and stray lines",
			doc:         "---\n# a comment\nkind: ticket\nowner: someone\n\nnot a key\n  stray: indented\nrole: task\n---\n" + specBody,
			wantKind:    aicodingsession.SpecKindTicket,
			wantRole:    aicodingsession.SpecRoleTask,
			wantContent: specBody,
		},
		{
			// The lines of an unknown key must not be read as known keys.
			name:        "an unknown key with indented lines",
			doc:         "---\ntags:\n  title: not a title\nkind: ticket\n---\n" + specBody,
			wantKind:    aicodingsession.SpecKindTicket,
			wantContent: specBody,
		},
		{
			name:        "a literal block scalar",
			doc:         "---\nkind: document\ndescription: |\n  first: line\n\n  second #2\n---\n" + specBody,
			wantKind:    aicodingsession.SpecKindDocument,
			wantDesc:    "first: line\n\nsecond #2",
			wantContent: specBody,
		},
		{
			name:        "a plain value over more than one line",
			doc:         "---\ndescription: The ticket that\n  the session implements.\nkind: ticket\n---\n" + specBody,
			wantKind:    aicodingsession.SpecKindTicket,
			wantDesc:    "The ticket that the session implements.",
			wantContent: specBody,
		},
		{
			// An indented line that starts with "#" is a YAML comment.
			name:        "an indented comment line after a plain value",
			doc:         "---\ntitle: Foo\n  # note\ndescription: Bar #1\n  continued\n  # another note\nkind: ticket\n---\n" + specBody,
			wantKind:    aicodingsession.SpecKindTicket,
			wantTitle:   "Foo",
			wantDesc:    "Bar #1 continued",
			wantContent: specBody,
		},
		{
			name:        "a repeated key keeps the last value",
			doc:         "---\nkind: text\nkind: ticket\n---\n" + specBody,
			wantKind:    aicodingsession.SpecKindTicket,
			wantContent: specBody,
		},
		{
			name:        "an unterminated frontmatter block",
			doc:         "---\nkind: ticket\nthe body, with no closing fence",
			wantKind:    aicodingsession.SpecKindText,
			wantContent: "---\nkind: ticket\nthe body, with no closing fence",
		},
		{
			// A markdown horizontal rule must not re-split the document.
			name:        "a horizontal rule in the body",
			doc:         "---\nkind: document\n---\nfirst\n\n---\n\nsecond",
			wantKind:    aicodingsession.SpecKindDocument,
			wantContent: "first\n\n---\n\nsecond",
		},
		{
			name:        "CRLF line endings",
			doc:         "---\r\nkind: ticket\r\n---\r\nbody\r\nmore",
			wantKind:    aicodingsession.SpecKindTicket,
			wantContent: "body\r\nmore",
		},
		{
			name:        "leading blank lines before the frontmatter",
			doc:         "\n\n---\nkind: image\n---\nwhat the mockup shows",
			wantKind:    aicodingsession.SpecKindImage,
			wantContent: "what the mockup shows",
		},
		{
			name:        "a role, a title and a description",
			doc:         "---\nkind: ticket\nrole: task\ntitle: \"ENG-1234: Add an export button\"\ndescription: The ticket that the session implements.\n---\n" + specBody,
			wantKind:    aicodingsession.SpecKindTicket,
			wantRole:    aicodingsession.SpecRoleTask,
			wantTitle:   "ENG-1234: Add an export button",
			wantDesc:    "The ticket that the session implements.",
			wantContent: specBody,
		},
		{
			name:        "a role with case and padding",
			doc:         "---\nrole: \" Reference \"\n---\n" + specBody,
			wantKind:    aicodingsession.SpecKindText,
			wantRole:    aicodingsession.SpecRoleReference,
			wantContent: specBody,
		},
		{
			// A bad role costs the role only: never the kind, and never a
			// role guessed from the kind.
			name:        "a role outside the vocabulary",
			doc:         "---\nkind: document\nrole: design\n---\n" + specBody,
			wantKind:    aicodingsession.SpecKindDocument,
			wantContent: specBody,
		},
		{
			name:        "a title and a description of only whitespace",
			doc:         "---\ntitle: \"   \"\ndescription: \"\\t\"\n---\n" + specBody,
			wantKind:    aicodingsession.SpecKindText,
			wantContent: specBody,
		},
		{
			// The limit counts characters, so a cut never splits one.
			name:        "a title over the limit is cut",
			doc:         "---\ntitle: " + strings.Repeat("é", MaxTitleLen+5) + "\n---\n" + specBody,
			wantKind:    aicodingsession.SpecKindText,
			wantTitle:   strings.Repeat("é", MaxTitleLen),
			wantContent: specBody,
		},
		{
			name:        "a description over the limit is cut",
			doc:         "---\ndescription: " + strings.Repeat("a", MaxDescriptionLen+1) + "\n---\n" + specBody,
			wantKind:    aicodingsession.SpecKindText,
			wantDesc:    strings.Repeat("a", MaxDescriptionLen),
			wantContent: specBody,
		},
		{name: "an empty document", doc: "", wantNil: true},
		{name: "frontmatter with no body", doc: "---\nkind: ticket\n---\n", wantNil: true},
		{name: "a whitespace-only body", doc: "---\nkind: ticket\n---\n   \n\n\t\n", wantNil: true},
		{name: "a document of only whitespace", doc: "  \n\n ", wantNil: true},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			got := Parse([]byte(tc.doc), capturedAt)

			if tc.wantNil {
				assert.Nil(t, got)
				return
			}

			require.NotNil(t, got)
			assert.Equal(t, tc.wantKind, got.Kind)
			assert.Equal(t, tc.wantURI, got.URI)
			assert.Equal(t, tc.wantRole, got.Role)
			assert.Equal(t, tc.wantTitle, got.Title)
			assert.Equal(t, tc.wantDesc, got.Description)
			// What follows the header is the text: split decides where the
			// header ends, which is also what decides the kind and the URI.
			_, body := split(tc.doc)
			assert.Equal(t, tc.wantContent, strings.TrimSpace(body))
			assert.Equal(t, "2026-09-16T10:12:03Z", got.CapturedAt)
		})
	}
}

func TestParseMeta(t *testing.T) {
	testCases := []struct {
		name string
		doc  string
		want Meta
	}{
		{
			name: "every field",
			doc:  "role: reference\ntitle: Export button mockup\ndescription: Where the button goes.\n",
			want: Meta{Role: aicodingsession.SpecRoleReference, Title: "Export button mockup", Description: "Where the button goes."},
		},
		{name: "a role outside the vocabulary", doc: "role: mockup\ntitle: x\n", want: Meta{Title: "x"}},
		{name: "a title over the limit is cut", doc: "title: " + strings.Repeat("a", MaxTitleLen+1), want: Meta{Title: strings.Repeat("a", MaxTitleLen)}},
		// A line that YAML rejects keeps its value, and costs the other lines nothing.
		{name: "not valid YAML", doc: "role: plan\ntitle: [unclosed\n", want: Meta{Role: aicodingsession.SpecRolePlan, Title: "[unclosed"}},
		{
			name: "unquoted values with colons, a bracket and a hash",
			doc:  "role: reference\ntitle: PFM-2: [WIP] fix #12\ndescription: The merged design spec (PR #3544) that defines R-001\n",
			want: Meta{Role: aicodingsession.SpecRoleReference, Title: "PFM-2: [WIP] fix #12", Description: "The merged design spec (PR #3544) that defines R-001"},
		},
		{
			name: "quoted values, a comment and a document marker",
			doc:  "---\n# the mockup\nrole: \"reference\"\ntitle: 'Export: mockup'\ndescription: \"Where the button goes.\"\n",
			want: Meta{Role: aicodingsession.SpecRoleReference, Title: "Export: mockup", Description: "Where the button goes."},
		},
		{name: "keys a companion file does not hold", doc: "kind: ticket\nuri: https://example.com\ntitle: x\n", want: Meta{Title: "x"}},
		{name: "an empty file", doc: "", want: Meta{}},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, ParseMeta([]byte(tc.doc)))
		})
	}
}
