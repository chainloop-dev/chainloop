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

package archiveio

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// zipBytes builds an in-memory zip containing the given entries.
func zipBytes(t *testing.T, files map[string]string) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for name, content := range files {
		w, err := zw.Create(name)
		require.NoError(t, err)
		_, err = w.Write([]byte(content))
		require.NoError(t, err)
	}
	require.NoError(t, zw.Close())
	return buf.Bytes()
}

// tarGzBytes builds an in-memory tar.gz containing the given regular files.
func tarGzBytes(t *testing.T, files map[string]string) []byte {
	t.Helper()
	var buf bytes.Buffer
	gw := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gw)
	for name, content := range files {
		require.NoError(t, tw.WriteHeader(&tar.Header{
			Name:     name,
			Typeflag: tar.TypeReg,
			Mode:     0o600,
			Size:     int64(len(content)),
		}))
		_, err := tw.Write([]byte(content))
		require.NoError(t, err)
	}
	require.NoError(t, tw.Close())
	require.NoError(t, gw.Close())
	return buf.Bytes()
}

// collect walks data and returns entry name → content.
func collect(t *testing.T, data []byte, format Format, limits Limits) (map[string]string, error) {
	t.Helper()
	got := map[string]string{}
	err := WalkBytes(data, format, limits, func(name string, r io.Reader) error {
		content, err := io.ReadAll(r)
		if err != nil {
			return err
		}
		got[name] = string(content)
		return nil
	})
	return got, err
}

func TestDetectBytes(t *testing.T) {
	tests := []struct {
		name string
		data []byte
		want Format
	}{
		{"zip local file header", []byte("PK\x03\x04rest"), Zip},
		{"empty zip end-of-central-directory", []byte("PK\x05\x06rest"), Zip},
		{"gzip", []byte{0x1f, 0x8b, 0x08, 0x00}, TarGz},
		{"plain text", []byte("Test Engine Version: $Rev: 96 $"), None},
		{"empty", nil, None},
		{"too short for ustar", make([]byte, 100), None},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, DetectBytes(tc.data))
		})
	}
}

func TestDetectBytesTar(t *testing.T) {
	// A tar is recognized by the "ustar" marker at offset 257 rather than a
	// leading magic, so build a real one instead of hand-rolling the header.
	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)
	require.NoError(t, tw.WriteHeader(&tar.Header{
		Name: "a.txt", Typeflag: tar.TypeReg, Mode: 0o600, Size: 1,
	}))
	_, err := tw.Write([]byte("x"))
	require.NoError(t, err)
	require.NoError(t, tw.Close())

	assert.Equal(t, Tar, DetectBytes(buf.Bytes()))
}

func TestWalkBytes(t *testing.T) {
	t.Run("zip yields every regular entry", func(t *testing.T) {
		data := zipBytes(t, map[string]string{"a.txt": "alpha", "nested/b.txt": "beta"})

		got, err := collect(t, data, Zip, DefaultLimits())

		require.NoError(t, err)
		assert.Equal(t, map[string]string{"a.txt": "alpha", "nested/b.txt": "beta"}, got)
	})

	t.Run("tar.gz yields every regular entry", func(t *testing.T) {
		data := tarGzBytes(t, map[string]string{"a.txt": "alpha", "b.txt": "beta"})

		got, err := collect(t, data, TarGz, DefaultLimits())

		require.NoError(t, err)
		assert.Equal(t, map[string]string{"a.txt": "alpha", "b.txt": "beta"}, got)
	})

	t.Run("skips directory entries", func(t *testing.T) {
		var buf bytes.Buffer
		zw := zip.NewWriter(&buf)
		_, err := zw.Create("adir/")
		require.NoError(t, err)
		w, err := zw.Create("adir/a.txt")
		require.NoError(t, err)
		_, err = w.Write([]byte("alpha"))
		require.NoError(t, err)
		require.NoError(t, zw.Close())

		got, err := collect(t, buf.Bytes(), Zip, DefaultLimits())

		require.NoError(t, err)
		assert.Equal(t, map[string]string{"adir/a.txt": "alpha"}, got)
	})

	t.Run("enforces the entry-count limit", func(t *testing.T) {
		data := zipBytes(t, map[string]string{"a.txt": "alpha", "b.txt": "beta"})

		_, err := collect(t, data, Zip, Limits{MaxEntries: 1, MaxTotalSize: 1 << 30})

		assert.ErrorIs(t, err, ErrTooManyEntries)
	})

	t.Run("enforces the uncompressed-size limit", func(t *testing.T) {
		data := zipBytes(t, map[string]string{"a.txt": "aaaaaaaaaa"})

		_, err := collect(t, data, Zip, Limits{MaxEntries: 10, MaxTotalSize: 4})

		assert.ErrorIs(t, err, ErrArchiveTooLarge)
	})

	// Entries that carry no content are skipped rather than yielded, but reaching
	// them still costs decompression, so they must count against the limits. A
	// tar.gz of nothing but directory entries compresses to almost nothing and
	// would otherwise be walked without either guard ever measuring it.
	t.Run("counts skipped entries against the entry limit", func(t *testing.T) {
		var buf bytes.Buffer
		gw := gzip.NewWriter(&buf)
		tw := tar.NewWriter(gw)
		for i := 0; i < 50; i++ {
			require.NoError(t, tw.WriteHeader(&tar.Header{
				Name: fmt.Sprintf("dir%d/", i), Typeflag: tar.TypeDir, Mode: 0o700,
			}))
		}
		require.NoError(t, tw.Close())
		require.NoError(t, gw.Close())

		_, err := collect(t, buf.Bytes(), TarGz, Limits{MaxEntries: 10, MaxTotalSize: 1 << 30})

		assert.ErrorIs(t, err, ErrTooManyEntries)
	})

	// Tar headers and padding are decompressed to reach the next entry, so the size
	// cap has to bound the whole stream and not merely the bodies handed to the
	// caller: a dir-only archive yields nothing yet still costs 512 bytes of output
	// per entry.
	t.Run("counts stream bytes read past skipped entries against the size limit", func(t *testing.T) {
		var buf bytes.Buffer
		gw := gzip.NewWriter(&buf)
		tw := tar.NewWriter(gw)
		for i := 0; i < 200; i++ {
			require.NoError(t, tw.WriteHeader(&tar.Header{
				Name: fmt.Sprintf("dir%d/", i), Typeflag: tar.TypeDir, Mode: 0o700,
			}))
		}
		require.NoError(t, tw.Close())
		require.NoError(t, gw.Close())

		// Generous entry allowance, tight size allowance: only a stream-level cap
		// can catch this.
		_, err := collect(t, buf.Bytes(), TarGz, Limits{MaxEntries: 1000, MaxTotalSize: 4096})

		assert.ErrorIs(t, err, ErrArchiveTooLarge)
	})

	t.Run("rejects a path-traversal entry", func(t *testing.T) {
		data := tarGzBytes(t, map[string]string{"../escape.txt": "evil"})

		_, err := collect(t, data, TarGz, DefaultLimits())

		assert.ErrorIs(t, err, ErrUnsafeEntry)
	})

	t.Run("rejects an unsupported format", func(t *testing.T) {
		_, err := collect(t, []byte("nope"), None, DefaultLimits())

		require.Error(t, err)
	})
}

