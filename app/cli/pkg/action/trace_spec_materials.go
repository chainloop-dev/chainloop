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
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/chainloop-dev/chainloop/app/cli/internal/trace/spec"
	"github.com/chainloop-dev/chainloop/pkg/attestation/crafter/materials"
	"github.com/chainloop-dev/chainloop/pkg/attestation/crafter/materials/aicodingsession"
	"github.com/rs/zerolog"
)

// Annotations on each spec material, so that a policy or a reader can find
// the spec of a session without opening the session material first.
const (
	specAnnotationSession = "chainloop.spec.session_id"
	specAnnotationKind    = "chainloop.spec.kind"
	specAnnotationURI     = "chainloop.spec.uri"
)

// specMaterialKind is the material type each captured spec is stored as.
const specMaterialKind = "EVIDENCE"

// maxSpecSlugLen bounds the part of a material name taken from the file name.
const maxSpecSlugLen = 40

// specMaterialAdder adds one file to the attestation being crafted, returning
// the digest it was stored under.
type specMaterialAdder interface {
	AddMaterial(ctx context.Context, name, path, kind string, annotations map[string]string) (string, error)
}

// attachSpecs stores each capture of a session as its own EVIDENCE material
// and returns the references the session material records in their place.
//
// The text never goes into the session material itself. A ticket that many
// sessions were built from is then stored once, and each source can be
// fetched and verified by its digest on its own.
//
// A capture that cannot be stored is dropped from the references and named in
// a warning, rather than failing the push: evidence without one of its specs
// is still evidence, and a reference to a material that is not in the
// attestation would point nowhere.
func attachSpecs(ctx context.Context, adder specMaterialAdder, redactor *specRedactor, names *materials.NameAllocator, sessionID string, captures []spec.Capture, log zerolog.Logger) ([]aicodingsession.SpecEntry, []string) {
	if len(captures) == 0 {
		return nil, nil
	}

	var (
		entries  []aicodingsession.SpecEntry
		warnings []string
	)

	tmpDir, err := os.MkdirTemp("", "chainloop-trace-spec-*")
	if err != nil {
		return nil, []string{fmt.Sprintf("%d spec entries were not recorded: %v", len(captures), err)}
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	for i, c := range captures {
		name := names.AllocateNamed(specMaterialName(sessionID, c.FileName))

		entry, err := storeCapture(ctx, adder, redactor, filepath.Join(tmpDir, strconv.Itoa(i)), name, sessionID, c)
		if err != nil {
			// The error stays in the local log. The warning goes into the
			// uploaded evidence, and error text can carry local paths.
			log.Warn().Err(err).Str("session", sessionID).Str("file", c.FileName).Msg("could not record a spec entry")
			warnings = append(warnings, fmt.Sprintf("spec entry %q was not recorded", c.FileName))

			continue
		}

		entries = append(entries, entry)
	}

	return entries, warnings
}

// storeCapture redacts one capture, adds it to the attestation, and returns
// the reference the session material records for it: everything but the text,
// which the digest points at.
func storeCapture(ctx context.Context, adder specMaterialAdder, redactor *specRedactor, dir, name, sessionID string, c spec.Capture) (aicodingsession.SpecEntry, error) {
	redacted, err := redactCapture(ctx, redactor, c)
	if err != nil {
		return aicodingsession.SpecEntry{}, err
	}

	digest, err := addSpecMaterial(ctx, adder, dir, name, sessionID, redacted)
	if err != nil {
		return aicodingsession.SpecEntry{}, err
	}

	return aicodingsession.SpecEntry{
		Kind:       redacted.Kind,
		URI:        redacted.URI,
		Digest:     digest,
		CapturedAt: redacted.CapturedAt,
	}, nil
}

// redactCapture redacts the text file a capture was read from. The redacted file
// is what gets stored, header included, so it is the file on disk with only
// its secrets taken out. It is parsed again for the source address that the
// annotation and the reference carry, so that address is redacted too.
//
// Redaction fails closed, as it does for the session material: a file that
// could not be scanned is not uploaded at all.
func redactCapture(ctx context.Context, redactor *specRedactor, c spec.Capture) (spec.Capture, error) {
	// An image, or any file that is not text, is stored as it is: the text
	// scanner has nothing to read in it, and a rewrite would break it.
	if c.Verbatim {
		return c, nil
	}

	doc, err := redactor.Redact(ctx, c.Raw)
	if err != nil {
		return c, fmt.Errorf("scanning for secrets: %w", err)
	}

	// The capture time comes from the file on disk, not from its content, so
	// the one already recorded stands.
	parsed := spec.Parse(doc, time.Time{})
	if parsed == nil {
		return c, errors.New("nothing left after redaction")
	}

	c.URI = parsed.URI
	c.Raw = doc

	return c, nil
}

// addSpecMaterial writes a redacted capture under dir with the file name the
// agent chose, adds it to the attestation, and returns the digest it was
// stored under.
func addSpecMaterial(ctx context.Context, adder specMaterialAdder, dir, name, sessionID string, c spec.Capture) (string, error) {
	// Each capture gets a directory of its own, so the stored file keeps the
	// agent's name even when two captures would collide on it.
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", fmt.Errorf("staging the file: %w", err)
	}

	path := filepath.Join(dir, filepath.Base(c.FileName))
	if err := os.WriteFile(path, c.Raw, 0o600); err != nil {
		return "", fmt.Errorf("staging the file: %w", err)
	}

	annotations := map[string]string{
		specAnnotationSession: sessionID,
		specAnnotationKind:    c.Kind,
	}
	if c.URI != "" {
		annotations[specAnnotationURI] = c.URI
	}

	return adder.AddMaterial(ctx, name, path, specMaterialKind, annotations)
}

