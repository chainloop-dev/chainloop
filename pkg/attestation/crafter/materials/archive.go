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

package materials

import (
	"fmt"
	"io"
	"path"
	"strings"

	schemaapi "github.com/chainloop-dev/chainloop/app/controlplane/api/workflowcontract/v1"
	"github.com/chainloop-dev/chainloop/pkg/attestation/crafter/materials/archiveio"
)

// The archive container primitives live in the archiveio leaf package so the
// policy-input projections can share them without importing this package (which
// would be an import cycle). The names below are kept as aliases so existing
// callers and their tests are unaffected.

// ArchiveFormat identifies a supported archive container.
type ArchiveFormat = archiveio.Format

const (
	ArchiveNone  = archiveio.None
	ArchiveZip   = archiveio.Zip
	ArchiveTar   = archiveio.Tar
	ArchiveTarGz = archiveio.TarGz
)

// ArchiveLimits bounds archive expansion to guard against zip bombs.
type ArchiveLimits = archiveio.Limits

var (
	// ErrTooManyEntries is returned when an archive has more qualifying entries
	// than the configured maximum.
	ErrTooManyEntries = archiveio.ErrTooManyEntries
	// ErrArchiveTooLarge is returned when the running uncompressed size of an
	// archive exceeds the configured maximum.
	ErrArchiveTooLarge = archiveio.ErrArchiveTooLarge
	// ErrUnsafeEntry is returned when an archive entry's path is absolute or escapes the extraction root.
	ErrUnsafeEntry = archiveio.ErrUnsafeEntry
)

// DetectArchive reports whether path is a supported archive and, if so, its
// format. A non-archive returns (ArchiveNone, nil).
func DetectArchive(path string) (ArchiveFormat, error) {
	return archiveio.DetectPath(path)
}

// DefaultArchiveLimits returns the safe defaults: 10000 entries and 1 GiB
// total uncompressed size.
func DefaultArchiveLimits() ArchiveLimits {
	return archiveio.DefaultLimits()
}

// WalkArchiveEntries calls yield for every regular file in the archive,
// enforcing the limits and skipping directories, symlinks, hardlinks, empty
// entries, and path-traversal entries.
func WalkArchiveEntries(path string, format ArchiveFormat, limits ArchiveLimits, yield func(name string, r io.Reader) error) error {
	return archiveio.WalkPath(path, format, limits, yield)
}

// explodableKinds is the allowlist of material kinds whose archive value is
// expanded into one material per entry. Every other kind (ARTIFACT, EVIDENCE,
// ZAP_DAST_ZIP, …) records the archive whole, so a customer can still provide a
// regular zip as a single material.
//
// A kind that accepts an archive has two strategies available, and this set
// selects the first:
//
//   - Explode (this set): each entry is independent evidence that stands on its
//     own, so each becomes its own material bound to policies by type.
//   - Record whole plus an aggregating projection: the entries are one artifact
//     whose parts only mean something together, so the archive fills a single
//     contract slot and the projection folds the entries for the policy engine.
//     CERTCC_DRANZER works this way — see dranzer.ParseBundle and the projection
//     in Attestation_Material.ingestMaterialToJSON — because one run's per-mode
//     reports are facets of a single result, not interchangeable evidence.
//
// Adding a kind here changes its contract-slot semantics from one material to N,
// so choose the strategy before extending the set.
var explodableKinds = map[string]struct{}{
	schemaapi.CraftingSchema_Material_SBOM_CYCLONEDX_JSON.String(): {},
	schemaapi.CraftingSchema_Material_SBOM_SPDX_JSON.String():      {},
	schemaapi.CraftingSchema_Material_SARIF.String():               {},
}

// IsExplodableKind reports whether an archive provided for kind should be
// expanded into one material per entry (true) or recorded whole (false).
func IsExplodableKind(kind string) bool {
	_, ok := explodableKinds[kind]
	return ok
}

// ArchiveEntryBaseName returns the final element of an archive entry name using
// archive ("/") path semantics, independent of the host OS. Archive entry names
// are "/"-separated by spec; backslashes are normalized first so names produced
// on Windows resolve to the same basename everywhere (filepath.Base would treat
// "\\" as a separator only on Windows, yielding OS-dependent results).
func ArchiveEntryBaseName(name string) string {
	return path.Base(NormalizeArchivePath(name))
}

// NormalizeArchivePath converts an archive entry path to its canonical
// "/"-separated form, independent of the host OS. Archive entry names are
// "/"-separated by spec; backslashes produced on Windows are folded to "/" so
// the same entry resolves identically everywhere.
func NormalizeArchivePath(name string) string {
	return strings.ReplaceAll(name, "\\", "/")
}

// defaultMaterialName is the fallback base used when a name cannot be derived
// (empty/symbol-only input or prefix).
const defaultMaterialName = "material"

// SanitizeMaterialName converts s into a valid DNS-1123 material-name component:
// lowercase, with every run of characters outside [a-z0-9] collapsed to a single
// "-" and leading/trailing "-" trimmed. It returns "" when nothing usable
// remains; callers supply their own fallback.
func SanitizeMaterialName(s string) string {
	var b strings.Builder
	pendingHyphen := false
	for _, r := range strings.ToLower(s) {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			if pendingHyphen && b.Len() > 0 {
				b.WriteByte('-')
			}
			b.WriteRune(r)
			pendingHyphen = false
		} else {
			pendingHyphen = true
		}
	}
	return b.String()
}

// NameAllocator hands out unique DNS-1123 material names of the form "<base>",
// then "<base>-1", "<base>-2", …. It is seeded with names already present in the
// attestation so derived names never overwrite existing materials.
type NameAllocator struct {
	used map[string]struct{}
	// named tracks, per sanitized base, the next suffix to try so AllocateNamed
	// yields "<base>", "<base>-1", "<base>-2", … deterministically.
	named map[string]int
}

// NewNameAllocator seeds the allocator with existing material names.
func NewNameAllocator(existing []string) *NameAllocator {
	used := make(map[string]struct{}, len(existing))
	for _, e := range existing {
		used[e] = struct{}{}
	}
	return &NameAllocator{used: used}
}

// AllocateNamed returns a unique material name derived from base: the bare
// sanitized base on first use, then "<base>-1", "<base>-2", …. It skips names
// already in use (seeded existing materials or a base reused across archives)
// and falls back to the "material" base when base sanitizes to empty. Callers
// that want the whole set named deterministically must feed entries in a stable
// order (the explode path sorts entries by name before allocating).
func (a *NameAllocator) AllocateNamed(base string) string {
	b := defaultMaterialName
	if s := SanitizeMaterialName(base); s != "" {
		b = s
	}
	if a.named == nil {
		a.named = make(map[string]int)
	}

	for {
		n := a.named[b]
		a.named[b]++
		candidate := b
		if n > 0 {
			candidate = fmt.Sprintf("%s-%d", b, n)
		}
		if _, taken := a.used[candidate]; !taken {
			a.used[candidate] = struct{}{}
			return candidate
		}
	}
}
