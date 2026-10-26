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

package pi

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/chainloop-dev/chainloop/app/cli/internal/trace"
	"github.com/chainloop-dev/chainloop/app/cli/internal/trace/state"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	firstSessionID   = "session-1"
	firstToolUseID   = "call-1"
	spawnFailureMode = "spawn"
	windowsOS        = "windows"
)

func TestInstallHooksOwnsOnlyMarkedExtension(t *testing.T) {
	provider := New()
	repoRoot := t.TempDir()
	path := provider.SettingsFile(repoRoot)

	require.NoError(t, provider.InstallHooks(repoRoot))
	installed, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, renderExtension(true), installed)

	require.NoError(t, os.Chmod(path, 0o400))
	require.NoError(t, provider.InstallHooks(repoRoot), "an identical install must not rewrite the file")
	info, err := os.Stat(path)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o400), info.Mode().Perm())

	require.NoError(t, os.Chmod(path, 0o600))
	require.NoError(t, os.WriteFile(path, []byte(ownershipMarker+"old\n"), 0o600))
	require.NoError(t, provider.InstallHooks(repoRoot))
	upgraded, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, renderExtension(true), upgraded)
}

func TestHooksInstalled(t *testing.T) {
	provider := New()
	repoRoot := t.TempDir()

	installed, err := provider.HooksInstalled(repoRoot)
	require.NoError(t, err)
	assert.False(t, installed)

	path := provider.SettingsFile(repoRoot)
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
	require.NoError(t, os.WriteFile(path, []byte("user extension\n"), 0o600))
	installed, err = provider.HooksInstalled(repoRoot)
	require.NoError(t, err)
	assert.False(t, installed)

	require.NoError(t, os.WriteFile(path, renderExtension(true), 0o600))
	installed, err = provider.HooksInstalled(repoRoot)
	require.NoError(t, err)
	assert.True(t, installed)
}

func TestInstallHooksRejectsUnknownCollision(t *testing.T) {
	provider := New()
	repoRoot := t.TempDir()
	path := provider.SettingsFile(repoRoot)
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
	original := []byte("export default function userExtension() {}\n")
	require.NoError(t, os.WriteFile(path, original, 0o600))

	err := provider.InstallHooks(repoRoot)
	require.Error(t, err)
	preserved, readErr := os.ReadFile(path)
	require.NoError(t, readErr)
	assert.Equal(t, original, preserved)
}

func TestInstallHooksRejectsDanglingSymlink(t *testing.T) {
	if runtime.GOOS == windowsOS {
		t.Skip("creating symlinks requires additional privileges on Windows")
	}

	for _, install := range []struct {
		name string
		fn   func(*Provider, string) error
	}{
		{name: "permanent", fn: (*Provider).InstallHooks},
		{name: "trace run", fn: (*Provider).InstallHooksForTraceRun},
	} {
		t.Run(install.name, func(t *testing.T) {
			provider := New()
			repoRoot := t.TempDir()
			path := provider.SettingsFile(repoRoot)
			target := filepath.Join(t.TempDir(), "outside.ts")
			require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
			require.NoError(t, os.Symlink(target, path))

			err := install.fn(provider, repoRoot)

			require.ErrorContains(t, err, "not a regular file")
			info, statErr := os.Lstat(path)
			require.NoError(t, statErr)
			assert.NotZero(t, info.Mode()&os.ModeSymlink)
			_, targetErr := os.Stat(target)
			assert.ErrorIs(t, targetErr, os.ErrNotExist)
			installed, installedErr := provider.HooksInstalled(repoRoot)
			require.NoError(t, installedErr)
			assert.False(t, installed)
			require.NoError(t, provider.UninstallHooks(repoRoot))
			_, linkErr := os.Lstat(path)
			require.NoError(t, linkErr)
		})
	}
}

func TestInstallHooksForTraceRunCanTemporarilyReplaceUnknownFile(t *testing.T) {
	provider := New()
	repoRoot := t.TempDir()
	path := provider.SettingsFile(repoRoot)
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
	require.NoError(t, os.WriteFile(path, []byte("user extension\n"), 0o600))

	require.NoError(t, provider.InstallHooksForTraceRun(repoRoot))
	installed, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, renderExtension(false), installed)
	assert.NotContains(t, string(installed), `pi.on("session_shutdown"`)
}

