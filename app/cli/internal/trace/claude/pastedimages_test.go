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
	"encoding/base64"
	"encoding/json"
	"testing"

	"github.com/chainloop-dev/chainloop/app/cli/internal/trace"
	"github.com/chainloop-dev/chainloop/app/cli/internal/trace/pointer"
	"github.com/stretchr/testify/assert"
)

var (
	pastedBytes = []byte("\x89PNG\r\n\x1a\npasted screenshot")
	otherPasted = []byte("\xff\xd8\xffanother pasted image")
)

const (
	pastedTime   = "2026-10-08T10:15:30.123Z"
	pastedUTC    = "2026-10-08T10:15:30Z"
	pngType      = "image/png"
	keyData      = "data"
	keyMediaType = "media_type"
	keySource    = "source"
)

// imageBlock is an image block with inline data, as the agent writes it.
func imageBlock(mediaType string, data []byte) obj {
	return obj{keyType: blockImage, keySource: obj{keyType: sourceBase64, keyMediaType: mediaType, keyData: base64.StdEncoding.EncodeToString(data)}}
}

// paste is a user prompt with pasted images, as Claude Code writes it: a
// text block, then one image block for each image, and the paste numbers
// that the user saw.
func paste(text string, pasteIDs []int, images ...obj) obj {
	content := make([]any, 0, 1+len(images))
	content = append(content, obj{keyType: resultText, resultText: text})
	for _, img := range images {
		content = append(content, img)
	}
	e := obj{keyType: typeUser, "timestamp": pastedTime, keyMessage: obj{"role": typeUser, keyContent: content}}
	if pasteIDs != nil {
		e["imagePasteIds"] = pasteIDs
	}

	return e
}

// queuedPaste is a prompt with pasted images that the user sent while the
// agent was busy, as Claude Code writes it: an attachment that holds the
// prompt blocks and the paste numbers. origin is the kind of its origin.
func queuedPaste(origin string, pasteIDs []int, images ...obj) obj {
	prompt := make([]any, 0, 1+len(images))
	prompt = append(prompt, obj{keyType: resultText, resultText: "look at this"})
	for _, img := range images {
		prompt = append(prompt, img)
	}

	return obj{keyType: "attachment", "timestamp": pastedTime, "attachment": obj{
		keyType: "queued_command", "prompt": prompt, "imagePasteIds": pasteIDs,
		"commandMode": "prompt", "origin": obj{"kind": origin}, "humanTurn": true,
	}}
}

