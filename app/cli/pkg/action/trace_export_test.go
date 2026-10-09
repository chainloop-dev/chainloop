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
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
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

// cleanSession builds the evidence of a session that wrote one spec file into
// the spec folder and holds a copy of it in its transcript. It takes a text so
// the same session can be built with or without a secret, and returns a fresh
// value each call so two assembly passes do not share mutable state.
func cleanSession(t *testing.T, sessionID, ticketText string) []sessionEvidence {
	t.Helper()

	ticketPath := "/repo/.chainloop/specs/" + sessionID + "/ticket.md"
	raw := []json.RawMessage{
		json.RawMessage(`{"type":"assistant","message":{"role":"assistant","content":[{"type":"tool_use","id":"toolu_w","name":"Write","input":{"file_path":` + strconv.Quote(ticketPath) + `,"content":` + strconv.Quote(ticketText) + `}}]}}`),
		json.RawMessage(`{"type":"user","message":{"role":"user","content":[{"type":"tool_result","tool_use_id":"toolu_w","content":"File created"}]},"toolUseResult":{"type":"create","filePath":` + strconv.Quote(ticketPath) + `,"content":` + strconv.Quote(ticketText) + `,"structuredPatch":[],"originalFile":null}}`),
	}

	evidence := aicodingsession.NewEvidence(aicodingsession.Data{
		SchemaVersion: "0.1",
		Agent:         aicodingsession.Agent{Name: "claude-code"},
		Session:       aicodingsession.Session{ID: sessionID, StartedAt: "2026-10-08T10:00:00Z", DurationSeconds: 42},
		RawSession:    map[string][]json.RawMessage{mainStream: raw},
	})

	ticket := specCapture(t, "ticket.md", ticketText, "2026-10-08T10:00:00Z")

	return []sessionEvidence{{
		sessionID: sessionID,
		provider:  claude.New(),
		evidence:  evidence,
		specs:     []spec.Capture{ticket},
	}}
}

// TestRunTraceExportMatchesPush covers R-002 and R-003: the disk export runs the
// same assembly a push does, so it produces the same materials with the same
// content digests, and every material resolves from its digest.
func TestRunTraceExportMatchesPush(t *testing.T) {
	const sessionID = "abc123-session"
	ctx := context.Background()
	ticketText := "---\nkind: ticket\nuri: https://tracker.example.com/ENG-7\ntitle: Add export\n---\n\nThe export flow is missing.\n"

	store := state.NewGitStore(t.TempDir())

	// The export writes each material to disk.
	outDir := t.TempDir()
	sink := newDiskMaterialSink(outDir, false, zerolog.Nop())
	diskAttested, _ := attachSessionEvidence(ctx, sink, store, cleanSession(t, sessionID, ticketText), zerolog.Nop(), false)
	require.NoError(t, sink.writeManifest(true))
	require.Equal(t, []string{sessionID}, diskAttested)

	// The same assembly, as a push records it: realDigests makes the fake adder
	// hash each material's content exactly as the CAS does.
	fake := &fakeMaterialAdder{realDigests: true}
	fakeAttested, _ := attachSessionEvidence(ctx, fake, store, cleanSession(t, sessionID, ticketText), zerolog.Nop(), false)
	require.Equal(t, []string{sessionID}, fakeAttested)

	// R-002: the same materials, in the same order, under the same digests.
	sinkNames := make([]string, 0, len(sink.materials))
	for _, m := range sink.materials {
		sinkNames = append(sinkNames, m.Name)
	}
	require.Equal(t, fake.names(), sinkNames)
	for _, m := range sink.materials {
		assert.Equal(t, fake.byName(m.Name).digest, m.Digest, "digest of material %q", m.Name)
	}

	// R-003: every material is stored under its digest, and its bytes hash back
	// to it.
	for _, m := range sink.materials {
		content, err := os.ReadFile(filepath.Join(outDir, m.File))
		require.NoError(t, err, "material %q is stored", m.Name)
		sum := sha256.Sum256(content)
		assert.Equal(t, m.Digest, "sha256:"+hex.EncodeToString(sum[:]), "content of %q matches its digest", m.Name)
	}

	// R-002: the exported evidence record equals the one the push would upload.
	sessionName := evidenceName(sessionID)
	sessionDigest := fake.byName(sessionName).digest
	exported, err := os.ReadFile(filepath.Join(outDir, materialsDirName, strings.ReplaceAll(sessionDigest, ":", "-")))
	require.NoError(t, err)
	assert.JSONEq(t, fake.byName(sessionName).content, string(exported))

	// The manifest lists the session and resolves every material it names.
	var manifest exportManifest
	data, err := os.ReadFile(filepath.Join(outDir, manifestFileName))
	require.NoError(t, err)
	require.NoError(t, json.Unmarshal(data, &manifest))
	assert.True(t, manifest.Redaction)
	assert.Contains(t, manifest.Sessions, sessionName)
	require.NotEmpty(t, manifest.Materials)
	for _, m := range manifest.Materials {
		assert.FileExists(t, filepath.Join(outDir, m.File))
	}
}

