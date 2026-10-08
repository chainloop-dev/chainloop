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

package claude

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/chainloop-dev/chainloop/app/cli/internal/trace"
	"github.com/chainloop-dev/chainloop/app/cli/internal/trace/pointer"
)

var _ trace.SpecCopyReplacer = (*Provider)(nil)

// The tools whose calls can hold a copy of a spec source.
const (
	toolRead  = "Read"
	toolWrite = "Write"
)

// The types of the blocks and of the metadata that hold a copy.
const (
	blockToolUse    = "tool_use"
	blockToolResult = "tool_result"
	// resultText is the type of the metadata of a read of a text file.
	resultText = "text"
)

// ReplaceSpecCopies replaces the copies of spec sources in a Claude Code
// transcript with pointers (spec issue-3556). A copy is a full read of a file
// whose content on disk is a source, or a write of the content of a source.
// Only the fields that hold a copy are rewritten; a line with nothing to
// replace keeps its bytes.
func (p *Provider) ReplaceSpecCopies(raw map[string][]json.RawMessage, sources pointer.Sources) pointer.Report {
	report := pointer.Report{}
	if len(sources) == 0 {
		return report
	}

	r := &specCopies{sources: sources, files: make(map[string]fileDigest), report: report}
	for _, lines := range raw {
		r.stream(lines)
	}

	return report
}

type specCopies struct {
	sources pointer.Sources
	// files holds the digest of each file read, so that a file read many
	// times is read from disk one time.
	files  map[string]fileDigest
	report pointer.Report
}

type fileDigest struct {
	digest string
	err    error
}

// toolCall is a call of the read or the write tool, with the entry and the
// block that hold it, and the entry and the block of its result.
type toolCall struct {
	name        string
	entry       *entry
	block       object
	input       object
	result      *entry
	resultBlock object
}

// blockHeads is the part of a line that tells whether the line holds a call
// or a result to look at. Decoding it skips the other fields with no copy,
// so most lines are never decoded in full.
type blockHeads struct {
	Message struct {
		Content []struct {
			Type      string `json:"type"`
			Name      string `json:"name"`
			ToolUseID string `json:"tool_use_id"`
		} `json:"content"`
	} `json:"message"`
}

// stream replaces the copies in one stream of the session: the main
// conversation or one subagent.
func (r *specCopies) stream(lines []json.RawMessage) {
	entries := make(map[int]*entry)
	calls := make(map[string]*toolCall)
	var order []*toolCall

	for i, l := range lines {
		if !hasBlock(l, blockToolUse, func(t, name, _ string) bool { return t == blockToolUse && (name == toolRead || name == toolWrite) }) {
			continue
		}
		e := decodeEntry(l)
		if e == nil {
			continue
		}
		entries[i] = e
		for _, b := range e.content {
			name := b.str("name")
			if b.str("type") != blockToolUse || (name != toolRead && name != toolWrite) {
				continue
			}
			if input := b.object("input"); input != nil {
				c := &toolCall{name: name, entry: e, block: b, input: input}
				calls[b.str("id")] = c
				order = append(order, c)
			}
		}
	}

	if len(calls) == 0 {
		return
	}

	for i, l := range lines {
		if !hasBlock(l, blockToolResult, func(t, _, id string) bool { return t == blockToolResult && calls[id] != nil }) {
			continue
		}
		e := entries[i]
		if e == nil {
			if e = decodeEntry(l); e == nil {
				continue
			}
			entries[i] = e
		}
		for _, b := range e.content {
			if c := calls[b.str("tool_use_id")]; c != nil && b.str("type") == blockToolResult {
				c.result, c.resultBlock = e, b
			}
		}
	}

	for _, c := range order {
		switch c.name {
		case toolRead:
			r.read(c)
		case toolWrite:
			r.write(c)
		}
	}

	for i, e := range entries {
		if !e.changed {
			continue
		}
		// A line that cannot be written again keeps its content inline.
		if out, err := e.encode(); err == nil {
			lines[i] = out
		}
	}
}

// hasBlock reports whether the line holds a block that match accepts, from
// its type, its tool name and the ID of the call it answers. The line must
// hold the text blockType before it is decoded at all.
func hasBlock(line json.RawMessage, blockType string, match func(blockType, name, toolUseID string) bool) bool {
	if !bytes.Contains(line, []byte(`"`+blockType+`"`)) {
		return false
	}

	var heads blockHeads
	if json.Unmarshal(line, &heads) != nil {
		return false
	}
	for _, h := range heads.Message.Content {
		if match(h.Type, h.Name, h.ToolUseID) {
			return true
		}
	}

	return false
}

