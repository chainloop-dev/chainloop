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
	"compress/gzip"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// writeZip creates a zip at dir/name containing the given files (name->content).
func writeZip(t *testing.T, dir, name string, files map[string]string) string {
	t.Helper()
	p := filepath.Join(dir, name)
	f, err := os.Create(p)
	require.NoError(t, err)
	defer f.Close()
	zw := zip.NewWriter(f)
	for n, c := range files {
		w, err := zw.Create(n)
		require.NoError(t, err)
		_, err = w.Write([]byte(c))
		require.NoError(t, err)
	}
	require.NoError(t, zw.Close())
	return p
}

// writeTarGz creates a .tar.gz at dir/name containing the given regular files.
func writeTarGz(t *testing.T, dir, name string, files map[string]string) string {
	t.Helper()
	p := filepath.Join(dir, name)
	f, err := os.Create(p)
	require.NoError(t, err)
	defer f.Close()
	gw := gzip.NewWriter(f)
	tw := tar.NewWriter(gw)
	for n, c := range files {
		require.NoError(t, tw.WriteHeader(&tar.Header{Name: n, Mode: 0o600, Size: int64(len(c)), Typeflag: tar.TypeReg}))
		_, err = tw.Write([]byte(c))
		require.NoError(t, err)
	}
	require.NoError(t, tw.Close())
	require.NoError(t, gw.Close())
	return p
}

// writeTar creates an uncompressed .tar at dir/name containing the given regular files.
func writeTar(t *testing.T, dir, name string, files map[string]string) string {
	t.Helper()
	p := filepath.Join(dir, name)
	f, err := os.Create(p)
	require.NoError(t, err)
	defer f.Close()
	tw := tar.NewWriter(f)
	for n, c := range files {
		require.NoError(t, tw.WriteHeader(&tar.Header{Name: n, Mode: 0o600, Size: int64(len(c)), Typeflag: tar.TypeReg}))
		_, err = tw.Write([]byte(c))
		require.NoError(t, err)
	}
	require.NoError(t, tw.Close())
	return p
}

func TestDetectArchive(t *testing.T) {
	dir := t.TempDir()
	zipPath := writeZip(t, dir, "a.zip", map[string]string{"x.txt": "hi"})
	tgzPath := writeTarGz(t, dir, "a.tar.gz", map[string]string{"x.txt": "hi"})
	tarPath := writeTar(t, dir, "a.tar", map[string]string{"x.txt": "hi"})
	tgzShortPath := writeTarGz(t, dir, "a.tgz", map[string]string{"x.txt": "hi"})

	plain := filepath.Join(dir, "app.bin")
	require.NoError(t, os.WriteFile(plain, []byte("not an archive"), 0o600))

	// A .zip renamed without extension — magic bytes must still detect it.
	noExt := filepath.Join(dir, "noext")
	require.NoError(t, os.WriteFile(noExt, mustRead(t, zipPath), 0o600))

	tests := []struct {
		name string
		path string
		want ArchiveFormat
	}{
		{"zip by extension", zipPath, ArchiveZip},
		{"tar.gz by extension", tgzPath, ArchiveTarGz},
		{"tar by extension", tarPath, ArchiveTar},
		{"tgz by extension", tgzShortPath, ArchiveTarGz},
		{"plain file", plain, ArchiveNone},
		{"zip without extension via magic", noExt, ArchiveZip},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := DetectArchive(tc.path)
			require.NoError(t, err)
			assert.Equal(t, tc.want, got)
		})
	}
}

func mustRead(t *testing.T, p string) []byte {
	t.Helper()
	b, err := os.ReadFile(p)
	require.NoError(t, err)
	return b
}