func TestPastedImages(t *testing.T) {
	testCases := []struct {
		name string
		raw  map[string][]obj
		want []trace.PastedImage
	}{
		{
			name: "one pasted image with its paste number",
			raw: map[string][]obj{mainStream: {
				paste("[Image #1] add this image in the readme", []int{1}, imageBlock(pngType, pastedBytes)),
			}},
			want: []trace.PastedImage{{Data: pastedBytes, MediaType: pngType, Number: 1, Timestamp: pastedUTC}},
		},
		{
			name: "two images in two prompts keep the paste numbers the user saw",
			raw: map[string][]obj{mainStream: {
				paste("[Image #3] first", []int{3}, imageBlock(pngType, pastedBytes)),
				paste("[Image #4] second", []int{4}, imageBlock("image/jpeg", otherPasted)),
			}},
			want: []trace.PastedImage{
				{Data: pastedBytes, MediaType: pngType, Number: 3, Timestamp: pastedUTC},
				{Data: otherPasted, MediaType: "image/jpeg", Number: 4, Timestamp: pastedUTC},
			},
		},
		{
			name: "with no paste number, the number is the position in the session",
			raw: map[string][]obj{mainStream: {
				paste("first", nil, imageBlock(pngType, pastedBytes)),
				paste("second", nil, imageBlock(pngType, otherPasted)),
			}},
			want: []trace.PastedImage{
				{Data: pastedBytes, MediaType: pngType, Number: 1, Timestamp: pastedUTC},
				{Data: otherPasted, MediaType: pngType, Number: 2, Timestamp: pastedUTC},
			},
		},
		{
			name: "paste numbers that do not match the images give the position",
			raw: map[string][]obj{mainStream: {
				paste("two images", []int{7}, imageBlock(pngType, pastedBytes), imageBlock(pngType, otherPasted)),
			}},
			want: []trace.PastedImage{
				{Data: pastedBytes, MediaType: pngType, Number: 1, Timestamp: pastedUTC},
				{Data: otherPasted, MediaType: pngType, Number: 2, Timestamp: pastedUTC},
			},
		},
		{
			name: "the same image pasted two times is given one time, from its first paste",
			raw: map[string][]obj{mainStream: {
				paste("[Image #1]", []int{1}, imageBlock(pngType, pastedBytes)),
				paste("[Image #2]", []int{2}, imageBlock(pngType, pastedBytes)),
			}},
			want: []trace.PastedImage{
				{Data: pastedBytes, MediaType: pngType, Number: 1, Timestamp: pastedUTC},
			},
		},
		{
			name: "an image pasted while the agent was busy keeps its paste number",
			raw: map[string][]obj{mainStream: {
				paste("[Image #1] first", []int{1}, imageBlock(pngType, pastedBytes)),
				queuedPaste("human", []int{2}, imageBlock("image/jpeg", otherPasted)),
			}},
			want: []trace.PastedImage{
				{Data: pastedBytes, MediaType: pngType, Number: 1, Timestamp: pastedUTC},
				{Data: otherPasted, MediaType: "image/jpeg", Number: 2, Timestamp: pastedUTC},
			},
		},
		{
			name: "a queued prompt that a human did not send is not a paste",
			raw: map[string][]obj{mainStream: {
				queuedPaste("task-notification", []int{1}, imageBlock(pngType, pastedBytes)),
			}},
		},
		{
			name: "another attachment with an image is not a paste",
			raw: map[string][]obj{mainStream: {
				func() obj {
					e := queuedPaste("human", []int{1}, imageBlock(pngType, pastedBytes))
					e["attachment"].(obj)[keyType] = "skill_listing"
					return e
				}(),
			}},
		},
		{
			name: "an image in the result of a file read is not a pasted image",
			raw: map[string][]obj{mainStream: {
				readCall("r1", "/repo/screenshot.png", nil),
				imageResult("r1", pastedBytes, len(pastedBytes)),
			}},
		},
		{
			name: "an image with data that is not base64 is not given",
			raw: map[string][]obj{mainStream: {
				paste("[Image #1]", []int{1}, obj{keyType: blockImage, keySource: obj{keyType: sourceBase64, keyMediaType: pngType, keyData: "not base64!"}}),
			}},
		},
		{
			name: "an image with a URL source is not given",
			raw: map[string][]obj{mainStream: {
				paste("[Image #1]", []int{1}, obj{keyType: blockImage, keySource: obj{keyType: "url", "url": "https://example.com/a.png"}}),
			}},
		},
		{
			name: "an image in an assistant message is not given",
			raw: map[string][]obj{mainStream: {
				{keyType: typeAssistant, keyMessage: obj{"role": typeAssistant, keyContent: []any{imageBlock(pngType, pastedBytes)}}},
			}},
		},
		{
			name: "the main stream comes first, then the subagents",
			raw: map[string][]obj{
				"agent-b":  {paste("b", nil, imageBlock(pngType, otherPasted))},
				mainStream: {paste("main", nil, imageBlock(pngType, pastedBytes))},
			},
			want: []trace.PastedImage{
				{Data: pastedBytes, MediaType: pngType, Number: 1, Timestamp: pastedUTC},
				{Data: otherPasted, MediaType: pngType, Number: 2, Timestamp: pastedUTC},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			raw := make(map[string][]json.RawMessage, len(tc.raw))
			for stream, entries := range tc.raw {
				for _, e := range entries {
					raw[stream] = append(raw[stream], line(t, e))
				}
			}

			got := (&Provider{}).PastedImages(raw)

			for i := range tc.want {
				tc.want[i].Digest = pointer.Digest(tc.want[i].Data)
			}

			assert.Equal(t, tc.want, got)
		})
	}
}

