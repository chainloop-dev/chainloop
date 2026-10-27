// chainloop-trace: managed Pi extension
import { spawn } from "node:child_process"
import { homedir } from "node:os"
import { resolve } from "node:path"
import { fileURLToPath } from "node:url"

const maxOutputBytes = 64 * 1024
const hookTimeoutMs = 60 * 1000
const killGraceMs = 5 * 1000
const instructionMarker = "chainloop-trace-instruction:"
const reminderMarker = "chainloop-trace-spec-reminder"
const treeMarker = "chainloop-trace-tree-marker"
const fileWritingTools = ["edit","write"]
const commandTools = ["bash"]
const sessionEndReasons = new Set(["quit", "new", "resume", "fork"])
const unicodeSpaces = /[\u00A0\u2000-\u200A\u202F\u205F\u3000]/g

function resolveToolPath(input, cwd) {
  let normalized = input.replace(unicodeSpaces, " ")
  if (normalized.startsWith("@")) normalized = normalized.slice(1)
  if (normalized === "~") return homedir()
  if (normalized.startsWith("~/") || (process.platform === "win32" && normalized.startsWith("~\\"))) {
    normalized = resolve(homedir(), normalized.slice(2))
  }
  if (/^file:\/\//.test(normalized)) normalized = fileURLToPath(normalized)
  return resolve(cwd, normalized)
}

function sessionID(ctx) {
  return ctx.sessionManager.getHeader()?.id ?? ctx.sessionManager.getSessionId()
}

function hookPayload(ctx, hookEventName) {
  const transcriptPath = ctx.sessionManager.getSessionFile()
  return {
    session_id: sessionID(ctx),
    hook_event_name: hookEventName,
    cwd: ctx.cwd,
    ...(transcriptPath ? { transcript_path: transcriptPath } : {}),
  }
}

function runHook(ctx, event, payload) {
  return new Promise((resolveHook) => {
    let settled = false
    let failed = false
    let stdoutBytes = 0
    let stderrBytes = 0
    const stdout = []
    const stderr = []
    let killTimer
    let timeoutTimer
    let exited = false
    let child

    const finish = (response) => {
      if (settled) return
      settled = true
      if (timeoutTimer) clearTimeout(timeoutTimer)
      if (killTimer) clearTimeout(killTimer)
      resolveHook(response)
    }

    const closeStreams = () => {
      try { child?.stdin.destroy() } catch {}
      try { child?.stdout.destroy() } catch {}
      try { child?.stderr.destroy() } catch {}
    }

    try {
      child = spawn("chainloop", ["trace", "hook", "pi", event], {
        cwd: ctx.cwd,
        stdio: ["pipe", "pipe", "pipe"],
      })

      const stop = () => {
        if (failed) return
        failed = true
        if (!exited) {
          try { child.kill("SIGTERM") } catch {}
        }
        if (!killTimer) {
          killTimer = setTimeout(() => {
            if (!exited) {
              try { child.kill("SIGKILL") } catch {}
            }
            closeStreams()
            finish()
          }, killGraceMs)
        }
      }

      const capture = (chunks, chunk, current) => {
        const remaining = maxOutputBytes - current
        if (remaining > 0) chunks.push(chunk.subarray(0, remaining))
        if (chunk.length > remaining) stop()
        return Math.min(maxOutputBytes, current + chunk.length)
      }

      child.stdout.on("data", (chunk) => { stdoutBytes = capture(stdout, chunk, stdoutBytes) })
      child.stderr.on("data", (chunk) => { stderrBytes = capture(stderr, chunk, stderrBytes) })
      child.on("error", () => {
        exited = true
        finish()
      })
      child.on("exit", () => { exited = true })
      child.on("close", (code) => {
        exited = true
        if (failed || code !== 0) return finish()
        const output = Buffer.concat(stdout).toString("utf8").trim()
        if (!output) return finish()
        try {
          finish(JSON.parse(output))
        } catch {
          finish()
        }
      })
      child.stdin.on("error", () => {})
      child.stdin.end(JSON.stringify(payload))
      timeoutTimer = setTimeout(stop, hookTimeoutMs)
    } catch {
      finish()
    }
  })
}

function showBanner(ctx, response) {
  try {
    if (ctx.hasUI && response?.banner) ctx.ui.notify(response.banner, "info")
  } catch {}
}

function showMessage(ctx, response) {
  if (!response?.message) return
  if (ctx.hasUI) ctx.ui.notify(response.message, "info")
  else process.stderr.write(response.message + "\n")
}

export default function (pi) {
  const starts = new Map()
  const instructed = new Set()

  const start = async (ctx, hookEventName) => {
    const id = sessionID(ctx)
    const response = await runHook(ctx, "session-start", hookPayload(ctx, hookEventName))
    if (response?.instruction) starts.set(id, response)
    showBanner(ctx, response)
    return response
  }

  pi.on("session_start", async (event, ctx) => {
    try {
      await start(ctx, event.type + ":" + event.reason)
    } catch {}
  })

  pi.on("session_shutdown", async (event, ctx) => {
    if (!sessionEndReasons.has(event.reason)) return
    const id = sessionID(ctx)
    try {
      await runHook(ctx, "session-end", hookPayload(ctx, event.type + ":" + event.reason))
    } catch {
    } finally {
      starts.delete(id)
      instructed.delete(id)
    }
  })

  pi.on("session_tree", (event, _ctx) => {
    try {
      pi.appendEntry(treeMarker, { selected: event.newLeafId })
    } catch {}
  })

  pi.on("before_agent_start", async (event, ctx) => {
    try {
      const id = sessionID(ctx)
      const marker = instructionMarker + id
      const persisted = ctx.sessionManager.getEntries().some((entry) =>
        entry.type === "custom_message" && entry.customType === marker,
      )
      const alreadyInstructed = instructed.has(id) || persisted
      if (persisted) instructed.add(id)

      const response = starts.get(id) ?? await start(ctx, event.type)
      const reminder = await runHook(ctx, "user-prompt-submit", hookPayload(ctx, event.type))
      const content = []
      let includesInstruction = false

      if (!alreadyInstructed && response?.instruction) {
        content.push(response.instruction)
        includesInstruction = true
      }
      if (reminder?.instruction) content.push(reminder.instruction)
      if (content.length === 0) return

      if (includesInstruction) instructed.add(id)
      return {
        message: {
          customType: includesInstruction ? marker : reminderMarker,
          content: content.join("\n\n"),
          display: false,
          details: { sessionId: id },
        },
      }
    } catch {
      return
    }
  })

  const toolPayload = (ctx, event) => {
    if (!fileWritingTools.includes(event.toolName) && !commandTools.includes(event.toolName)) return
    const payload = {
      ...hookPayload(ctx, event.type),
      tool_name: event.toolName,
      tool_use_id: event.toolCallId,
      ...(event.type === "tool_result" && event.isError ? { tool_failed: true } : {}),
    }
    if (fileWritingTools.includes(event.toolName)) {
      const path = event.input?.path
      if (typeof path !== "string" || path.length === 0) return
      payload.file_path = resolveToolPath(path, ctx.cwd)
    }
    if (commandTools.includes(event.toolName) && typeof event.input?.command === "string") {
      payload.command = event.input.command
    }
    return payload
  }

  pi.on("tool_call", async (event, ctx) => {
    try {
      const payload = toolPayload(ctx, event)
      if (payload) await runHook(ctx, "pre-tool-use", payload)
    } catch {}
    return undefined
  })

  pi.on("tool_result", async (event, ctx) => {
    try {
      const payload = toolPayload(ctx, event)
      if (!payload) return
      const response = await runHook(ctx, "post-tool-use", payload)
      showMessage(ctx, response)
    } catch {}
  })
}
