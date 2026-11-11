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

//nolint:goconst // Keep native event literals readable in regression fixtures.
package copilot

import (
	"encoding/base64"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/chainloop-dev/chainloop/app/cli/internal/trace"
	"github.com/chainloop-dev/chainloop/app/cli/internal/trace/pointer"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	testSource   = "# Task\n\nBuild the requested feature.\n"
	testMaterial = "sha256:stored-redacted-source"
	testCallID   = "call-1"
	toolView     = "view"
)

func toJSON(t *testing.T, v any) string {
	t.Helper()
	raw, err := json.Marshal(v)
	require.NoError(t, err)
	return string(raw)
}

// toolExchange is the assistant message with one tool request and its
// completion, as Copilot writes them.
func toolExchange(t *testing.T, tool string, args any, result map[string]any) []string {
	t.Helper()
	request := toJSON(t, map[string]any{
		"type": eventAssistantMessage, "id": "a1", "timestamp": "2026-04-07T21:00:01Z",
		"data": map[string]any{"content": "", "toolRequests": []map[string]any{{"toolCallId": testCallID, "name": tool, "arguments": args}}},
	})
	result["toolCallId"] = testCallID
	complete := toJSON(t, map[string]any{"type": eventToolComplete, "id": "t1", "timestamp": "2026-04-07T21:00:02Z", "data": result})
	return []string{request, complete}
}

// rawSession parses the event log lines, as the push does, and returns the
// raw session of the evidence.
func rawSession(t *testing.T, lines ...string) map[string][]json.RawMessage {
	t.Helper()
	d, err := parseLog(t, header+"\n"+strings.Join(lines, "\n")+"\n")
	require.NoError(t, err)
	return d.RawSession
}

func success(content string) map[string]any {
	return map[string]any{"success": true, "result": map[string]any{"content": content, "detailedContent": "diff --git a/x b/x\n" + content}}
}

func TestReplaceSpecCopies(t *testing.T) {
	patch := "*** Begin Patch\n*** Add File: ticket.md\n+# Task\n+\n+Build the requested feature.\n*** End Patch\n"
	tests := []struct {
		name   string
		tool   string
		args   any
		result map[string]any
		finder string
	}{
		{name: "full view", tool: toolView, args: map[string]any{"path": "/r/ticket.md"}, result: success(testSource), finder: pointer.FinderFileRead},
		{name: "partial view", tool: toolView, args: map[string]any{"path": "/r/ticket.md", "view_range": []int{1, 1}}, result: success("# Task\n")},
		{name: "failed view", tool: toolView, args: map[string]any{"path": "/r/ticket.md"}, result: map[string]any{"success": false, "result": map[string]any{"content": testSource}}},
		{name: "shell read", tool: toolBash, args: map[string]any{"command": "cat ticket.md"}, result: success(strings.TrimSuffix(testSource, "\n") + "\n<shellId: 0 completed with exit code 0>"), finder: pointer.FinderFileRead},
		{name: "older shell envelope", tool: toolBash, args: map[string]any{"command": "cat ticket.md"}, result: success(testSource + "<exited with exit code 0>"), finder: pointer.FinderFileRead},
		{name: "failed shell", tool: toolBash, args: map[string]any{"command": "cat ticket.md"}, result: success(testSource + "<shellId: 0 completed with exit code 1>")},
		{name: "failed shell with exit code field", tool: toolBash, args: map[string]any{"command": "cat ticket.md"}, result: map[string]any{"success": true, "shellExecution": map[string]any{"exitCode": 2}, "result": map[string]any{"content": testSource + "<shellId: 0 completed with exit code 0>"}}},
		{name: "changed content", tool: toolView, args: map[string]any{"path": "/r/ticket.md"}, result: success(testSource + "more\n")},
		{name: "other tool", tool: "glob", args: map[string]any{"pattern": "*"}, result: success(testSource)},
		{name: "create", tool: toolCreate, args: map[string]any{"path": "/r/ticket.md", "file_text": testSource}, result: success("Created file"), finder: pointer.FinderFileWrite},
		{name: "failed create", tool: toolCreate, args: map[string]any{"path": "/r/ticket.md", "file_text": testSource}, result: map[string]any{"success": false, "result": map[string]any{"content": "exists"}}},
		{name: "patch creation", tool: toolApplyPatch, args: patch, result: success("Added 1 file(s)"), finder: pointer.FinderFileWrite},
		{name: "patch update", tool: toolApplyPatch, args: strings.Replace(patch, "Add File", "Update File", 1), result: success("Updated")},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			raw := rawSession(t, toolExchange(t, tc.tool, tc.args, tc.result)...)
			before := toJSON(t, raw)
			sources := pointer.Sources{pointer.Digest([]byte(testSource)): testMaterial}

			report := New().ReplaceSpecCopies(raw, sources)
			after := toJSON(t, raw)
			if tc.finder == "" {
				assert.Equal(t, before, after)
				return
			}
			require.NotNil(t, report[tc.finder])
			assert.Equal(t, 1, report[tc.finder].Replaced)
			assert.NotContains(t, after, "Build the requested feature.")
			assert.Equal(t, 1, viewerPointers(t, raw["main"], testMaterial), "the viewer message shows the pointer")

			// A second assembly changes nothing.
			New().ReplaceSpecCopies(raw, sources)
			assert.Equal(t, after, toJSON(t, raw))
		})
	}
}

