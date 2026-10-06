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

// instructionFrom fires a hook that can answer with an instruction for the
// model, and returns the instruction it wrote to stdout, if Chainloop has one.
async function instructionFrom(directory: string, event: string, sessionID: string, hookEventName: string): Promise<string> {
  const out = await fire(directory, event, { session_id: sessionID, hook_event_name: hookEventName })
  if (!out.trim()) return ""
  try {
    return JSON.parse(out).instruction ?? ""
  } catch (err) {
    console.error("chainloop-trace: could not read the " + event + " response: " + err)
    return ""
  }
}

// Post adds an instruction to the session as a context-only message, which
// the model reads without replying to it. Each OpenCode major has its own API
// for it.
type Post = (sessionID: string, instruction: string) => Promise<void>

// postInstruction posts the instruction and waits until it is stored, so a
// turn sent right away still finds it. A failed post costs the instruction
// only, so it is logged and never fails the caller.
async function postInstruction(post: Post, sessionID: string, instruction: string) {
  try {
    await post(sessionID, instruction)
  } catch (err) {
    console.error("chainloop-trace: could not post the session instruction: " + err)
  }
}

async function sessionCreated(directory: string, sessionID: string, parentID: string | undefined, post: Post) {
  const instruction = await instructionFrom(directory, "session-start", sessionID, "session.created")
  // A child session belongs to a subagent, whose parent already has the
  // instruction.
  if (!instruction || parentID) return
  await postInstruction(post, sessionID, instruction)
}

// childSessions holds the OpenCode 1.x sessions of subagents, whose parent
// session already gets the instruction and the reminder. Session IDs are
// unique, so one set serves every server() call.
const childSessions = new Set<string>()

// promptSubmitted runs at each user message. The user-prompt-submit hook can
// answer with a short reminder to capture a new or changed spec, which is
// posted ahead of the turn.
async function promptSubmitted(directory: string, sessionID: string, post: Post) {
  const reminder = await instructionFrom(directory, "user-prompt-submit", sessionID, "chat.message")
  if (reminder) await postInstruction(post, sessionID, reminder)
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
