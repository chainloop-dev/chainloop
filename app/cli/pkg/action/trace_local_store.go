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
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/chainloop-dev/chainloop/app/cli/internal/trace/config"
	"github.com/chainloop-dev/chainloop/app/cli/internal/trace/state"
	v1 "github.com/chainloop-dev/chainloop/app/controlplane/api/workflowcontract/v1"
	"github.com/chainloop-dev/chainloop/pkg/attestation/crafter/materials"
	"github.com/chainloop-dev/chainloop/pkg/attestation/crafter/materials/aicodingsession"
	"github.com/chainloop-dev/chainloop/pkg/casclient"
	"github.com/rs/zerolog"
)

// traceNoRedactEnv turns secret redaction off for a local store. It is
// deliberately not a .chainloop.yml field: a committed no-redact would disable
// redaction for every teammate who clones the repository. The opt-out is per-run
// and for a trusted local store only, whose output can then hold secrets (R-004).
const traceNoRedactEnv = "CHAINLOOP_TRACE_NO_REDACT"

// traceNoRedactFromEnv reports whether CHAINLOOP_TRACE_NO_REDACT is set to a
// truthy value (1, t, true, …).
func traceNoRedactFromEnv() bool {
	v, err := strconv.ParseBool(strings.TrimSpace(os.Getenv(traceNoRedactEnv)))
	return err == nil && v
}

const (
	// manifestFileName is the index a reader opens first in a stored session.
	manifestFileName = "manifest.json"
	// materialsDirName holds every material of a session, each file named by its
	// content digest so a reader and a test find it from the reference in the
	// evidence record.
	materialsDirName = "materials"
)

// RunTraceLocalStoreOpts configures a disk store of the recorded session
// evidence.
type RunTraceLocalStoreOpts struct {
	// NoRedact disables secret redaction. Off by default: the store redacts
	// secrets exactly as a push does. The opt-out is for a trusted local run,
	// and the output can then hold secrets (R-004).
	NoRedact bool
	// Mode records how the session was driven, one of the aicodingsession.Mode*
	// constants. Empty means ModeCoding.
	Mode string
	// AllowEmpty assembles active sessions that produced no commit on this
	// branch, as `chainloop trace run` does. The pre-push path leaves it off, so
	// a push with no AI commits stores nothing.
	AllowEmpty bool
}

// RunTraceLocalStore runs the same push-time assembly that RunTracePush drives
// but writes the resulting evidence to disk instead of uploading a signed
// attestation. It makes no call to the control plane and needs no credentials:
// every material is crafted offline with an inline CAS backend, so the evidence
// record and the material digests equal what a push would upload, apart from the
// signature (R-001/R-002). It reuses gatherPushSessions and finalizePush, so the
// assembly cannot drift from a push and a repeated `git push` does not re-store
// (D-001).
//
// It returns the number of sessions stored and the directory they were written
// under (resolved even when nothing was stored, so the caller can report it).
func RunTraceLocalStore(ctx context.Context, log zerolog.Logger, opts RunTraceLocalStoreOpts) (stored int, tracePath string, err error) {
	store, repoRoot, err := state.Locate()
	if err != nil {
		return 0, "", err
	}

	tracePath = ResolveTracePath(repoRoot, store, config.LoadTracePathFromYML(repoRoot))
	sessionMode := aicodingsession.ResolveMode(opts.Mode)

	log.Debug().Str("state_dir", store.Dir()).Str("repo_root", repoRoot).
		Str("trace_path", tracePath).Bool("no_redact", opts.NoRedact).Str("mode", sessionMode).
		Bool("allow_empty", opts.AllowEmpty).Msg("trace local store invoked")

	sessions, aiCommits, err := gatherPushSessions(ctx, store, repoRoot, sessionMode, opts.AllowEmpty, log)
	if err != nil {
		return 0, tracePath, err
	}
	if len(sessions) == 0 {
		return 0, tracePath, nil
	}

	stored, err = storeSessionsLocally(ctx, log, store, sessions, tracePath, opts.NoRedact)
	if err != nil {
		return stored, tracePath, err
	}

	// Only prune the single-use state once something was stored, so a run that
	// stored nothing leaves the recorded session in place for a later retry.
	if stored > 0 {
		finalizePush(store, repoRoot, aiCommits, log)
	}

	return stored, tracePath, nil
}

