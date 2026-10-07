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
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/chainloop-dev/chainloop/app/cli/internal/trace/pointer"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	imageMaterial  = "sha256:image-material"
	ticketMaterial = "sha256:ticket-material"
	jsonMaterial   = "sha256:json-material"
)

// Values of the transcript that the tests repeat.
const (
	mainStream       = "main"
	typeUser         = "user"
	typeAssistant    = "assistant"
	keyContent       = "content"
	keyMessage       = "message"
	keyType          = "type"
	keyToolUseID     = "tool_use_id"
	keyToolUseResult = "toolUseResult"
)

var (
	imageBytes = []byte("\x89PNG\r\n\x1a\nnot really a png")
	ticketText = "---\nkind: ticket\n---\n\n# ENG-1234 <export>\n"
	jsonText   = `{"name": "config"}` + "\n"
)

// line encodes one transcript entry as the agent writes it, with no HTML
// escaping.
func line(t *testing.T, v any) json.RawMessage {
	t.Helper()

	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	require.NoError(t, enc.Encode(v))

	return bytes.TrimSpace(buf.Bytes())
}

type obj = map[string]any

func readCall(id, path string, input obj) obj {
	in := obj{"file_path": path}
	for k, v := range input {
		in[k] = v
	}

	return obj{keyType: typeAssistant, "cwd": "/repo", keyMessage: obj{keyContent: []any{
		obj{keyType: blockToolUse, "id": id, "name": "Read", "input": in},
	}}}
}

func imageResult(id string, data []byte, originalSize int) obj {
	b64 := base64.StdEncoding.EncodeToString(data)

	return obj{keyType: typeUser, keyMessage: obj{keyContent: []any{
		obj{keyType: blockToolResult, keyToolUseID: id, keyContent: []any{
			obj{keyType: "image", "source": obj{keyType: "base64", "media_type": "image/png", "data": b64}},
		}},
	}}, keyToolUseResult: obj{keyType: "image", "file": obj{"base64": b64, keyType: "image/png", "originalSize": originalSize}}}
}

func textResult(id, path, content string, startLine, numLines, totalLines int) obj {
	return obj{keyType: typeUser, keyMessage: obj{keyContent: []any{
		obj{keyType: blockToolResult, keyToolUseID: id, keyContent: "     1\t" + content},
	}}, keyToolUseResult: obj{keyType: resultText, "file": obj{
		"filePath": path, keyContent: content, "startLine": startLine, "numLines": numLines, "totalLines": totalLines,
	}}}
}

func writeCall(id, path, content string) obj {
	return obj{keyType: typeAssistant, keyMessage: obj{keyContent: []any{
		obj{keyType: blockToolUse, "id": id, "name": "Write", "input": obj{"file_path": path, keyContent: content}},
	}}}
}

func writeResult(id, path, content string) obj {
	return obj{keyType: typeUser, keyMessage: obj{keyContent: []any{
		obj{keyType: blockToolResult, keyToolUseID: id, keyContent: "File created successfully at: " + path},
	}}, keyToolUseResult: obj{keyType: "create", "filePath": path, keyContent: content, "structuredPatch": []any{}, "originalFile": nil}}
}

// decode reads back one transcript entry.
func decode(t *testing.T, raw json.RawMessage) obj {
	t.Helper()

	var out obj
	require.NoError(t, json.Unmarshal(raw, &out))

	return out
}

// at walks a decoded entry by keys and array indices.
func at(t *testing.T, v any, path ...any) any {
	t.Helper()

	for _, p := range path {
		switch k := p.(type) {
		case string:
			m, ok := v.(obj)
			require.True(t, ok, "not an object at %v", p)
			v = m[k]
		case int:
			a, ok := v.([]any)
			require.True(t, ok, "not an array at %v", p)
			require.Less(t, k, len(a))
			v = a[k]
		}
	}

	return v
}

func ptr(digest string) obj {
	return obj{keyType: pointer.Marker, "digest": digest}
}

