import type { Plugin } from "@opencode-ai/plugin"

// HookResponse is what a chainloop hook prints on stdout when it has
// something for the user. It mirrors the Go hookResponse type; the two are
// one contract and have to change together.
type HookResponse = {
  // message is shown directly, as a TUI toast.
  message?: string
  // relayToModel is appended to the tool output, so the model repeats it.
  relayToModel?: string
}

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

  // fire runs a chainloop hook and returns whatever it asked us to show the
  // user, or nothing at all, which is the common case. Tracing must never
  // block tool execution, so a chainloop that is missing, that fails, or
  // that prints something other than JSON is logged to stderr and otherwise
  // ignored — the thrown error says which it was.
  //
  // .text() implies .quiet(), so the hook's JSON reply is captured instead
  // of being echoed into the terminal as raw text.
  async function fire(event: string, payload: Record<string, any>): Promise<HookResponse> {
    const json = JSON.stringify(payload)
    try {
      const stdout = (await $`echo ${json} | chainloop trace hook opencode ${event}`.text()).trim()
      if (!stdout) return {}
      return JSON.parse(stdout) as HookResponse
    } catch (err) {
      console.error(`chainloop-trace: ${event} hook failed: ${err}`)
      return {}
    }
  }

  // toast puts a message in front of the user. Guarded: a headless run has
  // no TUI to show it in, and a notification is never worth interrupting a
  // session over.
  async function toast(message: string) {
    try {
      await client.tui.showToast({ body: { message, variant: "info" } })
    } catch (err) {
      console.error("chainloop-trace: could not show toast: " + err)
    }
  }

  return {
    event: async ({ event }) => {
      if (event.type === "session.created") {
        const sessionID = event.properties?.info?.id ?? ""
        const res = await fire("session-start", { session_id: sessionID, hook_event_name: "session.created" })
        if (res.message) await toast(res.message)
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
    "tool.execute.after": async (input, output) => {
      if (commandTools.includes(input.tool)) {
        const res = await fire("post-tool-use", {
          session_id: input.sessionID,
          hook_event_name: "tool.execute.after",
          tool_name: input.tool,
        })
        // A shell command may have been a git push, whose pre-push hook
        // attested the session and left a link to it. Show it on both
        // channels: the toast reaches the user now, the tool output reaches
        // the model, whose reply outlives the toast.
        if (res.message) await toast(res.message)
        if (res.relayToModel) output.output = output.output + "\n\n" + res.relayToModel
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
