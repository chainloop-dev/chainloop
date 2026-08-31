//
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

package opencode

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"github.com/chainloop-dev/chainloop/app/cli/internal/trace"
)

const (
	// settingsFile is the project-local opencode plugin file path.
	settingsFile = ".opencode/plugins/chainloop-trace.ts"
)

// fileWritingTools is the single source of truth for opencode tool names
// that modify files. The TypeScript plugin's array is generated from this
// slice (see {{FileWritingToolsArray}}) so the plugin and IsFileWritingTool
// can never drift.
var fileWritingTools = []string{"edit", "write", "apply_patch"}

// commandTools are opencode tool names that run shell commands. Their file
// changes are captured via a before/after working-tree snapshot (they have no
// single file-path argument), so the plugin fires the pre/post-tool-use hooks
// for them without a file_path and the handler branches on the tool kind.
var commandTools = []string{"bash"}

// bt is a single backtick, used to splice backticks into Go raw string literals.
const bt = "`"

// pluginTemplate is the TypeScript plugin written to .opencode/plugins/chainloop-trace.ts.
// It fires chainloop trace hook subcommands on session lifecycle and tool events.
// The {{SessionEndBlock}} placeholder is replaced with the session.deleted handler
// for full install, or removed entirely for trace-run install.
//
// Go raw string literals cannot contain backticks, so the TypeScript template
// literal (Bun shell) lines are split at backtick boundaries and spliced with bt.
const pluginTemplate = `import type { Plugin } from "@opencode-ai/plugin"

export const ChainloopTrace: Plugin = async ({ $ }) => {
  const fileWritingTools = {{FileWritingToolsArray}}
  const commandTools = {{CommandToolsArray}}

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
      await $` + bt + `echo ${json} | chainloop trace hook opencode ${event}` + bt + `
    } catch (err) {
      console.error(` + bt + `chainloop-trace: ${event} hook failed: ${err}` + bt + `)
    }
  }

  return {
    event: async ({ event }) => {
      if (event.type === "session.created") {
        const sessionID = event.properties?.info?.id ?? ""
        await fire("session-start", { session_id: sessionID, hook_event_name: "session.created" })
      }
{{SessionEndBlock}}
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
`

// sessionDeletedHandler is the session.deleted block inserted for full install.
// session.idle fires at the end of every agent turn and would prematurely end
// the trace session; session.deleted fires only when the session is destroyed.
const sessionDeletedHandler = `      if (event.type === "session.deleted") {
        const sessionID = event.properties?.info?.id ?? ""
        await fire("session-end", { session_id: sessionID, hook_event_name: "session.deleted" })
      }`

// SettingsFile returns the absolute path to the plugin file for the given repo.
func (p *Provider) SettingsFile(repoRoot string) string {
	return filepath.Join(repoRoot, settingsFile)
}

// InstallHooks writes the opencode plugin file with all hooks including session.deleted.
func (p *Provider) InstallHooks(repoRoot string) error {
	return p.writePluginFile(repoRoot, true)
}

// InstallHooksForTraceRun writes the plugin file without the session.deleted handler;
// trace run drives end-of-session itself.
func (p *Provider) InstallHooksForTraceRun(repoRoot string) error {
	return p.writePluginFile(repoRoot, false)
}

func (p *Provider) writePluginFile(repoRoot string, includeSessionEnd bool) error {
	pluginPath := filepath.Join(repoRoot, settingsFile)

	content := pluginTemplate
	content = strings.Replace(content, "{{FileWritingToolsArray}}", toolsArrayLiteral(fileWritingTools), 1)
	content = strings.Replace(content, "{{CommandToolsArray}}", toolsArrayLiteral(commandTools), 1)
	if includeSessionEnd {
		content = strings.Replace(content, "{{SessionEndBlock}}", sessionDeletedHandler, 1)
	} else {
		content = strings.Replace(content, "{{SessionEndBlock}}\n", "", 1)
	}

	if err := os.MkdirAll(filepath.Dir(pluginPath), 0755); err != nil {
		return fmt.Errorf("create plugin directory: %w", err)
	}

	if existing, err := os.ReadFile(pluginPath); err == nil && string(existing) == content {
		return nil
	}

	return os.WriteFile(pluginPath, []byte(content), 0600)
}

// toolsArrayLiteral renders a tool-name slice as a TypeScript array literal so
// the plugin and the Go IsFileWritingTool/IsCommandTool predicates share one
// source of truth. json.Marshal output is valid TypeScript for a []string.
func toolsArrayLiteral(tools []string) string {
	b, _ := json.Marshal(tools)
	return string(b)
}

// UninstallHooks removes the plugin file. The plugin file is entirely
// chainloop-owned, so we remove it without preserving content.
func (p *Provider) UninstallHooks(repoRoot string) error {
	pluginPath := filepath.Join(repoRoot, settingsFile)

	err := os.Remove(pluginPath)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}

	return err
}

// maxHookPayloadBytes caps hook payload reads to defend against runaway or
// malformed payloads.
const maxHookPayloadBytes = 16 * 1024 * 1024

// ReadHookInput reads and parses the opencode hook JSON from the given reader.
// The plugin pipes JSON to stdin with fields: session_id, hook_event_name,
// tool_name, file_path.
func (p *Provider) ReadHookInput(r io.Reader) (*trace.HookInput, error) {
	data, err := io.ReadAll(io.LimitReader(r, maxHookPayloadBytes))
	if err != nil {
		return nil, err
	}

	var raw struct {
		SessionID     string `json:"session_id"`
		HookEventName string `json:"hook_event_name"`
		ToolName      string `json:"tool_name"`
		FilePath      string `json:"file_path"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, err
	}

	return &trace.HookInput{
		SessionID:     raw.SessionID,
		HookEventName: raw.HookEventName,
		ToolName:      raw.ToolName,
		FilePath:      raw.FilePath,
	}, nil
}

// patchFileRe matches *** Add File:, *** Update File:, or *** Delete File:
// section headers in an apply_patch patchText payload. It mirrors the
// regex used by the TypeScript plugin's parsePatchPaths function so the
// parsing logic is testable from Go (the TS plugin splits per-file before
// the Go side ever sees patchText).
var patchFileRe = regexp.MustCompile(`(?m)^\*\*\* (?:Add|Update|Delete) File: (.+)$`)

// parsePatchPaths extracts affected file paths from an apply_patch
// patchText payload. Each section starts with *** Add File:, *** Update
// File:, or *** Delete File: followed by the path. Paths are deduplicated
// while preserving first-seen order. Both absolute and repository-relative
// paths are returned verbatim — the caller is responsible for any
// normalization.
func parsePatchPaths(patchText string) []string {
	var paths []string
	seen := make(map[string]bool)
	for _, m := range patchFileRe.FindAllStringSubmatch(patchText, -1) {
		p := strings.TrimSpace(m[1])
		if p == "" || seen[p] {
			continue
		}
		seen[p] = true
		paths = append(paths, p)
	}
	return paths
}

// IsFileWritingTool returns true if the named tool modifies files on disk.
func (p *Provider) IsFileWritingTool(toolName string) bool {
	return slices.Contains(fileWritingTools, toolName)
}

// IsCommandTool returns true if the named tool runs a shell command.
func (p *Provider) IsCommandTool(toolName string) bool {
	return slices.Contains(commandTools, toolName)
}

// opencodeBinaryAvailable checks whether the opencode CLI is on PATH.
func opencodeBinaryAvailable() bool {
	_, err := exec.LookPath("opencode")
	return err == nil
}
