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
	"encoding/json"

	"github.com/chainloop-dev/chainloop/app/cli/internal/trace/pointer"
)

// recordTypeRequestBlob is a record in which Claude Code keeps a copy of the
// blocks of one message of an API request: the prompt, the tool calls and
// their results. It holds the same copies of spec sources as the entries, a
// second time (issue 3654).
const recordTypeRequestBlob = "api-request-blob"

// isRequestBlob reports whether a transcript line is a request blob.
func isRequestBlob(line json.RawMessage) bool {
	if !bytes.Contains(line, []byte(`"`+recordTypeRequestBlob+`"`)) {
		return false
	}

	var head struct {
		Type string `json:"type"`
	}

	return json.Unmarshal(line, &head) == nil && head.Type == recordTypeRequestBlob
}

// requestBlobLines returns the positions of the request blobs of a stream.
func requestBlobLines(lines []json.RawMessage) map[int]bool {
	blobs := make(map[int]bool)
	for i, l := range lines {
		if isRequestBlob(l) {
			blobs[i] = true
		}
	}

	return blobs
}

// requestBlob replaces the copies of spec sources in one request blob, and
// returns its entry, or nil when the blob holds no blocks. A tool call or a
// tool result gets the pointer of the entry with the same tool call ID, only
// when it holds the same value: the copy in the request can differ, for
// example when the agent added text to it. An image gets a pointer when its
// data is a spec source.
func (r *specCopies) requestBlob(line json.RawMessage) *entry {
	e := decodeEntry(line)
	if e == nil {
		return nil
	}

	for _, b := range e.content {
		switch b.str("type") {
		case blockToolUse:
			c, ok := r.inputs[b.str("id")]
			input := b.object("input")
			if !ok || !sameJSON(input["content"], c.original) {
				continue
			}
			res := r.report.Finder(pointer.FinderFileWrite)
			res.BytesRemoved += swap(input, "content", c.pointer)
			res.Replaced++
			e.set(b, "input", input)
		case blockToolResult:
			if c, ok := r.results[b.str("tool_use_id")]; ok && sameJSON(b["content"], c.original) {
				res := r.report.Finder(pointer.FinderFileRead)
				res.BytesRemoved += len(b["content"])
				res.Replaced++
				b["content"] = c.pointer
				e.changed = true
				continue
			}
			// An image that the agent read is a block of the content.
			blocks := b.objects("content")
			n := 0
			for _, img := range blocks {
				if r.requestImage(img, pointer.FinderFileRead) {
					n++
				}
			}
			if n > 0 {
				e.set(b, "content", blocks)
			}
		case blockImage:
			if r.requestImage(b, pointer.FinderPastedImage) {
				e.changed = true
			}
		}
	}

	return e
}

// requestImage replaces the data of an image block whose image is a spec
// source with a pointer to its material. It reports whether it replaced it.
func (r *specCopies) requestImage(b object, finder string) bool {
	if b.str("type") != blockImage || b.object("source").str("type") != sourceBase64 {
		return false
	}
	data, err := pastedImage{block: b}.decode()
	if err != nil {
		return false
	}
	material, ok := r.sources[pointer.Digest(data)]
	if !ok {
		return false
	}

	res := r.report.Finder(finder)
	res.BytesRemoved += len(b["source"])
	res.Replaced++
	b["source"] = pointer.New(material)

	return true
}

// sameJSON reports whether two encoded JSON values are equal, apart from
// white space.
func sameJSON(a, b json.RawMessage) bool {
	if len(a) == 0 || len(b) == 0 {
		return false
	}
	var ca, cb bytes.Buffer
	if json.Compact(&ca, a) != nil || json.Compact(&cb, b) != nil {
		return false
	}

	return bytes.Equal(ca.Bytes(), cb.Bytes())
}
