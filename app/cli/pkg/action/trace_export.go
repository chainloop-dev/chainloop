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
	"strings"
	"time"

	tracegit "github.com/chainloop-dev/chainloop/app/cli/internal/trace/git"
	"github.com/chainloop-dev/chainloop/app/cli/internal/trace/state"
	v1 "github.com/chainloop-dev/chainloop/app/controlplane/api/workflowcontract/v1"
	"github.com/chainloop-dev/chainloop/pkg/attestation/crafter/materials"
	"github.com/chainloop-dev/chainloop/pkg/attestation/crafter/materials/aicodingsession"
	"github.com/chainloop-dev/chainloop/pkg/casclient"
	"github.com/rs/zerolog"
)

// manifestFileName is the index a reader opens first in an export directory.
const manifestFileName = "manifest.json"

// materialsDirName holds every material, each file named by its content digest
// so a reader and a test find it from the reference in the evidence record.
const materialsDirName = "materials"

// RunTraceExportOpts configures a disk export of the current session evidence.
type RunTraceExportOpts struct {
	// OutDir is the directory the evidence is written to. Required.
	OutDir string
	// NoRedact disables secret redaction. Off by default: the export redacts
	// secrets exactly as a push does. The opt-out is for a trusted local run,
	// and the output can then hold secrets.
	NoRedact bool
	// Mode records how the session was driven, as one of the
	// aicodingsession.Mode* constants. Empty means ModeCoding.
	Mode string
}

// RunTraceExport runs the same push-time assembly that RunTracePush drives, but
// writes the resulting evidence to a local directory instead of uploading a
// signed attestation. It makes no call to the control plane and needs no
// credentials: every material is crafted offline, with an inline CAS backend,
// so the evidence record and the material digests equal what a push would
// upload (spec issue export-to-disk, R-001/R-002).
//
// It returns the directory the evidence was written to, or an empty string when
// there was nothing to export.
func RunTraceExport(ctx context.Context, log zerolog.Logger, opts RunTraceExportOpts) (string, error) {
	if opts.OutDir == "" {
		return "", fmt.Errorf("output directory is required")
	}

	store, repoRoot, err := state.Locate()
	if err != nil {
		return "", err
	}

	sessionMode := aicodingsession.ResolveMode(opts.Mode)
	log.Debug().Str("state_dir", store.Dir()).Str("repo_root", repoRoot).
		Str("out_dir", opts.OutDir).Bool("no_redact", opts.NoRedact).Str("mode", sessionMode).
		Msg("trace export invoked")

	allCommits, err := store.LoadAllCommitRecords()
	if err != nil {
		return "", fmt.Errorf("load commit records: %w", err)
	}

	gitClient := tracegit.NewGoGitClient()
	allCommits = filterCurrentBranchCommits(gitClient, repoRoot, allCommits, log)

	var aiCommits []*state.CommitRecord
	for _, c := range allCommits {
		if len(c.SessionIDs) > 0 {
			aiCommits = append(aiCommits, c)
		}
	}

	sessionRecords, err := store.LoadAllSessionRecords()
	if err != nil {
		log.Debug().Err(err).Msg("could not load session records")
	}

	// Export always assembles every recorded session, with or without commits:
	// the point is to inspect what a session would send before it sends it.
	sessionCommits := sessionCommitGroups(aiCommits, sessionRecords, true)
	if len(sessionCommits) == 0 {
		log.Info().Msg("no AI coding sessions recorded, nothing to export")

		return "", nil
	}

	sessions := buildSessionEvidence(ctx, store, repoRoot, gitClient, sessionCommits, sessionRecords, sessionMode, log)
	if len(sessions) == 0 {
		log.Debug().Msg("no session evidence could be generated")

		return "", nil
	}

	if err := os.MkdirAll(opts.OutDir, 0o700); err != nil {
		return "", fmt.Errorf("create output directory: %w", err)
	}

	sink := newDiskMaterialSink(opts.OutDir, opts.NoRedact, log)

	exported, _ := attachSessionEvidence(ctx, sink, store, sessions, log, opts.NoRedact)
	if len(exported) == 0 {
		log.Debug().Msg("no evidence could be exported")

		return "", nil
	}

	if err := sink.writeManifest(!opts.NoRedact); err != nil {
		return "", fmt.Errorf("write manifest: %w", err)
	}

	log.Info().Str("dir", opts.OutDir).Int("sessions", len(exported)).Int("materials", len(sink.materials)).
		Msg("Session evidence exported")

	return opts.OutDir, nil
}

