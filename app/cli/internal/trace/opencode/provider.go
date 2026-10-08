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
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"time"

	"github.com/chainloop-dev/chainloop/app/cli/internal/trace"
	"github.com/chainloop-dev/chainloop/app/cli/internal/trace/state"
	"github.com/chainloop-dev/chainloop/pkg/attestation/crafter/materials/aicodingsession"
)

// Name is the provider identifier emitted to evidence and used as the key
// in SessionRecord.Provider. Kept as an exported constant so CLI wiring
// can refer to it without instantiating the provider.
const Name = "opencode"

// Compile-time check that Provider implements trace.Provider.
var _ trace.Provider = (*Provider)(nil)

// Provider implements trace.Provider for opencode sessions.
type Provider struct{}

// New creates a new opencode provider.
func New() *Provider {
	return &Provider{}
}

// Name returns the agent identifier.
func (p *Provider) Name() string {
	return Name
}

// CopySessionData runs the opencode export command for the session (see
// exportArgs) and streams the JSON output directly to the store's
// raw/<sanitized-id>.jsonl so
// pre-push can parse it independently of opencode's own storage. stdout is
// redirected to the destination file rather than a pipe: opencode
// (Node.js) doesn't reliably flush stdout to pipes, causing truncation at
// pipe buffer boundaries, but a regular file destination is safe. Fails
// gracefully (returns nil) when the opencode binary is missing or the
// export fails — the session is skipped rather than blocking the push,
// and any partially-written file is removed.
func (p *Provider) CopySessionData(store *state.Store, loc trace.SessionLocation) error {
	sessionID := loc.SessionID
	if !opencodeBinaryAvailable() {
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	rawDir := store.RawSessionDir()
	if err := os.MkdirAll(rawDir, 0755); err != nil {
		return fmt.Errorf("create raw dir: %w", err)
	}

	dst := state.RawSessionPath(rawDir, sessionID)
	f, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0600)
	if err != nil {
		return fmt.Errorf("create raw session file: %w", err)
	}

	//nolint:gosec // a fixed binary and subcommand with the session ID as its argument, run without a shell
	cmd := exec.CommandContext(ctx, "opencode", exportArgs(ctx, sessionID)...)
	cmd.Stdout = f
	runErr := cmd.Run()
	_ = f.Close()
	if runErr != nil {
		_ = os.Remove(dst)
		return nil
	}

	return nil
}

// exportArgs returns the opencode arguments that print the session export.
// OpenCode 2 moved the command from `opencode export` to `opencode session
// export`. Each command is unknown to the other major version, so the
// installed version picks one. If the version cannot be read, the original
// command is used.
func exportArgs(ctx context.Context, sessionID string) []string {
	out, err := exec.CommandContext(ctx, "opencode", "--version").Output()
	if err == nil && majorVersion(string(out)) >= 2 {
		return []string{"session", "export", sessionID}
	}

	return []string{"export", sessionID}
}

// versionRe matches the first semantic version in `opencode --version`
// output, which is "1.18.34" for OpenCode 1.x and "opencode v2.0.22" for
// OpenCode 2.
var versionRe = regexp.MustCompile(`(\d+)\.\d+\.\d+`)

// majorVersion returns the major version in opencode --version output, or 0
// when the output has no version.
func majorVersion(output string) int {
	m := versionRe.FindStringSubmatch(output)
	if m == nil {
		return 0
	}

	major, err := strconv.Atoi(m[1])
	if err != nil {
		return 0
	}

	return major
}

// CaptureFileSnapshot reads the file at input.FilePath and stores its
// content under the store's snapshots/ directory so the post-tool-use
// handler can compute line-range diffs after the edit. A missing file
// (e.g. write creating a new file) is treated as a non-error.
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

// CleanupAfterEdit removes the per-edit snapshot.
func (p *Provider) CleanupAfterEdit(store *state.Store, input *trace.HookInput) {
	if input == nil || input.FilePath == "" {
		return
	}

	store.DeleteFileSnapshot(input.SessionID, input.FilePath)
}

// hookResponse is what a hook writes to stdout for the Chainloop plugin. OpenCode
// defines no hook response of its own, so the plugin is the other half of this
// type, and the two change together. The plugin picks a channel per field; a
// hook with nothing to say writes nothing.
type hookResponse struct {
	// Instruction is posted to the session as a context-only message, which
	// the model reads without replying to it.
	Instruction string `json:"instruction,omitempty"`

	// Banner greets the user at session start: a TUI toast in OpenCode 1.x,
	// the description of the synthetic message in OpenCode 2.
	Banner string `json:"banner,omitempty"`

	// Message is shown to the user after a shell command, as a TUI toast in
	// OpenCode 1.x.
	Message string `json:"message,omitempty"`

	// RelayToModel is added to the result of the shell command, which the
	// model reads. OpenCode 2 also shows it in the command block.
	RelayToModel string `json:"relayToModel,omitempty"`
}

// writeHookResponse writes resp as one JSON document on stdout, which is
// reserved for it: the hook logs to stderr and to the trace log file.
func writeHookResponse(resp hookResponse) error {
	if resp == (hookResponse{}) {
		return nil
	}

	return json.NewEncoder(os.Stdout).Encode(resp)
}

// AnnounceSessionStart writes the session-start response that the Chainloop
// plugin reads. The plugin posts the instruction to the session as a
// context-only message (no model reply), and shows the banner to the user.
func (p *Provider) AnnounceSessionStart(msg trace.SessionStartMessage) error {
	return writeHookResponse(hookResponse{Instruction: msg.Instruction, Banner: msg.Banner})
}

// SupportsSessionStartBanner is true for opencode: the plugin shows the banner
// as a toast in OpenCode 1.x, and as a synthetic message in OpenCode 2.
func (p *Provider) SupportsSessionStartBanner() bool {
	return true
}

// SupportsSessionStartInstruction is true for opencode: the plugin posts the
// instruction to the session.
func (p *Provider) SupportsSessionStartInstruction() bool {
	return true
}

// SupportsPromptReminder is true for opencode: the plugin posts the reminder
// to the session at each user message.
func (p *Provider) SupportsPromptReminder() bool {
	return true
}

// AnnouncePromptSubmit writes the prompt-submit response that the Chainloop
// plugin reads, in the same shape as the session-start one. The plugin posts
// the reminder to the session as a context-only message.
func (p *Provider) AnnouncePromptSubmit(reminder string) error {
	return writeHookResponse(hookResponse{Instruction: reminder})
}

// AnnounceToUser writes the response after a shell command for the plugin to
// deliver on two channels: a toast that reaches the user without the model,
// and the command result, which the model reads so it repeats the message in
// its reply. A toast goes away after a few seconds, and the user who looked
// away is the one this message is for; the reply is still on screen after.
func (p *Provider) AnnounceToUser(msg string) error {
	if msg == "" {
		return nil
	}

	return writeHookResponse(hookResponse{Message: msg, RelayToModel: trace.RelayToModelInstruction + msg})
}

// ParseSession reads the copied export JSON for sessionID and returns
// structured evidence.
func (p *Provider) ParseSession(_ context.Context, opts *trace.ParseOpts) (*aicodingsession.Evidence, error) {
	path, err := state.FindRawSessionFile(opts.SessionDir, opts.SessionID)
	if err != nil {
		return nil, fmt.Errorf("locate opencode export: %w", err)
	}

	return parseExport(path)
}
