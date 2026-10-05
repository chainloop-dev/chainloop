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
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"github.com/chainloop-dev/chainloop/app/cli/internal/trace"
)

const (
	// settingsFile is the project-local opencode plugin file path.
	settingsFile = ".opencode/plugins/chainloop-trace.ts"
)

// fileWritingTools is the single source of truth for opencode tool names
// that modify files. The TypeScript plugin's array is generated from this
// slice (see {{FileWritingToolsArray}}) so the plugin and IsFileWritingTool
// can never drift. OpenCode 1.x names the patch tool apply_patch, and
// OpenCode 2 names it patch.
var fileWritingTools = []string{"edit", "write", "apply_patch", "patch"}

// commandTools are opencode tool names that run shell commands. Their file
// changes are captured via a before/after working-tree snapshot (they have no
// single file-path argument), so the plugin fires the pre/post-tool-use hooks
// for them without a file_path and the handler branches on the tool kind.
// OpenCode 1.x names the shell tool bash, and OpenCode 2 names it shell.
var commandTools = []string{"bash", "shell"}

// pluginTemplate is the TypeScript plugin written to .opencode/plugins/chainloop-trace.ts.
// It fires chainloop trace hook subcommands on session lifecycle and tool events,
// and posts the instruction the session-start hook returns to the session.
// The {{SessionEndBlock}} placeholder is replaced with the session.deleted handler
// for full install, or removed entirely for trace-run install.
//
// One file serves both plugin APIs through its default export: OpenCode 1.x
// (1.3.4 and later) calls server(), and OpenCode 2 calls setup(). OpenCode 2
// rejects a plugin without a default export (PFM-7555).
const pluginTemplate = `import { spawn } from "node:child_process"

const fileWritingTools = {{FileWritingToolsArray}}
const commandTools = {{CommandToolsArray}}

function filePathsFromArgs(args: any): string[] {
  if (args?.filePath) return [args.filePath]
  if (args?.path) return [args.path]
  if (args?.patchText) return parsePatchPaths(args.patchText)
  return []
}

// parsePatchPaths extracts affected file paths from an apply_patch
// patchText payload. Each section starts with *** Add File:, *** Update
// File:, or *** Delete File: followed by the path. Paths are deduplicated
// while preserving first-seen order.
function parsePatchPaths(patchText: string): string[] {
  const paths: string[] = []
  const seen = new Set<string>()
  const re = /^\*\*\* (?:Add|Update|Delete) File: (.+)$/gm
  let m
  while ((m = re.exec(patchText)) !== null) {
    const p = m[1].trim()
    if (p && !seen.has(p)) {
      seen.add(p)
      paths.push(p)
    }
  }
  return paths
}

// fire runs the chainloop hook from the session directory, with the payload
// on stdin, and returns what the hook wrote to stdout. Tracing must never
// block or fail a tool call: if chainloop is unavailable or errors, log to
// stderr and return "".
function fire(directory: string, event: string, payload: Record<string, any>): Promise<string> {
  return new Promise((resolve) => {
    let settled = false
    let out = ""
    const done = (err?: unknown) => {
      if (settled) return
      settled = true
      if (err) console.error("chainloop-trace: " + event + " hook failed: " + err)
      resolve(err ? "" : out)
    }
    try {
      const child = spawn("chainloop", ["trace", "hook", "opencode", event], { cwd: directory, stdio: ["pipe", "pipe", "ignore"] })
      child.stdout.setEncoding("utf8")
      child.stdout.on("data", (chunk: string) => { out += chunk })
      child.on("error", done)
      child.on("close", (code) => done(code === 0 ? undefined : "exit code " + code))
      child.stdin.on("error", () => {})
      child.stdin.end(JSON.stringify(payload))
    } catch (err) {
      done(err)
    }
  })
}

// startSession fires the session-start hook and returns the instruction for
// the model that it wrote to stdout, if Chainloop has one.
async function startSession(directory: string, sessionID: string): Promise<string> {
  const out = await fire(directory, "session-start", { session_id: sessionID, hook_event_name: "session.created" })
  if (!out.trim()) return ""
  try {
    return JSON.parse(out).instruction ?? ""
  } catch (err) {
    console.error("chainloop-trace: could not read the session-start response: " + err)
    return ""
  }
}

// Post adds the session-start instruction to the session as a context-only
// message, which the model reads without replying to it. Each OpenCode major
// has its own API for it.
type Post = (sessionID: string, instruction: string) => Promise<void>

async function sessionCreated(directory: string, sessionID: string, parentID: string | undefined, post: Post) {
  const instruction = await startSession(directory, sessionID)
  // A child session belongs to a subagent, whose parent already has the
  // instruction.
  if (!instruction || parentID) return
  try {
    await post(sessionID, instruction)
  } catch (err) {
    console.error("chainloop-trace: could not post the session instruction: " + err)
  }
}

async function sessionEvent(directory: string, type: string, sessionID: string, parentID: string | undefined, post: Post) {
  if (type === "session.created") {
    await sessionCreated(directory, sessionID, parentID, post)
  }
{{SessionEndBlock}}
}

async function toolEvent(directory: string, hook: string, hookEventName: string, sessionID: string, tool: string, callID: string, args: any) {
  const payload = { session_id: sessionID, hook_event_name: hookEventName, tool_name: tool }
  if (commandTools.includes(tool)) {
    // The call ID pairs this hook with the other hook of the same call, so
    // overlapping commands keep their own snapshots.
    await fire(directory, hook, { ...payload, tool_use_id: callID })
    return
  }
  if (!fileWritingTools.includes(tool)) return
  for (const fp of filePathsFromArgs(args)) {
    await fire(directory, hook, { ...payload, file_path: fp })
  }
}

// server is the OpenCode 1.x entry point.
async function server({ directory, client }: any) {
  // noReply stores the message without asking the model for an answer.
  const post: Post = async (sessionID, instruction) => {
    await client.session.prompt({
      path: { id: sessionID },
      body: { noReply: true, parts: [{ type: "text", text: instruction, synthetic: true }] },
    })
  }
  return {
    // The event handler waits until the message is stored, so a first turn
    // sent right away still finds it.
    event: async ({ event }: any) => {
      const info = event.properties?.info
      await sessionEvent(directory, event.type, info?.id ?? "", info?.parentID, post)
    },
    "tool.execute.before": async (input: any, output: any) => {
      await toolEvent(directory, "pre-tool-use", "tool.execute.before", input.sessionID, input.tool, input.callID, output.args)
    },
    "tool.execute.after": async (input: any) => {
      await toolEvent(directory, "post-tool-use", "tool.execute.after", input.sessionID, input.tool, input.callID, input.args)
    },
  }
}

// setup is the OpenCode 2 entry point. OpenCode 2 runs plugins in a shared
// background server, so the session directory is the plugin location, not
// the process working directory.
async function setup(ctx: any) {
  const directory = ctx.location.directory
  await ctx.tool.hook("execute.before", async (event: any) => {
    await toolEvent(directory, "pre-tool-use", "tool.execute.before", event.sessionID, event.tool, event.id, event.input)
  })
  await ctx.tool.hook("execute.after", async (event: any) => {
    await toolEvent(directory, "post-tool-use", "tool.execute.after", event.sessionID, event.tool, event.id, event.input)
  })

  // resume: false stores the message without asking the model for an answer.
  const post: Post = async (sessionID, instruction) => {
    await ctx.session.synthetic({ sessionID, text: instruction, resume: false })
  }

  // Events arrive asynchronously, so a session's first prompt can come in
  // while the session is still starting. The prompt hook runs before the
  // prompt is admitted, and waits for that start, so the instruction is
  // stored ahead of the prompt.
  const starting = new Map<string, Promise<void>>()
  await ctx.session.hook("prompt", async (event: any) => {
    const started = starting.get(event.sessionID)
    if (!started) return
    starting.delete(event.sessionID)
    await started
  })

  const controller = new AbortController()
  const consuming = (async () => {
    for await (const event of ctx.event.subscribe({ signal: controller.signal })) {
      const sessionID = event.data?.sessionID ?? ""
      const handled = sessionEvent(directory, event.type, sessionID, event.data?.parentID, post)
      if (event.type === "session.created") starting.set(sessionID, handled)
      await handled
    }
  })().catch((err) => {
    if (!controller.signal.aborted) console.error("chainloop-trace: event subscription failed: " + err)
  })

  return async () => {
    controller.abort()
    await consuming
  }
}

export default {
  id: "chainloop-trace",
  server,
  setup,
}
`

