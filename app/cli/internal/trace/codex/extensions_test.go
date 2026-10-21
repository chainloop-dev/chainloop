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

//nolint:goconst // Keep native protocol literals readable in regression fixtures.
package codex

import (
	"encoding/base64"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/chainloop-dev/chainloop/app/cli/internal/trace"
	"github.com/chainloop-dev/chainloop/app/cli/internal/trace/pointer"
	"github.com/chainloop-dev/chainloop/app/cli/internal/trace/state"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	testReadCallID      = "read"
	testExtensionCallID = "call-1"
	testSuccess         = "Success"
)

func extensionEvent(t *testing.T, kind string, payload any) json.RawMessage {
	t.Helper()
	b, err := json.Marshal(payload)
	require.NoError(t, err)
	raw, err := json.Marshal(rolloutEntry{Timestamp: "2026-10-21T10:00:00Z", Type: kind, Payload: b})
	require.NoError(t, err)
	return raw
}

func extensionMessage(t *testing.T, text string) json.RawMessage {
	t.Helper()
	return extensionEvent(t, responseItem, map[string]any{typeField: "message", "role": "user", contentField: []map[string]string{{typeField: "input_text", textField: text}}})
}

func normalizedExtension(t *testing.T, raw json.RawMessage) json.RawMessage {
	t.Helper()
	var e rolloutEntry
	var payload rolloutPayload
	require.NoError(t, json.Unmarshal(raw, &e))
	require.NoError(t, json.Unmarshal(e.Payload, &payload))
	out, err := transcriptEntry(e, payload, raw, testModel, 1)
	require.NoError(t, err)
	return out
}

func TestCodexSpecCopies(t *testing.T) {
	const source = "# Task\n\nBuild the requested feature.\n"
	const material = "sha256:stored-redacted-source"
	for _, tc := range []struct {
		name, tool, input, output string
		want                      bool
	}{
		{name: "plain shell read", tool: toolExecCommand, input: `{"cmd":"cat ticket.md"}`, output: source, want: true},
		{name: "framed shell read", tool: toolExecCommand, input: `{"cmd":"cat ticket.md"}`, output: "Chunk ID: test\nWall time: 0.1 seconds\nProcess exited with code 0\nFinal output:\n" + source, want: true},
		{name: "partial read", tool: toolExecCommand, input: `{"cmd":"head ticket.md"}`, output: "# Task\n", want: false},
		{name: "trimmed source stays inline", tool: toolExecCommand, input: `{"cmd":"cat ticket.md"}`, output: strings.TrimSuffix(source, "\n"), want: false},
		{name: "failed shell", tool: toolExecCommand, input: `{"cmd":"cat ticket.md"}`, output: "Process exited with code 1\nFinal output:\n" + source, want: false},
		{name: "structured write", tool: "write_file", input: `{"path":"ticket.md","content":` + string(mustJSON(t, source)) + `}`, output: testSuccess, want: true},
		{name: "patch creation", tool: toolApplyPatch, input: "*** Begin Patch\n*** Add File: ticket.md\n+# Task\n+\n+Build the requested feature.\n*** End Patch", output: testSuccess, want: true},
		{name: "code mode patch creation", tool: "exec", input: "const patch = " + string(mustJSON(t, "*** Begin Patch\n*** Add File: ticket.md\n+# Task\n+\n+Build the requested feature.\n*** End Patch")) + "; text(await tools.apply_patch(patch));", output: "Script completed", want: true},
		{name: "failed write", tool: "write_file", input: `{"path":"ticket.md","content":` + string(mustJSON(t, source)) + `}`, output: "Error: permission denied", want: false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			call := map[string]string{typeField: functionCall, "name": tc.tool, callIDField: testExtensionCallID, "arguments": tc.input}
			resultType := functionCallOutput
			if tc.tool == toolApplyPatch || tc.tool == "exec" {
				call = map[string]string{typeField: customToolCall, "name": tc.tool, callIDField: testExtensionCallID, "input": tc.input}
				resultType = "custom_tool_call_output"
			}
			raw := map[string][]json.RawMessage{mainTranscriptKey: {
				normalizedExtension(t, extensionEvent(t, responseItem, call)),
				normalizedExtension(t, extensionEvent(t, responseItem, map[string]string{typeField: resultType, callIDField: testExtensionCallID, outputField: tc.output})),
			}}
			before := string(mustJSON(t, raw))
			report := New().ReplaceSpecCopies(raw, pointer.Sources{pointer.Digest([]byte(source)): material})
			after := string(mustJSON(t, raw))
			if !tc.want {
				assert.Equal(t, before, after)
				return
			}
			assert.Contains(t, after, pointer.Marker)
			assert.Contains(t, after, material)
			assert.NotContains(t, after, "Build the requested feature.")
			assert.NotEmpty(t, report)
			// Repeating assembly cannot replace a pointer or lose native metadata.
			New().ReplaceSpecCopies(raw, pointer.Sources{pointer.Digest([]byte(source)): material})
			assert.Equal(t, after, string(mustJSON(t, raw)))
		})
	}
}

