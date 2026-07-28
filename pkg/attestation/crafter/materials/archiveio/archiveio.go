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

// Package archiveio detects and walks the archive containers Chainloop accepts
// as material values (zip, tar, tar.gz), enforcing the guards that keep a
// hostile archive from exhausting the host: entry-count and uncompressed-size
// limits, and rejection of paths that escape the extraction root.
//
// It is a leaf package so that both the material crafters (which validate an
// archive from a path on disk) and the policy-input projections (which
// aggregate an archive already held in memory as material bytes) can share one
// implementation of those guards. Duplicating them would risk the two paths
// disagreeing about what is safe.
package archiveio

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
)

// Format identifies a supported archive container.
type Format int

const (
	None Format = iota
	Zip
	Tar
	TarGz
)

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

// Limits bounds archive expansion to guard against zip bombs.
type Limits struct {
	MaxEntries   int
	MaxTotalSize int64
}

// DefaultLimits returns the safe defaults: 10000 entries and 1 GiB total
// uncompressed size.
func DefaultLimits() Limits {
	return Limits{MaxEntries: 10000, MaxTotalSize: 1 << 30}
}

// magicPeek is the number of leading bytes needed to recognize every supported
// container: enough for the gzip/zip magic and the tar "ustar" marker at
// offset 257.
const magicPeek = 512

// DetectPath reports whether path is a supported archive and, if so, its
// format. Detection is by extension first; for files whose extension does not
// match, magic bytes are used as a backstop so renamed archives are still
// caught. A non-archive returns (None, nil).
func DetectPath(p string) (Format, error) {
	if f := detectByExtension(p); f != None {
		return f, nil
	}

	return detectPathByMagic(p)
}

// detectByExtension recognizes a container from a filename suffix, returning
// None when the name carries no archive extension.
func detectByExtension(p string) Format {
	lower := strings.ToLower(p)
	switch {
	case strings.HasSuffix(lower, ".zip"):
		return Zip
	case strings.HasSuffix(lower, ".tar.gz"), strings.HasSuffix(lower, ".tgz"):
		return TarGz
	case strings.HasSuffix(lower, ".tar"):
		return Tar
	}
	return None
}

// DetectFile reports the container format of the file at p from its content
// alone, ignoring its name. A non-archive returns (None, nil).
//
// Prefer this over DetectPath when the same content will later be detected from
// bytes with DetectBytes and the two decisions must agree — for example a crafter
// validating a value that a policy-input projection will re-read from the
// recorded material. DetectPath trusts the filename first, so a name can
// disagree with the content: a zip carrying a prepended stub still opens (its
// central directory is at the end) but no longer starts with the zip magic, so
// DetectPath calls it an archive and DetectBytes does not. Detecting from content
// on both sides makes such a value fail loudly at craft time instead of being
// accepted and then silently failing to project.
func DetectFile(p string) (Format, error) {
	return detectPathByMagic(p)
}

func detectPathByMagic(p string) (Format, error) {
	f, err := os.Open(p)
	if err != nil {
		// These errors mean the value is not a file path at all (e.g. "hello
		// world" for STRING, or "registry/app:v1" for CONTAINER_IMAGE where
		// "registry" happens to be a regular file in the working directory, which
		// yields ENOTDIR); treat them as a non-archive so callers passing non-file
		// values are not surprised. Any other error (permissions, I/O) is real and
		// must surface.
		if errors.Is(err, fs.ErrNotExist) || errors.Is(err, syscall.ENOTDIR) {
			return None, nil
		}
		return None, fmt.Errorf("opening %q: %w", p, err)
	}
	defer f.Close()

	// Fill the buffer rather than trusting a single Read to return it all: the tar
	// marker sits at offset 257, so a short read would misdetect a valid tar as a
	// plain file. A file smaller than the buffer is an expected short read; any
	// other read failure is real and must surface instead of being reported as
	// "not an archive".
	header := make([]byte, magicPeek)
	n, err := io.ReadFull(f, header)
	if err != nil && !errors.Is(err, io.EOF) && !errors.Is(err, io.ErrUnexpectedEOF) {
		return None, fmt.Errorf("reading %q: %w", p, err)
	}

	return DetectBytes(header[:n]), nil
}

// DetectBytes reports the container format of an in-memory blob from its magic
// bytes alone, returning None when the bytes are not a supported archive. It is
// the counterpart to DetectPath for callers that already hold the content, such
// as a policy-input projection working from recorded material bytes.
func DetectBytes(data []byte) Format {
	switch {
	case bytes.HasPrefix(data, []byte("PK\x03\x04")), bytes.HasPrefix(data, []byte("PK\x05\x06")):
		return Zip
	case bytes.HasPrefix(data, []byte{0x1f, 0x8b}):
		return TarGz
	case len(data) >= 262 && bytes.Equal(data[257:262], []byte("ustar")):
		return Tar
	}
	return None
}

// capReader wraps a reader and fails once the shared running total exceeds max,
// so we never trust an archive's declared sizes.
type capReader struct {
	r     io.Reader
	total *int64
	max   int64
}

