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
	assert.Contains(t, content, "chainloop trace hook opencode")
	assert.Contains(t, content, "session.created")
	assert.Contains(t, content, "session.deleted")
	assert.Contains(t, content, "tool.execute.before")
	assert.Contains(t, content, "tool.execute.after")
	assert.Contains(t, content, `const fileWritingTools = ["edit","write","apply_patch"]`)
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
	assert.Contains(t, content, "tool.execute.before")
	assert.Contains(t, content, "tool.execute.after")
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

func TestIsFileWritingTool(t *testing.T) {
	p := New()
	assert.True(t, p.IsFileWritingTool("edit"))
	assert.True(t, p.IsFileWritingTool("write"))
	assert.True(t, p.IsFileWritingTool("apply_patch"))
	assert.False(t, p.IsFileWritingTool("read"))
	assert.False(t, p.IsFileWritingTool("bash"))
	assert.False(t, p.IsFileWritingTool(""))
}

func TestIsCommandTool(t *testing.T) {
	p := New()
	assert.True(t, p.IsCommandTool("bash"))
	assert.False(t, p.IsCommandTool("edit"))
	assert.False(t, p.IsCommandTool("write"))
	assert.False(t, p.IsCommandTool(""))
}

func TestSettingsFile(t *testing.T) {
	p := New()
	assert.Equal(t, filepath.Join("/repo", settingsFile), p.SettingsFile("/repo"))
}

func TestPluginTemplateHasNoUnreplacedPlaceholders(t *testing.T) {
	repoRoot := t.TempDir()
	p := New()
	require.NoError(t, p.InstallHooks(repoRoot))

	data, err := os.ReadFile(filepath.Join(repoRoot, settingsFile))
	require.NoError(t, err)
	assert.NotContains(t, string(data), "{{SessionEndBlock}}")
	assert.NotContains(t, string(data), "{{FileWritingToolsArray}}")
}

func TestPluginTemplateFileWritingToolsMatchesGoSlice(t *testing.T) {
	repoRoot := t.TempDir()
	p := New()
	require.NoError(t, p.InstallHooks(repoRoot))

	data, err := os.ReadFile(filepath.Join(repoRoot, settingsFile))
	require.NoError(t, err)

	goLiteral, err := json.Marshal(fileWritingTools)
	require.NoError(t, err)
	assert.Contains(t, string(data), "const fileWritingTools = "+string(goLiteral))
}

func TestPluginTemplateContainsBacktickShellCommand(t *testing.T) {
	// The plugin uses Bun's $ template literal for shell commands. Verify
	// the backticks survived the Go raw string splice so the plugin is
	// syntactically valid TypeScript.
	repoRoot := t.TempDir()
	p := New()
	require.NoError(t, p.InstallHooks(repoRoot))

	data, err := os.ReadFile(filepath.Join(repoRoot, settingsFile))
	require.NoError(t, err)
	assert.Contains(t, string(data), "$`echo")
	assert.Contains(t, string(data), "chainloop trace hook opencode")
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