func mustJSON(t *testing.T, v any) json.RawMessage {
	t.Helper()
	raw, err := json.Marshal(v)
	require.NoError(t, err)
	return raw
}

func TestCodexPastedImages(t *testing.T) {
	data := []byte("test image bytes")
	url := "data:image/png;base64," + base64.StdEncoding.EncodeToString(data)
	image := map[string]string{typeField: blockInputImage, "image_url": url}
	prompt := extensionEvent(t, responseItem, map[string]any{typeField: "message", "role": "user", contentField: []any{image, image, map[string]string{typeField: blockInputImage, "image_url": "data:image/png;base64,!invalid"}}})
	tool := extensionEvent(t, responseItem, map[string]any{typeField: functionCallOutput, callIDField: "image-read", outputField: []any{image}})
	raw := map[string][]json.RawMessage{mainTranscriptKey: {normalizedExtension(t, prompt), normalizedExtension(t, tool)}}
	images := New().PastedImages(raw)
	require.Len(t, images, 1)
	assert.Equal(t, data, images[0].Data)
	assert.Equal(t, 1, images[0].Number)
	assert.Equal(t, "image/png", images[0].MediaType)
	// A storage failure leaves all copies intact.
	before := string(mustJSON(t, raw))
	New().ReplaceSpecCopies(raw, nil)
	assert.Equal(t, before, string(mustJSON(t, raw)))
	New().ReplaceSpecCopies(raw, pointer.Sources{images[0].Digest: "sha256:stored-image"})
	assert.NotContains(t, string(raw[mainTranscriptKey][0]), url)
	assert.Contains(t, string(raw[mainTranscriptKey][0]), "!invalid")
	assert.Contains(t, string(raw[mainTranscriptKey][0]), "sha256:stored-image")
	assert.Contains(t, string(raw[mainTranscriptKey][1]), url, "tool images remain inline")
	var viewer trace.RawSessionEntry
	require.NoError(t, json.Unmarshal(raw[mainTranscriptKey][0], &viewer))
	assert.Contains(t, string(viewer.Message.Content), `"type":"image"`)
	var blocks []struct {
		Source json.RawMessage `json:"source"`
	}
	require.NoError(t, json.Unmarshal(viewer.Message.Content, &blocks))
	assert.JSONEq(t, string(pointer.New("sha256:stored-image")), string(blocks[0].Source))
}

func TestCodexCodePatchBoundaries(t *testing.T) {
	const source = "original\n"
	patch := string(mustJSON(t, "*** Begin Patch\n*** Add File: task.md\n+original\n*** End Patch"))
	for _, tc := range []struct {
		name, code string
		want       bool
	}{
		{name: "direct literal", code: "text(await tools.apply_patch(" + patch + "));", want: true},
		{name: "constant literal", code: "const patch = " + patch + "; text(await tools.apply_patch(patch));", want: true},
		{name: "bare awaited literal", code: "await tools.apply_patch(" + patch + ");", want: true},
		{name: "changed constant", code: "const patch = " + patch + "; text(await tools.apply_patch(patch.replace('original', 'changed')));"},
		{name: "changed literal", code: "text(await tools.apply_patch(" + patch + ".replace('original', 'changed')));"},
		{name: "concatenated literal", code: "text(await tools.apply_patch(" + patch + " + suffix));"},
		{name: "unused literal", code: "const patch = " + patch + "; text(await tools.apply_patch(other));"},
		{name: "unrelated call", code: "text(" + patch + "); text(await tools.apply_patch(other));"},
		{name: "commented call", code: "// text(await tools.apply_patch(" + patch + "));"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			call := extensionEvent(t, responseItem, map[string]string{typeField: customToolCall, "name": "exec", callIDField: testExtensionCallID, "input": tc.code})
			result := extensionEvent(t, responseItem, map[string]string{typeField: customToolCallOutput, callIDField: testExtensionCallID, outputField: testSuccess})
			raw := map[string][]json.RawMessage{mainTranscriptKey: {normalizedExtension(t, call), normalizedExtension(t, result)}}
			before := string(mustJSON(t, raw))
			report := New().ReplaceSpecCopies(raw, pointer.Sources{pointer.Digest([]byte(source)): "sha256:original"})
			after := string(mustJSON(t, raw))
			if !tc.want {
				assert.Equal(t, before, after)
				assert.Empty(t, report)
				return
			}
			assert.Contains(t, after, pointer.Marker)
			assert.NotContains(t, after, "*** Begin Patch")
			assertCanonicalPointers(t, raw[mainTranscriptKey][0])
		})
	}
}