func TestDetectFileSurfacesReadErrors(t *testing.T) {
	// A path that opens but cannot be read must not be reported as "not an
	// archive": swallowing the read error would turn an I/O failure into a
	// silent misclassification, and the caller would go on to treat the value as
	// a plain file.
	_, err := DetectFile(t.TempDir())

	require.Error(t, err)
}

func TestDetectFileReadsEnoughForTheTarMarker(t *testing.T) {
	// tar is recognized by "ustar" at offset 257 rather than a leading magic, so
	// detection has to fill the peek buffer instead of trusting a single Read to
	// return it all.
	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)
	require.NoError(t, tw.WriteHeader(&tar.Header{
		Name: "a.txt", Typeflag: tar.TypeReg, Mode: 0o600, Size: 1,
	}))
	_, err := tw.Write([]byte("x"))
	require.NoError(t, err)
	require.NoError(t, tw.Close())

	p := filepath.Join(t.TempDir(), "noextension")
	require.NoError(t, os.WriteFile(p, buf.Bytes(), 0o600))

	got, err := DetectFile(p)

	require.NoError(t, err)
	assert.Equal(t, Tar, got)
}

func TestDetectFileOnShortAndMissingFiles(t *testing.T) {
	dir := t.TempDir()

	short := filepath.Join(dir, "short.bin")
	require.NoError(t, os.WriteFile(short, []byte("tiny"), 0o600))
	empty := filepath.Join(dir, "empty.bin")
	require.NoError(t, os.WriteFile(empty, nil, 0o600))

	// A file smaller than the peek buffer is an expected short read, not an error.
	got, err := DetectFile(short)
	require.NoError(t, err)
	assert.Equal(t, None, got)

	got, err = DetectFile(empty)
	require.NoError(t, err)
	assert.Equal(t, None, got)

	// A value that is not a file at all stays a non-archive without erroring, so
	// callers passing non-path values (a STRING material) are not surprised.
	got, err = DetectFile(filepath.Join(dir, "nope"))
	require.NoError(t, err)
	assert.Equal(t, None, got)
}

func TestSafePath(t *testing.T) {
	tests := []struct {
		name string
		path string
		want bool
	}{
		{"absolute path", "/etc/passwd", false},
		{"windows drive-letter backslash", "C:\\Windows\\system32", false},
		{"windows drive-letter forward slash", "c:/windows/system32", false},
		{"path traversal", "../escape.txt", false},
		{"nested path traversal", "foo/../../../etc/passwd", false},
		{"double dot in filename is ok", "foo..bar.json", true},
		{"escape via nested double dot", "a/../../etc/passwd", false},
		{"valid nested path", "a/b.txt", true},
		{"valid simple path", "file.txt", true},
		{"valid with subdirs", "nested/dir/file.txt", true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := safePath(tc.path)
			assert.Equal(t, tc.want, got)
		})
	}
}