// sessionDeletedHandler is the session.deleted block inserted for full install.
// session.idle fires at the end of every agent turn and would prematurely end
// the trace session; session.deleted fires only when the session is destroyed.
const sessionDeletedHandler = `  if (type === "session.deleted") {
    await fire(directory, "session-end", { session_id: sessionID, hook_event_name: type })
  }`

// SettingsFile returns the absolute path to the plugin file for the given repo.
func (p *Provider) SettingsFile(repoRoot string) string {
	return filepath.Join(repoRoot, settingsFile)
}

// InstallHooks writes the opencode plugin file with all hooks including session.deleted.
func (p *Provider) InstallHooks(repoRoot string) error {
	return p.writePluginFile(repoRoot, true)
}

// InstallHooksForTraceRun writes the plugin file without the session.deleted handler;
// trace run drives end-of-session itself.
func (p *Provider) InstallHooksForTraceRun(repoRoot string) error {
	return p.writePluginFile(repoRoot, false)
}

func (p *Provider) writePluginFile(repoRoot string, includeSessionEnd bool) error {
	pluginPath := filepath.Join(repoRoot, settingsFile)

	content := pluginTemplate
	content = strings.Replace(content, "{{FileWritingToolsArray}}", toolsArrayLiteral(fileWritingTools), 1)
	content = strings.Replace(content, "{{CommandToolsArray}}", toolsArrayLiteral(commandTools), 1)
	if includeSessionEnd {
		content = strings.Replace(content, "{{SessionEndBlock}}", sessionDeletedHandler, 1)
	} else {
		content = strings.Replace(content, "{{SessionEndBlock}}\n", "", 1)
	}

	if err := os.MkdirAll(filepath.Dir(pluginPath), 0755); err != nil {
		return fmt.Errorf("create plugin directory: %w", err)
	}

	if existing, err := os.ReadFile(pluginPath); err == nil && string(existing) == content {
		return nil
	}

	return os.WriteFile(pluginPath, []byte(content), 0600)
}