func TestCodexFailedWriteEnvelopes(t *testing.T) {
	const source = "original\n"
	patch := string(mustJSON(t, "*** Begin Patch\n*** Add File: task.md\n+original\n*** End Patch"))
	for _, tc := range []struct {
		name   string
		output any
	}{
		{name: "JSON text", output: `{"exit_code":1,"output":"permission denied"}`},
		{name: "text block", output: []map[string]string{{typeField: "input_text", textField: `{"exit_code":1,"output":"permission denied"}`}}},
		{name: "promise", output: map[string]any{"status": "fulfilled", valueField: map[string]any{"exit_code": 1, outputField: "permission denied"}}},
		{name: "JSON promise", output: `{"status":"fulfilled","value":{"exit_code":1,"output":"permission denied"}}`},
		{name: "nested output", output: map[string]any{outputField: map[string]any{"is_error": true}}},
		{name: "tool error", output: `{"isError":true,"content":[{"type":"text","text":"permission denied"}]}`},
		{name: "rejected promise", output: map[string]any{"status": "rejected", "reason": "permission denied"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			call := extensionEvent(t, responseItem, map[string]string{typeField: customToolCall, "name": "exec", callIDField: testExtensionCallID, "input": "text(await tools.apply_patch(" + patch + "));"})
			result := extensionEvent(t, responseItem, map[string]any{typeField: customToolCallOutput, callIDField: testExtensionCallID, outputField: tc.output})
			raw := map[string][]json.RawMessage{mainTranscriptKey: {normalizedExtension(t, call), normalizedExtension(t, result)}}
			before := string(mustJSON(t, raw))
			New().ReplaceSpecCopies(raw, pointer.Sources{pointer.Digest([]byte(source)): "sha256:original"})
			assert.Equal(t, before, string(mustJSON(t, raw)))
		})
	}
}

func TestCodexSessionSkills(t *testing.T) {
	dir := t.TempDir()
	skillDir := filepath.Join(dir, ".agents", "skills", "test-skill")
	require.NoError(t, os.MkdirAll(skillDir, 0700))
	definition := "---\nname: test-skill\ndescription: Test skill.\n---\n\nDo the task.\n"
	require.NoError(t, os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(definition), 0600))
	header := extensionEvent(t, nativeSessionMeta, map[string]string{"id": testSessionID, cwdField: dir})
	injected := extensionMessage(t, "<skill>\n<name>test-skill</name>\n<path>"+filepath.Join(skillDir, "SKILL.md")+"</path>\n"+definition+"</skill>")
	catalog := extensionMessage(t, "Available skills: test-skill (file: "+filepath.Join(skillDir, "SKILL.md")+")")
	call := extensionEvent(t, responseItem, map[string]string{typeField: customToolCall, "name": "exec", callIDField: "read-skill", "input": "const path = " + string(mustJSON(t, filepath.Join(skillDir, "SKILL.md"))) + `; text(await tools.exec_command({cmd:"cat .agents/skills/test-skill/SKILL.md"}));`})
	result := extensionEvent(t, responseItem, map[string]string{typeField: customToolCallOutput, callIDField: "read-skill", outputField: definition})
	path := state.RawSessionPath(dir, testSessionID)
	transcript := strings.Join([]string{string(header), string(catalog), string(injected), string(call), string(result)}, "\n") + "\n"
	require.NoError(t, os.WriteFile(path, []byte(transcript), 0600))
	uses, warnings, err := New().SessionSkills(&trace.ParseOpts{SessionID: testSessionID, SessionDir: dir})
	require.NoError(t, err)
	assert.Empty(t, warnings)
	require.Len(t, uses, 1)
	assert.Equal(t, skillDir, uses[0].Dir)
	assert.Equal(t, 1, uses[0].ByUser)
	assert.Equal(t, 1, uses[0].ByModel)
	assert.Equal(t, "2026-10-21T10:00:00Z", uses[0].FirstUsedAt)
}