func TestUninstallHooksPreservesUnknownFiles(t *testing.T) {
	provider := New()

	t.Run("missing", func(t *testing.T) {
		assert.NoError(t, provider.UninstallHooks(t.TempDir()))
	})

	t.Run("unknown", func(t *testing.T) {
		repoRoot := t.TempDir()
		path := provider.SettingsFile(repoRoot)
		require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
		require.NoError(t, os.WriteFile(path, []byte("user extension\n"), 0o600))
		require.NoError(t, provider.UninstallHooks(repoRoot))
		_, err := os.Stat(path)
		assert.NoError(t, err)
	})

	t.Run("managed", func(t *testing.T) {
		repoRoot := t.TempDir()
		require.NoError(t, provider.InstallHooks(repoRoot))
		require.NoError(t, provider.UninstallHooks(repoRoot))
		_, err := os.Stat(provider.SettingsFile(repoRoot))
		assert.ErrorIs(t, err, os.ErrNotExist)
	})
}

func TestGeneratedExtensionContainsRequiredSafetyAndLifecycleBehavior(t *testing.T) {
	source := string(renderExtension(true))
	for _, required := range []string{
		`spawn("chainloop", ["trace", "hook", "pi", event]`,
		`cwd: ctx.cwd`,
		`const maxOutputBytes = 64 * 1024`,
		`const hookTimeoutMs = 60 * 1000`,
		`const killGraceMs = 5 * 1000`,
		`child.kill("SIGTERM")`,
		`child.kill("SIGKILL")`,
		`child.on("exit"`,
		`closeStreams()`,
		`pi.on("session_start"`,
		`pi.on("session_shutdown"`,
		`const sessionEndReasons = new Set(["quit", "new", "resume", "fork"])`,
		`if (!sessionEndReasons.has(event.reason)) return`,
		`pi.on("session_tree"`,
		`pi.appendEntry(treeMarker`,
		`pi.on("before_agent_start"`,
		`ctx.sessionManager.getEntries().some`,
		`pi.on("tool_call"`,
		`return undefined`,
		`pi.on("tool_result"`,
		`tool_use_id: event.toolCallId`,
		`payload.file_path = resolveToolPath(path, ctx.cwd)`,
		`payload.command = event.input.command`,
		`ctx.sessionManager.getSessionFile()`,
		`process.stderr.write(response.message`,
	} {
		assert.Contains(t, source, required)
	}
	assert.NotContains(t, source, "shell: true")
}

func TestGeneratedExtensionToolArraysFollowGoSlices(t *testing.T) {
	originalFileWritingTools := fileWritingTools
	originalCommandTools := commandTools
	fileWritingTools = []string{"review-write"}
	commandTools = []string{"review-command"}
	t.Cleanup(func() {
		fileWritingTools = originalFileWritingTools
		commandTools = originalCommandTools
	})

	source := string(renderExtension(true))
	assert.Contains(t, source, `const fileWritingTools = ["review-write"]`)
	assert.Contains(t, source, `const commandTools = ["review-command"]`)
}

