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
	"os"
	"path"
	"strings"
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
	visit := func(name string, size int64, r io.Reader) error {
		if !safeArchivePath(name) {
			return fmt.Errorf("unsafe entry path %q in archive", name)
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
// extraction root via "..".
func safeArchivePath(name string) bool {
	normalized := strings.ReplaceAll(name, "\\", "/")
	// Reject absolute paths
	if strings.HasPrefix(normalized, "/") {
		return false
	}
	// Reject any path containing ".." which could escape the root
	if strings.Contains(normalized, "..") {
		return false
	}
	// Further validation: ensure no traversal after normalization
	clean := path.Clean("/" + normalized)
	return !strings.Contains(clean, "/../") && clean != "/.."
}

func walkZip(p string, visit func(name string, size int64, r io.Reader) error) error {
	zr, err := zip.OpenReader(p)
	if err != nil {
		return fmt.Errorf("opening zip: %w", err)
	}
	defer zr.Close()

	for _, f := range zr.File {
		if f.FileInfo().IsDir() || f.Mode()&os.ModeSymlink != 0 || f.UncompressedSize64 == 0 {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			return fmt.Errorf("opening entry %q: %w", f.Name, err)
		}
		err = visit(f.Name, int64(f.UncompressedSize64), rc)
		rc.Close()
		if err != nil {
			return err
		}
	}
	return nil
}

func walkTar(p string, gzipped bool, visit func(name string, size int64, r io.Reader) error) error {
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
		if hdr.Typeflag != tar.TypeReg || hdr.Size == 0 {
			continue
		}
		if err := visit(hdr.Name, hdr.Size, tr); err != nil {
			return err
		}
	}
}