func TestCodexCodeModeCopies(t *testing.T) {
	const source = "# Task\n\nImplement this task.\n"
	const digest = "sha256:redacted-material"
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "ticket.md"), []byte(source), 0600))
	for _, tc := range []struct {
		name   string
		output any
	}{
		{name: "promise envelope", output: []map[string]string{{typeField: "input_text", textField: string(mustJSON(t, map[string]any{"status": "fulfilled", "value": map[string]any{"exit_code": 0, outputField: source}}))}}},
		{name: "combined stdout", output: []map[string]string{{typeField: "input_text", textField: source + "## main...origin/main\n"}}},
		{name: "repeated combined stdout", output: []map[string]string{{typeField: "input_text", textField: source + "## main...origin/main\n" + source}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			call := extensionEvent(t, responseItem, map[string]string{typeField: customToolCall, "name": "exec", callIDField: testReadCallID, "input": `text(await tools.exec_command({cmd:"cat ticket.md && git status"}));`})
			result := extensionEvent(t, responseItem, map[string]any{typeField: customToolCallOutput, callIDField: testReadCallID, outputField: tc.output})
			timeline := extensionEvent(t, nativeEventMsg, map[string]any{typeField: "item_completed", "item": map[string]any{typeField: "CommandExecution", "id": testReadCallID, "command": "cat ticket.md && git status", cwdField: dir, "aggregated_output": source, "exit_code": 0}})
			fileChange := extensionEvent(t, nativeEventMsg, map[string]any{typeField: "item_completed", "item": map[string]any{typeField: "FileChange", "changes": map[string]any{"ticket.md": map[string]string{typeField: "add", contentField: "# Capture metadata\n" + source + "\n"}}}})
			raw := map[string][]json.RawMessage{mainTranscriptKey: {extensionEvent(t, nativeSessionMeta, map[string]string{"id": testSessionID, cwdField: dir}), normalizedExtension(t, call), normalizedExtension(t, result), timeline, fileChange}}
			New().ReplaceSpecCopies(raw, pointer.Sources{pointer.Digest([]byte(source)): digest})
			for _, line := range raw[mainTranscriptKey] {
				assertCanonicalPointers(t, line)
			}
			for _, index := range []int{2, 3, 4} {
				assert.NotContains(t, string(raw[mainTranscriptKey][index]), "Implement this task.")
				assert.Contains(t, string(raw[mainTranscriptKey][index]), digest)
			}
			var viewer trace.RawSessionEntry
			require.NoError(t, json.Unmarshal(raw[mainTranscriptKey][2], &viewer))
			var blocks []trace.RawSessionToolResultBlock
			require.NoError(t, json.Unmarshal(viewer.Message.Content, &blocks))
			assert.Contains(t, string(blocks[0].Content), pointer.Marker)
		})
	}
}

func assertCanonicalPointers(t *testing.T, raw json.RawMessage) {
	t.Helper()
	if fields := object(raw); fields != nil {
		if jsonString(fields[typeField]) == pointer.Marker {
			assert.Len(t, fields, 2, "a pointer contains only its type and digest")
		}
		if replaced, ok := fields[pointer.Marker]; ok {
			value := object(replaced)
			require.NotNil(t, value, "a text-field marker holds one pointer, not content blocks")
			assert.Equal(t, pointer.Marker, jsonString(value[typeField]))
			assert.Len(t, value, 2)
		}
		for _, value := range fields {
			assertCanonicalPointers(t, value)
		}
		return
	}
	var values []json.RawMessage
	if json.Unmarshal(raw, &values) == nil {
		for _, value := range values {
			assertCanonicalPointers(t, value)
		}
	}
}