// Read reports ErrArchiveTooLarge with n == 0 rather than alongside the bytes
// that broke the budget. Returning the error together with a full buffer would
// let it be swallowed: io.ReadFull — which archive/tar uses for every 512-byte
// header — discards an error that accompanies a complete read, so a stream read in
// exact block sizes would blow past the limit unreported.
//
// One byte beyond the budget is read deliberately, so a stream sized exactly at
// the limit is allowed while one that exceeds it is caught.
func (c *capReader) Read(p []byte) (int, error) {
	if *c.total > c.max {
		return 0, ErrArchiveTooLarge
	}
	if probe := c.max - *c.total + 1; int64(len(p)) > probe {
		p = p[:probe]
	}

	n, err := c.r.Read(p)
	*c.total += int64(n)
	if *c.total > c.max {
		return 0, ErrArchiveTooLarge
	}

	return n, err
}

// walkGuard applies the entry-count, size and path-safety limits shared by every
// walk entry point.
//
// The limits are checked for every entry the walker reads a header for, not only
// the ones handed to the caller: skipping an entry still costs the work of
// decompressing enough of the archive to reach the next one, so an archive of
// nothing but directory entries must not walk unmeasured.
type walkGuard struct {
	limits Limits
	yield  func(name string, r io.Reader) error
	total  int64
	count  int
	// capEntries bounds each entry body individually, for containers whose entries
	// decompress independently (zip). Stream containers (tar) bound the whole
	// stream instead — see capStream — which already covers their entry bodies.
	capEntries bool
}

// consider records an entry the walker has read a header for, whether or not it
// will be handed to the caller.
func (g *walkGuard) consider(name string) error {
	if !safePath(name) {
		return fmt.Errorf("%w: %q", ErrUnsafeEntry, name)
	}
	g.count++
	if g.count > g.limits.MaxEntries {
		return ErrTooManyEntries
	}
	return nil
}

// visit hands a qualifying entry's body to the caller.
func (g *walkGuard) visit(name string, r io.Reader) error {
	if g.capEntries {
		r = &capReader{r: r, total: &g.total, max: g.limits.MaxTotalSize}
	}
	if err := g.yield(name, r); err != nil {
		return fmt.Errorf("processing entry %q: %w", name, err)
	}
	return nil
}

// capStream bounds every byte read from a stream container, so headers and the
// padding skipped past unqualifying entries count towards the size limit too.
func (g *walkGuard) capStream(r io.Reader) io.Reader {
	return &capReader{r: r, total: &g.total, max: g.limits.MaxTotalSize}
}

// WalkPath calls yield for every regular file in the archive at p, enforcing
// limits and skipping directories, symlinks, hardlinks, empty entries, and
// path-traversal entries.
func WalkPath(p string, format Format, limits Limits, yield func(name string, r io.Reader) error) error {
	switch format {
	case Zip:
		zr, err := zip.OpenReader(p)
		if err != nil {
			return fmt.Errorf("opening zip: %w", err)
		}
		defer zr.Close()
		return walkZipEntries(zr.File, &walkGuard{limits: limits, yield: yield, capEntries: true})
	case Tar, TarGz:
		f, err := os.Open(p)
		if err != nil {
			return fmt.Errorf("opening tar: %w", err)
		}
		defer f.Close()
		return walkTarStream(f, format == TarGz, &walkGuard{limits: limits, yield: yield})
	default:
		return fmt.Errorf("unsupported archive format")
	}
}

// WalkBytes is WalkPath for an archive already held in memory. It applies the
// same guards, so a projection reading recorded material bytes cannot be
// tricked by an archive a crafter would have rejected.
func WalkBytes(data []byte, format Format, limits Limits, yield func(name string, r io.Reader) error) error {
	switch format {
	case Zip:
		zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
		if err != nil {
			return fmt.Errorf("opening zip: %w", err)
		}
		return walkZipEntries(zr.File, &walkGuard{limits: limits, yield: yield, capEntries: true})
	case Tar, TarGz:
		return walkTarStream(bytes.NewReader(data), format == TarGz, &walkGuard{limits: limits, yield: yield})
	default:
		return fmt.Errorf("unsupported archive format")
	}
}

// safePath rejects absolute paths and any path that escapes the extraction root
// via ".." path components. A filename that merely contains ".." as a substring
// (e.g. "foo..bar.json") is accepted; only actual path components equal to ".."
// are rejected.
func safePath(name string) bool {
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

func walkZipEntries(files []*zip.File, g *walkGuard) error {
	for _, f := range files {
		if err := g.consider(f.Name); err != nil {
			return err
		}
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
		err = g.visit(f.Name, rc)
		rc.Close()
		if err != nil {
			return err
		}
	}
	return nil
}

func walkTarStream(src io.Reader, gzipped bool, g *walkGuard) error {
	if gzipped {
		gz, err := gzip.NewReader(src)
		if err != nil {
			return fmt.Errorf("opening gzip: %w", err)
		}
		defer gz.Close()
		src = gz
	}

	// A tar is one stream, so capping it bounds every byte the walk decompresses:
	// entry bodies, headers, and the padding skipped past entries that never reach
	// the caller.
	tr := tar.NewReader(g.capStream(src))
	for {
		hdr, err := tr.Next()
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			if errors.Is(err, ErrArchiveTooLarge) {
				return err
			}
			return fmt.Errorf("reading tar: %w", err)
		}
		if err := g.consider(hdr.Name); err != nil {
			return err
		}
		// Only regular files become materials; directories, symlinks, hardlinks
		// and other special entries are skipped via the typeflag. Empty entries
		// are skipped intentionally (an empty evidence file produces no material).
		if hdr.Typeflag != tar.TypeReg || hdr.Size == 0 {
			continue
		}
		if err := g.visit(hdr.Name, tr); err != nil {
			return err
		}
	}
}
