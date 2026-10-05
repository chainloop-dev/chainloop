import type { Plugin } from "@opencode-ai/plugin"

export const ChainloopTrace: Plugin = async ({ $, client }) => {
  const fileWritingTools = ["edit","write","apply_patch"]
  const commandTools = ["bash"]

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

  // fire-and-forget: tracing must never block tool execution. If chainloop
  // is unavailable or errors, log to stderr and move on.
  async function fire(event: string, payload: Record<string, any>) {
    const json = JSON.stringify(payload)
    try {
      await $`echo ${json} | chainloop trace hook opencode ${event}`
    } catch (err) {
      console.error(`chainloop-trace: ${event} hook failed: ${err}`)
    }
  }

  // instructionFrom fires a hook that can answer with an instruction for the
  // model, and returns what it wrote to stdout: that instruction, if
  // Chainloop has one.
  async function instructionFrom(event: string, sessionID: string, hookEventName: string): Promise<string> {
    const json = JSON.stringify({ session_id: sessionID, hook_event_name: hookEventName })
    try {
      const out = await $`echo ${json} | chainloop trace hook opencode ${event}`.text()
      return out.trim() ? (JSON.parse(out).instruction ?? "") : ""
    } catch (err) {
      console.error(`chainloop-trace: ${event} hook failed: ${err}`)
      return ""
    }
  }

  // childSessions holds the sessions of subagents, whose parent session
  // already gets the instruction and the reminder.
  const childSessions = new Set<string>()
  // reminding holds the sessions that have a reminder being posted. The post
  // is a message too, and must not ask for another reminder.
  const reminding = new Set<string>()

  // postInstruction adds the instruction to the session as a context-only
  // message: noReply stores it without asking the model for an answer. The
  // caller waits for it, so a first turn sent right away still finds it.
  async function postInstruction(sessionID: string, instruction: string) {
    try {
      await client.session.prompt({
        path: { id: sessionID },
        body: { noReply: true, parts: [{ type: "text", text: instruction, synthetic: true }] },
      })
    } catch (err) {
      console.error(`chainloop-trace: could not post the session instruction: ${err}`)
    }
  }

  return {
    event: async ({ event }) => {
      if (event.type === "session.created") {
        const info = event.properties?.info
        const sessionID = info?.id ?? ""
        // A child session belongs to a subagent, whose parent already has
        // the instruction.
        if (info?.parentID) childSessions.add(sessionID)
        const instruction = await instructionFrom("session-start", sessionID, "session.created")
        if (instruction && !info?.parentID) await postInstruction(sessionID, instruction)
      }
      if (event.type === "session.deleted") {
        const sessionID = event.properties?.info?.id ?? ""
        await fire("session-end", { session_id: sessionID, hook_event_name: "session.deleted" })
      }
    },
    // At each user message, the user-prompt-submit hook can answer with a
    // short reminder to capture a new or changed spec. A message made only of
    // synthetic parts is one that the plugin posted itself.
    "chat.message": async (input, output) => {
      const sessionID = input.sessionID
      if (childSessions.has(sessionID) || reminding.has(sessionID)) return
      const parts: any[] = output?.parts ?? []
      if (parts.length > 0 && parts.every((p) => p?.synthetic)) return
      const reminder = await instructionFrom("user-prompt-submit", sessionID, "chat.message")
      if (!reminder) return
      reminding.add(sessionID)
      try {
        await postInstruction(sessionID, reminder)
      } finally {
        reminding.delete(sessionID)
      }
    },
    "tool.execute.before": async (input, output) => {
      if (commandTools.includes(input.tool)) {
        await fire("pre-tool-use", {
          session_id: input.sessionID,
          hook_event_name: "tool.execute.before",
          tool_name: input.tool,
        })
        return
      }
      if (!fileWritingTools.includes(input.tool)) return
      for (const fp of filePathsFromArgs(output.args)) {
        await fire("pre-tool-use", {
          session_id: input.sessionID,
          hook_event_name: "tool.execute.before",
          tool_name: input.tool,
          file_path: fp,
        })
      }
    },
    "tool.execute.after": async (input) => {
      if (commandTools.includes(input.tool)) {
        await fire("post-tool-use", {
          session_id: input.sessionID,
          hook_event_name: "tool.execute.after",
          tool_name: input.tool,
        })
        return
      }
      if (!fileWritingTools.includes(input.tool)) return
      for (const fp of filePathsFromArgs(input.args)) {
        await fire("post-tool-use", {
          session_id: input.sessionID,
          hook_event_name: "tool.execute.after",
          tool_name: input.tool,
          file_path: fp,
        })
      }
    },
  }
}
