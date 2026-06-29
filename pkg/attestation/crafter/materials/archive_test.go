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
	"os"
	"path/filepath"
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

func TestDetectArchive(t *testing.T) {
	dir := t.TempDir()
	zipPath := writeZip(t, dir, "a.zip", map[string]string{"x.txt": "hi"})
	tgzPath := writeTarGz(t, dir, "a.tar.gz", map[string]string{"x.txt": "hi"})

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