func TestGeneratedExtensionLifecycle(t *testing.T) {
	if runtime.GOOS == windowsOS {
		t.Skip("fake chainloop executable is a POSIX script")
	}
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("node is not installed")
	}

	dir := t.TempDir()
	repoRoot := filepath.Join(dir, "repo")
	repoRootTwo := filepath.Join(dir, "repo-two")
	homeDir := filepath.Join(dir, "home")
	fileURLPath := filepath.Join(dir, "url-file.go")
	require.NoError(t, os.MkdirAll(repoRoot, 0o755))
	require.NoError(t, os.MkdirAll(repoRootTwo, 0o755))
	require.NoError(t, os.MkdirAll(homeDir, 0o755))
	extensionPath := filepath.Join(dir, "extension.mjs")
	require.NoError(t, os.WriteFile(extensionPath, renderExtension(true), 0o600))
	logPath := filepath.Join(dir, "calls.log")
	startOncePath := filepath.Join(dir, "start-once")

	fakeChainloop := filepath.Join(dir, "chainloop")
	fake := `#!/bin/sh
payload=$(cat)
printf '%s\t%s\t%s\n' "$PWD" "$*" "$payload" >> "$CHAINLOOP_HOOK_LOG"
case "$4" in
  session-start)
    if [ ! -e "$CHAINLOOP_START_ONCE" ]; then : > "$CHAINLOOP_START_ONCE"
    else printf '%s' '{"banner":"banner","instruction":"full"}'
    fi ;;
  user-prompt-submit) printf '%s' '{"instruction":"reminder"}' ;;
  post-tool-use) printf '%s' '{"message":"link-1\nlink-2"}' ;;
esac
`
	writeExecutable(t, fakeChainloop, fake)

	runnerPath := filepath.Join(dir, "runner.mjs")
	runner := `import extension from "./extension.mjs"
const handlers = {}
const appended = []
const entries = []
const notifications = []
let currentID = "session-1"
const pi = {
  on(name, handler) { handlers[name] = handler },
  appendEntry(customType, data) { appended.push({ customType, data }) },
}
extension(pi)
const ctx = {
  cwd: process.env.TEST_REPO,
  hasUI: false,
  ui: { notify() { throw new Error("headless mode must not use UI") } },
  sessionManager: {
    getHeader() { return { id: currentID } },
    getSessionId() { return currentID },
    getSessionFile() { return "/sessions/" + currentID + ".jsonl" },
    getEntries() { return entries },
  },
}
await handlers.session_start({ type: "session_start", reason: "startup" }, ctx)
const first = await handlers.before_agent_start({ type: "before_agent_start" }, ctx)
entries.push({ type: "custom_message", ...first.message })
const second = await handlers.before_agent_start({ type: "before_agent_start" }, ctx)
const reloadedHandlers = {}
extension({ on(name, handler) { reloadedHandlers[name] = handler }, appendEntry(customType, data) { appended.push({ customType, data }) } })
const reloaded = await reloadedHandlers.before_agent_start({ type: "before_agent_start" }, ctx)
const toolResult = await handlers.tool_call({ type: "tool_call", toolName: "write", toolCallId: "call-1", input: { path: "@src/main.go" } }, ctx)
await handlers.tool_call({ type: "tool_call", toolName: "edit", toolCallId: "call-home", input: { path: "~/home.go" } }, ctx)
await handlers.tool_call({ type: "tool_call", toolName: "write", toolCallId: "call-url", input: { path: process.env.TEST_FILE_URL } }, ctx)
await handlers.tool_call({ type: "tool_call", toolName: "write", toolCallId: "call-unicode", input: { path: "src/narrow\u202Fspace.go" } }, ctx)
await handlers.tool_call({ type: "tool_call", toolName: "bash", toolCallId: "call-bash", input: { command: "git status" } }, ctx)
await handlers.tool_result({ type: "tool_result", toolName: "write", toolCallId: "call-1", input: { path: "@src/main.go" } }, ctx)
await handlers.session_tree({ type: "session_tree", newLeafId: "leaf-1" }, ctx)
for (const reason of ["reload", "unknown", "quit", "new", "resume", "fork"]) {
  await handlers.session_shutdown({ type: "session_shutdown", reason }, ctx)
}
currentID = "session-2"
ctx.cwd = process.env.TEST_REPO_TWO
await handlers.session_start({ type: "session_start", reason: "fork" }, ctx)
const forked = await handlers.before_agent_start({ type: "before_agent_start" }, ctx)
currentID = "session-abandoned"
entries.push({ type: "custom_message", customType: "chainloop-trace-instruction:" + currentID, content: "old branch marker" })
const abandonedHandlers = {}
extension({ on(name, handler) { abandonedHandlers[name] = handler }, appendEntry(customType, data) { appended.push({ customType, data }) } })
const abandoned = await abandonedHandlers.before_agent_start({ type: "before_agent_start" }, ctx)
currentID = "session-3"
ctx.hasUI = true
ctx.ui = { notify(message) { notifications.push(message) } }
await handlers.session_start({ type: "session_start", reason: "new" }, ctx)
await handlers.tool_result({ type: "tool_result", toolName: "bash", toolCallId: "call-2", input: {} }, ctx)
currentID = "session-4"
ctx.ui = { notify() { throw new Error("UI unavailable") } }
await handlers.session_start({ type: "session_start", reason: "new" }, ctx)
process.stdout.write(JSON.stringify({ first, second, reloaded, forked, abandoned, toolBlocked: toolResult !== undefined, appended, notifications }))
`
	require.NoError(t, os.WriteFile(runnerPath, []byte(runner), 0o600))

	command := exec.Command(node, runnerPath)
	command.Dir = dir
	command.Env = append(os.Environ(),
		"PATH="+dir+string(os.PathListSeparator)+os.Getenv("PATH"),
		"CHAINLOOP_HOOK_LOG="+logPath,
		"CHAINLOOP_START_ONCE="+startOncePath,
		"TEST_REPO="+repoRoot,
		"TEST_REPO_TWO="+repoRootTwo,
		"TEST_FILE_URL=file://"+filepath.ToSlash(fileURLPath),
		"HOME="+homeDir,
	)
	var stderr bytes.Buffer
	command.Stderr = &stderr
	output, err := command.Output()
	require.NoError(t, err, stderr.String())

	var got struct {
		First struct {
			Message struct {
				CustomType string `json:"customType"`
				Content    string `json:"content"`
			} `json:"message"`
		} `json:"first"`
		Second struct {
			Message struct {
				CustomType string `json:"customType"`
				Content    string `json:"content"`
			} `json:"message"`
		} `json:"second"`
		Forked struct {
			Message struct {
				CustomType string `json:"customType"`
				Content    string `json:"content"`
			} `json:"message"`
		} `json:"forked"`
		Reloaded struct {
			Message struct {
				CustomType string `json:"customType"`
				Content    string `json:"content"`
			} `json:"message"`
		} `json:"reloaded"`
		Abandoned struct {
			Message struct {
				CustomType string `json:"customType"`
				Content    string `json:"content"`
			} `json:"message"`
		} `json:"abandoned"`
		ToolBlocked   bool     `json:"toolBlocked"`
		Notifications []string `json:"notifications"`
		Appended      []struct {
			CustomType string `json:"customType"`
		} `json:"appended"`
	}
	require.NoError(t, json.Unmarshal(output, &got), string(output))
	assert.Equal(t, instructionMarkerForTest(firstSessionID), got.First.Message.CustomType)
	assert.Equal(t, "full\n\nreminder", got.First.Message.Content)
	assert.Equal(t, "chainloop-trace-spec-reminder", got.Second.Message.CustomType)
	assert.Equal(t, "reminder", got.Second.Message.Content)
	assert.Equal(t, "chainloop-trace-spec-reminder", got.Reloaded.Message.CustomType)
	assert.Equal(t, "reminder", got.Reloaded.Message.Content)
	assert.Equal(t, instructionMarkerForTest("session-2"), got.Forked.Message.CustomType)
	assert.Equal(t, "full\n\nreminder", got.Forked.Message.Content)
	assert.Equal(t, "chainloop-trace-spec-reminder", got.Abandoned.Message.CustomType)
	assert.Equal(t, "reminder", got.Abandoned.Message.Content)
	assert.False(t, got.ToolBlocked)
	assert.Equal(t, []string{"banner", "link-1\nlink-2"}, got.Notifications)
	require.Len(t, got.Appended, 1)
	assert.Equal(t, "chainloop-trace-tree-marker", got.Appended[0].CustomType)
	assert.Equal(t, "link-1\nlink-2\n", stderr.String(), "headless notifications must use stderr only")

	callLog, err := os.ReadFile(logPath)
	require.NoError(t, err)
	lines := strings.Split(strings.TrimSpace(string(callLog)), "\n")
	var sessionEnds int
	var foundToolPayload bool
	var foundBashPayload bool
	expectedPaths := map[string]string{
		firstToolUseID: filepath.Join(repoRoot, "src/main.go"),
		"call-home":    filepath.Join(homeDir, "home.go"),
		"call-url":     fileURLPath,
		"call-unicode": filepath.Join(repoRoot, "src/narrow space.go"),
	}
	foundPaths := make(map[string]bool, len(expectedPaths))
	for _, line := range lines {
		parts := strings.SplitN(line, "\t", 3)
		require.Len(t, parts, 3)
		var payload map[string]any
		require.NoError(t, json.Unmarshal([]byte(parts[2]), &payload))
		expectedRoot := repoRoot
		if payload["session_id"] != firstSessionID {
			expectedRoot = repoRootTwo
		}
		expectedDir, statErr := os.Stat(expectedRoot)
		require.NoError(t, statErr)
		//nolint:gosec // parts[0] is written by the test-controlled fake executable.
		actualDir, statErr := os.Stat(parts[0])
		require.NoError(t, statErr)
		assert.True(t, os.SameFile(expectedDir, actualDir), "every hook must run in the current ctx.cwd")
		if parts[1] == "trace hook pi session-end" {
			sessionEnds++
		}
		if parts[1] == "trace hook pi pre-tool-use" {
			switch payload["tool_name"] {
			case "write", "edit":
				callID, ok := payload["tool_use_id"].(string)
				require.True(t, ok)
				assert.Equal(t, expectedPaths[callID], payload["file_path"])
				foundPaths[callID] = true
				if callID == firstToolUseID {
					assert.Equal(t, "/sessions/session-1.jsonl", payload["transcript_path"])
				}
				foundToolPayload = true
			case "bash":
				assert.Equal(t, "call-bash", payload["tool_use_id"])
				assert.Equal(t, "git status", payload["command"])
				foundBashPayload = true
			}
		}
	}
	assert.Equal(t, 4, sessionEnds, "quit/new/resume/fork end the session; reload does not")
	assert.True(t, foundToolPayload)
	assert.True(t, foundBashPayload)
	assert.Equal(t, map[string]bool{
		firstToolUseID: true,
		"call-home":    true,
		"call-url":     true,
		"call-unicode": true,
	}, foundPaths)
}

