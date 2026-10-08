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
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"github.com/chainloop-dev/chainloop/pkg/attestation/crafter/materials/aicodingsession"
	"gopkg.in/yaml.v3"
)

// delimiter opens and closes a frontmatter block.
const delimiter = "---"

// The limits of a title and a description, in characters. A model sometimes
// writes a paragraph where a label is asked for, and a cut keeps a usable one.
const (
	MaxTitleLen       = 120
	MaxDescriptionLen = 300
)

// Meta holds what the agent states about a source next to its kind: its
// purpose, a short name, and what it holds. A text file carries it in its
// header, and a binary file in a companion file.
type Meta struct {
	// Role is one of the aicodingsession.SpecRole* constants, or empty.
	Role string
	// Title is a short name for the source, or empty.
	Title string
	// Description says what the source holds, or is empty.
	Description string
}

// frontmatter is the header a spec document carries. parseHeader fills only
// these fields, so an unexpected key cannot reach the evidence.
type frontmatter struct {
	Kind string
	URI  string
	Meta
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
	// Meta holds the role, the title and the description of the source. For
	// a binary file they stay empty until its companion file is redacted.
	Meta
	// CapturedAt is the file's modification time, RFC3339.
	CapturedAt string
	// Raw is the file as the agent wrote it, header included. It is what
	// gets redacted before anything from the file is stored.
	Raw []byte
	// Verbatim reports a file that is stored exactly as it is on disk: an
	// image, or any file that is not text. The agent copied it into the
	// folder, so it has no header and nothing in it is rewritten.
	Verbatim bool
	// MetaRaw is the companion file of a verbatim file as the agent wrote it,
	// or nil when there is none. It is redacted before ParseMeta reads it.
	MetaRaw []byte
}

