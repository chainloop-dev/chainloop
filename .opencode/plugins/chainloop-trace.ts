import type { Plugin } from "@opencode-ai/plugin"

export const ChainloopTrace: Plugin = async ({ $ }) => {
  const fileWritingTools = ["edit","write","apply_patch"]

  function filePathFromArgs(args: any): string {
    if (args?.filePath) return args.filePath
    if (args?.path) return args.path
    return ""
  }

  async function fire(event: string, payload: Record<string, any>) {
    const json = JSON.stringify(payload)
    await $`echo ${json} | chainloop trace hook opencode ${event}`
  }

  return {
    event: async ({ event }) => {
      if (event.type === "session.created") {
        const sessionID = event.properties?.info?.id ?? ""
        await fire("session-start", { session_id: sessionID, hook_event_name: "session.created" })
      }
      if (event.type === "session.deleted") {
        const sessionID = event.properties?.info?.id ?? ""
        await fire("session-end", { session_id: sessionID, hook_event_name: "session.deleted" })
      }
    },
    "tool.execute.before": async (input, output) => {
      if (!fileWritingTools.includes(input.tool)) return
      await fire("pre-tool-use", {
        session_id: input.sessionID,
        hook_event_name: "tool.execute.before",
        tool_name: input.tool,
        file_path: filePathFromArgs(output.args),
      })
    },
    "tool.execute.after": async (input) => {
      if (!fileWritingTools.includes(input.tool)) return
      await fire("post-tool-use", {
        session_id: input.sessionID,
        hook_event_name: "tool.execute.after",
        tool_name: input.tool,
        file_path: filePathFromArgs(input.args),
      })
    },
  }
}
