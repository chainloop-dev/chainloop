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

package opencode

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInstallHooksCreatesPluginFile(t *testing.T) {
	repoRoot := t.TempDir()
	p := New()

	require.NoError(t, p.InstallHooks(repoRoot))

	pluginPath := filepath.Join(repoRoot, settingsFile)
	data, err := os.ReadFile(pluginPath)
	require.NoError(t, err)

	content := string(data)
	assert.Contains(t, content, `spawn("chainloop", ["trace", "hook", "opencode", event]`)
	assert.Contains(t, content, "session.created")
	assert.Contains(t, content, "session.deleted")
	// OpenCode 1.x hooks.
	assert.Contains(t, content, "tool.execute.before")
	assert.Contains(t, content, "tool.execute.after")
	// OpenCode 2.x hooks.
	assert.Contains(t, content, `ctx.tool.hook("execute.before"`)
	assert.Contains(t, content, `ctx.tool.hook("execute.after"`)
	assert.Contains(t, content, "ctx.event.subscribe(")
}

// TestPluginDefaultExportServesBothOpenCodeMajors pins the module shape both
// plugin loaders accept. OpenCode 2 only loads a default export with an id and
// a setup function, and rejects anything else (PFM-7555). OpenCode 1.x reads
// server() from the same default export. Neither loader then looks at named
// exports.
func TestPluginDefaultExportServesBothOpenCodeMajors(t *testing.T) {
	repoRoot := t.TempDir()
	p := New()
	require.NoError(t, p.InstallHooks(repoRoot))

	data, err := os.ReadFile(filepath.Join(repoRoot, settingsFile))
	require.NoError(t, err)
	content := string(data)

	assert.Contains(t, content, "export default {\n  id: \"chainloop-trace\",\n  server,\n  setup,\n}")
	assert.Equal(t, 1, strings.Count(content, "export "), "the default export must be the only export")
	// OpenCode 2 runs the plugin as an Effect when the default export has an
	// effect key, and then never calls setup.
	assert.NotContains(t, content, "effect:")
	// OpenCode 2 passes no Bun shell to plugins, so the plugin must not use $.
	assert.NotContains(t, content, "$`")
}

// TestPluginPostsSessionStartInstruction pins how the model receives the spec
// capture instruction: the plugin reads the session-start response and posts it
// to the session as a context-only message, which gets no model reply. Each
// OpenCode major posts it through its own API.
func TestPluginPostsSessionStartInstruction(t *testing.T) {
	for _, install := range []struct {
		name string
		fn   func(*Provider, string) error
	}{
		{name: "full install", fn: (*Provider).InstallHooks},
		{name: "trace run install", fn: (*Provider).InstallHooksForTraceRun},
	} {
		t.Run(install.name, func(t *testing.T) {
			repoRoot := t.TempDir()
			require.NoError(t, install.fn(New(), repoRoot))

			data, err := os.ReadFile(filepath.Join(repoRoot, settingsFile))
			require.NoError(t, err)
			content := string(data)

			assert.Contains(t, content, ".instruction")
			// OpenCode 1.x: the SDK client posts the message, and noReply
			// stores it without a model reply.
			assert.Contains(t, content, "server({ directory, client }", "the plugin needs the SDK client to post the message")
			assert.Contains(t, content, "client.session.prompt(")
			assert.Contains(t, content, "noReply: true")
			// OpenCode 2: a synthetic message, and resume: false stores it
			// without a model reply.
			assert.Contains(t, content, "ctx.session.synthetic({ sessionID, text: instruction, resume: false })")
			// The handler waits until the message is stored, so a first turn
			// sent right away cannot reach the model without it. OpenCode 2
			// delivers events asynchronously, so its prompt hook also waits for
			// a session start still in flight.
			assert.Contains(t, content, "await post(sessionID, instruction)")
			assert.Contains(t, content, `ctx.session.hook("prompt"`)
			// A child session belongs to a subagent, whose parent already has
			// the instruction.
			assert.Contains(t, content, "parentID")
		})
	}
}

func TestInstallHooksForTraceRunOmitsSessionEnd(t *testing.T) {
	repoRoot := t.TempDir()
	p := New()

	require.NoError(t, p.InstallHooksForTraceRun(repoRoot))

	pluginPath := filepath.Join(repoRoot, settingsFile)
	data, err := os.ReadFile(pluginPath)
	require.NoError(t, err)

	content := string(data)
	assert.Contains(t, content, "session.created")
	assert.NotContains(t, content, "session.deleted", "trace run must not install session.deleted; trace run drives end-of-session itself")
	assert.NotContains(t, content, "session-end")
	assert.Contains(t, content, "tool.execute.before")
	assert.Contains(t, content, "tool.execute.after")
	assert.Contains(t, content, `ctx.tool.hook("execute.before"`)
	assert.Contains(t, content, `ctx.tool.hook("execute.after"`)
}