// verbatimCapture describes a file that is stored as it is. Its kind comes
// from its content, since it has no header to state one, and it carries no URI.
func verbatimCapture(name string, doc []byte, modTime time.Time, image bool) Capture {
	kind := aicodingsession.SpecKindDocument
	if image {
		kind = aicodingsession.SpecKindImage
	}

	return Capture{
		FileName:   name,
		Kind:       kind,
		CapturedAt: modTime.UTC().Format(time.RFC3339),
		Raw:        doc,
		Verbatim:   true,
	}
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
//   - A header line that is not a known key: skipped. The other lines keep
//     their values.
//   - A kind outside the vocabulary: normalised to "text".
//   - A role outside the vocabulary: no role.
//   - A title or a description over its limit: cut to the limit.
//
// It returns nil when nothing is left once the body is trimmed. That is the
// common shape of a session with nothing to capture: the agent was told to
// write nothing at all, and a stray empty file means the same thing.
func Parse(doc []byte, capturedAt time.Time) *Capture {
	header, body := split(string(doc))

	meta := parseHeader(header)

	if strings.TrimSpace(body) == "" {
		return nil
	}

	return &Capture{
		Kind:       aicodingsession.ResolveSpecKind(meta.Kind),
		URI:        strings.TrimSpace(meta.URI),
		Meta:       meta.normalize(),
		CapturedAt: capturedAt.UTC().Format(time.RFC3339),
	}
}

// ParseMeta reads the companion file of a binary file. It never fails: a line
// it cannot read gives no value, and costs the binary file nothing.
func ParseMeta(doc []byte) Meta {
	return parseHeader(string(doc)).normalize()
}

// parseHeader reads the known keys of a header line by line. It is not a YAML
// decoder: agents write values unquoted, and YAML cuts a value at " #" and
// rejects one that holds ": " or starts with "[". Here the value is the rest of
// the line after the first colon, kept as written. A value in quotes is
// unquoted, and a "|" or ">" block scalar is decoded, as YAML does. A blank
// line, a comment, an unknown key or a line with no colon is skipped. When a
// key repeats, the last value wins.
func parseHeader(header string) frontmatter {
	var meta frontmatter
	fields := map[string]*string{
		"kind":        &meta.Kind,
		"uri":         &meta.URI,
		"role":        &meta.Role,
		"title":       &meta.Title,
		"description": &meta.Description,
	}

	lines := strings.Split(header, "\n")
	for i := 0; i < len(lines); i++ {
		line := strings.TrimRight(lines[i], "\r\t ")
		// An indented line here has no key above it to continue.
		if line == "" || isIndented(line) || line[0] == '#' || isDocumentMarker(line) {
			continue
		}

		key, value, found := strings.Cut(line, ":")
		if !found {
			continue
		}

		// The key takes its continuation lines even when it is unknown, so
		// that they cannot be read as keys of their own.
		var continuation []string
		for i+1 < len(lines) && continues(lines[i+1:]) {
			i++
			continuation = append(continuation, strings.TrimRight(lines[i], "\r"))
		}

		field, known := fields[strings.TrimSpace(key)]
		if !known {
			continue
		}

		*field = decodeValue(strings.TrimSpace(value), continuation)
	}

	return meta
}

// decodeValue gives the value of one key from the rest of its line and the
// indented lines that follow it.
func decodeValue(value string, continuation []string) string {
	if strings.HasPrefix(value, "|") || strings.HasPrefix(value, ">") {
		if decoded, ok := decodeScalar(value + "\n" + strings.Join(continuation, "\n") + "\n"); ok {
			return decoded
		}
	}

	// A quoted value with more text after its closing quote is not a YAML
	// string, so it fails to decode and is kept as written.
	if strings.HasPrefix(value, `"`) || strings.HasPrefix(value, "'") {
		if decoded, ok := decodeScalar(strings.Join(append([]string{value}, continuation...), "\n")); ok {
			return decoded
		}
	}

	// A plain value that runs on over indented lines is folded into one line.
	parts := []string{value}
	for _, line := range continuation {
		if line = strings.TrimSpace(line); line != "" {
			parts = append(parts, line)
		}
	}

	return strings.TrimSpace(strings.Join(parts, " "))
}

// decodeScalar decodes one YAML scalar. It reports false when YAML cannot read
// the scalar as a string, so that the caller keeps the value as written.
func decodeScalar(scalar string) (string, bool) {
	var decoded string
	if err := yaml.Unmarshal([]byte(scalar), &decoded); err != nil {
		return "", false
	}

	return decoded, true
}

// continues reports whether the first of lines belongs to the key above it: an
// indented line, or a blank line that an indented line follows, as in a block
// scalar with more than one paragraph.
func continues(lines []string) bool {
	for _, line := range lines {
		line = strings.TrimRight(line, "\r\t ")
		if line != "" {
			return isIndented(line)
		}
	}

	return false
}

func isIndented(line string) bool {
	return line != "" && (line[0] == ' ' || line[0] == '\t')
}

// isDocumentMarker reports a YAML document start or end line. A companion file
// can carry one.
func isDocumentMarker(line string) bool {
	return line == delimiter || line == "..."
}

// normalize drops a role outside the vocabulary, trims the title and the
// description, and cuts them to their limits.
func (m Meta) normalize() Meta {
	return Meta{
		Role:        aicodingsession.ResolveSpecRole(m.Role),
		Title:       truncate(strings.TrimSpace(m.Title), MaxTitleLen),
		Description: truncate(strings.TrimSpace(m.Description), MaxDescriptionLen),
	}
}

// truncate keeps the first limit characters of s, so a cut never splits one.
func truncate(s string, limit int) string {
	runes := []rune(s)
	if len(runes) <= limit {
		return s
	}

	return strings.TrimSpace(string(runes[:limit]))
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

// isImage reports whether a spec file is an image. Content sniffing finds the
// binary formats. An SVG is text, so only its extension tells it apart from a
// spec the agent wrote.
func isImage(name string, doc []byte) bool {
	if strings.EqualFold(filepath.Ext(name), ".svg") {
		return true
	}

	return strings.HasPrefix(http.DetectContentType(doc), "image/")
}