func TestReplace(t *testing.T) {
	dir := t.TempDir()
	screenshot := filepath.Join(dir, "screenshot.png")
	require.NoError(t, os.WriteFile(screenshot, imageBytes, 0o600))
	other := filepath.Join(dir, "other.png")
	require.NoError(t, os.WriteFile(other, []byte("another image"), 0o600))
	config := filepath.Join(dir, "config.json")
	require.NoError(t, os.WriteFile(config, []byte(jsonText), 0o600))
	ticket := filepath.Join(dir, ".chainloop", "specs", "s1", "ticket-eng-1234.md")

	sources := pointer.Sources{
		pointer.Digest(imageBytes):         imageMaterial,
		pointer.Digest([]byte(ticketText)): ticketMaterial,
		pointer.Digest([]byte(jsonText)):   jsonMaterial,
	}

	testCases := []struct {
		name    string
		session []obj
		// check inspects the transcript after the replacement.
		check      func(t *testing.T, out []json.RawMessage)
		wantReport map[string]pointer.Result
	}{
		{
			name: "full read of an image that is a spec source",
			session: []obj{
				readCall("r1", screenshot, nil),
				imageResult("r1", imageBytes, len(imageBytes)),
			},
			check: func(t *testing.T, out []json.RawMessage) {
				res := decode(t, out[1])
				block := at(t, res, keyMessage, keyContent, 0, keyContent, 0)
				assert.Equal(t, "image", at(t, block, keyType))
				assert.Equal(t, ptr(imageMaterial), at(t, block, "source"))

				file := at(t, res, keyToolUseResult, "file").(obj)
				assert.NotContains(t, file, "base64")
				assert.Equal(t, ptr(imageMaterial), file[pointer.Marker])
				assert.Equal(t, "image/png", file[keyType])
			},
			wantReport: map[string]pointer.Result{pointer.FinderFileRead: {Replaced: 2}},
		},
		{
			name: "full read of a text file that is a spec source",
			session: []obj{
				readCall("r1", config, nil),
				textResult("r1", config, jsonText, 1, 2, 2),
			},
			check: func(t *testing.T, out []json.RawMessage) {
				res := decode(t, out[1])
				assert.Equal(t, []any{ptr(jsonMaterial)}, at(t, res, keyMessage, keyContent, 0, keyContent))

				file := at(t, res, keyToolUseResult, "file").(obj)
				assert.NotContains(t, file, keyContent)
				assert.Equal(t, ptr(jsonMaterial), file[pointer.Marker])
				assert.Equal(t, config, file["filePath"])
			},
			wantReport: map[string]pointer.Result{pointer.FinderFileRead: {Replaced: 2}},
		},
		{
			name: "read of a relative path resolves against the session directory",
			session: []obj{
				func() obj {
					c := readCall("r1", "config.json", nil)
					c["cwd"] = dir
					return c
				}(),
				textResult("r1", "config.json", jsonText, 1, 2, 2),
			},
			wantReport: map[string]pointer.Result{pointer.FinderFileRead: {Replaced: 2}},
		},
		{
			name: "read of an image that is not a spec source stays inline",
			session: []obj{
				readCall("r1", other, nil),
				imageResult("r1", []byte("another image"), len("another image")),
			},
			check: func(t *testing.T, out []json.RawMessage) {
				res := decode(t, out[1])
				assert.Equal(t, "base64", at(t, res, keyMessage, keyContent, 0, keyContent, 0, "source", keyType))
			},
			wantReport: map[string]pointer.Result{pointer.FinderFileRead: {Skipped: map[string]int{pointer.ReasonNoSpecEntry: 2}}},
		},
		{
			name: "read with a line range stays inline",
			session: []obj{
				readCall("r1", config, obj{"offset": 1, "limit": 1}),
				textResult("r1", config, jsonText, 1, 2, 2),
			},
			wantReport: map[string]pointer.Result{pointer.FinderFileRead: {Skipped: map[string]int{pointer.ReasonPartialRead: 2}}},
		},
		{
			name: "read cut by the file tool stays inline",
			session: []obj{
				readCall("r1", config, nil),
				textResult("r1", config, jsonText, 1, 1, 2),
			},
			wantReport: map[string]pointer.Result{pointer.FinderFileRead: {Skipped: map[string]int{pointer.ReasonPartialRead: 2}}},
		},
		{
			name: "read of a file that is gone stays inline",
			session: []obj{
				readCall("r1", filepath.Join(dir, "gone.png"), nil),
				imageResult("r1", imageBytes, len(imageBytes)),
			},
			wantReport: map[string]pointer.Result{pointer.FinderFileRead: {Skipped: map[string]int{pointer.ReasonFileGone: 2}}},
		},
		{
			name: "read of an image whose size changed since the read stays inline",
			session: []obj{
				readCall("r1", screenshot, nil),
				imageResult("r1", imageBytes, len(imageBytes)+10),
			},
			check: func(t *testing.T, out []json.RawMessage) {
				res := decode(t, out[1])
				assert.Equal(t, "base64", at(t, res, keyMessage, keyContent, 0, keyContent, 0, "source", keyType))
			},
			wantReport: map[string]pointer.Result{pointer.FinderFileRead: {Skipped: map[string]int{pointer.ReasonFileChanged: 2}}},
		},
		{
			name: "read that failed stays inline",
			session: []obj{
				readCall("r1", screenshot, nil),
				{keyType: typeUser, keyMessage: obj{keyContent: []any{
					obj{keyType: blockToolResult, keyToolUseID: "r1", "is_error": true, keyContent: "File does not exist."},
				}}},
			},
			wantReport: map[string]pointer.Result{pointer.FinderFileRead: {Skipped: map[string]int{pointer.ReasonToolError: 1}}},
		},
		{
			name: "write of a spec file with its final content",
			session: []obj{
				writeCall("w1", ticket, ticketText),
				writeResult("w1", ticket, ticketText),
			},
			check: func(t *testing.T, out []json.RawMessage) {
				input := at(t, decode(t, out[0]), keyMessage, keyContent, 0, "input").(obj)
				assert.NotContains(t, input, keyContent)
				assert.Equal(t, ptr(ticketMaterial), input[pointer.Marker])
				assert.Equal(t, ticket, input["file_path"])

				result := at(t, decode(t, out[1]), keyToolUseResult).(obj)
				assert.NotContains(t, result, keyContent)
				assert.Equal(t, ptr(ticketMaterial), result[pointer.Marker])
				assert.Equal(t, "create", result[keyType])
			},
			wantReport: map[string]pointer.Result{pointer.FinderFileWrite: {Replaced: 2}},
		},
		{
			name: "earlier write of a spec file stays inline, the final one is replaced",
			session: []obj{
				writeCall("w1", ticket, "draft"),
				writeResult("w1", ticket, "draft"),
				writeCall("w2", ticket, ticketText),
				writeResult("w2", ticket, ticketText),
			},
			check: func(t *testing.T, out []json.RawMessage) {
				assert.Equal(t, "draft", at(t, decode(t, out[0]), keyMessage, keyContent, 0, "input", keyContent))
				assert.Equal(t, "draft", at(t, decode(t, out[1]), keyToolUseResult, keyContent))
				assert.Equal(t, ptr(ticketMaterial), at(t, decode(t, out[2]), keyMessage, keyContent, 0, "input", pointer.Marker))
			},
			wantReport: map[string]pointer.Result{pointer.FinderFileWrite: {Replaced: 2, Skipped: map[string]int{pointer.ReasonNoSpecEntry: 2}}},
		},
		{
			name: "same image read twice gives two pointers each",
			session: []obj{
				readCall("r1", screenshot, nil),
				imageResult("r1", imageBytes, len(imageBytes)),
				readCall("r2", screenshot, nil),
				imageResult("r2", imageBytes, len(imageBytes)),
			},
			wantReport: map[string]pointer.Result{pointer.FinderFileRead: {Replaced: 4}},
		},
		{
			name: "entries with no tool calls are left as they are",
			session: []obj{
				{keyType: typeUser, keyMessage: obj{keyContent: "hello <world>"}},
				{keyType: typeAssistant, keyMessage: obj{keyContent: []any{obj{keyType: resultText, resultText: "hi"}}}},
			},
			wantReport: nil,
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

			if len(tc.wantReport) == 0 || bytesRemoved == 0 {
				// Nothing replaced: each entry keeps its bytes.
				assert.Equal(t, before, raw[mainStream])
			}

			if tc.check != nil {
				tc.check(t, raw[mainStream])
			}
		})
	}
}