// read replaces the copies of one full read of a file whose content on disk
// is a spec source: the content of the tool result, and the copy in its
// metadata.
func (r *specCopies) read(c *toolCall) {
	res := r.report.Finder(pointer.FinderFileRead)
	if c.result == nil {
		// The call has no result, so there is no copy.
		return
	}

	meta := c.result.toolUseResult()
	file := meta.object("file")
	kind := meta.str("type")
	// The field of the metadata that holds the copy.
	field := "base64"
	if kind == resultText {
		field = "content"
	}

	copies := 1
	if file.has(field) {
		copies++
	}

	material, reason := r.readMatch(c, kind, file)
	if reason != "" {
		res.Skip(reason, copies)
		return
	}

	p := pointer.New(material)
	replaced, removed := 0, 0

	if kind == resultText {
		removed += len(c.resultBlock["content"])
		c.resultBlock["content"] = json.RawMessage("[" + string(p) + "]")
		replaced++
	} else if blocks := c.resultBlock.objects("content"); blocks != nil {
		// A media block keeps its type, and the pointer becomes its source.
		n := 0
		for _, b := range blocks {
			if b.object("source").str("type") == "base64" {
				removed += len(b["source"])
				b["source"] = p
				n++
			}
		}
		if n > 0 {
			c.result.set(c.resultBlock, "content", blocks)
			replaced += n
		}
	}

	if file.has(field) {
		removed += swap(file, field, p)
		c.result.set(meta, "file", file)
		c.result.set(c.result.fields, "toolUseResult", meta)
		replaced++
	}

	if replaced > 0 {
		c.result.changed = true
	}
	res.Replaced += replaced
	res.BytesRemoved += removed
}

// write replaces the copies of one write of a file whose written content is a
// spec source: the content in the input of the call, and the copy in the
// metadata of its result.
func (r *specCopies) write(c *toolCall) {
	res := r.report.Finder(pointer.FinderFileWrite)

	content, ok := c.input.string("content")
	if !ok {
		return
	}

	var meta object
	if c.result != nil {
		meta = c.result.toolUseResult()
	}
	metaContent, metaOK := meta.string("content")
	// The metadata holds a copy only when it holds the same content.
	metaCopy := metaOK && metaContent == content

	copies := 1
	if metaCopy {
		copies++
	}

	material, ok := r.sources[pointer.Digest([]byte(content))]
	if !ok {
		res.Skip(pointer.ReasonNoSpecEntry, copies)
		return
	}

	p := pointer.New(material)

	res.BytesRemoved += swap(c.input, "content", p)
	c.entry.set(c.block, "input", c.input)
	res.Replaced++

	if metaCopy {
		res.BytesRemoved += swap(meta, "content", p)
		c.result.set(c.result.fields, "toolUseResult", meta)
		res.Replaced++
	}
}

// readMatch returns the material of the spec source that a read is a full
// copy of, or the reason why the read is not one.
func (r *specCopies) readMatch(c *toolCall, kind string, file object) (material, reason string) {
	if c.resultBlock.bool("is_error") {
		return "", pointer.ReasonToolError
	}

	if c.input.has("offset") || c.input.has("limit") || c.input.has("pages") {
		return "", pointer.ReasonPartialRead
	}

	switch kind {
	case resultText:
		// The file tool cuts a long file. Only a read from the first line to
		// the last is a copy.
		start, okStart := file.int("startLine")
		num, okNum := file.int("numLines")
		total, okTotal := file.int("totalLines")
		if !okStart || !okNum || !okTotal || start != 1 || num != total {
			return "", pointer.ReasonPartialRead
		}
	case "image", "pdf":
	default:
		return "", pointer.ReasonUnsupported
	}

	path := c.input.str("file_path")
	if !filepath.IsAbs(path) {
		cwd := c.entry.fields.str("cwd")
		if path == "" || cwd == "" {
			return "", pointer.ReasonFileGone
		}
		path = filepath.Join(cwd, path)
	}

	info, err := os.Stat(path)
	if err != nil {
		return "", pointer.ReasonFileGone
	}

	// The file can change after the read and become a source later. When the
	// agent recorded the size of the file it read, a different size shows that.
	if size, ok := file.int("originalSize"); ok && int64(size) != info.Size() {
		return "", pointer.ReasonFileChanged
	}

	fd := r.digest(path)
	if fd.err != nil {
		return "", pointer.ReasonFileGone
	}

	if !readAsOnDisk(kind, file, fd.digest) {
		return "", pointer.ReasonFileChanged
	}

	material, ok := r.sources[fd.digest]
	if !ok {
		return "", pointer.ReasonNoSpecEntry
	}

	return material, ""
}