func TestGeneratedExtensionBoundsHungAndExcessiveHooks(t *testing.T) {
	if runtime.GOOS == windowsOS {
		t.Skip("fake chainloop executable uses a POSIX shebang")
	}
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("node is not installed")
	}

	for _, mode := range []string{"term", "kill", "overflow", "pending-loss", "inherited-pipes"} {
		t.Run(mode, func(t *testing.T) {
			dir := t.TempDir()
			source := string(renderExtension(true))
			source = strings.Replace(source, "const hookTimeoutMs = 60 * 1000", "const hookTimeoutMs = 750", 1)
			source = strings.Replace(source, "const killGraceMs = 5 * 1000", "const killGraceMs = 250", 1)
			require.NoError(t, os.WriteFile(filepath.Join(dir, "extension.mjs"), []byte(source), 0o600))

			fake := `#!/usr/bin/env node
import { appendFileSync } from "node:fs"
import { spawn } from "node:child_process"
const mode = process.env.FAKE_MODE
const event = process.argv.at(-1)
if (mode === "pending-loss" && event !== "post-tool-use") process.exit(0)
process.on("SIGTERM", () => {
  appendFileSync(process.env.SIGNAL_LOG, "TERM\n")
  if (mode === "term") process.exit(0)
})
if (mode === "inherited-pipes") {
  spawn(process.execPath, ["-e", "setTimeout(() => {}, 10000)"], { stdio: ["ignore", "inherit", "inherit"] })
  process.exit(0)
}
process.stdin.resume()
if (mode === "overflow") process.stdout.write("x".repeat(65537))
if (mode === "pending-loss") {
  process.stdout.write('{"message":"lost-link"}')
  appendFileSync(process.env.CLEARED_LOG, "cleared\n")
}
setInterval(() => {}, 1000)
`
			if mode == "term" || mode == "kill" {
				fake = `#!/bin/sh
trap 'printf "TERM\n" >> "$SIGNAL_LOG"; [ "$FAKE_MODE" = "term" ] && exit 0' TERM
while :; do :; done
`
			}
			writeExecutable(t, filepath.Join(dir, "chainloop"), fake)

			runner := `import extension from "./extension.mjs"
const handlers = {}
extension({ on(name, handler) { handlers[name] = handler }, appendEntry() {} })
const ctx = {
  cwd: process.env.TEST_REPO,
  hasUI: false,
  ui: { notify() {} },
  sessionManager: {
    getHeader() { return { id: "session-1" } },
    getSessionId() { return "session-1" },
    getSessionFile() { return "/sessions/session-1.jsonl" },
    getEntries() { return [] },
  },
}
await handlers.session_start({ type: "session_start", reason: "startup" }, ctx)
if (process.env.FAKE_MODE === "pending-loss") {
  await handlers.tool_result({ type: "tool_result", toolName: "bash", toolCallId: "call-1", input: {} }, ctx)
}
process.stdout.write("done")
`
			require.NoError(t, os.WriteFile(filepath.Join(dir, "runner.mjs"), []byte(runner), 0o600))

			repoRoot := filepath.Join(dir, "repo")
			require.NoError(t, os.Mkdir(repoRoot, 0o755))
			signalLog := filepath.Join(dir, "signals.log")
			clearedLog := filepath.Join(dir, "cleared.log")
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()
			//nolint:gosec // node and the runner path are test-controlled.
			command := exec.CommandContext(ctx, node, filepath.Join(dir, "runner.mjs"))
			command.Dir = dir
			command.Env = append(os.Environ(),
				"PATH="+dir+string(os.PathListSeparator)+os.Getenv("PATH"),
				"FAKE_MODE="+mode,
				"SIGNAL_LOG="+signalLog,
				"CLEARED_LOG="+clearedLog,
				"TEST_REPO="+repoRoot,
			)

			output, err := command.CombinedOutput()
			require.NoError(t, err, string(output))
			assert.Equal(t, "done", string(output))
			signals, err := os.ReadFile(signalLog)
			if mode == "inherited-pipes" {
				assert.ErrorIs(t, err, os.ErrNotExist)
			} else {
				require.NoError(t, err)
				assert.Contains(t, string(signals), "TERM")
			}
			if mode == "pending-loss" {
				cleared, readErr := os.ReadFile(clearedLog)
				require.NoError(t, readErr)
				assert.Equal(t, "cleared\n", string(cleared))
				assert.NotContains(t, string(output), "lost-link", "a response lost after clear is not reconstructed")
			}
		})
	}
}