// hookRecords are the hook.start events that Copilot writes for the Trace
// hooks of one tool call: the preToolUse batch and the postToolUse call.
func hookRecords(t *testing.T, tool string, args any, resultType, text string) []string {
	t.Helper()
	pre := toJSON(t, map[string]any{
		"type": eventHookStart, "id": "h1", "timestamp": "2026-04-07T21:00:01Z",
		"data": map[string]any{"hookInvocationId": "i1", "hookType": "preToolUse", "input": map[string]any{
			"sessionId": testSessionID, "toolCalls": []map[string]any{{"id": testCallID, "name": tool, "args": args}},
		}},
	})
	post := toJSON(t, map[string]any{
		"type": eventHookStart, "id": "h2", "timestamp": "2026-04-07T21:00:02Z",
		"data": map[string]any{"hookInvocationId": "i2", "hookType": "postToolUse", "input": map[string]any{
			"sessionId": testSessionID, "toolCallId": testCallID, "toolName": tool, "toolArgs": args,
			"toolResult": map[string]any{"resultType": resultType, "textResultForLlm": text},
		}},
	})
	return []string{pre, post}
}

func TestReplaceHookCopies(t *testing.T) {
	tests := []struct {
		name       string
		tool       string
		args       any
		result     map[string]any
		resultType string
		text       string
		want       map[string]int
	}{
		{
			name: "full view", tool: toolView, args: map[string]any{"path": "/r/ticket.md"},
			result: success(testSource), resultType: "success", text: testSource,
			want: map[string]int{pointer.FinderFileRead: 2},
		},
		{
			name: "create", tool: toolCreate, args: map[string]any{"path": "/r/ticket.md", "file_text": testSource},
			result: success("Created file"), resultType: "success", text: "Created file",
			want: map[string]int{pointer.FinderFileWrite: 3},
		},
		{
			name: "failed view", tool: toolView, args: map[string]any{"path": "/r/ticket.md"},
			result: map[string]any{"success": false, "result": map[string]any{"content": testSource}}, resultType: "failure", text: testSource,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			exchange := toolExchange(t, tc.tool, tc.args, tc.result)
			hooks := hookRecords(t, tc.tool, tc.args, tc.resultType, tc.text)
			raw := rawSession(t, exchange[0], hooks[0], hooks[1], exchange[1])
			before := toJSON(t, raw)
			sources := pointer.Sources{pointer.Digest([]byte(testSource)): testMaterial}

			report := New().ReplaceSpecCopies(raw, sources)
			after := toJSON(t, raw)
			if tc.want == nil {
				assert.Equal(t, before, after)
				return
			}
			for finder, n := range tc.want {
				require.NotNil(t, report[finder])
				assert.Equal(t, n, report[finder].Replaced, finder)
			}
			assert.NotContains(t, after, "Build the requested feature.")

			New().ReplaceSpecCopies(raw, sources)
			assert.Equal(t, after, toJSON(t, raw))
		})
	}
}

