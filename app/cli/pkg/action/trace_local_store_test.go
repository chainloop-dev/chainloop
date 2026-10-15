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

package action

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/chainloop-dev/chainloop/app/cli/internal/trace/claude"
	"github.com/chainloop-dev/chainloop/app/cli/internal/trace/spec"
	"github.com/chainloop-dev/chainloop/app/cli/internal/trace/state"
	"github.com/chainloop-dev/chainloop/pkg/attestation/crafter/materials/aicodingsession"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestStoreSessionsLocally follows R-002/R-003/R-004/R-006: the same assembly a
// push runs is written to disk, content-addressed, redacted by default, with a
// manifest that declares the output is unsigned and not an attestation.
func TestStoreSessionsLocally(t *testing.T) {
	const sessionID = "7c1e2d-session"
	// Assembled from fragments so nothing credential-shaped is committed.
	const pat = "ghp_erOZlZv0B1e3amrQ" + "ugdwZ8Ro2W4kDql9WPTf"
	const ticketText = "---\nkind: ticket\nuri: https://tracker.example.com/ENG-1234\n---\n\nAdd the export button.\nToken: " + pat + "\n"
	ticket := specCapture(t, "ticket-eng-1234.md", ticketText, "2026-10-08T10:00:00Z")

	// A fresh session each run: attachSessionEvidence mutates the evidence
	// (spec entries, pointerized transcript), so the two runs must not share it.
	newSession := func() sessionEvidence {
		evidence := aicodingsession.NewEvidence(aicodingsession.Data{
			SchemaVersion: "v1",
			Agent:         aicodingsession.Agent{Name: "claude"},
			Session:       aicodingsession.Session{ID: sessionID, StartedAt: "2026-10-08T10:00:00Z", DurationSeconds: 100},
			RawSession:    map[string][]json.RawMessage{mainStream: {json.RawMessage(`{"type":"user","message":{"role":"user","content":"hi"}}`)}},
		})
		return sessionEvidence{
			sessionID: sessionID,
			provider:  claude.New(),
			evidence:  evidence,
			specs:     []spec.Capture{ticket},
		}
	}

	ctx := context.Background()

	// A push against the fake adder is the reference for the EVIDENCE (spec)
	// material digests: it and the disk sink receive the identical redacted spec
	// copy, so the digests must match (R-002).
	pushAdder := &fakeMaterialAdder{realDigests: true}
	pushAttested, _ := attachSessionEvidence(ctx, pushAdder, state.NewGitStore(t.TempDir()), []sessionEvidence{newSession()}, zerolog.Nop(), false)
	require.Equal(t, []string{sessionID}, pushAttested)

	t.Run("content-addressed, redacted, unsigned session on disk", func(t *testing.T) {
		tracePath := t.TempDir()
		stored, err := storeSessionsLocally(ctx, zerolog.Nop(), state.NewGitStore(t.TempDir()), []sessionEvidence{newSession()}, tracePath, false)
		require.NoError(t, err)
		require.Equal(t, 1, stored)

		sessionDir := filepath.Join(tracePath, state.SanitizeID(sessionID))
		var m localManifest
		readJSONFile(t, filepath.Join(sessionDir, manifestFileName), &m)

		// R-006: the index declares the stored evidence is not a signed attestation.
		assert.False(t, m.Signed)
		assert.False(t, m.IsAttestation)
		assert.True(t, m.Redaction)
		assert.Equal(t, "ai-coding-session-7c1e2d", m.EvidenceRecord)
		require.NotEmpty(t, m.Materials)

		var specDigest string
		for _, mat := range m.Materials {
			content, err := os.ReadFile(filepath.Join(sessionDir, mat.File))
			require.NoError(t, err)

			// R-003: every material is stored under its content digest.
			sum := sha256.Sum256(content)
			assert.Equal(t, "sha256:"+hex.EncodeToString(sum[:]), mat.Digest, "the stored file hashes back to its digest for %s", mat.Name)

			// R-004: redaction on by default, so no stored file holds the secret.
			assert.NotContains(t, string(content), pat, "no stored file holds the secret: %s", mat.Name)

			if mat.Kind == "EVIDENCE" {
				specDigest = mat.Digest
				// R-002: the spec material digest equals what a push produces.
				assert.Equal(t, pushAdder.byName(mat.Name).digest, mat.Digest, "same digest as a push for %s", mat.Name)
			}
		}
		require.NotEmpty(t, specDigest, "the ticket became an EVIDENCE material")

		// The evidence record references the stored spec material by digest.
		record := materialByName(t, m, m.EvidenceRecord)
		var ev aicodingsession.Evidence
		readJSONFile(t, filepath.Join(sessionDir, record.File), &ev)
		referenced := make([]string, 0, len(ev.Data.Spec))
		for _, e := range ev.Data.Spec {
			referenced = append(referenced, e.Digest)
		}
		assert.Contains(t, referenced, specDigest, "the evidence record points at the stored spec material")
	})

	t.Run("the no-redact opt-out keeps the raw secret", func(t *testing.T) {
		tracePath := t.TempDir()
		stored, err := storeSessionsLocally(ctx, zerolog.Nop(), state.NewGitStore(t.TempDir()), []sessionEvidence{newSession()}, tracePath, true)
		require.NoError(t, err)
		require.Equal(t, 1, stored)

		sessionDir := filepath.Join(tracePath, state.SanitizeID(sessionID))
		var m localManifest
		readJSONFile(t, filepath.Join(sessionDir, manifestFileName), &m)
		assert.False(t, m.Redaction, "the manifest records that redaction was off")

		var found bool
		for _, mat := range m.Materials {
			content, err := os.ReadFile(filepath.Join(sessionDir, mat.File))
			require.NoError(t, err)
			if strings.Contains(string(content), pat) {
				found = true
			}
		}
		assert.True(t, found, "the opt-out keeps the raw secret in the output")
	})
}