func TestInstallHooksIdempotent(t *testing.T) {
	repoRoot := t.TempDir()
	p := New()

	require.NoError(t, p.InstallHooks(repoRoot))
	firstStat, err := os.Stat(filepath.Join(repoRoot, settingsFile))
	require.NoError(t, err)

	require.NoError(t, p.InstallHooks(repoRoot))
	secondStat, err := os.Stat(filepath.Join(repoRoot, settingsFile))
	require.NoError(t, err)

	// File should not be rewritten when content is identical.
	assert.Equal(t, firstStat.ModTime(), secondStat.ModTime())
}

func TestUninstallHooksRemovesPluginFile(t *testing.T) {
	repoRoot := t.TempDir()
	p := New()

	require.NoError(t, p.InstallHooks(repoRoot))
	require.NoError(t, p.UninstallHooks(repoRoot))

	_, err := os.Stat(filepath.Join(repoRoot, settingsFile))
	assert.True(t, os.IsNotExist(err), "plugin file should be removed")
}

func TestUninstallHooksNoopWhenFileMissing(t *testing.T) {
	repoRoot := t.TempDir()
	p := New()
	assert.NoError(t, p.UninstallHooks(repoRoot))
}

func TestReadHookInputParsesValidInput(t *testing.T) {
	r := bytes.NewBufferString(`{"session_id":"abc-123","hook_event_name":"session.created","tool_name":"edit","file_path":"/some/file.go"}`)
	p := New()
	input, err := p.ReadHookInput(r)
	require.NoError(t, err)
	assert.Equal(t, "abc-123", input.SessionID)
	assert.Equal(t, "session.created", input.HookEventName)
	assert.Equal(t, "edit", input.ToolName)
	assert.Equal(t, "/some/file.go", input.FilePath)
}

// TestPluginSendsShellCallID pins that both entry points send the tool call ID
// as tool_use_id on shell hooks, so that the pre and post hooks of one command
// pair their snapshots. OpenCode 1.x names it callID, and OpenCode 2 id.
func TestPluginSendsShellCallID(t *testing.T) {
	repoRoot := t.TempDir()
	require.NoError(t, New().InstallHooks(repoRoot))

	data, err := os.ReadFile(filepath.Join(repoRoot, settingsFile))
	require.NoError(t, err)
	content := string(data)

	assert.Contains(t, content, "{ ...payload, tool_use_id: callID }")
	// OpenCode 1.x.
	assert.Contains(t, content, `toolEvent(directory, "pre-tool-use", "tool.execute.before", input.sessionID, input.tool, input.callID, output.args)`)
	assert.Contains(t, content, `toolEvent(directory, "post-tool-use", "tool.execute.after", input.sessionID, input.tool, input.callID, input.args)`)
	// OpenCode 2.
	assert.Contains(t, content, `toolEvent(directory, "pre-tool-use", "tool.execute.before", event.sessionID, event.tool, event.id, event.input)`)
	assert.Contains(t, content, `toolEvent(directory, "post-tool-use", "tool.execute.after", event.sessionID, event.tool, event.id, event.input)`)
}

// The plugin sends opencode's callID as tool_use_id on shell hooks, so that
// the pre and post hooks of one command pair their snapshots.
func TestReadHookInputParsesToolUseID(t *testing.T) {
	r := bytes.NewBufferString(`{"session_id":"ses_1","hook_event_name":"tool.execute.before","tool_name":"bash","tool_use_id":"call_01"}`)
	p := New()
	input, err := p.ReadHookInput(r)
	require.NoError(t, err)
	assert.Equal(t, "call_01", input.ToolUseID)
}

func TestReadHookInputApplyPatchSingleFile(t *testing.T) {
	// apply_patch fires one hook per file, so each invocation still carries
	// a single file_path — this is the shape the plugin emits after the fix.
	r := bytes.NewBufferString(`{"session_id":"ses_apply_patch_test","hook_event_name":"tool.execute.after","tool_name":"apply_patch","file_path":"/tmp/trace-fixture/existing.txt"}`)
	p := New()
	input, err := p.ReadHookInput(r)
	require.NoError(t, err)
	assert.Equal(t, "ses_apply_patch_test", input.SessionID)
	assert.Equal(t, "apply_patch", input.ToolName)
	assert.Equal(t, "/tmp/trace-fixture/existing.txt", input.FilePath)
}

