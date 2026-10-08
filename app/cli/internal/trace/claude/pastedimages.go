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
	"errors"
	"sort"

	"github.com/chainloop-dev/chainloop/app/cli/internal/trace"
	"github.com/chainloop-dev/chainloop/app/cli/internal/trace/pointer"
)

var _ trace.PastedImageFinder = (*Provider)(nil)

const (
	blockImage   = "image"
	sourceBase64 = "base64"
)

// PastedImages returns the images that the user pasted into the session
// (spec issue-3569). A pasted image is an image block with inline data in a
// user prompt. An image in a tool result is not one. An image pasted many
// times is returned one time, from its first paste.
func (p *Provider) PastedImages(raw map[string][]json.RawMessage) []trace.PastedImage {
	var out []trace.PastedImage
	seen := make(map[string]bool)
	position := 0
	for _, key := range streamOrder(raw) {
		for _, l := range raw[key] {
			e, blocks := pastedImages(l)
			for _, b := range blocks {
				position++
				data, err := b.decode()
				if err != nil {
					continue
				}
				digest := pointer.Digest(data)
				if seen[digest] {
					continue
				}
				seen[digest] = true

				number := b.number
				if number == 0 {
					number = position
				}
				out = append(out, trace.PastedImage{
					Data:      data,
					Digest:    digest,
					MediaType: b.block.object("source").str("media_type"),
					Number:    number,
					Timestamp: normalizeTimestamp(e.fields.str("timestamp")),
				})
			}
		}
	}

	return out
}

// replacePastedImages replaces the data of each pasted image whose digest is
// a source with a pointer to its material. The block keeps its type and its
// position, and the pointer becomes its source, as for an image in a file
// read. It looks at every stream, also one with no tool call.
func replacePastedImages(raw map[string][]json.RawMessage, sources pointer.Sources, report pointer.Report) {
	for _, lines := range raw {
		for i, l := range lines {
			e, blocks := pastedImages(l)
			if len(blocks) == 0 {
				continue
			}

			res := report.Finder(pointer.FinderPastedImage)
			for _, b := range blocks {
				data, err := b.decode()
				if err != nil {
					res.Skip(pointer.ReasonInvalidImage, 1)
					continue
				}
				// The push stores each pasted image before the replacement,
				// so an image with no source is one it could not store.
				material, ok := sources[pointer.Digest(data)]
				if !ok {
					res.Skip(pointer.ReasonMaterialNotStored, 1)
					continue
				}

				res.BytesRemoved += len(b.block["source"])
				b.block["source"] = pointer.New(material)
				res.Replaced++
				e.changed = true
			}

			if !e.changed {
				continue
			}
			// A line that cannot be written again keeps its content inline.
			if out, err := e.encode(); err == nil {
				lines[i] = out
			}
		}
	}
}

// pastedImage is one pasted image block of a transcript entry.
type pastedImage struct {
	// block is the image block, which belongs to the content of the entry.
	block object
	// number is the number of the paste that the agent showed to the user,
	// or 0 when it showed none.
	number int
}

var errNoImageData = errors.New("no image data")

// decode returns the image that the block holds.
func (b pastedImage) decode() ([]byte, error) {
	encoded, ok := b.block.object("source").string("data")
	if !ok {
		return nil, errNoImageData
	}
	data, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return nil, err
	}
	if len(data) == 0 {
		return nil, errNoImageData
	}

	return data, nil
}

// pastedImages decodes a line that is a user prompt with pasted images, and
// returns the entry and its image blocks with inline data, in order. It
// returns no blocks for any other line.
func pastedImages(line json.RawMessage) (*entry, []pastedImage) {
	if !bytes.Contains(line, []byte(`"`+blockImage+`"`)) || !bytes.Contains(line, []byte(`"`+sourceBase64+`"`)) {
		return nil, nil
	}

	e := decodeEntry(line)
	if e == nil || !isUserPrompt(e) {
		return nil, nil
	}

	var blocks []pastedImage
	for _, b := range e.content {
		if b.str("type") != blockImage {
			continue
		}
		if b.object("source").str("type") != sourceBase64 {
			continue
		}
		blocks = append(blocks, pastedImage{block: b})
	}

	// Claude Code records the paste numbers that the user saw, one for each
	// image of the prompt. When they do not match the images, none is used.
	var ids []int
	if v, ok := e.fields["imagePasteIds"]; ok && json.Unmarshal(v, &ids) == nil && len(ids) == len(blocks) {
		for i := range blocks {
			blocks[i].number = ids[i]
		}
	}

	return e, blocks
}

// isUserPrompt reports whether the entry is a message that the user wrote,
// and not a tool result, a skill that a tool loaded or a message that Claude
// Code added.
func isUserPrompt(e *entry) bool {
	if e.fields.str("type") != recordTypeUser || e.fields.bool("isMeta") ||
		e.fields.has("toolUseResult") || e.fields.has("sourceToolUseID") {
		return false
	}
	for _, b := range e.content {
		if b.str("type") == blockToolResult {
			return false
		}
	}

	return true
}

// streamOrder returns the keys of the streams of a raw session in the order
// of the session: the main stream first, then the subagents by name.
func streamOrder(raw map[string][]json.RawMessage) []string {
	keys := make([]string, 0, len(raw))
	for k := range raw {
		if k != mainTranscriptKey {
			keys = append(keys, k)
		}
	}
	sort.Strings(keys)
	if _, ok := raw[mainTranscriptKey]; ok {
		keys = append([]string{mainTranscriptKey}, keys...)
	}

	return keys
}
