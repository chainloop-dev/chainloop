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
	"time"
	"unicode/utf8"

	"github.com/chainloop-dev/chainloop/pkg/attestation/crafter/materials/aicodingsession"
	"gopkg.in/yaml.v3"
)

// delimiter opens and closes a frontmatter block.
const delimiter = "---"

// frontmatter is the header a spec document carries. It is decoded into a typed
// struct rather than a map so an unexpected key cannot reach the evidence.
type frontmatter struct {
	Kind string `yaml:"kind"`
	URI  string `yaml:"uri"`
}

// Capture is one spec document as read from the session folder: what the agent
// wrote, decided on and cleaned up, but not yet stored anywhere.
type Capture struct {
	// FileName is the base name the agent gave the file. It names the material
	// the text is stored as.
	FileName string
	// Kind is one of the aicodingsession.SpecKind* constants.
	Kind string
	// URI is where the text came from, empty for a spec stated in the session.
	URI string
	// Content is the spec text, without its frontmatter.
	Content string
	// CapturedAt is the file's modification time, RFC3339.
	CapturedAt string
	// Raw is the file as the agent wrote it, header included. It is what
	// gets redacted before anything from the file is stored.
	Raw []byte
}

// Parse converts one spec document into a capture. It never fails.
//
// A spec is the best record we have of what a session was asked to build, and
// the file is written by a model from a free-text instruction: it will
// sometimes be malformed. Every degradation below records something rather than
// nothing, because losing the spec — or failing the push that carries it — is a
// far worse outcome than recording it imperfectly.
//
//   - No frontmatter, or an unterminated block: the whole document is content,
//     with kind "text" and no URI.
//   - Frontmatter that is not valid YAML: the body after the block is content,
//     again with kind "text" and no URI.
//   - A kind outside the vocabulary: normalised to "text".
//
// It returns nil when nothing is left once the body is trimmed. That is the
// common shape of a session with nothing to capture: the agent was told to
// write nothing at all, and a stray empty file means the same thing.
func Parse(doc []byte, capturedAt time.Time) *Capture {
	header, body := split(string(doc))

	var meta frontmatter
	if header != "" {
		// A header we cannot read costs the metadata, never the body. The
		// decoder can fill some fields before it fails, so a failure clears
		// what it filled.
		if err := yaml.Unmarshal([]byte(header), &meta); err != nil {
			meta = frontmatter{}
		}
	}

	content := strings.TrimSpace(body)
	if content == "" {
		return nil
	}

	// Explicit rather than leaning on encoding/json to substitute silently
	// later: what is attested should be what this function decided on.
	content = strings.ToValidUTF8(content, string(utf8.RuneError))

	return &Capture{
		Kind:       aicodingsession.ResolveSpecKind(meta.Kind),
		URI:        strings.TrimSpace(meta.URI),
		Content:    content,
		CapturedAt: capturedAt.UTC().Format(time.RFC3339),
	}
}

// split separates a leading frontmatter block from the body. A document with no
// well-formed block is all body, so that a missing or unterminated header
// cannot swallow the text it was supposed to introduce.
func split(doc string) (header, body string) {
	rest := strings.TrimLeft(doc, "\r\n\t ")
	if !strings.HasPrefix(rest, delimiter) {
		return "", doc
	}

	lines := strings.Split(rest, "\n")
	if !isDelimiter(lines[0]) {
		return "", doc
	}

	// The first closing fence wins, so a markdown horizontal rule further down
	// the body cannot re-split the document.
	for i := 1; i < len(lines); i++ {
		if isDelimiter(lines[i]) {
			return strings.Join(lines[1:i], "\n"), strings.Join(lines[i+1:], "\n")
		}
	}

	return "", doc
}

// isDelimiter reports whether a line is a frontmatter fence, tolerating the
// carriage return a CRLF document leaves behind and any trailing whitespace.
func isDelimiter(line string) bool {
	return strings.TrimRight(line, "\r\t ") == delimiter
}
