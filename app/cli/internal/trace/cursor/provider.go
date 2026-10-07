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

package cursor

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"sort"
	"time"

	"github.com/chainloop-dev/chainloop/app/cli/internal/trace"
	"github.com/chainloop-dev/chainloop/app/cli/internal/trace/attribution"
	"github.com/chainloop-dev/chainloop/app/cli/internal/trace/state"
	"github.com/chainloop-dev/chainloop/pkg/attestation/crafter/materials/aicodingsession"
)

// Name is the provider identifier emitted to evidence and used as the key
// in SessionRecord.Provider. Kept as an exported constant so CLI wiring
// can refer to it without instantiating the provider.
const Name = "cursor"

// agentName aliases Name for use inside this package's non-Provider code
// (parse output, log fields) so we don't collide with the Provider.Name()
// method name at read time.
const agentName = Name

// Compile-time check that Provider implements trace.Provider.
var _ trace.Provider = (*Provider)(nil)

// Provider implements trace.Provider for Cursor agent sessions.
type Provider struct{}

// New creates a new Cursor provider.
func New() *Provider {
	return &Provider{}
}

// Name returns the agent identifier.
func (p *Provider) Name() string {
	return agentName
}

// IsFileWritingTool returns true for the synthetic "edit" tool name we set on
// afterFileEdit hook inputs. Cursor has no tool-name concept for generic
// preToolUse/postToolUse events in our flow, so "edit" is the sole signal.
func (p *Provider) IsFileWritingTool(toolName string) bool {
	return toolName == syntheticEditToolName
}

// IsCommandTool always returns false for Cursor: shell-execution hooks are not
// yet wired (Cursor installs only sessionStart/sessionEnd/afterFileEdit), so
// shell-driven file changes are not captured for this provider.
func (p *Provider) IsCommandTool(_ string) bool {
	return false
}

// AnnounceSessionStart writes Cursor's sessionStart hook response. Cursor adds
// its additional_context field to the conversation's initial system context,
// which is how the model receives the instruction. The response has no field
// that Cursor shows to the user, so the banner is dropped.
//
// A message with no instruction writes nothing, so that Cursor's JSON response
// parser never sees an empty document.
func (p *Provider) AnnounceSessionStart(msg trace.SessionStartMessage) error {
	if msg.Instruction == "" {
		return nil
	}

	resp := struct {
		AdditionalContext string `json:"additional_context"`
	}{AdditionalContext: msg.Instruction}

	return json.NewEncoder(os.Stdout).Encode(resp)
}

// SupportsSessionStartBanner is false for Cursor: its sessionStart response has
// no field that reaches the user.
func (p *Provider) SupportsSessionStartBanner() bool {
	return false
}

// SupportsSessionStartInstruction is true for Cursor: additional_context in the
// sessionStart response reaches the model.
func (p *Provider) SupportsSessionStartInstruction() bool {
	return true
}

// SupportsPromptReminder is false for Cursor: it documents no channel that
// adds context at prompt submit, so it gets the session-start instruction
// only.
func (p *Provider) SupportsPromptReminder() bool {
	return false
}

// AnnouncePromptSubmit writes nothing for Cursor, which installs no
// prompt-submit hook.
func (p *Provider) AnnouncePromptSubmit(_ string) error {
	return nil
}

// AnnounceToUser is unsupported for Cursor: it installs only sessionStart,
// sessionEnd and afterFileEdit, so no hook fires after a shell command and
// there is nowhere to deliver the message.
func (p *Provider) AnnounceToUser(_ string) error {
	return trace.ErrAnnounceUnsupported
}

// CaptureFileSnapshot is a no-op for Cursor: the afterFileEdit hook
// delivers old/new strings directly, so no pre-edit snapshot is needed.
func (p *Provider) CaptureFileSnapshot(_ *state.Store, _ *trace.HookInput) error {
	return nil
}