// TestRunTraceExportRedaction covers R-004: the export redacts secrets by
// default, and the opt-out keeps them.
func TestRunTraceExportRedaction(t *testing.T) {
	const sessionID = "redact1-session"
	// Assembled from fragments so that nothing credential-shaped is committed.
	const pat = "ghp_erOZlZv0B1e3amrQ" + "ugdwZ8Ro2W4kDql9WPTf"
	ticketText := "---\nkind: ticket\nuri: https://tracker.example.com/ENG-9\ntitle: Wire up auth\n---\n\nUse the token " + pat + " for now.\n"

	store := state.NewGitStore(t.TempDir())
	ctx := context.Background()

	containsSecret := func(t *testing.T, dir string) bool {
		t.Helper()
		found := false
		require.NoError(t, filepath.WalkDir(filepath.Join(dir, materialsDirName), func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() {
				return nil
			}
			content, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			if strings.Contains(string(content), pat) {
				found = true
			}

			return nil
		}))

		return found
	}

	t.Run("redaction on by default removes the secret", func(t *testing.T) {
		outDir := t.TempDir()
		sink := newDiskMaterialSink(outDir, false, zerolog.Nop())
		attested, _ := attachSessionEvidence(ctx, sink, store, cleanSession(t, sessionID, ticketText), zerolog.Nop(), false)
		require.Equal(t, []string{sessionID}, attested)

		assert.False(t, containsSecret(t, outDir), "no exported material holds the secret")
	})

	t.Run("opt-out keeps the secret", func(t *testing.T) {
		outDir := t.TempDir()
		sink := newDiskMaterialSink(outDir, true, zerolog.Nop())
		attested, _ := attachSessionEvidence(ctx, sink, store, cleanSession(t, sessionID, ticketText), zerolog.Nop(), true)
		require.Equal(t, []string{sessionID}, attested)

		assert.True(t, containsSecret(t, outDir), "the raw export holds the secret")
	})
}

// TestRunTraceExportNoSessions covers R-001 in the degenerate case: with no
// recorded sessions the export touches no control plane and writes nothing.
func TestRunTraceExportNoSessions(t *testing.T) {
	repoDir := initTempGitRepo(t)
	require.NoError(t, state.NewGitStore(filepath.Join(repoDir, ".git")).InitTraceDir())
	t.Chdir(repoDir)

	outDir := filepath.Join(t.TempDir(), "evidence")
	got, err := RunTraceExport(context.Background(), zerolog.Nop(), RunTraceExportOpts{OutDir: outDir})
	require.NoError(t, err)
	assert.Empty(t, got)
	assert.NoDirExists(t, outDir)
}

// TestRunTraceExportRequiresOutDir rejects an empty output directory.
func TestRunTraceExportRequiresOutDir(t *testing.T) {
	_, err := RunTraceExport(context.Background(), zerolog.Nop(), RunTraceExportOpts{})
	require.Error(t, err)
}
