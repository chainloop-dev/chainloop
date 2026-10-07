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
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/chainloop-dev/chainloop/app/cli/internal/trace"
	"github.com/chainloop-dev/chainloop/app/cli/internal/trace/state"
	"github.com/chainloop-dev/chainloop/pkg/attestation/crafter/materials/aicodingsession"
)

// Name is the provider identifier stored in session records and evidence.
const Name = "pi"

// Provider captures and parses Pi sessions. Hook integration is added before
// the provider is registered with the shared registry.
type Provider struct{}

// New creates a Pi provider.
func New() *Provider {
	return &Provider{}
}

// Name returns the agent identifier.
func (p *Provider) Name() string {
	return Name
}

// CopySessionData refreshes the store's private copy of a persisted Pi JSONL
// session. Pi reports the authoritative path through its extension; there is
// no cwd-derived fallback because --no-session has no transcript to discover.
//
// The old copy is removed before the source is opened. The shared pre-push
// path still attempts parsing after a refresh error, so leaving the old file
// in place could attest stale evidence. A successful refresh becomes visible
// only after its temporary file is closed and atomically renamed.
func (p *Provider) CopySessionData(store *state.Store, loc trace.SessionLocation) error {
	if store == nil || !state.ValidSessionID(loc.SessionID) {
		return copyError("invalid session")
	}

	rawDir := store.RawSessionDir()
	if err := os.MkdirAll(rawDir, 0o755); err != nil {
		return copyError("prepare destination")
	}

	dst := state.RawSessionPath(rawDir, loc.SessionID)
	cleanupPrevious, err := quarantinePrevious(dst)
	if err != nil {
		return err
	}
	defer cleanupPrevious()

	if loc.TranscriptPath == "" {
		return copyError("transcript unavailable")
	}

	src, err := os.Open(loc.TranscriptPath)
	if err != nil {
		return copyError("transcript unavailable")
	}
	defer func() { _ = src.Close() }()

	tmp, err := os.CreateTemp(rawDir, ".pi-session-*")
	if err != nil {
		return copyError("create temporary copy")
	}
	tmpPath := tmp.Name()
	keep := false
	defer func() {
		_ = tmp.Close()
		if !keep {
			_ = os.Remove(tmpPath)
		}
	}()

	if _, err := io.Copy(tmp, src); err != nil {
		return copyError("read transcript")
	}
	if err := tmp.Close(); err != nil {
		return copyError("close temporary copy")
	}
	if err := os.Rename(tmpPath, dst); err != nil {
		return copyError("publish transcript")
	}

	keep = true
	return nil
}

// quarantinePrevious moves the old copy out of the exact path ParseSession
// reads. If the directory cannot rename it, truncation is a last-resort
// invalidation so the shared pre-push flow cannot parse stale evidence after
// this refresh returns an error.
func quarantinePrevious(dst string) (func(), error) {
	if _, err := os.Stat(dst); errors.Is(err, os.ErrNotExist) {
		return func() {}, nil
	} else if err != nil {
		return nil, copyError("inspect previous copy")
	}

	quarantine := fmt.Sprintf("%s.stale-%d-%d", dst, os.Getpid(), time.Now().UnixNano())
	if err := os.Rename(dst, quarantine); err == nil {
		return func() { _ = os.Remove(quarantine) }, nil
	}

	if err := os.Truncate(dst, 0); err == nil {
		return nil, copyError("quarantine previous copy")
	}

	return nil, copyError("quarantine previous copy")
}

func copyError(stage string) error {
	return fmt.Errorf("%w: copy Pi session: %s", trace.ErrSessionDataNotFresh, stage)
}

// ParseSession reads the copied Pi JSONL and maps its selected branch to AI
// coding-session evidence. The live Pi transcript is never parsed in place.
func (p *Provider) ParseSession(_ context.Context, opts *trace.ParseOpts) (*aicodingsession.Evidence, error) {
	if opts == nil || !state.ValidSessionID(opts.SessionID) {
		return nil, errors.New("parse Pi session: invalid options")
	}

	path, err := state.FindRawSessionFile(opts.SessionDir, opts.SessionID)
	if err != nil {
		return nil, errors.New("parse Pi session: copied transcript unavailable")
	}

	return parseSession(path, opts.SessionID, opts.AgentVersion)
}