// TestRunTraceLocalStoreNoSessions covers the local pre-push path when nothing
// was recorded: it stores nothing, returns the resolved default path, and
// reaches no control plane (it never builds an executor), so a human-only push
// is never blocked (decision 4) and needs no network (R-005).
func TestRunTraceLocalStoreNoSessions(t *testing.T) {
	_, gitDir := initGitRepo(t)
	require.NoError(t, state.NewGitStore(gitDir).InitTraceDir())

	stored, tracePath, err := RunTraceLocalStore(t.Context(), zerolog.Nop(), RunTraceLocalStoreOpts{Mode: aicodingsession.ModeCoding})
	require.NoError(t, err)
	assert.Zero(t, stored, "nothing recorded, nothing stored")
	assert.Equal(t, state.NewGitStore(gitDir).LocalSessionsDir(), tracePath, "the default path is resolved and returned")

	_, statErr := os.Stat(tracePath)
	assert.True(t, os.IsNotExist(statErr), "no output directory is created when there is nothing to store")
}

// TestResolveTracePath covers the path resolver: a persisted absolute or
// repo-relative path wins, and an empty one falls back to the store default.
func TestResolveTracePath(t *testing.T) {
	store := state.NewGitStore(filepath.Join(t.TempDir(), ".git"))
	repoRoot := "/repo"

	assert.Equal(t, store.LocalSessionsDir(), ResolveTracePath(repoRoot, store, ""))
	assert.Equal(t, "/abs/sessions", ResolveTracePath(repoRoot, store, "/abs/sessions"))
	assert.Equal(t, filepath.Join(repoRoot, "evidence"), ResolveTracePath(repoRoot, store, "evidence"))
}

func TestLocalSessionStartBanner(t *testing.T) {
	t.Run("names the on-disk destination and the push timing", func(t *testing.T) {
		got := localSessionStartBanner("/repo/.git/chainloop-trace/local-sessions", false)
		assert.Contains(t, got, "recording this session")
		assert.Contains(t, got, "local mode")
		assert.Contains(t, got, "/repo/.git/chainloop-trace/local-sessions")
		assert.Contains(t, got, "when you push")
		// No Chainloop/control-plane vocabulary: a reader should not think a
		// backend is involved.
		assert.NotContains(t, got, "attest")
		assert.NotContains(t, got, "dashboard")
	})

	t.Run("says this run finishes under trace run", func(t *testing.T) {
		got := localSessionStartBanner("/repo/.git/chainloop-trace/local-sessions", true)
		assert.Contains(t, got, "local mode")
		assert.Contains(t, got, "when this run finishes")
		assert.NotContains(t, got, "when you push")
	})

	t.Run("drops the destination when unknown", func(t *testing.T) {
		got := localSessionStartBanner("", false)
		assert.Contains(t, got, "recording this session")
		assert.Contains(t, got, "local mode")
		assert.NotContains(t, got, "writes the session evidence")
	})
}

func readJSONFile(t *testing.T, path string, v any) {
	t.Helper()
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	require.NoError(t, json.Unmarshal(data, v))
}

func materialByName(t *testing.T, m localManifest, name string) storedMaterial {
	t.Helper()
	for _, mat := range m.Materials {
		if mat.Name == name {
			return mat
		}
	}
	require.Failf(t, "material not found", "material %q not found in manifest", name)
	return storedMaterial{}
}
