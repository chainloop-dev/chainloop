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

  // startSession fires the session-start hook and returns what it wrote to
  // stdout: the instruction for the model, if Chainloop has one.
  async function startSession(sessionID: string): Promise<string> {
    const json = JSON.stringify({ session_id: sessionID, hook_event_name: "session.created" })
    try {
      const out = await $`echo ${json} | chainloop trace hook opencode session-start`.text()
      return out.trim() ? (JSON.parse(out).instruction ?? "") : ""
    } catch (err) {
      console.error(`chainloop-trace: session-start hook failed: ${err}`)
      return ""
    }
  }

  // postInstruction adds the instruction to the session as a context-only
  // message: noReply stores it without asking the model for an answer.
  // It is not awaited, so the session never waits on it.
  function postInstruction(sessionID: string, instruction: string) {
    client.session
      .prompt({
        path: { id: sessionID },
        body: { noReply: true, parts: [{ type: "text", text: instruction, synthetic: true }] },
      })
      .catch((err: unknown) => console.error(`chainloop-trace: could not post the session instruction: ${err}`))
  }

  return {
    event: async ({ event }) => {
      if (event.type === "session.created") {
        const info = event.properties?.info
        const sessionID = info?.id ?? ""
        const instruction = await startSession(sessionID)
        // A child session belongs to a subagent, whose parent already has
        // the instruction.
        if (instruction && !info?.parentID) postInstruction(sessionID, instruction)
      }
      if (event.type === "session.deleted") {
        const sessionID = event.properties?.info?.id ?? ""
        await fire("session-end", { session_id: sessionID, hook_event_name: "session.deleted" })
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
