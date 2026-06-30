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
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"strings"
	"syscall"

	schemaapi "github.com/chainloop-dev/chainloop/app/controlplane/api/workflowcontract/v1"
)

// ArchiveFormat identifies a supported archive container.
type ArchiveFormat int

const (
	ArchiveNone ArchiveFormat = iota
	ArchiveZip
	ArchiveTar
	ArchiveTarGz
)

// DetectArchive reports whether path is a supported archive and, if so, its
// format. Detection is by extension first; for files whose extension does not
// match, magic bytes are used as a backstop so renamed archives are still
// caught. A non-archive returns (ArchiveNone, nil).
func DetectArchive(path string) (ArchiveFormat, error) {
	lower := strings.ToLower(path)
	switch {
	case strings.HasSuffix(lower, ".zip"):
		return ArchiveZip, nil
	case strings.HasSuffix(lower, ".tar.gz"), strings.HasSuffix(lower, ".tgz"):
		return ArchiveTarGz, nil
	case strings.HasSuffix(lower, ".tar"):
		return ArchiveTar, nil
	}

	return detectByMagic(path)
}

func detectByMagic(path string) (ArchiveFormat, error) {
	f, err := os.Open(path)
	if err != nil {
		// These errors mean the value is not a file path at all (e.g. "hello
		// world" for STRING, or "registry/app:v1" for CONTAINER_IMAGE where
		// "registry" happens to be a regular file in the working directory, which
		// yields ENOTDIR); treat them as a non-archive so callers passing non-file
		// values are not surprised. Any other error (permissions, I/O) is real and
		// must surface.
		if errors.Is(err, fs.ErrNotExist) || errors.Is(err, syscall.ENOTDIR) {
			return ArchiveNone, nil
		}
		return ArchiveNone, fmt.Errorf("opening %q: %w", path, err)
	}
	defer f.Close()

	// 512 bytes is enough for the gzip/zip magic and the tar "ustar" marker at
	// offset 257.
	header := make([]byte, 512)
	n, _ := f.Read(header)
	header = header[:n]

	switch {
	case bytes.HasPrefix(header, []byte("PK\x03\x04")), bytes.HasPrefix(header, []byte("PK\x05\x06")):
		return ArchiveZip, nil
	case bytes.HasPrefix(header, []byte{0x1f, 0x8b}):
		return ArchiveTarGz, nil
	case len(header) >= 262 && bytes.Equal(header[257:262], []byte("ustar")):
		return ArchiveTar, nil
	}

	return ArchiveNone, nil
}

var (
	// ErrTooManyEntries is returned when an archive has more qualifying entries
	// than the configured maximum.
	ErrTooManyEntries = errors.New("archive exceeds the maximum number of entries")
	// ErrArchiveTooLarge is returned when the running uncompressed size of an
	// archive exceeds the configured maximum.
	ErrArchiveTooLarge = errors.New("archive exceeds the maximum uncompressed size")
	// ErrUnsafeEntry is returned when an archive entry's path is absolute or escapes the extraction root.
	ErrUnsafeEntry = errors.New("unsafe entry path in archive")
)

// ArchiveLimits bounds archive expansion to guard against zip bombs.
type ArchiveLimits struct {
	MaxEntries   int
	MaxTotalSize int64
}

// DefaultArchiveLimits returns the safe defaults: 10000 entries and 1 GiB
// total uncompressed size.
func DefaultArchiveLimits() ArchiveLimits {
	return ArchiveLimits{MaxEntries: 10000, MaxTotalSize: 1 << 30}
}

// capReader wraps a reader and fails once the shared running total exceeds max,
// so we never trust an archive's declared sizes.
type capReader struct {
	r     io.Reader
	total *int64
	max   int64
}

func (c *capReader) Read(p []byte) (int, error) {
	n, err := c.r.Read(p)
	*c.total += int64(n)
	if *c.total > c.max {
		return n, ErrArchiveTooLarge
	}
	return n, err
}

// WalkArchiveEntries calls yield for every regular file in the archive,
// enforcing the limits and skipping directories, symlinks, hardlinks, empty
// entries, and path-traversal entries.
func WalkArchiveEntries(path string, format ArchiveFormat, limits ArchiveLimits, yield func(name string, r io.Reader) error) error {
	var total int64
	count := 0
	visit := func(name string, r io.Reader) error {
		if !safeArchivePath(name) {
			return fmt.Errorf("%w: %q", ErrUnsafeEntry, name)
		}
		count++
		if count > limits.MaxEntries {
			return ErrTooManyEntries
		}
		if err := yield(name, &capReader{r: r, total: &total, max: limits.MaxTotalSize}); err != nil {
			return fmt.Errorf("processing entry %q: %w", name, err)
		}
		return nil
	}

	switch format {
	case ArchiveZip:
		return walkZip(path, visit)
	case ArchiveTar:
		return walkTar(path, false, visit)
	case ArchiveTarGz:
		return walkTar(path, true, visit)
	default:
		return fmt.Errorf("unsupported archive format")
	}
}