func TestReplacePastedImages(t *testing.T) {
	const pastedMaterial = "sha256:pasted-material"
	sources := pointer.Sources{pointer.Digest(pastedBytes): pastedMaterial}

	testCases := []struct {
		name       string
		session    []obj
		check      func(t *testing.T, out []json.RawMessage)
		wantReport map[string]pointer.Result
	}{
		{
			name: "a pasted image that is a spec source becomes a pointer, with no tool call in the transcript",
			session: []obj{
				paste("[Image #1] add this image in the readme", []int{1}, imageBlock(pngType, pastedBytes)),
			},
			check: func(t *testing.T, out []json.RawMessage) {
				e := decode(t, out[0])
				// The text block and the image block keep their positions.
				assert.Equal(t, "[Image #1] add this image in the readme", at(t, e, keyMessage, keyContent, 0, resultText))
				assert.Equal(t, blockImage, at(t, e, keyMessage, keyContent, 1, keyType))
				assert.Equal(t, ptr(pastedMaterial), at(t, e, keyMessage, keyContent, 1, keySource))
				assert.Equal(t, pastedTime, at(t, e, "timestamp"))
			},
			wantReport: map[string]pointer.Result{pointer.FinderPastedImage: {Replaced: 1}},
		},
		{
			name: "a pasted image whose material was not stored stays inline",
			session: []obj{
				paste("[Image #1]", []int{1}, imageBlock(pngType, otherPasted)),
			},
			check: func(t *testing.T, out []json.RawMessage) {
				assert.Equal(t, "base64", at(t, decode(t, out[0]), keyMessage, keyContent, 1, keySource, keyType))
			},
			wantReport: map[string]pointer.Result{pointer.FinderPastedImage: {Skipped: map[string]int{pointer.ReasonMaterialNotStored: 1}}},
		},
		{
			name: "a pasted image with invalid data stays inline",
			session: []obj{
				paste("[Image #1]", []int{1}, obj{keyType: blockImage, keySource: obj{keyType: sourceBase64, keyMediaType: pngType, keyData: "not base64!"}}),
			},
			wantReport: map[string]pointer.Result{pointer.FinderPastedImage: {Skipped: map[string]int{pointer.ReasonInvalidImage: 1}}},
		},
		{
			name: "two pastes of the same image give two pointers",
			session: []obj{
				paste("[Image #1]", []int{1}, imageBlock(pngType, pastedBytes)),
				paste("[Image #2]", []int{2}, imageBlock(pngType, pastedBytes)),
			},
			wantReport: map[string]pointer.Result{pointer.FinderPastedImage: {Replaced: 2}},
		},
		{
			name: "an image pasted while the agent was busy becomes a pointer in the attachment",
			session: []obj{
				queuedPaste("human", []int{3}, imageBlock(pngType, pastedBytes)),
			},
			check: func(t *testing.T, out []json.RawMessage) {
				e := decode(t, out[0])
				assert.Equal(t, "look at this", at(t, e, "attachment", "prompt", 0, resultText))
				assert.Equal(t, blockImage, at(t, e, "attachment", "prompt", 1, keyType))
				assert.Equal(t, ptr(pastedMaterial), at(t, e, "attachment", "prompt", 1, keySource))
				// The other fields of the attachment are kept.
				assert.Equal(t, []any{float64(3)}, at(t, e, "attachment", "imagePasteIds"))
				assert.Equal(t, "queued_command", at(t, e, "attachment", keyType))
			},
			wantReport: map[string]pointer.Result{pointer.FinderPastedImage: {Replaced: 1}},
		},
		{
			// The file read finder looks at the image of the result, not
			// the pasted image finder.
			name: "an image in a file read result is not a pasted image",
			session: []obj{
				readCall("r1", "/repo/gone.png", nil),
				imageResult("r1", pastedBytes, len(pastedBytes)),
			},
			wantReport: map[string]pointer.Result{pointer.FinderFileRead: {Skipped: map[string]int{pointer.ReasonFileGone: 2}}},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			lines := make([]json.RawMessage, 0, len(tc.session))
			for _, e := range tc.session {
				lines = append(lines, line(t, e))
			}
			before := make([]json.RawMessage, len(lines))
			copy(before, lines)

			raw := map[string][]json.RawMessage{mainStream: lines}
			report := replaceSpecCopies(raw, sources)

			got, bytesRemoved := results(report)
			assert.Equal(t, tc.wantReport, got)
			if bytesRemoved == 0 {
				assert.Equal(t, before, raw[mainStream])
			}

			if tc.check != nil {
				tc.check(t, raw[mainStream])
			}
		})
	}
}

// TestReplacePastedImagesSubagents checks that the pasted images of each
// stream are replaced.
func TestReplacePastedImagesSubagents(t *testing.T) {
	raw := map[string][]json.RawMessage{
		mainStream: {line(t, obj{keyType: typeUser, keyMessage: obj{keyContent: "hi"}})},
		"agent-ab": {line(t, paste("look", nil, imageBlock(pngType, pastedBytes)))},
	}

	report := replaceSpecCopies(raw, pointer.Sources{pointer.Digest(pastedBytes): "sha256:m"})

	assert.Equal(t, 1, report[pointer.FinderPastedImage].Replaced)
	assert.NotContains(t, string(raw["agent-ab"][0]), base64.StdEncoding.EncodeToString(pastedBytes))
}