func TestCodexSkillResultEnvelopes(t *testing.T) {
	const definition = "---\nname: bundled\n---\nDo the task.\n"
	for _, tc := range []struct {
		name  string
		value any
		want  []string
	}{
		{name: "successful promise", value: map[string]any{"status": "fulfilled", "value": map[string]any{"exit_code": 0, "output": definition}}, want: []string{definition}},
		{name: "failed nested command", value: map[string]any{"status": "fulfilled", "value": map[string]any{"exit_code": 1, "output": definition}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			raw := mustJSON(t, tc.value)
			assert.Equal(t, tc.want, outputTexts(raw))
			assert.Equal(t, tc.want, outputTexts(mustJSON(t, string(raw))))
		})
	}
}

func TestCodexSkillPathResolution(t *testing.T) {
	assert.Equal(t, []string{"/skills/one/SKILL.md"}, skillReadPaths("cat relative/SKILL.md; cat /skills/one/SKILL.md", ""))
}

func TestCodexSourceLooksLikeFailure(t *testing.T) {
	const source = "Error: the page is missing.\n"
	sources := pointer.Sources{pointer.Digest([]byte(source)): "sha256:source"}
	for _, tc := range []struct {
		name   string
		output any
		want   bool
	}{
		{name: "plain text", output: source, want: true},
		{name: "code mode text", output: []map[string]string{{typeField: "input_text", textField: "Script completed\nWall time 0.1 seconds\nOutput:\n"}, {typeField: "input_text", textField: source}}, want: true},
		{name: "successful command", output: map[string]any{"exit_code": 0, "output": source}, want: true},
		{name: "failed command", output: map[string]any{"exit_code": 1, "output": source}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			raw := mustJSON(t, tc.output)
			updated, n := replaceOutput(raw, sources, nil)
			assert.Equal(t, tc.want, n > 0)
			if tc.want {
				assert.NotContains(t, string(updated), source)
				assertCanonicalPointers(t, updated)
			} else {
				assert.Equal(t, raw, updated)
			}
		})
	}
}

func TestCodexLiveSkillLoads(t *testing.T) {
	home := t.TempDir()
	t.Setenv("CODEX_HOME", home)
	store := state.NewOutOfTreeStore(t.TempDir())
	dir := filepath.Join(home, "sessions")
	require.NoError(t, os.MkdirAll(dir, 0700))
	path := filepath.Join(dir, "rollout-"+testSessionID+".jsonl")
	header := extensionEvent(t, nativeSessionMeta, map[string]string{"id": testSessionID})
	load := extensionMessage(t, "<skill>\n<name>one</name>\n<path>/skills/one/SKILL.md</path>\nDo the task.\n</skill>")
	input := &trace.HookInput{SessionID: testSessionID, TranscriptPath: path}
	// An in-flight line remains for the next hook, even when its JSON parses.
	require.NoError(t, os.WriteFile(path, append(append(header, '\n'), load...), 0600))
	assert.Empty(t, New().NewSkillLoads(store, input))
	require.NoError(t, os.WriteFile(path, append(append(append(header, '\n'), load...), '\n'), 0600))
	assert.Equal(t, []string{"/skills/one"}, New().NewSkillLoads(store, input))
	assert.Empty(t, New().NewSkillLoads(store, input))
	// Truncation resets the scan; a foreign session never yields skill paths.
	require.NoError(t, os.WriteFile(path, append(header, '\n'), 0600))
	assert.Empty(t, New().NewSkillLoads(store, input))
	foreign := extensionEvent(t, nativeSessionMeta, map[string]string{"id": "other-session"})
	require.NoError(t, os.WriteFile(path, append(append(append(foreign, '\n'), load...), '\n'), 0600))
	assert.Empty(t, New().NewSkillLoads(store, input))
	input.TranscriptPath = filepath.Join(t.TempDir(), "foreign.jsonl")
	require.NoError(t, os.WriteFile(input.TranscriptPath, append(append(append(header, '\n'), load...), '\n'), 0600))
	assert.Empty(t, New().NewSkillLoads(store, input))
	link := filepath.Join(dir, "escaped.jsonl")
	require.NoError(t, os.Symlink(input.TranscriptPath, link))
	input.TranscriptPath = link
	assert.Empty(t, New().NewSkillLoads(store, input))
}
