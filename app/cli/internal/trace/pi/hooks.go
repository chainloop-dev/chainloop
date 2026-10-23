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

package pi

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/chainloop-dev/chainloop/app/cli/internal/trace"
	"github.com/chainloop-dev/chainloop/app/cli/internal/trace/state"
)

const (
	settingsFile       = ".pi/extensions/chainloop-trace.ts"
	ownershipMarker    = "// chainloop-trace: managed Pi extension\n"
	maxHookPayloadSize = 16 * 1024 * 1024
)

var (
	fileWritingTools = []string{"edit", "write"}
	commandTools     = []string{"bash"}
)

const extensionTemplate = ownershipMarker + `import { spawn } from "node:child_process"
import { homedir } from "node:os"
import { resolve } from "node:path"
import { fileURLToPath } from "node:url"

const maxOutputBytes = 64 * 1024
const hookTimeoutMs = 60 * 1000
const killGraceMs = 5 * 1000
const instructionMarker = "chainloop-trace-instruction:"
const reminderMarker = "chainloop-trace-spec-reminder"
const treeMarker = "chainloop-trace-tree-marker"
const fileWritingTools = {{FileWritingToolsArray}}
const commandTools = {{CommandToolsArray}}
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

{{SessionEndHandler}}

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
`

const sessionEndHandler = `  pi.on("session_shutdown", async (event, ctx) => {
    if (!sessionEndReasons.has(event.reason)) return
    const id = sessionID(ctx)
    try {
      await runHook(ctx, "session-end", hookPayload(ctx, event.type + ":" + event.reason))
    } catch {
    } finally {
      starts.delete(id)
      instructed.delete(id)
    }
  })`

// SettingsFile returns the generated project-local Pi extension path.
func (p *Provider) SettingsFile(repoRoot string) string {
	return filepath.Join(repoRoot, settingsFile)
}

// InstallHooks writes the permanent extension, including session shutdown.
func (p *Provider) InstallHooks(repoRoot string) error {
	return p.writeExtension(repoRoot, true, false)
}

// InstallHooksForTraceRun writes the temporary extension without session end;
// trace run owns final attestation and restores the previous file.
func (p *Provider) InstallHooksForTraceRun(repoRoot string) error {
	return p.writeExtension(repoRoot, false, true)
}

func (p *Provider) writeExtension(repoRoot string, includeSessionEnd, allowUnowned bool) error {
	path := p.SettingsFile(repoRoot)
	content := renderExtension(includeSessionEnd)

	info, err := os.Lstat(path)
	switch {
	case err == nil:
		if !info.Mode().IsRegular() {
			return errors.New("install Pi extension: existing path is not a regular file")
		}
		existing, readErr := os.ReadFile(path)
		if readErr != nil {
			return fmt.Errorf("install Pi extension: read existing file: %w", readErr)
		}
		if bytes.Equal(existing, content) {
			return nil
		}
		if !allowUnowned && !bytes.HasPrefix(existing, []byte(ownershipMarker)) {
			return errors.New("install Pi extension: existing file is not managed by Chainloop")
		}
	case !errors.Is(err, os.ErrNotExist):
		return fmt.Errorf("install Pi extension: inspect existing path: %w", err)
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("install Pi extension: create directory: %w", err)
	}
	if err := os.WriteFile(path, content, 0o600); err != nil {
		return fmt.Errorf("install Pi extension: write file: %w", err)
	}
	return nil
}

func renderExtension(includeSessionEnd bool) []byte {
	handler := ""
	if includeSessionEnd {
		handler = sessionEndHandler
	}
	source := strings.Replace(extensionTemplate, "{{FileWritingToolsArray}}", toolsArrayLiteral(fileWritingTools), 1)
	source = strings.Replace(source, "{{CommandToolsArray}}", toolsArrayLiteral(commandTools), 1)
	return []byte(strings.Replace(source, "{{SessionEndHandler}}", handler, 1))
}

func toolsArrayLiteral(tools []string) string {
	data, _ := json.Marshal(tools)
	return string(data)
}

// UninstallHooks removes only an extension carrying Chainloop's ownership marker.
func (p *Provider) UninstallHooks(repoRoot string) error {
	path := p.SettingsFile(repoRoot)
	info, err := os.Lstat(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("uninstall Pi extension: inspect file: %w", err)
	}
	if !info.Mode().IsRegular() {
		return nil
	}
	content, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("uninstall Pi extension: read file: %w", err)
	}
	if !bytes.HasPrefix(content, []byte(ownershipMarker)) {
		return nil
	}
	if err := os.Remove(path); err != nil {
		return fmt.Errorf("uninstall Pi extension: remove file: %w", err)
	}
	return nil
}