func TestReplaceCombinedShellReads(t *testing.T) {
	repo := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(repo, "docs"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(repo, "docs", "ticket.md"), []byte(testSource), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(repo, "other.md"), []byte("other text\n"), 0o600))
	start := toJSON(t, map[string]any{
		"type": eventSessionStartRecord, "id": "s", "timestamp": "2026-04-07T21:00:00Z",
		"data": map[string]any{"sessionId": testSessionID, "context": map[string]any{"cwd": repo}},
	})
	envelope := "<shellId: 1 completed with exit code 0>"

	tests := []struct {
		name    string
		command string
		output  string
		want    bool
	}{
		{name: "two files", command: "cat docs/ticket.md && cat other.md", output: "?? x\n" + testSource + "other text\n" + envelope, want: true},
		{name: "quoted path", command: "git status && cat 'docs/ticket.md'", output: "clean\n" + testSource + envelope, want: true},
		{name: "source inside a line", command: "cat docs/ticket.md", output: "prefix " + testSource + envelope},
		{name: "file not read with cat", command: "head -n 9 docs/ticket.md && echo", output: "x\n" + testSource + envelope},
		{name: "failed command", command: "cat docs/ticket.md && false", output: "x\n" + testSource + "<shellId: 1 completed with exit code 1>"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			exchange := toolExchange(t, toolBash, map[string]any{"command": tc.command}, success(tc.output))
			hooks := hookRecords(t, toolBash, map[string]any{"command": tc.command}, "success", tc.output)
			// The test log has its own session.start, with the working folder.
			d, err := parseLog(t, strings.Replace(header+"\n", header, start, 1)+strings.Join([]string{exchange[0], hooks[0], hooks[1], exchange[1]}, "\n")+"\n")
			require.NoError(t, err)
			raw := d.RawSession
			before := toJSON(t, raw)

			New().ReplaceSpecCopies(raw, pointer.Sources{pointer.Digest([]byte(testSource)): testMaterial})
			after := toJSON(t, raw)
			if !tc.want {
				assert.Equal(t, before, after)
				return
			}
			assert.NotContains(t, after, "Build the requested feature.")
			// The text around the copy, and the status line, stay.
			assert.Contains(t, after, "completed with exit code 0")
			assert.Contains(t, after, strings.Split(tc.output, "\n")[0])
			assert.Equal(t, 1, viewerPointers(t, raw["main"], testMaterial))

			New().ReplaceSpecCopies(raw, pointer.Sources{pointer.Digest([]byte(testSource)): testMaterial})
			assert.Equal(t, after, toJSON(t, raw))
		})
	}
}

// viewerPointers counts the viewer messages, outside source_event, that hold
// a pointer to the material.
func viewerPointers(t *testing.T, lines []json.RawMessage, material string) int {
	t.Helper()
	n := 0
	for _, line := range lines {
		var entry trace.RawSessionEntry
		if json.Unmarshal(line, &entry) != nil || len(entry.Message.Content) == 0 {
			continue
		}
		if strings.Contains(string(entry.Message.Content), string(pointer.New(material))) {
			n++
		}
	}
	return n
}

var testImage = []byte("\x89PNG test image bytes")

func assetEvent(t *testing.T, id string, data []byte) string {
	t.Helper()
	return toJSON(t, map[string]any{
		"type": eventBinaryAsset, "id": "asset-" + id, "timestamp": "2026-04-07T21:00:00Z",
		"data": map[string]any{"assetId": id, "type": "image", "mimeType": "image/png", "byteLength": len(data), "data": base64.StdEncoding.EncodeToString(data)},
	})
}

func pastedMessage(t *testing.T, assetIDs ...string) string {
	t.Helper()
	attachments := make([]map[string]any, 0, len(assetIDs))
	for _, id := range assetIDs {
		attachments = append(attachments, map[string]any{"type": "file", "path": "/tmp/pasted.png", "displayName": "pasted.png", "assetId": id, "mimeType": "image/png"})
	}
	return toJSON(t, map[string]any{
		"type": eventUserMessage, "id": "u1", "timestamp": "2026-04-07T21:00:00.500Z",
		"data": map[string]any{"content": "What color is it?", "attachments": attachments},
	})
}

