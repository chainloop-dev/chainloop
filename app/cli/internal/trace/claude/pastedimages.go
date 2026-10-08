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
	// recordTypeAttachment is a record that Claude Code adds to the
	// conversation, such as a prompt that the user queued.
	recordTypeAttachment    = "attachment"
	attachmentQueuedCommand = "queued_command"
	originHuman             = "human"
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
			pl := pastedImages(l)
			if pl == nil {
				continue
			}
			for _, b := range pl.images {
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
					Timestamp: normalizeTimestamp(pl.fields.str("timestamp")),
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
			pl := pastedImages(l)
			if pl == nil {
				continue
			}

			res := report.Finder(pointer.FinderPastedImage)
			changed := false
			for _, b := range pl.images {
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
				changed = true
			}

			if !changed {
				continue
			}
			// A line that cannot be written again keeps its content inline.
			if out, err := pl.encode(); err == nil {
				lines[i] = out
			}
		}
	}
}

// pastedImage is one pasted image block of a transcript line.
type pastedImage struct {
	// block is the image block, which belongs to the prompt of the line.
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

// pastedLine is a transcript line that holds pasted images.
type pastedLine struct {
	fields object
	images []pastedImage
	// encode writes the line again, with its changed image blocks.
	encode func() (json.RawMessage, error)
}

// pastedImages decodes a line that holds pasted images, or returns nil. Claude
// Code writes a prompt in two shapes: a user message, or, when the user sent
// it while the agent was busy, a queued command attachment. A queued prompt
// is not written again as a user message, so both shapes hold pastes.
func pastedImages(line json.RawMessage) *pastedLine {
	if !bytes.Contains(line, []byte(`"`+blockImage+`"`)) || !bytes.Contains(line, []byte(`"`+sourceBase64+`"`)) {
		return nil
	}

	fields := decodeObject(line)
	switch fields.str("type") {
	case recordTypeUser:
		e := newEntry(fields)
		if e == nil || !isUserPrompt(e) {
			return nil
		}

		return newPastedLine(fields, e.content, fields["imagePasteIds"], e.encode)
	case recordTypeAttachment:
		att := fields.object("attachment")
		if !isQueuedPrompt(att) {
			return nil
		}
		prompt := att.objects("prompt")

		return newPastedLine(fields, prompt, att["imagePasteIds"], func() (json.RawMessage, error) {
			raw, err := marshal(prompt)
			if err != nil {
				return nil, err
			}
			att["prompt"] = raw
			if fields["attachment"], err = marshal(att); err != nil {
				return nil, err
			}

			return marshal(fields)
		})
	}

	return nil
}

// newPastedLine selects the image blocks with inline data of a prompt, in
// order, or returns nil when there is none.
func newPastedLine(fields object, prompt []object, pasteIDs json.RawMessage, encode func() (json.RawMessage, error)) *pastedLine {
	var images []pastedImage
	for _, b := range prompt {
		if b.str("type") == blockImage && b.object("source").str("type") == sourceBase64 {
			images = append(images, pastedImage{block: b})
		}
	}
	if len(images) == 0 {
		return nil
	}

	// Claude Code records the paste numbers that the user saw, one for each
	// image of the prompt. When they do not match the images, none is used.
	var ids []int
	if json.Unmarshal(pasteIDs, &ids) == nil && len(ids) == len(images) {
		for i := range images {
			images[i].number = ids[i]
		}
	}

	return &pastedLine{fields: fields, images: images, encode: encode}
}

// isQueuedPrompt reports whether an attachment is a prompt that the user
// sent while the agent was busy, and not one that Claude Code queued itself.
func isQueuedPrompt(att object) bool {
	if att.str("type") != attachmentQueuedCommand {
		return false
	}
	origin := att.object("origin")

	return origin == nil || origin.str("kind") == originHuman
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
// to look at them: the main stream first, then the subagents by name.
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