// ResolveBeforeContent reverses the edits reported by Cursor's
// afterFileEdit hook against the file's current content to reconstruct
// the pre-edit state. Returns nil when no edits were reported.
func (p *Provider) ResolveBeforeContent(_ *state.Store, input *trace.HookInput, after []byte) []byte {
	if input == nil || len(input.Edits) == 0 {
		return nil
	}

	return attribution.ReverseApplyEdits(after, input.Edits)
}

// CleanupAfterEdit is a no-op for Cursor: there's no per-edit state to
// release because no snapshot was taken.
func (p *Provider) CleanupAfterEdit(_ *state.Store, _ *trace.HookInput) {}

// CopySessionData copies the Cursor transcript for sessionID into
// the store's raw/<sessionID>.jsonl. Handles both flat and nested
// source layouts; the destination is always flat so downstream consumers
// (parse, pre-push) don't need to re-resolve.
func (p *Provider) CopySessionData(store *state.Store, loc trace.SessionLocation) error {
	sessionID := loc.SessionID
	sourceDir := transcriptDirForRepo(loc.Cwd)
	if sourceDir == "" {
		return nil
	}

	src, err := resolveSessionJSONL(sourceDir, sessionID)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}

		return err
	}

	return store.CopyRawSessionFile(sessionID, src)
}

// ParseSession reads the copied transcript JSONL for sessionID and returns
// a best-effort Evidence. Cursor's transcript format does not include token
// usage, so Usage stays empty and the result carries a warning.
func (p *Provider) ParseSession(_ context.Context, opts *trace.ParseOpts) (*aicodingsession.Evidence, error) {
	jsonlPath, err := state.FindRawSessionFile(opts.SessionDir, opts.SessionID)
	if err != nil {
		return nil, fmt.Errorf("locate cursor transcript: %w", err)
	}

	parsed, err := parseCursorJSONL(jsonlPath)
	if err != nil {
		return nil, err
	}

	info, err := os.Stat(jsonlPath)
	var startedAt string
	if err == nil {
		startedAt = info.ModTime().UTC().Format(time.RFC3339)
	}

	warnings := []string{
		"cursor transcripts do not expose token usage; usage and cost are reported as zero",
	}

	rawSession := map[string][]json.RawMessage{"main": parsed.RawRecords}
	if parsed.RawRecords == nil {
		rawSession["main"] = []json.RawMessage{}
	}

	return aicodingsession.NewEvidence(aicodingsession.Data{
		SchemaVersion: "v1",
		Agent: aicodingsession.Agent{
			Name:    agentName,
			Version: opts.AgentVersion,
		},
		Session: aicodingsession.Session{
			ID:        opts.SessionID,
			StartedAt: startedAt,
		},
		Model: &aicodingsession.Model{
			Primary:  opts.Model,
			Provider: "cursor",
		},
		ToolsUsed: buildToolsUsed(parsed.ToolCounts),
		Conversation: &aicodingsession.Conversation{
			TotalMessages:     parsed.UserMessages + parsed.AssistantMessages,
			UserMessages:      parsed.UserMessages,
			AssistantMessages: parsed.AssistantMessages,
		},
		RawSession: rawSession,
		Warnings:   warnings,
	}), nil
}

// buildToolsUsed converts a tool-name → invocation-count map into the
// Evidence summary, sorted by invocation count descending so the most-used
// tools surface first.
func buildToolsUsed(counts map[string]int) *aicodingsession.ToolsUsed {
	if len(counts) == 0 {
		return nil
	}

	summary := make([]aicodingsession.ToolSummary, 0, len(counts))
	total := 0
	for name, count := range counts {
		summary = append(summary, aicodingsession.ToolSummary{ToolName: name, InvocationCount: count})
		total += count
	}
	sort.Slice(summary, func(i, j int) bool {
		if summary[i].InvocationCount != summary[j].InvocationCount {
			return summary[i].InvocationCount > summary[j].InvocationCount
		}

		return summary[i].ToolName < summary[j].ToolName
	})

	return &aicodingsession.ToolsUsed{
		Summary:          summary,
		TotalInvocations: total,
	}
}
