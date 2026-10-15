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
	"encoding/base64"
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
	"github.com/chainloop-dev/chainloop/pkg/attestation/renderer/chainloop"
	intoto "github.com/in-toto/attestation/go/v1"
	"github.com/rs/zerolog"
	"github.com/secure-systems-lab/go-securesystemslib/dsse"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestStoreSessionsLocally follows R-002/R-003/R-004/R-006: the same assembly a
// push runs is written to disk, content-addressed, redacted by default, with an
// unsigned in-toto statement as its index. The statement has no envelope and no
// signature, so it is not an attestation.
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
		stored, err := storeSessionsLocally(ctx, zerolog.Nop(), state.NewGitStore(t.TempDir()), []sessionEvidence{newSession()}, tracePath, false, t.TempDir(), "test")
		require.NoError(t, err)
		require.Equal(t, 1, stored)

		sessionDir := filepath.Join(tracePath, state.SanitizeID(sessionID))
		statement, mats := readLocalStatement(t, sessionDir)

		// R-006: the index is a Chainloop in-toto statement, with no envelope.
		assert.Equal(t, chainloop.PredicateTypeV02, statement.GetPredicateType())
		assert.Equal(t, "chainloop.workflow."+localWorkflowName, statement.GetSubject()[0].GetName())
		entries, err := os.ReadDir(sessionDir)
		require.NoError(t, err)
		names := make([]string, 0, len(entries))
		for _, e := range entries {
			names = append(names, e.Name())
		}
		assert.ElementsMatch(t, []string{statementFileName, materialsDirName}, names, "no envelope, no bundle")
		require.NotEmpty(t, mats)

		var specDigest, recordFile string
		for _, mat := range mats {
			// The content is on disk, not inline in the statement.
			assert.False(t, mat.EmbeddedInline, "material %s is not inline", mat.Name)
			// The content is on local disk, not in a CAS a reader could
			// download it from.
			assert.False(t, mat.UploadedToCAS, "material %s is not in a CAS", mat.Name)

			digest := mat.Hash.String()
			file := filepath.Join(sessionDir, materialsDirName, strings.ReplaceAll(digest, ":", "-"))
			content, err := os.ReadFile(file)
			require.NoError(t, err, "material %s is stored under its digest", mat.Name)

			// R-003: every material is stored under its content digest.
			sum := sha256.Sum256(content)
			assert.Equal(t, "sha256:"+hex.EncodeToString(sum[:]), digest, "the stored file hashes back to its digest for %s", mat.Name)

			// R-004: redaction on by default, so no stored file holds the secret.
			assert.NotContains(t, string(content), pat, "no stored file holds the secret: %s", mat.Name)

			switch mat.Type {
			case "EVIDENCE":
				specDigest = digest
				// R-002: the spec material digest equals what a push produces.
				assert.Equal(t, pushAdder.byName(mat.Name).digest, digest, "same digest as a push for %s", mat.Name)
			case "CHAINLOOP_AI_CODING_SESSION":
				assert.Equal(t, "ai-coding-session-7c1e2d", mat.Name)
				recordFile = file
			}
		}
		require.NotEmpty(t, specDigest, "the ticket became an EVIDENCE material")
		require.NotEmpty(t, recordFile, "the session evidence record is a material")

		// The evidence record references the stored spec material by digest.
		var ev aicodingsession.Evidence
		readJSONFile(t, recordFile, &ev)
		referenced := make([]string, 0, len(ev.Data.Spec))
		for _, e := range ev.Data.Spec {
			referenced = append(referenced, e.Digest)
		}
		assert.Contains(t, referenced, specDigest, "the evidence record points at the stored spec material")
	})

	t.Run("the no-redact opt-out keeps the raw secret", func(t *testing.T) {
		tracePath := t.TempDir()
		stored, err := storeSessionsLocally(ctx, zerolog.Nop(), state.NewGitStore(t.TempDir()), []sessionEvidence{newSession()}, tracePath, true, t.TempDir(), "test")
		require.NoError(t, err)
		require.Equal(t, 1, stored)

		sessionDir := filepath.Join(tracePath, state.SanitizeID(sessionID))
		_, mats := readLocalStatement(t, sessionDir)

		var found bool
		for _, mat := range mats {
			content, err := os.ReadFile(filepath.Join(sessionDir, materialsDirName, strings.ReplaceAll(mat.Hash.String(), ":", "-")))
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

// readLocalStatement reads the in-toto statement of a stored session and the
// materials of its Chainloop predicate.
func readLocalStatement(t *testing.T, sessionDir string) (*intoto.Statement, []*chainloop.NormalizedMaterial) {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(sessionDir, statementFileName))
	require.NoError(t, err)

	// The renderer reads predicates from an envelope. An unsigned one carries
	// the statement as is.
	envelope := &dsse.Envelope{PayloadType: "application/vnd.in-toto+json", Payload: base64.StdEncoding.EncodeToString(raw)}
	statement, err := chainloop.ExtractStatement(envelope)
	require.NoError(t, err)
	predicate, err := chainloop.ExtractPredicate(envelope)
	require.NoError(t, err)

	return statement, predicate.GetMaterials()
}

// TestRunTraceLocalStorePartialFailure covers a session that cannot be stored.
// The store returns an error, so the pre-push hook can warn, or block the push
// when traceRequired is set. The commit stays untracked, so the next push
// tries the session again.
func TestRunTraceLocalStorePartialFailure(t *testing.T) {
	const sessionID = "5d2b9e7a-1c3f-4a6b-8d9e-0f1a2b3c4d5e"

	home := t.TempDir()
	t.Setenv("HOME", home)

	repoDir, gitDir := initGitRepo(t)
	store := state.NewGitStore(gitDir)
	require.NoError(t, store.InitTraceDir())
	t.Chdir(repoDir)

	projectDir := claudeProjectDir(home, repoDir)
	require.NoError(t, os.MkdirAll(projectDir, 0o700))
	tr := transcript{t: t, path: filepath.Join(projectDir, sessionID+".jsonl")}
	tr.append(`{"type":"user","uuid":"u1","timestamp":"2026-10-15T10:00:00.000Z","sessionId":"` + sessionID + `","message":{"role":"user","content":"Add a hello world."}}`)

	payload, err := json.Marshal(struct {
		SessionID      string `json:"session_id"`
		HookEventName  string `json:"hook_event_name"`
		TranscriptPath string `json:"transcript_path"`
		Cwd            string `json:"cwd"`
	}{sessionID, "SessionStart", tr.path, repoDir})
	require.NoError(t, err)
	withStdin(t, string(payload))
	captureStdout(t, func() { require.NoError(t, HandleAgentSessionStart(claude.New(), zerolog.Nop())) })

	sha := headSHA(t, repoDir)
	require.NoError(t, store.SaveCommitRecord(&state.CommitRecord{
		SHA:        sha,
		Message:    "initial",
		SessionIDs: []string{sessionID},
		Timestamp:  state.NowTimestamp(),
	}))

	// A file where the session directory must go makes the store of this
	// session fail.
	require.NoError(t, os.MkdirAll(store.LocalSessionsDir(), 0o700))
	require.NoError(t, os.WriteFile(filepath.Join(store.LocalSessionsDir(), state.SanitizeID(sessionID)), nil, 0o600))

	stored, _, err := RunTraceLocalStore(t.Context(), zerolog.Nop(), RunTraceLocalStoreOpts{Mode: aicodingsession.ModeCoding})
	require.ErrorContains(t, err, "stored 0 of 1")
	assert.Zero(t, stored)
	assert.False(t, mustLoadRecord(t, store, sha).Tracked, "the commit stays for the next push")
}
