import { spawn } from "node:child_process"

const fileWritingTools = ["edit","write","apply_patch","patch"]
const commandTools = ["bash","shell"]
const skillTool = "skill"

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
  // The banner is the description the user sees. A session with a banner and
  // no instruction gives the model the banner.
  await postInstruction(post, sessionID, res.instruction || res.banner!, res.banner)
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
  if (type === "session.deleted") {
    childSessions.delete(sessionID)
    await fire(directory, "session-end", { session_id: sessionID, hook_event_name: type })
  }
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
  // reads its text.
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
