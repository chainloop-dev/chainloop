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

// Package copilot captures GitHub Copilot sessions, from Copilot CLI and from
// the Copilot agent in VS Code, through the Copilot hook files and the
// session event log that Copilot keeps on disk.
package copilot

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"slices"

	"github.com/chainloop-dev/chainloop/app/cli/internal/trace"
	"github.com/chainloop-dev/chainloop/app/cli/internal/trace/state"
)

// Name is the identifier stored in session records and evidence.
const Name = "copilot"

var _ trace.Provider = (*Provider)(nil)

// Provider connects GitHub Copilot to Trace. It holds no session state.
type Provider struct {
	// out is where hook responses go. Nil means os.Stdout.
	out io.Writer
}

// New creates a Copilot provider.
func New() *Provider { return &Provider{} }

// Name returns the provider identifier.
func (p *Provider) Name() string { return Name }

// CaptureFileSnapshot stores the content of input.FilePath before a file
// tool runs. A missing file (a new file) is not an error.
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

	return store.SaveFileSnapshot(fileSnapshotKey(input), content)
}

// ResolveBeforeContent returns the snapshot that CaptureFileSnapshot saved.
func (p *Provider) ResolveBeforeContent(store *state.Store, input *trace.HookInput, _ []byte) []byte {
	if input == nil || input.FilePath == "" {
		return nil
	}

	snap, err := store.LoadFileSnapshot(fileSnapshotKey(input))
	if err != nil {
		return nil
	}
	return snap
}

// CleanupAfterEdit deletes the snapshot of the edit.
func (p *Provider) CleanupAfterEdit(store *state.Store, input *trace.HookInput) {
	if input == nil || input.FilePath == "" {
		return
	}
	store.DeleteFileSnapshot(fileSnapshotKey(input))
}

// fileSnapshotKey has no tool call ID, because Copilot sends none: the key is
// the session and the path (TRC-006).
func fileSnapshotKey(input *trace.HookInput) state.FileSnapshotKey {
	return state.FileSnapshotKey{SessionID: input.SessionID, FilePath: input.FilePath}
}

// IsFileWritingTool returns true if the named tool writes one file.
func (p *Provider) IsFileWritingTool(toolName string) bool {
	return slices.Contains(fileWritingTools, toolName)
}

// IsCommandTool returns true if the named tool runs a shell command.
func (p *Provider) IsCommandTool(toolName string) bool {
	return slices.Contains(commandTools, toolName)
}

// SupportsSessionStartBanner is true: Copilot shows the progress messages of a
// hook to the user.
func (p *Provider) SupportsSessionStartBanner() bool { return true }

// SupportsSessionStartInstruction is true: Copilot adds the additionalContext
// of a sessionStart hook to the session.
func (p *Provider) SupportsSessionStartInstruction() bool { return true }

// SupportsPromptReminder is false: Copilot drops the output of a
// userPromptSubmitted command hook.
func (p *Provider) SupportsPromptReminder() bool { return false }

// AnnounceSessionStart shows the banner to the user and gives the
// instruction to the model.
func (p *Provider) AnnounceSessionStart(msg trace.SessionStartMessage) error {
	return p.writeResponse(msg.Banner, msg.Instruction)
}

// AnnouncePromptSubmit does nothing: Copilot drops the output of the hook.
func (p *Provider) AnnouncePromptSubmit(string) error { return nil }

// AnnounceToUser shows the message to the user as a progress line. The model
// gets no copy: it would show the message to the user a second time.
func (p *Provider) AnnounceToUser(msg string) error {
	return p.writeResponse(msg, "")
}

// progressMessage is a line that Copilot shows to the user, before the final
// output of a hook. It does not change the output.
type progressMessage struct {
	Type    string `json:"type"`
	Message string `json:"message"`
}

// hookResponse is the final output of a sessionStart or postToolUse hook.
type hookResponse struct {
	AdditionalContext string `json:"additionalContext"`
}

// writeResponse writes the message for the user as a progress line, then the
// context for the model as the final JSON document. Each is one line, and an
// empty one is left out. Copilot reads only the last document as the output
// of the hook.
func (p *Provider) writeResponse(userMessage, context string) error {
	out := p.out
	if out == nil {
		out = os.Stdout
	}
	enc := json.NewEncoder(out)
	if userMessage != "" {
		if err := enc.Encode(progressMessage{Type: "progress", Message: userMessage}); err != nil {
			return err
		}
	}
	if context == "" {
		return nil
	}
	return enc.Encode(hookResponse{AdditionalContext: context})
}