// safeArchivePath rejects absolute paths and any path that escapes the
// extraction root via ".." path components. A filename that merely contains
// ".." as a substring (e.g. "foo..bar.json") is accepted; only actual path
// components equal to ".." are rejected.
func safeArchivePath(name string) bool {
	normalized := strings.ReplaceAll(name, "\\", "/")
	// Reject absolute paths, including Windows drive-letter (e.g. "C:/x") and
	// UNC paths (which normalize to a leading "/").
	if strings.HasPrefix(normalized, "/") || hasWindowsDriveLetter(normalized) {
		return false
	}
	// Canonicalise against a virtual root and check that the result stays
	// within it. path.Clean will resolve ".." components so a path like
	// "a/../../etc/passwd" becomes "/etc/passwd" which does not start with
	// the virtual prefix "/root/"; a safe path like "a/b.txt" becomes
	// "/root/a/b.txt" which does.
	const root = "/root"
	clean := path.Clean(root + "/" + normalized)
	return strings.HasPrefix(clean, root+"/") || clean == root
}

// hasWindowsDriveLetter reports whether name begins with a Windows drive-letter
// prefix such as "C:" or "c:/", which denotes an absolute path on Windows.
func hasWindowsDriveLetter(name string) bool {
	if len(name) < 2 || name[1] != ':' {
		return false
	}
	c := name[0]
	return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z')
}

func walkZip(p string, visit func(name string, r io.Reader) error) error {
	zr, err := zip.OpenReader(p)
	if err != nil {
		return fmt.Errorf("opening zip: %w", err)
	}
	defer zr.Close()

	for _, f := range zr.File {
		// Skip directories, symlinks, and empty entries: they carry no file
		// content worth recording as a material. Empty-entry skipping is
		// intentional per the explode design (an empty evidence file produces
		// no material). Note: symlink detection relies on Unix mode bits stored
		// in the zip; archives written without Unix metadata won't carry the
		// symlink bit, so such a symlink would be treated as a regular file
		// (its content being the stored target path). Tar symlinks are detected
		// reliably via the typeflag below.
		if f.FileInfo().IsDir() || f.Mode()&os.ModeSymlink != 0 || f.UncompressedSize64 == 0 {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			return fmt.Errorf("opening entry %q: %w", f.Name, err)
		}
		err = visit(f.Name, rc)
		rc.Close()
		if err != nil {
			return err
		}
	}
	return nil
}

func walkTar(p string, gzipped bool, visit func(name string, r io.Reader) error) error {
	f, err := os.Open(p)
	if err != nil {
		return fmt.Errorf("opening tar: %w", err)
	}
	defer f.Close()

	var src io.Reader = f
	if gzipped {
		gz, err := gzip.NewReader(f)
		if err != nil {
			return fmt.Errorf("opening gzip: %w", err)
		}
		defer gz.Close()
		src = gz
	}

	tr := tar.NewReader(src)
	for {
		hdr, err := tr.Next()
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return fmt.Errorf("reading tar: %w", err)
		}
		// Only regular files become materials; directories, symlinks, hardlinks
		// and other special entries are skipped via the typeflag. Empty entries
		// are skipped intentionally (an empty evidence file produces no material).
		if hdr.Typeflag != tar.TypeReg || hdr.Size == 0 {
			continue
		}
		if err := visit(hdr.Name, tr); err != nil {
			return err
		}
	}
}

// archiveNativeKinds lists material kinds whose value is the archive itself.
// For these, --kind short-circuits the explode path and the archive is
// recorded whole. Extend this set as new "the archive is the material" kinds
// are added.
var archiveNativeKinds = map[string]struct{}{
	schemaapi.CraftingSchema_Material_ZAP_DAST_ZIP.String(): {},
}

// IsArchiveNativeKind reports whether kind treats the archive as a single
// material (recorded whole) rather than something to explode.
func IsArchiveNativeKind(kind string) bool {
	_, ok := archiveNativeKinds[kind]
	return ok
}

// ArchiveEntryBaseName returns the final element of an archive entry name using
// archive ("/") path semantics, independent of the host OS. Archive entry names
// are "/"-separated by spec; backslashes are normalized first so names produced
// on Windows resolve to the same basename everywhere (filepath.Base would treat
// "\\" as a separator only on Windows, yielding OS-dependent results).
func ArchiveEntryBaseName(name string) string {
	return path.Base(strings.ReplaceAll(name, "\\", "/"))
}

// SanitizeMaterialName converts s into a valid DNS-1123 material name:
// lowercase, with every run of characters outside [a-z0-9] collapsed to a
// single "-" and leading/trailing "-" trimmed. Falls back to "material".
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
	if b.Len() == 0 {
		return "material"
	}
	return b.String()
}

// NameAllocator hands out sequential, unique DNS-1123 material names of the
// form "<prefix>-<n>" (n starting at 1). It is seeded with names already present
// in the attestation so derived names never overwrite existing materials.
type NameAllocator struct {
	used map[string]struct{}
	seq  int
}

// NewNameAllocator seeds the allocator with existing material names.
func NewNameAllocator(existing []string) *NameAllocator {
	used := make(map[string]struct{}, len(existing))
	for _, e := range existing {
		used[e] = struct{}{}
	}
	return &NameAllocator{used: used}
}

// AllocateSequential returns the next unused "<prefix>-<n>" material name, where
// n is a zero-based counter that advances across calls and skips names already
// in use. prefix is sanitized to DNS-1123; an empty or symbol-only prefix yields
// the base "material" (so entries are named material-0, material-1, …).
func (a *NameAllocator) AllocateSequential(prefix string) string {
	base := "material"
	if prefix != "" {
		base = SanitizeMaterialName(prefix)
	}

	for {
		candidate := fmt.Sprintf("%s-%d", base, a.seq)
		a.seq++
		if _, taken := a.used[candidate]; !taken {
			a.used[candidate] = struct{}{}
			return candidate
		}
	}
}