func TestGeneratedExtensionAbsorbsHookFailures(t *testing.T) {
	if runtime.GOOS == windowsOS {
		t.Skip("fake chainloop executable uses a POSIX shebang")
	}
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("node is not installed")
	}

	for _, mode := range []string{spawnFailureMode, "nonzero", "malformed", "extra"} {
		t.Run(mode, func(t *testing.T) {
			dir := t.TempDir()
			require.NoError(t, os.WriteFile(filepath.Join(dir, "extension.mjs"), renderExtension(true), 0o600))
			if mode != spawnFailureMode {
				fake := `#!/usr/bin/env node
const mode = process.env.FAKE_MODE
if (mode === "nonzero") process.exit(2)
if (mode === "malformed") process.stdout.write("not-json")
if (mode === "extra") process.stdout.write("{}\n{}")
`
				writeExecutable(t, filepath.Join(dir, "chainloop"), fake)
			}

			runner := `import extension from "./extension.mjs"
const handlers = {}
extension({ on(name, handler) { handlers[name] = handler }, appendEntry() {} })
const ctx = {
  cwd: process.cwd(), hasUI: false, ui: { notify() {} },
  sessionManager: {
    getHeader() { return { id: "session-1" } }, getSessionId() { return "session-1" },
    getSessionFile() { return "/sessions/session-1.jsonl" }, getEntries() { return [] },
  },
}
await handlers.session_start({ type: "session_start", reason: "startup" }, ctx)
const result = await handlers.tool_call({ type: "tool_call", toolName: "bash", toolCallId: "call-1", input: {} }, ctx)
process.stdout.write(result === undefined ? "done" : "blocked")
`
			require.NoError(t, os.WriteFile(filepath.Join(dir, "runner.mjs"), []byte(runner), 0o600))

			path := dir + string(os.PathListSeparator) + os.Getenv("PATH")
			if mode == spawnFailureMode {
				path = t.TempDir()
			}
			//nolint:gosec // node and the runner path are test-controlled.
			command := exec.Command(node, filepath.Join(dir, "runner.mjs"))
			command.Dir = dir
			command.Env = append(os.Environ(), "PATH="+path, "FAKE_MODE="+mode)
			output, err := command.CombinedOutput()
			require.NoError(t, err, string(output))
			assert.Equal(t, "done", string(output))
		})
	}
}

