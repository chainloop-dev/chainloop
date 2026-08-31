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
	"errors"
	"fmt"
	"os"
	"os/exec"
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

// DiscoverSession finds the most recent opencode session for the given repo root.
// Returns nil, nil if no matching session is found or the opencode binary is unavailable.
func (p *Provider) DiscoverSession(repoRoot string) (*trace.DiscoveredSession, error) {
	session, err := discoverOpenCodeSession(repoRoot)
	if err != nil || session == nil {
		return nil, err
	}

	return &trace.DiscoveredSession{
		SessionID:  session.ID,
		SessionDir: "",
		// opencode sessions live in a SQLite DB, not a directory; there's
		// no reliable "alive" signal from session list alone. Treat
		// discovered sessions as potentially active.
		IsActive: true,
	}, nil
}

// SessionDirForRepo returns "" for opencode — sessions are stored in a
// SQLite database, not a per-repo directory. The method exists to satisfy
// the interface; callers use CopySessionData for the actual data extraction.
func (p *Provider) SessionDirForRepo(_ string) string {
	return ""
}

// CopySessionData runs `opencode export <sessionID>` and streams the JSON
// output directly to the store's raw/<sanitized-id>.jsonl so
// pre-push can parse it independently of opencode's own storage. stdout is
// redirected to the destination file rather than a pipe: opencode
// (Node.js) doesn't reliably flush stdout to pipes, causing truncation at
// pipe buffer boundaries, but a regular file destination is safe. Fails
// gracefully (returns nil) when the opencode binary is missing or the
// export fails — the session is skipped rather than blocking the push,
// and any partially-written file is removed.
func (p *Provider) CopySessionData(store *state.Store, _, sessionID string) error {
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

	cmd := exec.CommandContext(ctx, "opencode", "export", sessionID)
	cmd.Stdout = f
	runErr := cmd.Run()
	_ = f.Close()
	if runErr != nil {
		_ = os.Remove(dst)
		return nil
	}

	return nil
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

// SystemMessage is a no-op for opencode: the plugin system has no
// systemMessage channel comparable to Claude Code's SessionStart output.
func (p *Provider) SystemMessage(_ string) error {
	return nil
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