// toolsArrayLiteral renders a tool-name slice as a TypeScript array literal so
// the plugin and the Go IsFileWritingTool/IsCommandTool predicates share one
// source of truth. json.Marshal output is valid TypeScript for a []string.
func toolsArrayLiteral(tools []string) string {
	b, _ := json.Marshal(tools)
	return string(b)
}

// UninstallHooks removes the plugin file. The plugin file is entirely
// chainloop-owned, so we remove it without preserving content.
func (p *Provider) UninstallHooks(repoRoot string) error {
	pluginPath := filepath.Join(repoRoot, settingsFile)

	err := os.Remove(pluginPath)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}

	return err
}

// maxHookPayloadBytes caps hook payload reads to defend against runaway or
// malformed payloads.
const maxHookPayloadBytes = 16 * 1024 * 1024

// ReadHookInput reads and parses the opencode hook JSON from the given reader.
// The plugin pipes JSON to stdin with fields: session_id, hook_event_name,
// tool_name, file_path. A relative file_path is resolved against the working
// directory.
func (p *Provider) ReadHookInput(r io.Reader) (*trace.HookInput, error) {
	data, err := io.ReadAll(io.LimitReader(r, maxHookPayloadBytes))
	if err != nil {
		return nil, err
	}

	var raw struct {
		SessionID     string `json:"session_id"`
		HookEventName string `json:"hook_event_name"`
		ToolName      string `json:"tool_name"`
		FilePath      string `json:"file_path"`
		ToolUseID     string `json:"tool_use_id"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, err
	}

	// OpenCode 2 tools take paths relative to the session directory, which
	// is the directory the plugin runs this hook from. The handlers need an
	// absolute path to find the checkout that owns the file.
	filePath := raw.FilePath
	if filePath != "" && !filepath.IsAbs(filePath) {
		abs, err := filepath.Abs(filePath)
		if err != nil {
			return nil, fmt.Errorf("resolve file path %q: %w", filePath, err)
		}
		filePath = abs
	}

	return &trace.HookInput{
		SessionID:     raw.SessionID,
		HookEventName: raw.HookEventName,
		ToolName:      raw.ToolName,
		FilePath:      filePath,
		ToolUseID:     raw.ToolUseID,
	}, nil
}

// patchFileRe matches *** Add File:, *** Update File:, or *** Delete File:
// section headers in an apply_patch patchText payload. It mirrors the
// regex used by the TypeScript plugin's parsePatchPaths function so the
// parsing logic is testable from Go (the TS plugin splits per-file before
// the Go side ever sees patchText).
var patchFileRe = regexp.MustCompile(`(?m)^\*\*\* (?:Add|Update|Delete) File: (.+)$`)

// parsePatchPaths extracts affected file paths from an apply_patch
// patchText payload. Each section starts with *** Add File:, *** Update
// File:, or *** Delete File: followed by the path. Paths are deduplicated
// while preserving first-seen order. Both absolute and repository-relative
// paths are returned verbatim — the caller is responsible for any
// normalization.
func parsePatchPaths(patchText string) []string {
	var paths []string
	seen := make(map[string]bool)
	for _, m := range patchFileRe.FindAllStringSubmatch(patchText, -1) {
		p := strings.TrimSpace(m[1])
		if p == "" || seen[p] {
			continue
		}
		seen[p] = true
		paths = append(paths, p)
	}
	return paths
}

// IsFileWritingTool returns true if the named tool modifies files on disk.
func (p *Provider) IsFileWritingTool(toolName string) bool {
	return slices.Contains(fileWritingTools, toolName)
}

// IsCommandTool returns true if the named tool runs a shell command.
func (p *Provider) IsCommandTool(toolName string) bool {
	return slices.Contains(commandTools, toolName)
}

// opencodeBinaryAvailable checks whether the opencode CLI is on PATH.
func opencodeBinaryAvailable() bool {
	_, err := exec.LookPath("opencode")
	return err == nil
}