func instructionMarkerForTest(sessionID string) string {
	return "chainloop-trace-instruction:" + sessionID
}

func TestReadHookInput(t *testing.T) {
	cwd := t.TempDir()
	input, err := New().ReadHookInput(bytes.NewBufferString(`{
		"session_id":"session-1",
		"hook_event_name":"tool_call",
		"tool_name":"edit",
		"file_path":"src/main.go",
		"cwd":` + quoted(t, cwd) + `,
		"transcript_path":"/sessions/session-1.jsonl",
		"tool_use_id":"call-1",
		"tool_failed":true,
		"command":"git status"
	}`))
	require.NoError(t, err)
	assert.Equal(t, firstSessionID, input.SessionID)
	assert.Equal(t, "tool_call", input.HookEventName)
	assert.Equal(t, "edit", input.ToolName)
	assert.Equal(t, filepath.Join(cwd, "src/main.go"), input.FilePath)
	assert.Equal(t, cwd, input.Cwd)
	assert.Equal(t, "/sessions/session-1.jsonl", input.TranscriptPath)
	assert.Equal(t, firstToolUseID, input.ToolUseID)
	assert.True(t, input.ToolFailed)
	assert.Equal(t, "git status", input.Command)
}

func TestPiToolClassification(t *testing.T) {
	provider := New()
	assert.True(t, provider.IsFileWritingTool("write"))
	assert.True(t, provider.IsFileWritingTool("edit"))
	assert.False(t, provider.IsFileWritingTool("read"))
	assert.True(t, provider.IsCommandTool("bash"))
	assert.False(t, provider.IsCommandTool("write"))
}