// ResolveTracePath returns where LOCAL trace storage writes its evidence sets:
// the persisted tracePath when set (absolute, or relative to the repository
// root), otherwise the per-clone default under the trace state directory.
func ResolveTracePath(repoRoot string, store *state.Store, persisted string) string {
	switch {
	case persisted == "":
		return store.LocalSessionsDir()
	case filepath.IsAbs(persisted):
		return persisted
	default:
		return filepath.Join(repoRoot, persisted)
	}
}

// storeSessionsLocally writes each assembled session under tracePath in its own
// content-addressed directory: tracePath/<session-id>/manifest.json plus a
// materials/ folder whose files are named by digest. Each session is
// self-contained, so it uses a fresh sink and runs the shared assembly for that
// one session. It returns how many sessions were stored.
func storeSessionsLocally(ctx context.Context, log zerolog.Logger, store *state.Store, sessions []sessionEvidence, tracePath string, noRedact bool) (int, error) {
	if err := os.MkdirAll(tracePath, 0o700); err != nil {
		return 0, fmt.Errorf("create trace path: %w", err)
	}

	stored := 0
	for _, se := range sessions {
		sessionDir := filepath.Join(tracePath, state.SanitizeID(se.sessionID))
		sink := newDiskMaterialSink(sessionDir, noRedact, log)

		// A single-session slice keeps each export self-contained: the assembly
		// already uses a fresh name allocator and redactor per call, so nothing
		// crosses between sessions.
		attested, _ := attachSessionEvidence(ctx, sink, store, []sessionEvidence{se}, log, noRedact)
		if len(attested) == 0 {
			log.Warn().Str("session", se.sessionID).Msg("no evidence stored for this session")
			continue
		}

		if err := sink.writeManifest(!noRedact); err != nil {
			return stored, fmt.Errorf("write manifest for session %s: %w", se.sessionID, err)
		}

		log.Debug().Str("session", se.sessionID).Str("dir", sessionDir).
			Int("materials", len(sink.materials)).Msg("session evidence stored")
		stored++
	}

	return stored, nil
}

// diskMaterialSink implements specMaterialAdder by crafting each material
// offline and writing its content under its content digest. It is the local
// counterpart of AttestationExecutor: it runs the identical crafting — same
// redaction, same content-addressed digest — but stores the result on disk
// rather than uploading it, so the stored evidence matches a push byte for byte
// apart from the signature and the attestation wrapper (R-002, R-003).
type diskMaterialSink struct {
	outDir   string
	noRedact bool
	logger   zerolog.Logger
	// backend has no uploader, so uploadAndCraft stores each artifact inline and
	// never reaches the network.
	backend   *casclient.CASBackend
	materials []storedMaterial
}

// storedMaterial is one material's entry in the manifest.
type storedMaterial struct {
	// Name is the material name the assembly allocated for it.
	Name string `json:"name"`
	// Kind is the material type (e.g. EVIDENCE, CHAINLOOP_AI_CODING_SESSION).
	Kind string `json:"kind"`
	// Digest is the content digest the material is stored under, sha256:<hex>.
	Digest string `json:"digest"`
	// File is the path of the material's content, relative to the session
	// directory.
	File string `json:"file"`
	// Annotations are the attestation annotations the material carries.
	Annotations map[string]string `json:"annotations,omitempty"`
}

// localManifest is the index of one stored session.
type localManifest struct {
	GeneratedAt string `json:"generated_at"`
	// Signed and IsAttestation are always false: the stored evidence is an
	// unsigned evidence set, not a signed in-toto attestation, so it proves
	// nothing on its own. The index declares this so a reader never mistakes the
	// stored evidence for an attestation (R-006).
	Signed        bool `json:"signed"`
	IsAttestation bool `json:"is_attestation"`
	// Redaction records whether secrets were stripped, so a reader can tell a
	// safe-to-share session from a raw one.
	Redaction bool `json:"redaction"`
	// EvidenceRecord names the CHAINLOOP_AI_CODING_SESSION material that holds
	// the session evidence record (and the warnings of the run).
	EvidenceRecord string `json:"evidence_record,omitempty"`
	// Materials lists every stored material by name, kind and digest, so a
	// reader sees the whole set from the index with no need to parse each file.
	Materials []storedMaterial `json:"materials"`
}

func newDiskMaterialSink(outDir string, noRedact bool, logger zerolog.Logger) *diskMaterialSink {
	return &diskMaterialSink{
		outDir:   outDir,
		noRedact: noRedact,
		logger:   logger,
		backend:  &casclient.CASBackend{Name: "chainloop-trace-local", Uploader: nil},
	}
}

