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
// on stdin. Tracing must never block or fail a tool call: if chainloop is
// unavailable or errors, log to stderr and move on.
function fire(directory: string, event: string, payload: Record<string, any>): Promise<void> {
  return new Promise((resolve) => {
    let settled = false
    const done = (err?: unknown) => {
      if (settled) return
      settled = true
      if (err) console.error("chainloop-trace: " + event + " hook failed: " + err)
      resolve()
    }
    try {
      const child = spawn("chainloop", ["trace", "hook", "opencode", event], { cwd: directory, stdio: ["pipe", "ignore", "ignore"] })
      child.on("error", done)
      child.on("close", (code) => done(code === 0 ? undefined : "exit code " + code))
      child.stdin.on("error", () => {})
      child.stdin.end(JSON.stringify(payload))
    } catch (err) {
      done(err)
    }
  })
}

async function sessionEvent(directory: string, type: string, sessionID: string) {
  if (type === "session.created") {
    await fire(directory, "session-start", { session_id: sessionID, hook_event_name: type })
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
async function server({ directory }: any) {
  return {
    event: async ({ event }: any) => {
      await sessionEvent(directory, event.type, event.properties?.info?.id ?? "")
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

  const controller = new AbortController()
  const consuming = (async () => {
    for await (const event of ctx.event.subscribe({ signal: controller.signal })) {
      await sessionEvent(directory, event.type, event.data?.sessionID ?? "")
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