// readAsOnDisk reports whether the metadata of a read shows that the agent
// read the file as it is on disk, whose digest is digest. The metadata of a
// text read holds the file as the agent read it, so the two must be equal.
// The metadata of a media read holds the bytes the agent saw, which are the
// file only when the file tool did not resize it, that is when they have the
// original size. A resized image cannot be compared; its size was checked.
func readAsOnDisk(kind string, file object, digest string) bool {
	if kind == resultText {
		content, ok := file.string("content")
		return ok && pointer.Digest([]byte(content)) == digest
	}

	encoded, ok := file.string("base64")
	if !ok {
		return true
	}
	data, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return false
	}
	if size, ok := file.int("originalSize"); ok && size != len(data) {
		return true
	}

	return pointer.Digest(data) == digest
}

// digest returns the digest of the file at path, from disk the first time.
func (r *specCopies) digest(path string) fileDigest {
	if fd, ok := r.files[path]; ok {
		return fd
	}

	var fd fileDigest
	f, err := os.Open(path)
	if err != nil {
		fd.err = err
	} else {
		fd.digest, fd.err = pointer.DigestReader(f)
		_ = f.Close()
	}
	r.files[path] = fd

	return fd
}

// swap replaces the text field key of o with a field that holds the pointer,
// and returns the size of what it removed. A consumer that reads the old
// field does not get an object where it expects text.
func swap(o object, key string, p json.RawMessage) int {
	n := len(o[key])
	delete(o, key)
	o[pointer.Marker] = p

	return n
}

// entry is one decoded line of the transcript.
type entry struct {
	fields  object
	message object
	content []object
	// pending are the decoded values to write back into their parents when
	// the entry is encoded, inner ones first.
	pending []pendingWrite
	changed bool
}

type pendingWrite struct {
	parent object
	key    string
	value  any
}

// decodeEntry decodes a line whose message holds blocks, or returns nil.
func decodeEntry(line json.RawMessage) *entry {
	fields := decodeObject(line)
	message := fields.object("message")
	content := message.objects("content")
	if content == nil {
		return nil
	}

	return &entry{fields: fields, message: message, content: content}
}

// set records that value goes back into parent at key when the entry is
// encoded.
func (e *entry) set(parent object, key string, value any) {
	e.pending = append(e.pending, pendingWrite{parent: parent, key: key, value: value})
	e.changed = true
}

// toolUseResult returns the metadata of the tool result that the entry
// holds. The metadata belongs to the entry, so it is used only when the entry
// holds one tool result.
func (e *entry) toolUseResult() object {
	n := 0
	for _, b := range e.content {
		if b.str("type") == blockToolResult {
			n++
		}
	}
	if n != 1 {
		return nil
	}

	return e.fields.object("toolUseResult")
}

// encode writes the entry again, with its changed values.
func (e *entry) encode() (json.RawMessage, error) {
	for _, w := range e.pending {
		raw, err := marshal(w.value)
		if err != nil {
			return nil, err
		}
		w.parent[w.key] = raw
	}

	content, err := marshal(e.content)
	if err != nil {
		return nil, err
	}
	e.message["content"] = content

	message, err := marshal(e.message)
	if err != nil {
		return nil, err
	}
	e.fields["message"] = message

	return marshal(e.fields)
}

// object is a decoded JSON object whose values keep their encoded form, so
// that the fields that are not changed are written again as they were.
type object map[string]json.RawMessage

func decodeObject(raw json.RawMessage) object {
	var o object
	if json.Unmarshal(raw, &o) != nil {
		return nil
	}

	return o
}

func (o object) has(key string) bool {
	v, ok := o[key]
	return ok && !bytes.Equal(v, []byte("null"))
}

func (o object) object(key string) object {
	if v, ok := o[key]; ok {
		return decodeObject(v)
	}

	return nil
}

// objects returns the value at key when it is an array of objects.
func (o object) objects(key string) []object {
	var out []object
	if v, ok := o[key]; !ok || json.Unmarshal(v, &out) != nil {
		return nil
	}

	return out
}

func (o object) string(key string) (string, bool) {
	var s string
	if v, ok := o[key]; ok && json.Unmarshal(v, &s) == nil {
		return s, true
	}

	return "", false
}

func (o object) str(key string) string {
	s, _ := o.string(key)
	return s
}

func (o object) int(key string) (int, bool) {
	var n int
	if v, ok := o[key]; ok && json.Unmarshal(v, &n) == nil {
		return n, true
	}

	return 0, false
}

func (o object) bool(key string) bool {
	var b bool
	if v, ok := o[key]; ok {
		_ = json.Unmarshal(v, &b)
	}

	return b
}

// marshal encodes v with no HTML escaping, as the agent writes its
// transcript, so that the fields that are not changed keep their text.
func marshal(v any) (json.RawMessage, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(v); err != nil {
		return nil, err
	}

	return bytes.TrimRight(buf.Bytes(), "\n"), nil
}