// specRedactor removes secrets from spec files, and keeps each redacted copy
// under dir keyed by the digest of the file it came from.
//
// The spec files stay for the whole session and every push records them, so
// without the copies each push would scan every file again. Redaction runs in
// the client and is expensive, and an unchanged file gives the same result.
type specRedactor struct {
	// dir holds one redacted copy per file, named by the SHA-256 of the file
	// as the agent wrote it. It is dropped when the session ends.
	dir    string
	redact func(ctx context.Context, doc []byte) ([]byte, error)
}

// newSpecRedactor returns a redactor that uses the secret scanner of the
// session material and keeps its copies under dir.
func newSpecRedactor(dir string) *specRedactor {
	return &specRedactor{dir: dir, redact: func(ctx context.Context, doc []byte) ([]byte, error) {
		redacted, _, err := aicodingsession.RedactSpecText(ctx, string(doc))
		return []byte(redacted), err
	}}
}

// Redact returns the redacted form of a spec file, from the stored copy when
// an earlier push already scanned the same file. The whole file is scanned,
// header included, so the source address is redacted like the text. A copy
// that cannot be stored costs the next push a scan, never this one its spec.
func (r *specRedactor) Redact(ctx context.Context, doc []byte) ([]byte, error) {
	sum := sha256.Sum256(doc)
	path := filepath.Join(r.dir, hex.EncodeToString(sum[:]))

	if stored, err := os.ReadFile(path); err == nil {
		return stored, nil
	}

	redacted, err := r.redact(ctx, doc)
	if err != nil {
		return nil, err
	}

	if err := os.MkdirAll(r.dir, 0o700); err == nil {
		_ = os.WriteFile(path, redacted, 0o600)
	}

	return redacted, nil
}

// specMaterialName derives the base of a material name from the session and
// the file name the agent chose: spec-<first 6 characters of the session
// ID>-<file name>. The allocator sanitizes it into a valid material name and
// makes it unique within the attestation.
func specMaterialName(sessionID, fileName string) string {
	short := sessionID
	if len(short) > 6 {
		short = short[:6]
	}

	stem := materials.SanitizeMaterialName(strings.TrimSuffix(fileName, filepath.Ext(fileName)))
	if len(stem) > maxSpecSlugLen {
		stem = strings.TrimRight(stem[:maxSpecSlugLen], "-")
	}
	if stem == "" {
		stem = "source"
	}

	return "spec-" + short + "-" + stem
}