// results returns the finders of a report that looked at a copy, with no
// byte counts, and the bytes they removed in all.
func results(report pointer.Report) (map[string]pointer.Result, int) {
	var out map[string]pointer.Result
	bytesRemoved := 0
	for finder, r := range report {
		if r.Replaced == 0 && len(r.Skipped) == 0 {
			continue
		}
		if out == nil {
			out = make(map[string]pointer.Result)
		}
		bytesRemoved += r.BytesRemoved
		r := *r
		r.BytesRemoved = 0
		out[finder] = r
	}

	return out, bytesRemoved
}

// TestReplaceMatchReport follows the example of R-012: 12 image reads, of
// which 8 are spec sources.
func TestReplaceMatchReport(t *testing.T) {
	dir := t.TempDir()
	sources := pointer.Sources{}
	session := make([]json.RawMessage, 0, 24)

	for i := range 12 {
		content := []byte(fmt.Sprintf("image %d", i))
		path := filepath.Join(dir, fmt.Sprintf("img-%d.png", i))
		require.NoError(t, os.WriteFile(path, content, 0o600))
		if i < 8 {
			sources[pointer.Digest(content)] = fmt.Sprintf("sha256:material-%d", i)
		}

		id := fmt.Sprintf("r%d", i)
		session = append(session, line(t, readCall(id, path, nil)), line(t, imageResult(id, content, len(content))))
	}

	report := replaceSpecCopies(map[string][]json.RawMessage{mainStream: session}, sources)

	assert.Equal(t, 16, report[pointer.FinderFileRead].Replaced)
	assert.Equal(t, map[string]int{pointer.ReasonNoSpecEntry: 8}, report[pointer.FinderFileRead].Skipped)
}