func TestFileSnapshotsArePairedByToolCall(t *testing.T) {
	provider := New()
	store := state.NewGitStore(t.TempDir())
	require.NoError(t, store.InitTraceDir())
	path := filepath.Join(t.TempDir(), "same.go")

	first := &trace.HookInput{SessionID: firstSessionID, FilePath: path, ToolUseID: firstToolUseID}
	second := &trace.HookInput{SessionID: firstSessionID, FilePath: path, ToolUseID: "call-2"}
	require.NoError(t, os.WriteFile(path, []byte("first"), 0o600))
	require.NoError(t, provider.CaptureFileSnapshot(store, first))
	require.NoError(t, os.WriteFile(path, []byte("second"), 0o600))
	require.NoError(t, provider.CaptureFileSnapshot(store, second))

	assert.Equal(t, "first", string(provider.ResolveBeforeContent(store, first, nil)))
	assert.Equal(t, "second", string(provider.ResolveBeforeContent(store, second, nil)))

	provider.CleanupAfterEdit(store, first)
	assert.Nil(t, provider.ResolveBeforeContent(store, first, nil))
	assert.Equal(t, "second", string(provider.ResolveBeforeContent(store, second, nil)))
	provider.CleanupAfterEdit(store, second)
	assert.Nil(t, provider.ResolveBeforeContent(store, second, nil))
}

func TestPiAnnouncements(t *testing.T) {
	provider := New()

	assert.JSONEq(t, `{"banner":"banner","instruction":"instruction"}`, captureStdout(t, func() {
		require.NoError(t, provider.AnnounceSessionStart(trace.SessionStartMessage{Banner: "banner", Instruction: "instruction"}))
	}))
	assert.JSONEq(t, `{"instruction":"reminder"}`, captureStdout(t, func() {
		require.NoError(t, provider.AnnouncePromptSubmit("reminder"))
	}))
	assert.JSONEq(t, `{"message":"link"}`, captureStdout(t, func() {
		require.NoError(t, provider.AnnounceToUser("link"))
	}))
}

func writeExecutable(t *testing.T, path, content string) {
	t.Helper()
	require.NoError(t, os.WriteFile(path, []byte(content), 0o600))
	require.NoError(t, os.Chmod(path, 0o700))
}

func quoted(t *testing.T, value string) string {
	t.Helper()
	encoded, err := json.Marshal(value)
	require.NoError(t, err)
	return string(encoded)
}

func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	original := os.Stdout
	reader, writer, err := os.Pipe()
	require.NoError(t, err)
	os.Stdout = writer
	t.Cleanup(func() { os.Stdout = original })

	fn()
	require.NoError(t, writer.Close())
	os.Stdout = original
	output, err := io.ReadAll(reader)
	require.NoError(t, err)
	require.NoError(t, reader.Close())
	return string(output)
}