// HooksInstalled reports whether the Pi extension is Chainloop-managed.
func (p *Provider) HooksInstalled(repoRoot string) (bool, error) {
	path := p.SettingsFile(repoRoot)
	info, err := os.Lstat(path)
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if !info.Mode().IsRegular() {
		return false, nil
	}
	content, err := os.ReadFile(path)
	if err != nil {
		return false, err
	}
	return bytes.HasPrefix(content, []byte(ownershipMarker)), nil
}

// ReadHookInput decodes payloads emitted by the generated Pi extension.
func (p *Provider) ReadHookInput(r io.Reader) (*trace.HookInput, error) {
	data, err := io.ReadAll(io.LimitReader(r, maxHookPayloadSize))
	if err != nil {
		return nil, err
	}

	var raw struct {
		SessionID      string `json:"session_id"`
		HookEventName  string `json:"hook_event_name"`
		ToolName       string `json:"tool_name"`
		FilePath       string `json:"file_path"`
		Cwd            string `json:"cwd"`
		TranscriptPath string `json:"transcript_path"`
		ToolUseID      string `json:"tool_use_id"`
		ToolFailed     bool   `json:"tool_failed"`
		Command        string `json:"command"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, err
	}

	filePath := raw.FilePath
	if filePath != "" && !filepath.IsAbs(filePath) {
		base := raw.Cwd
		if base == "" {
			base, err = os.Getwd()
			if err != nil {
				return nil, err
			}
		}
		filePath = filepath.Join(base, filePath)
	}

	return &trace.HookInput{
		SessionID:      raw.SessionID,
		HookEventName:  raw.HookEventName,
		ToolName:       raw.ToolName,
		FilePath:       filePath,
		Cwd:            raw.Cwd,
		TranscriptPath: raw.TranscriptPath,
		ToolUseID:      raw.ToolUseID,
		ToolFailed:     raw.ToolFailed,
		Command:        raw.Command,
	}, nil
}

// IsFileWritingTool reports whether Pi's tool writes one named file.
func (p *Provider) IsFileWritingTool(toolName string) bool {
	return slices.Contains(fileWritingTools, toolName)
}

// IsCommandTool reports whether Pi's tool can mutate the working tree broadly.
func (p *Provider) IsCommandTool(toolName string) bool {
	return slices.Contains(commandTools, toolName)
}

// SupportsSessionStartBanner reports that the extension can notify Pi's UI.
func (p *Provider) SupportsSessionStartBanner() bool { return true }

// SupportsSessionStartInstruction reports that before_agent_start can inject context.
func (p *Provider) SupportsSessionStartInstruction() bool { return true }

// DeduplicatesSessionStartInstruction reports that the extension persists a
// marker carrying Pi's session ID and owns exactly-once delivery.
func (p *Provider) DeduplicatesSessionStartInstruction() bool { return true }

// SupportsPromptReminder reports that before_agent_start runs for every user turn.
func (p *Provider) SupportsPromptReminder() bool { return true }

// AnnounceSessionStart writes the response consumed by the Pi extension.
func (p *Provider) AnnounceSessionStart(msg trace.SessionStartMessage) error {
	if msg.Empty() {
		return nil
	}
	return json.NewEncoder(os.Stdout).Encode(struct {
		Banner      string `json:"banner,omitempty"`
		Instruction string `json:"instruction,omitempty"`
	}{Banner: msg.Banner, Instruction: msg.Instruction})
}

// AnnouncePromptSubmit writes the per-turn reminder response.
func (p *Provider) AnnouncePromptSubmit(reminder string) error {
	if reminder == "" {
		return nil
	}
	return json.NewEncoder(os.Stdout).Encode(struct {
		Instruction string `json:"instruction"`
	}{Instruction: reminder})
}

// AnnounceToUser writes a message for Pi's UI or headless stderr channel.
func (p *Provider) AnnounceToUser(msg string) error {
	if msg == "" {
		return nil
	}
	return json.NewEncoder(os.Stdout).Encode(struct {
		Message string `json:"message"`
	}{Message: msg})
}

func fileSnapshotKey(input *trace.HookInput) state.FileSnapshotKey {
	return state.FileSnapshotKey{SessionID: input.SessionID, FilePath: input.FilePath, ToolUseID: input.ToolUseID}
}