// diskMaterialSink implements specMaterialAdder by crafting each material
// offline and writing its content under its content digest. It is the export
// counterpart of AttestationExecutor: it runs the identical crafting — same
// redaction, same content-addressed digest — but stores the result on disk
// rather than uploading it, so the exported evidence matches a push byte for
// byte apart from the signature and the attestation wrapper (R-002, R-003).
type diskMaterialSink struct {
	outDir   string
	noRedact bool
	logger   zerolog.Logger
	// backend has no uploader, so uploadAndCraft stores each artifact inline
	// and never reaches the network.
	backend   *casclient.CASBackend
	materials []exportedMaterial
}

// exportedMaterial is one material's entry in the manifest.
type exportedMaterial struct {
	// Name is the material name the assembly allocated for it.
	Name string `json:"name"`
	// Kind is the material type (e.g. EVIDENCE, CHAINLOOP_AI_CODING_SESSION).
	Kind string `json:"kind"`
	// Digest is the content digest the material is stored under, sha256:<hex>.
	Digest string `json:"digest"`
	// File is the path of the material's content, relative to the export
	// directory.
	File string `json:"file"`
	// Annotations are the attestation annotations the material carries.
	Annotations map[string]string `json:"annotations,omitempty"`
}

// exportManifest is the top-level index of an export directory.
type exportManifest struct {
	GeneratedAt string             `json:"generated_at"`
	Redaction   bool               `json:"redaction"`
	Sessions    []string           `json:"sessions"`
	Materials   []exportedMaterial `json:"materials"`
}

func newDiskMaterialSink(outDir string, noRedact bool, logger zerolog.Logger) *diskMaterialSink {
	return &diskMaterialSink{
		outDir:   outDir,
		noRedact: noRedact,
		logger:   logger,
		backend:  &casclient.CASBackend{Name: "chainloop-trace-export", Uploader: nil},
	}
}

// AddMaterial crafts the file at path as a material of the given kind and writes
// its stored content under its digest. It returns the digest, which the
// assembly records in the evidence in place of the content, exactly as the push
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
		return "", fmt.Errorf("material kind %q produced no artifact to export", kind)
	}

	content, err := materialContent(artifact.GetContent(), res.Content, path)
	if err != nil {
		return "", err
	}

	file, err := s.writeMaterial(artifact.GetDigest(), content)
	if err != nil {
		return "", err
	}

	s.materials = append(s.materials, exportedMaterial{
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
// it on the result. When neither is set the material was stored verbatim, so
// the file on disk is the content.
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
// relative to the export directory. Materials are content-addressed, so a
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

// writeManifest writes the top-level index of the export. redaction records
// whether secrets were stripped, so a reader can tell a safe-to-share export
// from a raw one.
func (s *diskMaterialSink) writeManifest(redaction bool) error {
	sessionKind := v1.CraftingSchema_Material_CHAINLOOP_AI_CODING_SESSION.String()

	var sessions []string
	for _, m := range s.materials {
		if m.Kind == sessionKind {
			sessions = append(sessions, m.Name)
		}
	}

	manifest := exportManifest{
		GeneratedAt: time.Now().UTC().Format(time.RFC3339),
		Redaction:   redaction,
		Sessions:    sessions,
		Materials:   s.materials,
	}

	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return fmt.Errorf("encode manifest: %w", err)
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

	merged := make(map[string]string, len(crafted)+len(extra))
	for k, v := range crafted {
		merged[k] = v
	}
	for k, v := range extra {
		merged[k] = v
	}

	return merged
}