// AddMaterial crafts the file at path as a material of the given kind and writes
// its stored content under its digest. It returns the digest, which the
// assembly records in the evidence in place of the content, exactly as a push
// records the CAS digest.
func (s *diskMaterialSink) AddMaterial(ctx context.Context, name, path, kind string, annotations map[string]string) (string, error) {
	kindValue, ok := v1.CraftingSchema_Material_MaterialType_value[kind]
	if !ok {
		return "", fmt.Errorf("unknown material kind %q", kind)
	}

	schema := &v1.CraftingSchema_Material{
		Type: v1.CraftingSchema_Material_MaterialType(kindValue),
		Name: name,
	}

	res, err := materials.Craft(ctx, schema, path, s.backend, nil, &s.logger, &materials.CraftingOpts{
		SkipSecretRedaction: s.noRedact,
	})
	if err != nil {
		return "", fmt.Errorf("crafting %s material: %w", kind, err)
	}

	artifact := res.Material.GetArtifact()
	if artifact == nil {
		return "", fmt.Errorf("material kind %q produced no artifact to store", kind)
	}

	content, err := materialContent(artifact.GetContent(), res.Content, path)
	if err != nil {
		return "", err
	}

	file, err := s.writeMaterial(artifact.GetDigest(), content)
	if err != nil {
		return "", err
	}

	s.materials = append(s.materials, storedMaterial{
		Name:        name,
		Kind:        kind,
		Digest:      artifact.GetDigest(),
		File:        file,
		Annotations: mergeAnnotations(res.Material.GetAnnotations(), annotations),
	})

	return artifact.GetDigest(), nil
}

// materialContent returns the bytes to store for a material. An inline craft
// records the stored content on the artifact; a redacting crafter also returns
// it on the result. When neither is set the material was stored verbatim, so the
// file on disk is the content.
func materialContent(inline, override []byte, path string) ([]byte, error) {
	switch {
	case inline != nil:
		return inline, nil
	case override != nil:
		return override, nil
	default:
		content, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("reading material content: %w", err)
		}

		return content, nil
	}
}

// writeMaterial stores content under materials/<digest> and returns the path
// relative to the session directory. Materials are content-addressed, so a
// repeated digest is the same bytes and the rewrite is a harmless no-op.
func (s *diskMaterialSink) writeMaterial(digest string, content []byte) (string, error) {
	// A ':' in the digest is awkward on some filesystems, so the stored file
	// uses '-'; the digest itself is kept intact in the manifest.
	rel := filepath.Join(materialsDirName, strings.ReplaceAll(digest, ":", "-"))
	abs := filepath.Join(s.outDir, rel)

	if err := os.MkdirAll(filepath.Dir(abs), 0o700); err != nil {
		return "", fmt.Errorf("create materials directory: %w", err)
	}

	if err := os.WriteFile(abs, content, 0o600); err != nil {
		return "", fmt.Errorf("write material %s: %w", digest, err)
	}

	return rel, nil
}

// writeManifest writes the index of one stored session. redaction records
// whether secrets were stripped.
func (s *diskMaterialSink) writeManifest(redaction bool) error {
	sessionKind := v1.CraftingSchema_Material_CHAINLOOP_AI_CODING_SESSION.String()

	var evidenceRecord string
	for _, m := range s.materials {
		if m.Kind == sessionKind {
			evidenceRecord = m.Name
			break
		}
	}

	manifest := localManifest{
		GeneratedAt:    time.Now().UTC().Format(time.RFC3339),
		Signed:         false,
		IsAttestation:  false,
		Redaction:      redaction,
		EvidenceRecord: evidenceRecord,
		Materials:      s.materials,
	}

	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return fmt.Errorf("encode manifest: %w", err)
	}

	if err := os.MkdirAll(s.outDir, 0o700); err != nil {
		return fmt.Errorf("create session directory: %w", err)
	}

	return os.WriteFile(filepath.Join(s.outDir, manifestFileName), data, 0o600)
}

// mergeAnnotations combines the annotations the crafter set on a material with
// the ones the assembly passed for it. The assembly's annotations win on a
// conflict, matching the attestation-add path that layers them on last.
func mergeAnnotations(crafted, extra map[string]string) map[string]string {
	if len(crafted) == 0 && len(extra) == 0 {
		return nil
	}

	// Both maps hold a handful of annotation keys, so a capacity hint from
	// crafted alone is enough and the map grows for the few extra entries.
	// Summing the two lengths is an overflow pattern a scanner flags for no gain.
	merged := make(map[string]string, len(crafted))
	for k, v := range crafted {
		merged[k] = v
	}
	for k, v := range extra {
		merged[k] = v
	}

	return merged
}
