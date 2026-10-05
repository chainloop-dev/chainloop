import { spawn } from "node:child_process"

const fileWritingTools = ["edit","write","apply_patch","patch"]
const commandTools = ["bash","shell"]

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
  if (type === "session.deleted") {
    await fire(directory, "session-end", { session_id: sessionID, hook_event_name: type })
  }
}

async function toolEvent(directory: string, hook: string, hookEventName: string, sessionID: string, tool: string, args: any) {
  const payload = { session_id: sessionID, hook_event_name: hookEventName, tool_name: tool }
  if (commandTools.includes(tool)) {
    await fire(directory, hook, payload)
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
      await toolEvent(directory, "pre-tool-use", "tool.execute.before", input.sessionID, input.tool, output.args)
    },
    "tool.execute.after": async (input: any) => {
      await toolEvent(directory, "post-tool-use", "tool.execute.after", input.sessionID, input.tool, input.args)
    },
  }
}

// setup is the OpenCode 2 entry point. OpenCode 2 runs plugins in a shared
// background server, so the session directory is the plugin location, not
// the process working directory.
async function setup(ctx: any) {
  const directory = ctx.location.directory
  await ctx.tool.hook("execute.before", async (event: any) => {
    await toolEvent(directory, "pre-tool-use", "tool.execute.before", event.sessionID, event.tool, event.input)
  })
  await ctx.tool.hook("execute.after", async (event: any) => {
    await toolEvent(directory, "post-tool-use", "tool.execute.after", event.sessionID, event.tool, event.input)
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
