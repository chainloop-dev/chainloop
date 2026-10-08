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
	"strconv"
	"strings"

	"github.com/chainloop-dev/chainloop/app/cli/internal/trace"
	"github.com/chainloop-dev/chainloop/app/cli/internal/trace/skill"
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
// and posts to the session the instruction the session-start hook returns and
// the reminder the user-prompt-submit hook returns at each user message. It
// shows the user the banner the session-start hook returns and the session
// link the hook after a shell command returns.
// The {{SessionEndBlock}} placeholder is replaced with the session.deleted handler
// for full install, or removed entirely for trace-run install.
//
// One file serves both plugin APIs through its default export: OpenCode 1.x
// (1.3.4 and later) calls server(), and OpenCode 2 calls setup(). OpenCode 2
// rejects a plugin without a default export (PFM-7555).
const pluginTemplate = `import { spawn } from "node:child_process"

const fileWritingTools = {{FileWritingToolsArray}}
const commandTools = {{CommandToolsArray}}
const skillTool = {{SkillTool}}

// skillDirFrom returns the folder of the skill that a skill tool call loaded:
// the dir field of the result metadata, or else the base directory line of
// the result text. Each OpenCode major gives the result in its own shape, so
// each candidate is tried in turn.
function skillDirFrom(...results: any[]): string {
  for (const r of results) {
    const dir = r?.metadata?.dir
    if (typeof dir === "string" && dir) return dir
    const text = typeof r === "string" ? r : typeof r?.output === "string" ? r.output : ""
    const m = /^Base directory for this skill:\s*(.+)$/m.exec(text)
    if (m) return m[1].trim()
  }
  return ""
}

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

// HookResponse is what a chainloop hook writes to stdout when it has something
// to deliver. It mirrors the Go hookResponse type: the two are one contract
// and change together. A hook with nothing to say writes nothing.
type HookResponse = {
  // instruction is for the model, posted to the session as context.
  instruction?: string
  // banner greets the user when the session starts.
  banner?: string
  // message is shown to the user after a shell command.
  message?: string
  // relayToModel is added to the shell command result, so the model repeats
  // the message in its reply.
  relayToModel?: string
}

// responseFrom fires a hook and returns the response it wrote to stdout.
async function responseFrom(directory: string, event: string, payload: Record<string, any>): Promise<HookResponse> {
  const out = await fire(directory, event, payload)
  if (!out.trim()) return {}
  try {
    return JSON.parse(out) ?? {}
  } catch (err) {
    console.error("chainloop-trace: could not read the " + event + " response: " + err)
    return {}
  }
}

// Post adds an instruction to the session as a context-only message, which
// the model reads without replying to it, and shows the user the description,
// if any. Each OpenCode major has its own API for it.
type Post = (sessionID: string, instruction: string, description?: string) => Promise<void>

// postInstruction posts the instruction and waits until it is stored, so a
// turn sent right away still finds it. A failed post costs the instruction
// only, so it is logged and never fails the caller.
async function postInstruction(post: Post, sessionID: string, instruction: string, description?: string) {
  try {
    await post(sessionID, instruction, description)
  } catch (err) {
    console.error("chainloop-trace: could not post the session instruction: " + err)
  }
}

async function sessionCreated(directory: string, sessionID: string, parentID: string | undefined, post: Post) {
  const res = await responseFrom(directory, "session-start", { session_id: sessionID, hook_event_name: "session.created" })
  // A child session belongs to a subagent, whose parent already has the
  // instruction and the banner.
  if (parentID || (!res.instruction && !res.banner)) return
  // The banner is the description the user sees. It never goes to the model,
  // so a session with no instruction posts nothing to the model.
  await postInstruction(post, sessionID, res.instruction ?? "", res.banner)
}

// childSessions holds the OpenCode 1.x sessions of subagents, whose parent
// session already gets the instruction and the reminder. Session IDs are
// unique, so one set serves every server() call.
const childSessions = new Set<string>()

// promptSubmitted runs at each user message. The user-prompt-submit hook can
// answer with a short reminder to capture a new or changed spec, which is
// posted ahead of the turn.
async function promptSubmitted(directory: string, sessionID: string, post: Post) {
  const res = await responseFrom(directory, "user-prompt-submit", { session_id: sessionID, hook_event_name: "chat.message" })
  if (res.instruction) await postInstruction(post, sessionID, res.instruction)
}

async function sessionEvent(directory: string, type: string, sessionID: string, parentID: string | undefined, post: Post) {
  if (type === "session.created") {
    await sessionCreated(directory, sessionID, parentID, post)
  }
{{SessionEndBlock}}
}

// toolEvent fires the hook of a tool call and returns its response. Only the
// hook after a shell command can have one: the session link left by a push.
async function toolEvent(directory: string, hook: string, hookEventName: string, sessionID: string, tool: string, callID: string, args: any, skillDir = ""): Promise<HookResponse | undefined> {
  const payload = { session_id: sessionID, hook_event_name: hookEventName, tool_name: tool }
  if (tool === skillTool) {
    // The skill is loaded after the call, and the result names its folder.
    // The hook copies the folder for the evidence.
    if (skillDir) await fire(directory, hook, { ...payload, skill_dir: skillDir })
    return
  }
  if (commandTools.includes(tool)) {
    // The call ID pairs this hook with the other hook of the same call, so
    // overlapping commands keep their own snapshots.
    return responseFrom(directory, hook, { ...payload, tool_use_id: callID })
  }
  if (!fileWritingTools.includes(tool)) return
  for (const fp of filePathsFromArgs(args)) {
    await fire(directory, hook, { ...payload, file_path: fp })
  }
}

// server is the OpenCode 1.x entry point.
async function server({ directory, client }: any) {
  // toast shows a message in the TUI. It is not awaited, so a slow TUI never
  // holds back the session or a tool result, and a run without a TUI only
  // logs the failure.
  const toast = (message: string) => {
    Promise.resolve()
      .then(() => client.tui.showToast({ body: { message, variant: "info" } }))
      .catch((err: unknown) => console.error("chainloop-trace: could not show the message: " + err))
  }
  // noReply stores the message without asking the model for an answer. The
  // description is shown as a toast.
  const post: Post = async (sessionID, instruction, description) => {
    if (description) toast(description)
    if (!instruction) return
    await client.session.prompt({
      path: { id: sessionID },
      body: { noReply: true, parts: [{ type: "text", text: instruction, synthetic: true }] },
    })
  }
  // reminding holds the sessions that have a reminder being posted. The post
  // is a message too, and must not ask for another reminder.
  const reminding = new Set<string>()
  const remind: Post = async (sessionID, reminder) => {
    reminding.add(sessionID)
    try {
      await post(sessionID, reminder)
    } finally {
      reminding.delete(sessionID)
    }
  }
  return {
    // The event handler waits until the message is stored, so a first turn
    // sent right away still finds it.
    event: async ({ event }: any) => {
      const info = event.properties?.info
      const sessionID = info?.id ?? ""
      if (event.type === "session.created" && info?.parentID) childSessions.add(sessionID)
      await sessionEvent(directory, event.type, sessionID, info?.parentID, post)
    },
    // At each user message, the plugin asks for the spec reminder. A message
    // made only of synthetic parts is one that the plugin posted itself.
    "chat.message": async (input: any, output: any) => {
      const sessionID = input.sessionID
      if (childSessions.has(sessionID) || reminding.has(sessionID)) return
      const parts: any[] = output?.parts ?? []
      if (parts.length > 0 && parts.every((p: any) => p?.synthetic)) return
      await promptSubmitted(directory, sessionID, remind)
    },
    "tool.execute.before": async (input: any, output: any) => {
      await toolEvent(directory, "pre-tool-use", "tool.execute.before", input.sessionID, input.tool, input.callID, output.args)
    },
    "tool.execute.after": async (input: any, output: any) => {
      const res = await toolEvent(directory, "post-tool-use", "tool.execute.after", input.sessionID, input.tool, input.callID, input.args, input.tool === skillTool ? skillDirFrom(output) : "")
      // The toast reaches the user now. The tool output reaches the model,
      // whose reply stays on screen after the toast is gone. An aborted tool
      // can have no output to add to.
      if (res?.message) toast(res.message)
      if (res?.relayToModel && typeof output?.output === "string") {
        output.output = output.output + "\n\n" + res.relayToModel
      }
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
    const res = await toolEvent(directory, "post-tool-use", "tool.execute.after", event.sessionID, event.tool, event.id, event.input, event.tool === skillTool ? skillDirFrom(event.output, event.result, event) : "")
    // The model reads the content parts of the result, and the TUI shows them
    // in the command block, so one part reaches both the user and the model.
    // A failed command has no result to add to.
    if (res?.relayToModel && Array.isArray(event.result?.content)) {
      event.result.content.push({ type: "text", text: res.relayToModel })
    }
  })

  // resume: false stores the message without asking the model for an answer.
  // The TUI shows a synthetic message by its description only, and the model
  // reads its text, which is empty when there is only a banner.
  const post: Post = async (sessionID, instruction, description) => {
    await ctx.session.synthetic({ sessionID, text: instruction, description, resume: false })
  }

  // A child session belongs to a subagent, whose parent session already gets
  // the reminder. A subagent's first prompt can come before its
  // session.created event, so this reads the parent from the session itself.
  // A session that cannot be read gets the reminder: a missed reminder costs
  // more than an extra one.
  const isSubagent = async (sessionID: string) => {
    try {
      return Boolean((await ctx.session.get({ sessionID }))?.parentID)
    } catch (err) {
      console.error("chainloop-trace: could not read session " + sessionID + ": " + err)
      return false
    }
  }

  // Events arrive asynchronously, so a session's first prompt can come in
  // while the session is still starting. The prompt hook runs before the
  // prompt is admitted, and waits for that start, so the instruction is
  // stored ahead of the prompt. The reminder then lands ahead of the prompt
  // too. A synthetic message does not pass through the prompt hook, so the
  // reminder asks for no other.
  const starting = new Map<string, Promise<void>>()
  await ctx.session.hook("prompt", async (event: any) => {
    const sessionID = event.sessionID
    const started = starting.get(sessionID)
    if (started) {
      starting.delete(sessionID)
      await started
    }
    if (await isSubagent(sessionID)) return
    await promptSubmitted(directory, sessionID, post)
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
// A deleted session gets no more messages, so it also leaves childSessions.
// Trace run needs no such cleanup: the plugin ends with the wrapped command.
const sessionDeletedHandler = `  if (type === "session.deleted") {
    childSessions.delete(sessionID)
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
	content = strings.Replace(content, "{{SkillTool}}", strconv.Quote(skillTool), 1)
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

// HooksInstalled reports whether the plugin file is present. The file is
// entirely chainloop-owned, so its presence is the installation.
func (p *Provider) HooksInstalled(repoRoot string) (bool, error) {
	_, err := os.Stat(filepath.Join(repoRoot, settingsFile))
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}

	return err == nil, err
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
		SkillDir      string `json:"skill_dir"`
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

	// The plugin takes the skill folder from the result text when the
	// metadata has none, and there it is a file URL. A folder that is not
	// absolute is dropped.
	skillDir, _ := skill.ParseDir(raw.SkillDir)

	return &trace.HookInput{
		SessionID:     raw.SessionID,
		HookEventName: raw.HookEventName,
		ToolName:      raw.ToolName,
		FilePath:      filePath,
		ToolUseID:     raw.ToolUseID,
		SkillDir:      skillDir,
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
