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
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/chainloop-dev/chainloop/app/cli/internal/trace/pointer"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// keyRole is the role of the message of a request blob.
const keyRole = "role"

// requestBlob is an api-request-blob record, as Claude Code writes it: a copy
// of the blocks of one message of an API request.
func requestBlob(role string, content ...any) obj {
	return obj{keyType: recordTypeRequestBlob, "hash": "h1", "sessionId": "s1", keyMessage: obj{keyRole: role, keyContent: content}}
}

// blocksOf returns the content blocks of the message of an entry.
func blocksOf(e obj) []any {
	return e[keyMessage].(obj)[keyContent].([]any)
}

func TestReplaceRequestBlobs(t *testing.T) {
	const pastedMaterial = "sha256:pasted-material"

	dir := t.TempDir()
	config := filepath.Join(dir, "config.json")
	require.NoError(t, os.WriteFile(config, []byte(jsonText), 0o600))
	ticket := filepath.Join(dir, ".chainloop", "specs", "s1", "ticket-eng-1234.md")

	sources := pointer.Sources{
		pointer.Digest([]byte(ticketText)): ticketMaterial,
		pointer.Digest([]byte(jsonText)):   jsonMaterial,
		pointer.Digest(pastedBytes):        pastedMaterial,
	}

	read := readCall("r1", config, nil)
	readResult := textResult("r1", config, jsonText, 1, 2, 2)
	write := writeCall("w1", ticket, ticketText)
	written := writeResult("w1", ticket, ticketText)

	testCases := []struct {
		name       string
		session    []obj
		check      func(t *testing.T, out []json.RawMessage)
		wantReport map[string]pointer.Result
	}{
		{
			name: "the copy of a write in a request blob after the entry",
			session: []obj{
				write,
				written,
				requestBlob(typeAssistant, blocksOf(write)...),
				requestBlob(typeUser, blocksOf(written)...),
			},
			check: func(t *testing.T, out []json.RawMessage) {
				// The entry itself gets its pointers, also when a blob with the
				// same call comes after it.
				input := at(t, decode(t, out[0]), keyMessage, keyContent, 0, "input").(obj)
				assert.NotContains(t, input, keyContent)
				assert.Equal(t, ptr(ticketMaterial), input[pointer.Marker])
				meta := at(t, decode(t, out[1]), keyToolUseResult).(obj)
				assert.Equal(t, ptr(ticketMaterial), meta[pointer.Marker])

				blobInput := at(t, decode(t, out[2]), keyMessage, keyContent, 0, "input").(obj)
				assert.NotContains(t, blobInput, keyContent)
				assert.Equal(t, ptr(ticketMaterial), blobInput[pointer.Marker])
				// The result of a write holds no copy, so it keeps its bytes.
				assert.Equal(t, line(t, requestBlob(typeUser, blocksOf(written)...)), out[3])
			},
			wantReport: map[string]pointer.Result{pointer.FinderFileWrite: {Replaced: 3}},
		},
		{
			name: "the copy of a full read in a request blob",
			session: []obj{
				read,
				readResult,
				requestBlob(typeAssistant, blocksOf(read)...),
				requestBlob(typeUser, blocksOf(readResult)...),
			},
			check: func(t *testing.T, out []json.RawMessage) {
				assert.Equal(t, []any{ptr(jsonMaterial)}, at(t, decode(t, out[1]), keyMessage, keyContent, 0, keyContent))
				assert.Equal(t, []any{ptr(jsonMaterial)}, at(t, decode(t, out[3]), keyMessage, keyContent, 0, keyContent))
				// The call holds no copy.
				assert.Equal(t, line(t, requestBlob(typeAssistant, blocksOf(read)...)), out[2])
			},
			wantReport: map[string]pointer.Result{pointer.FinderFileRead: {Replaced: 3}},
		},
		{
			name: "a request blob whose result is not the same as the entry keeps its content",
			session: []obj{
				read,
				readResult,
				requestBlob(typeUser, obj{keyType: blockToolResult, keyToolUseID: "r1", keyContent: "     1\t" + jsonText + "<system-reminder>more</system-reminder>"}),
			},
			check: func(t *testing.T, out []json.RawMessage) {
				assert.Contains(t, string(out[2]), "<system-reminder>more</system-reminder>")
				assert.NotContains(t, string(out[2]), pointer.Marker)
			},
			wantReport: map[string]pointer.Result{pointer.FinderFileRead: {Replaced: 2}},
		},
		{
			name: "a pasted image in a request blob",
			session: []obj{
				paste("[Image #1] look", []int{1}, imageBlock(pngType, pastedBytes)),
				requestBlob(typeUser, obj{keyType: resultText, resultText: "<system-reminder>x</system-reminder>"}, imageBlock(pngType, pastedBytes), obj{keyType: resultText, resultText: "look"}),
			},
			check: func(t *testing.T, out []json.RawMessage) {
				assert.Equal(t, ptr(pastedMaterial), at(t, decode(t, out[0]), keyMessage, keyContent, 1, keySource))
				blob := decode(t, out[1])
				assert.Equal(t, blockImage, at(t, blob, keyMessage, keyContent, 1, keyType))
				assert.Equal(t, ptr(pastedMaterial), at(t, blob, keyMessage, keyContent, 1, keySource))
			},
			wantReport: map[string]pointer.Result{pointer.FinderPastedImage: {Replaced: 2}},
		},
		{
			name: "an image in a request blob that is not a source keeps its data",
			session: []obj{
				requestBlob(typeUser, imageBlock(pngType, otherPasted)),
			},
			check: func(t *testing.T, out []json.RawMessage) {
				assert.Equal(t, line(t, requestBlob(typeUser, imageBlock(pngType, otherPasted))), out[0])
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			lines := make([]json.RawMessage, 0, len(tc.session))
			for _, e := range tc.session {
				lines = append(lines, line(t, e))
			}

			raw := map[string][]json.RawMessage{mainStream: lines}
			report := replaceSpecCopies(raw, sources)

			got, _ := results(report)
			assert.Equal(t, tc.wantReport, got)
			tc.check(t, raw[mainStream])
		})
	}
}
