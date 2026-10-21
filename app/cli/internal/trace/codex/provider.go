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

// Package codex captures sessions from the Codex CLI's native hooks and JSONL rollouts.
package codex

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"slices"

	"github.com/chainloop-dev/chainloop/app/cli/internal/trace"
	"github.com/chainloop-dev/chainloop/app/cli/internal/trace/state"
)

// Name is the identifier stored in session records and evidence.
const Name = "codex"

var _ trace.Provider = (*Provider)(nil)

// Provider connects Codex to Trace. It holds no session state.
type Provider struct{}

// New creates a Codex provider.
func New() *Provider { return &Provider{} }

// Name returns the provider identifier.
func (p *Provider) Name() string { return Name }

// CaptureFileSnapshot reads the file at input.FilePath and stores its
// content under the store's snapshots/ directory so the post-tool-use
// handler can compute line-range diffs after the edit. A missing file
// (a patch creating a new file) is treated as a non-error;
// other read errors (permission denied, I/O failures) are surfaced.
func (p *Provider) CaptureFileSnapshot(store *state.Store, input *trace.HookInput) error {
	if input == nil || input.FilePath == "" {
		return nil
	}

	content, err := os.ReadFile(input.FilePath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}

		return fmt.Errorf("read %q: %w", input.FilePath, err)
	}

	return store.SaveFileSnapshot(input.SessionID, input.FilePath, content)
}

// ResolveBeforeContent returns the snapshot saved by CaptureFileSnapshot.
// The "after" argument is unused for Codex — we always rely on the pre-edit
// snapshot — but is part of the interface for providers that reconstruct
// from edits instead of snapshots.
func (p *Provider) ResolveBeforeContent(store *state.Store, input *trace.HookInput, _ []byte) []byte {
	if input == nil || input.FilePath == "" {
		return nil
	}

	snap, err := store.LoadFileSnapshot(input.SessionID, input.FilePath)
	if err != nil {
		return nil
	}

	return snap
}

// CleanupAfterEdit removes the per-edit snapshot. Safe to call when no
// snapshot was ever taken.
func (p *Provider) CleanupAfterEdit(store *state.Store, input *trace.HookInput) {
	if input == nil || input.FilePath == "" {
		return
	}

	store.DeleteFileSnapshot(input.SessionID, input.FilePath)
}

// IsFileWritingTool returns true if the named tool modifies files on disk.
func (p *Provider) IsFileWritingTool(toolName string) bool {
	return slices.Contains(fileWritingTools, toolName)
}

// IsCommandTool returns true if the named tool runs a shell command.
func (p *Provider) IsCommandTool(toolName string) bool {
	return slices.Contains(commandTools, toolName)
}

// SupportsSessionStartBanner reports whether Codex shows systemMessage to the user.
func (p *Provider) SupportsSessionStartBanner() bool { return true }

// SupportsSessionStartInstruction reports whether Codex accepts startup context.
func (p *Provider) SupportsSessionStartInstruction() bool { return true }

// SupportsPromptReminder reports whether Codex accepts context at each prompt.
func (p *Provider) SupportsPromptReminder() bool { return true }

// AnnounceSessionStart sends the banner and instruction in one hook response.
func (p *Provider) AnnounceSessionStart(msg trace.SessionStartMessage) error {
	return writeHookResponse(eventSessionStart, msg.Banner, msg.Instruction)
}

// AnnouncePromptSubmit adds the reminder to the model's context.
func (p *Provider) AnnouncePromptSubmit(reminder string) error {
	return writeHookResponse(eventUserPromptSubmit, "", reminder)
}

// AnnounceToUser shows the session link and gives the model context to repeat it.
func (p *Provider) AnnounceToUser(msg string) error {
	if msg == "" {
		return nil
	}
	return writeHookResponse(eventPostToolUse, msg, "Tell the user the following, including any link verbatim: "+msg)
}

type hookSpecificOutput struct {
	HookEventName     string `json:"hookEventName"`
	AdditionalContext string `json:"additionalContext,omitempty"`
}

type hookResponse struct {
	SystemMessage      string             `json:"systemMessage,omitempty"`
	HookSpecificOutput hookSpecificOutput `json:"hookSpecificOutput"`
}

// Codex reads one JSON document per hook. Empty messages emit nothing.
func writeHookResponse(event, banner, instruction string) error {
	if banner == "" && instruction == "" {
		return nil
	}
	return json.NewEncoder(os.Stdout).Encode(hookResponse{
		SystemMessage:      banner,
		HookSpecificOutput: hookSpecificOutput{HookEventName: event, AdditionalContext: instruction},
	})
}