// TestReplaceSubagents checks that every stream of the session is processed.
func TestReplaceSubagents(t *testing.T) {
	ticket := "/repo/.chainloop/specs/s1/ticket.md"
	raw := map[string][]json.RawMessage{
		mainStream: {line(t, obj{keyType: typeUser, keyMessage: obj{keyContent: "hi"}})},
		"agent-ab": {line(t, writeCall("w1", ticket, ticketText)), line(t, writeResult("w1", ticket, ticketText))},
	}

	report := replaceSpecCopies(raw, pointer.Sources{pointer.Digest([]byte(ticketText)): ticketMaterial})

	assert.Equal(t, 2, report[pointer.FinderFileWrite].Replaced)
	assert.NotContains(t, string(raw["agent-ab"][0]), "ENG-1234")
	assert.NotContains(t, string(raw["agent-ab"][1]), "ENG-1234")
}

// TestReplaceKeepsOtherBytes checks that a replaced entry keeps the text of
// its other fields as it was, with no HTML escaping added.
func TestReplaceKeepsOtherBytes(t *testing.T) {
	ticket := "/repo/.chainloop/specs/s1/ticket.md"
	call := writeCall("w1", ticket, ticketText)
	call["note"] = "a <b> & c"
	raw := map[string][]json.RawMessage{mainStream: {line(t, call)}}

	replaceSpecCopies(raw, pointer.Sources{pointer.Digest([]byte(ticketText)): ticketMaterial})

	assert.Contains(t, string(raw[mainStream][0]), pointer.Marker)
	assert.True(t, strings.Contains(string(raw[mainStream][0]), `"a <b> & c"`), string(raw[mainStream][0]))
}

func TestReplaceNoSources(t *testing.T) {
	session := []json.RawMessage{line(t, writeCall("w1", "/x", ticketText))}
	raw := map[string][]json.RawMessage{mainStream: session}

	report := replaceSpecCopies(raw, nil)

	assert.Empty(t, report)
	assert.Contains(t, string(raw[mainStream][0]), "ENG-1234")
}

func replaceSpecCopies(raw map[string][]json.RawMessage, sources pointer.Sources) pointer.Report {
	return (&Provider{}).ReplaceSpecCopies(raw, sources)
}