// TestReadHookInputResolvesRelativeFilePath covers OpenCode 2, whose edit,
// write and patch tools accept paths relative to the session directory. The
// plugin runs the hook from that directory, so a relative path resolves
// against the working directory. The hook handlers need an absolute path to
// find the checkout that owns the file and to key the line ranges by a
// repository-relative path.
func TestReadHookInputResolvesRelativeFilePath(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	cwd, err := os.Getwd()
	require.NoError(t, err)

	cases := []struct {
		name     string
		filePath string
		want     string
	}{
		{"relative path", "src/main.go", filepath.Join(cwd, "src", "main.go")},
		{"dot-relative path", "./README.md", filepath.Join(cwd, "README.md")},
		{"absolute path is kept", "/some/file.go", "/some/file.go"},
		{"no path", "", ""},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			payload, err := json.Marshal(map[string]string{
				"session_id": "ses_rel", "tool_name": "write", "file_path": tc.filePath,
			})
			require.NoError(t, err)

			input, err := New().ReadHookInput(bytes.NewReader(payload))
			require.NoError(t, err)
			assert.Equal(t, tc.want, input.FilePath)
		})
	}
}

func TestReadHookInputHandlesMissingFields(t *testing.T) {
	r := bytes.NewBufferString(`{"session_id":"abc-123"}`)
	p := New()
	input, err := p.ReadHookInput(r)
	require.NoError(t, err)
	assert.Equal(t, "abc-123", input.SessionID)
	assert.Empty(t, input.HookEventName)
	assert.Empty(t, input.ToolName)
	assert.Empty(t, input.FilePath)
}

func TestReadHookInputReturnsErrorForInvalidJSON(t *testing.T) {
	r := bytes.NewBufferString(`not json`)
	p := New()
	_, err := p.ReadHookInput(r)
	assert.Error(t, err)
}

func TestToolKinds(t *testing.T) {
	cases := []struct {
		tool        string
		fileWriting bool
		command     bool
	}{
		{tool: "edit", fileWriting: true},
		{tool: "write", fileWriting: true},
		// OpenCode 1.x name of the patch tool.
		{tool: "apply_patch", fileWriting: true},
		// OpenCode 2.x name of the patch tool.
		{tool: "patch", fileWriting: true},
		// OpenCode 1.x name of the shell tool.
		{tool: "bash", command: true},
		// OpenCode 2.x name of the shell tool.
		{tool: "shell", command: true},
		{tool: "read"},
		{tool: "execute"},
		{tool: ""},
	}

	p := New()
	for _, tc := range cases {
		t.Run(tc.tool, func(t *testing.T) {
			assert.Equal(t, tc.fileWriting, p.IsFileWritingTool(tc.tool))
			assert.Equal(t, tc.command, p.IsCommandTool(tc.tool))
		})
	}
}

func TestSettingsFile(t *testing.T) {
	p := New()
	assert.Equal(t, filepath.Join("/repo", settingsFile), p.SettingsFile("/repo"))
}

func TestPluginTemplateHasNoUnreplacedPlaceholders(t *testing.T) {
	p := New()
	for _, install := range []func(string) error{p.InstallHooks, p.InstallHooksForTraceRun} {
		repoRoot := t.TempDir()
		require.NoError(t, install(repoRoot))

		data, err := os.ReadFile(filepath.Join(repoRoot, settingsFile))
		require.NoError(t, err)
		assert.NotContains(t, string(data), "{{")
	}
}

func TestPluginTemplateToolArraysMatchGoSlices(t *testing.T) {
	repoRoot := t.TempDir()
	p := New()
	require.NoError(t, p.InstallHooks(repoRoot))

	data, err := os.ReadFile(filepath.Join(repoRoot, settingsFile))
	require.NoError(t, err)

	cases := []struct {
		name  string
		tools []string
	}{
		{"fileWritingTools", fileWritingTools},
		{"commandTools", commandTools},
	}
	for _, tc := range cases {
		goLiteral, err := json.Marshal(tc.tools)
		require.NoError(t, err)
		assert.Contains(t, string(data), "const "+tc.name+" = "+string(goLiteral))
	}
}