// toolImage is a view of an image file: its asset is in a tool result, not
// in a user message.
func toolImage(t *testing.T, id string) []string {
	t.Helper()
	return toolExchange(t, toolView, map[string]any{"path": "/r/media.png"}, map[string]any{
		"success": true,
		"result":  map[string]any{"content": "Viewed image file successfully.", "binaryResultsForLlm": []map[string]any{{"type": "image", "assetId": id, "mimeType": "image/png"}}},
	})
}

func TestPastedImages(t *testing.T) {
	digest := pointer.Digest(testImage)
	other := []byte("\x89PNG another image")
	otherDigest := pointer.Digest(other)

	tests := []struct {
		name  string
		lines []string
		want  []trace.PastedImage
	}{
		{
			name:  "one image pasted twice",
			lines: []string{assetEvent(t, digest, testImage), pastedMessage(t, digest, digest)},
			want:  []trace.PastedImage{{Data: testImage, Digest: digest, MediaType: "image/png", Number: 1, Timestamp: "2026-04-07T21:00:00Z"}},
		},
		{
			name:  "two images",
			lines: []string{assetEvent(t, digest, testImage), assetEvent(t, otherDigest, other), pastedMessage(t, digest, otherDigest)},
			want: []trace.PastedImage{
				{Data: testImage, Digest: digest, MediaType: "image/png", Number: 1, Timestamp: "2026-04-07T21:00:00Z"},
				{Data: other, Digest: otherDigest, MediaType: "image/png", Number: 2, Timestamp: "2026-04-07T21:00:00Z"},
			},
		},
		{
			name:  "image of a tool result",
			lines: append([]string{assetEvent(t, digest, testImage)}, toolImage(t, digest)...),
		},
		{
			name:  "data that cannot be decoded",
			lines: []string{strings.Replace(assetEvent(t, digest, testImage), base64.StdEncoding.EncodeToString(testImage), "%%%", 1), pastedMessage(t, digest)},
		},
		{
			name:  "data that does not match its asset ID",
			lines: []string{assetEvent(t, otherDigest, testImage), pastedMessage(t, otherDigest)},
		},
		{
			name:  "asset with no data",
			lines: []string{pastedMessage(t, digest)},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, New().PastedImages(rawSession(t, tc.lines...)))
		})
	}
}

func TestReplacePastedImages(t *testing.T) {
	digest := pointer.Digest(testImage)
	media := []byte("\x89PNG media file")
	mediaDigest := pointer.Digest(media)
	encoded := base64.StdEncoding.EncodeToString(testImage)

	lines := append([]string{assetEvent(t, digest, testImage), pastedMessage(t, digest, digest), assetEvent(t, mediaDigest, media)}, toolImage(t, mediaDigest)...)

	t.Run("stored image", func(t *testing.T) {
		raw := rawSession(t, lines...)
		// The media file is a source too, but the agent read it with a tool:
		// it stays inline.
		sources := pointer.Sources{digest: "sha256:image-material", mediaDigest: "sha256:media-material"}
		report := New().ReplaceSpecCopies(raw, sources)

		after := toJSON(t, raw)
		assert.NotContains(t, after, encoded)
		assert.Contains(t, after, base64.StdEncoding.EncodeToString(media))
		require.NotNil(t, report[pointer.FinderPastedImage])
		assert.Equal(t, 1, report[pointer.FinderPastedImage].Replaced)
		assert.Equal(t, 1, viewerPointers(t, raw["main"], "sha256:image-material"), "the user message shows the image pointer")

		New().ReplaceSpecCopies(raw, sources)
		assert.Equal(t, after, toJSON(t, raw))
	})

	t.Run("image not stored", func(t *testing.T) {
		raw := rawSession(t, lines...)
		before := toJSON(t, raw)
		report := New().ReplaceSpecCopies(raw, pointer.Sources{"sha256:unrelated": "sha256:x"})
		assert.Equal(t, before, toJSON(t, raw))
		require.NotNil(t, report[pointer.FinderPastedImage])
		assert.Equal(t, 1, report[pointer.FinderPastedImage].Skipped[pointer.ReasonMaterialNotStored])
	})
}