func TestWalkArchiveEntries(t *testing.T) {
	dir := t.TempDir()

	t.Run("yields regular files, skips dirs", func(t *testing.T) {
		// Build a zip with a directory entry + two files.
		p := filepath.Join(dir, "files.zip")
		f, err := os.Create(p)
		require.NoError(t, err)
		zw := zip.NewWriter(f)
		_, err = zw.Create("nested/") // directory entry
		require.NoError(t, err)
		for _, n := range []string{"a.json", "nested/b.json"} {
			w, err := zw.Create(n)
			require.NoError(t, err)
			_, err = w.Write([]byte("{}"))
			require.NoError(t, err)
		}
		require.NoError(t, zw.Close())
		require.NoError(t, f.Close())

		var got []string
		err = WalkArchiveEntries(p, ArchiveZip, DefaultArchiveLimits(), func(name string, r io.Reader) error {
			b, _ := io.ReadAll(r)
			assert.Equal(t, "{}", string(b))
			got = append(got, name)
			return nil
		})
		require.NoError(t, err)
		assert.ElementsMatch(t, []string{"a.json", "nested/b.json"}, got)
	})

	t.Run("max entries exceeded", func(t *testing.T) {
		p := writeTarGz(t, dir, "many.tar.gz", map[string]string{"a": "1", "b": "2", "c": "3"})
		err := WalkArchiveEntries(p, ArchiveTarGz, ArchiveLimits{MaxEntries: 2, MaxTotalSize: 1 << 30}, func(string, io.Reader) error { return nil })
		require.ErrorIs(t, err, ErrTooManyEntries)
	})

	t.Run("max total size exceeded while streaming", func(t *testing.T) {
		p := writeTarGz(t, dir, "big.tar.gz", map[string]string{"a": strings.Repeat("x", 1000)})
		err := WalkArchiveEntries(p, ArchiveTarGz, ArchiveLimits{MaxEntries: 100, MaxTotalSize: 100}, func(_ string, r io.Reader) error {
			_, err := io.ReadAll(r)
			return err
		})
		require.ErrorIs(t, err, ErrArchiveTooLarge)
	})

	t.Run("rejects traversal via tar with .. entries", func(t *testing.T) {
		// tar allows .. in header, so we can test via tar.
		p := filepath.Join(dir, "evil.tar.gz")
		f, err := os.Create(p)
		require.NoError(t, err)
		gw := gzip.NewWriter(f)
		tw := tar.NewWriter(gw)
		require.NoError(t, tw.WriteHeader(&tar.Header{Name: "../escape.txt", Mode: 0o600, Size: 1, Typeflag: tar.TypeReg}))
		_, err = tw.Write([]byte("x"))
		require.NoError(t, err)
		require.NoError(t, tw.Close())
		require.NoError(t, gw.Close())
		require.NoError(t, f.Close())

		err = WalkArchiveEntries(p, ArchiveTarGz, DefaultArchiveLimits(), func(string, io.Reader) error { return nil })
		require.Error(t, err, "entry ../escape.txt must be rejected")
	})
}

func TestSafeArchivePath(t *testing.T) {
	tests := []struct {
		name string
		path string
		want bool
	}{
		{"absolute path", "/etc/passwd", false},
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
			got := safeArchivePath(tc.path)
			assert.Equal(t, tc.want, got)
		})
	}
}

func TestSanitizeMaterialName(t *testing.T) {
	tests := []struct{ in, want string }{
		{"scan.json", "scan-json"},
		{"results.XML", "results-xml"},
		{"weird__name!!", "weird-name"},
		{"___", "material"},
	}
	for _, tc := range tests {
		assert.Equal(t, tc.want, SanitizeMaterialName(tc.in))
	}
}

func TestNameAllocator(t *testing.T) {
	a := NewNameAllocator([]string{"existing"})

	assert.Equal(t, "scan-json", a.Allocate("", "scan.json"))
	assert.Equal(t, "scan-json-1", a.Allocate("", "scan.json")) // collision
	assert.Equal(t, "results-xml", a.Allocate("", "results.xml"))
	assert.Equal(t, "existing-1", a.Allocate("", "existing"))      // collides with pre-existing
	assert.Equal(t, "sboms-a-json", a.Allocate("sboms", "a.json")) // prefix
}

func TestIsArchiveNativeKind(t *testing.T) {
	assert.True(t, IsArchiveNativeKind("ZAP_DAST_ZIP"))
	assert.False(t, IsArchiveNativeKind("SBOM_CYCLONEDX_JSON"))
	assert.False(t, IsArchiveNativeKind("ARTIFACT"))
}