// TestPluginRunsHookFromSessionDirectory covers the hook's working directory.
// OpenCode 2 runs plugins in a shared background server whose working
// directory is unrelated to the session, and the hook handler finds the
// trace state from its own working directory.
func TestPluginRunsHookFromSessionDirectory(t *testing.T) {
	repoRoot := t.TempDir()
	p := New()
	require.NoError(t, p.InstallHooks(repoRoot))

	data, err := os.ReadFile(filepath.Join(repoRoot, settingsFile))
	require.NoError(t, err)
	content := string(data)

	assert.Contains(t, content, `spawn("chainloop", ["trace", "hook", "opencode", event], { cwd: directory`)
	// OpenCode 1.x passes the session directory to server().
	assert.Contains(t, content, "async function server({ directory, client }: any)")
	// OpenCode 2.x passes it as the plugin location.
	assert.Contains(t, content, "const directory = ctx.location.directory")
}

func TestParsePatchPaths(t *testing.T) {
	cases := []struct {
		name     string
		patch    string
		expected []string
	}{
		{
			name: "single-file update (absolute path)",
			patch: "*** Begin Patch\n" +
				"*** Update File: /tmp/trace-fixture/existing.txt\n" +
				"@@\n" +
				"-old value\n" +
				"+new value\n" +
				"*** End Patch\n",
			expected: []string{"/tmp/trace-fixture/existing.txt"},
		},
		{
			name: "add file (absolute path)",
			patch: "*** Begin Patch\n" +
				"*** Add File: /tmp/trace-fixture/added.txt\n" +
				"+created by the agent\n" +
				"*** End Patch\n",
			expected: []string{"/tmp/trace-fixture/added.txt"},
		},
		{
			name: "delete file (absolute path)",
			patch: "*** Begin Patch\n" +
				"*** Delete File: /tmp/trace-fixture/deleted.txt\n" +
				"*** End Patch\n",
			expected: []string{"/tmp/trace-fixture/deleted.txt"},
		},
		{
			name: "multi-file patch preserving order",
			patch: "*** Begin Patch\n" +
				"*** Update File: /tmp/trace-fixture/first.txt\n" +
				"@@\n" +
				"-before\n" +
				"+after\n" +
				"*** Add File: /tmp/trace-fixture/second.txt\n" +
				"+new file\n" +
				"*** Delete File: /tmp/trace-fixture/third.txt\n" +
				"*** End Patch\n",
			expected: []string{
				"/tmp/trace-fixture/first.txt",
				"/tmp/trace-fixture/second.txt",
				"/tmp/trace-fixture/third.txt",
			},
		},
		{
			name: "repeated file sections are deduplicated",
			patch: "*** Begin Patch\n" +
				"*** Update File: /tmp/trace-fixture/dup.txt\n" +
				"@@\n" +
				"-a\n" +
				"+b\n" +
				"*** Update File: /tmp/trace-fixture/dup.txt\n" +
				"@@\n" +
				"-c\n" +
				"+d\n" +
				"*** End Patch\n",
			expected: []string{"/tmp/trace-fixture/dup.txt"},
		},
		{
			name: "repository-relative paths",
			patch: "*** Begin Patch\n" +
				"*** Update File: src/main.go\n" +
				"@@\n" +
				"-old\n" +
				"+new\n" +
				"*** Add File: src/new.go\n" +
				"+package main\n" +
				"*** End Patch\n",
			expected: []string{"src/main.go", "src/new.go"},
		},
		{
			name:     "empty patchText returns nil",
			patch:    "",
			expected: nil,
		},
		{
			name: "patch with no file sections returns nil",
			patch: "*** Begin Patch\n" +
				"*** End Patch\n",
			expected: nil,
		},
		{
			name: "paths with surrounding whitespace are trimmed",
			patch: "*** Begin Patch\n" +
				"*** Update File:   /tmp/trace-fixture/spaced.txt  \n" +
				"@@\n" +
				"-a\n" +
				"+b\n" +
				"*** End Patch\n",
			expected: []string{"/tmp/trace-fixture/spaced.txt"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := parsePatchPaths(tc.patch)
			assert.Equal(t, tc.expected, got)
		})
	}
}

func TestPluginTemplateContainsPatchParsing(t *testing.T) {
	repoRoot := t.TempDir()
	p := New()
	require.NoError(t, p.InstallHooks(repoRoot))

	data, err := os.ReadFile(filepath.Join(repoRoot, settingsFile))
	require.NoError(t, err)
	content := string(data)
	assert.Contains(t, content, "filePathsFromArgs")
	assert.Contains(t, content, "parsePatchPaths")
	assert.Contains(t, content, "patchText")
	assert.Contains(t, content, "Add|Update|Delete")
	// Verify the plugin loops over paths rather than sending a single path.
	assert.Contains(t, content, "for (const fp of filePathsFromArgs")
}
