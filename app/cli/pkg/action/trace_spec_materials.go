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

	"github.com/chainloop-dev/chainloop/app/cli/internal/trace/pointer"
	"github.com/chainloop-dev/chainloop/app/cli/internal/trace/spec"
	"github.com/chainloop-dev/chainloop/pkg/attestation/crafter/materials"
	"github.com/chainloop-dev/chainloop/pkg/attestation/crafter/materials/aicodingsession"
	"github.com/rs/zerolog"
)

// Annotations on each spec material, so that a policy or a reader can find
// the spec of a session without opening the session material first.
const (
	specAnnotationSession     = "chainloop.spec.session_id"
	specAnnotationKind        = "chainloop.spec.kind"
	specAnnotationURI         = "chainloop.spec.uri"
	specAnnotationRole        = "chainloop.spec.role"
	specAnnotationTitle       = "chainloop.spec.title"
	specAnnotationDescription = "chainloop.spec.description"
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
//
// It also returns the names of the files it stored, in order, so that a
// later push of the session keeps them first, and the sources that the
// transcript can hold copies of: the digest of each stored file as the push
// read it, and of the local file its body came from, mapped to the digest of
// its material.
func attachSpecs(ctx context.Context, adder specMaterialAdder, redactor *specRedactor, names *materials.NameAllocator, sessionID string, captures []spec.Capture, log zerolog.Logger) (entries []aicodingsession.SpecEntry, warnings, stored []string, sources pointer.Sources) {
	if len(captures) == 0 {
		return nil, nil, nil, nil
	}

	tmpDir, err := os.MkdirTemp("", "chainloop-trace-spec-*")
	if err != nil {
		return nil, []string{fmt.Sprintf("%d spec entries were not recorded: %v", len(captures), err)}, nil, nil
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	for i, c := range captures {
		name := names.AllocateNamed(specMaterialName(sessionID, c.FileName))

		entry, err := storeCapture(ctx, adder, redactor, filepath.Join(tmpDir, strconv.Itoa(i)), name, sessionID, c, log)
		if err != nil {
			// The error stays in the local log. The warning goes into the
			// uploaded evidence, and error text can carry local paths.
			log.Warn().Err(err).Str("session", sessionID).Str("file", c.FileName).Msg("could not record a spec entry")
			warnings = append(warnings, fmt.Sprintf("spec entry %q was not recorded", c.FileName))

			continue
		}

		entries = append(entries, entry)
		stored = append(stored, c.FileName)
		if sources == nil {
			sources = make(pointer.Sources)
		}
		sources[pointer.Digest(c.Raw)] = entry.Digest
		// A body the push read from a local file is a source too: a full
		// read or write of that file holds the same content (spec
		// issue-3561, R-005).
		if c.SourceDigest != "" {
			sources[c.SourceDigest] = entry.Digest
		}
	}

	return entries, warnings, stored, sources
}

// storeCapture redacts one capture, adds it to the attestation, and returns
// the reference the session material records for it: everything but the text,
// which the digest points at.
func storeCapture(ctx context.Context, adder specMaterialAdder, redactor *specRedactor, dir, name, sessionID string, c spec.Capture, log zerolog.Logger) (aicodingsession.SpecEntry, error) {
	redacted, err := redactCapture(ctx, redactor, c)
	if err != nil {
		return aicodingsession.SpecEntry{}, err
	}

	if redacted.MetaRaw != nil {
		redacted.Meta = redactMeta(ctx, redactor, redacted.MetaRaw, log)
	}

	digest, err := addSpecMaterial(ctx, adder, dir, name, sessionID, redacted)
	if err != nil {
		return aicodingsession.SpecEntry{}, err
	}

	return aicodingsession.SpecEntry{
		Kind:        redacted.Kind,
		Role:        redacted.Role,
		Title:       redacted.Title,
		Description: redacted.Description,
		URI:         redacted.URI,
		Digest:      digest,
		CapturedAt:  redacted.CapturedAt,
	}, nil
}

// redactMeta redacts the companion file of a binary file and reads its values.
// A companion file that cannot be scanned gives no values: they are what could
// not be scanned, and the binary file itself is stored as it is anyway.
func redactMeta(ctx context.Context, redactor *specRedactor, raw []byte, log zerolog.Logger) spec.Meta {
	doc, err := redactor.Redact(ctx, raw)
	if err != nil {
		log.Warn().Err(err).Msg("could not scan a spec companion file; its values are not recorded")
		return spec.Meta{}
	}

	return spec.ParseMeta(doc)
}

// redactCapture redacts the text file a capture was read from. The redacted file
// is what gets stored, header included, so it is the file on disk with only
// its secrets taken out. It is parsed again for the source address, the title
// and the description that the annotations and the reference carry, so they
// are redacted too.
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
	c.Meta = parsed.Meta
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
	for key, value := range map[string]string{
		specAnnotationURI:         c.URI,
		specAnnotationRole:        c.Role,
		specAnnotationTitle:       c.Title,
		specAnnotationDescription: c.Description,
	} {
		if value != "" {
			annotations[key] = value
		}
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
	dir string
	// skip returns each file unchanged, cache and scanner both bypassed: the
	// opt-out is for a trusted local export, which can then hold secrets
	// (R-004).
	skip   bool
	redact func(ctx context.Context, doc []byte) ([]byte, error)
}

// newSpecRedactor returns a redactor that uses the secret scanner of the
// session material and keeps its copies under dir. When skip is set the
// redactor returns each file unchanged.
func newSpecRedactor(dir string, skip bool) *specRedactor {
	return &specRedactor{dir: dir, skip: skip, redact: func(ctx context.Context, doc []byte) ([]byte, error) {
		redacted, _, err := aicodingsession.RedactSpecText(ctx, string(doc))
		return []byte(redacted), err
	}}
}

// Redact returns the redacted form of a spec file, from the stored copy when
// an earlier push already scanned the same file. The whole file is scanned,
// header included, so the source address is redacted like the text. A copy
// that cannot be stored costs the next push a scan, never this one its spec.
func (r *specRedactor) Redact(ctx context.Context, doc []byte) ([]byte, error) {
	// The opt-out bypasses the cache too: a cached copy from an earlier
	// redacting run would otherwise override it.
	if r.skip {
		return doc, nil
	}

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
	return specMaterialPrefix(sessionID) + materialStem(strings.TrimSuffix(fileName, filepath.Ext(fileName)), "source")
}

// specMaterialPrefix is the start of the name of each spec material of a
// session: spec-<first 6 characters of the session ID>-.
func specMaterialPrefix(sessionID string) string {
	short := sessionID
	if len(short) > 6 {
		short = short[:6]
	}

	return "spec-" + short + "-"
}

// materialStem sanitizes name into the part of a material name taken from it,
// cut to maxSpecSlugLen, or returns fallback when nothing is left.
func materialStem(name, fallback string) string {
	stem := materials.SanitizeMaterialName(name)
	if len(stem) > maxSpecSlugLen {
		stem = strings.TrimRight(stem[:maxSpecSlugLen], "-")
	}
	if stem == "" {
		return fallback
	}

	return stem
}
