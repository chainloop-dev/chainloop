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
	"archive/zip"
	"os"
	"path/filepath"
	"testing"

	"github.com/chainloop-dev/chainloop/pkg/attestation/crafter/materials"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// writeTestZip creates a zip archive at dir/name containing a single file
// "entry.txt" and returns its path.
func writeTestZip(t *testing.T, dir, name string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	f, err := os.Create(path)
	require.NoError(t, err)
	defer f.Close()

	w := zip.NewWriter(f)
	entry, err := w.Create("entry.txt")
	require.NoError(t, err)
	_, err = entry.Write([]byte("hello"))
	require.NoError(t, err)
	require.NoError(t, w.Close())
	return path
}

func TestShouldExplode(t *testing.T) {
	dir := t.TempDir()
	zipPath := writeTestZip(t, dir, "s.zip")

	// non-archive: a plain temp file with an unrecognised extension
	plainPath := filepath.Join(dir, "plain.bin")
	require.NoError(t, os.WriteFile(plainPath, []byte("not an archive"), 0600))

	tests := []struct {
		name       string
		kind       string
		value      string
		wantFormat materials.ArchiveFormat
	}{
		// A non-ArchiveNone format means the value will be exploded. Only
		// explodable kinds (SBOM, SARIF) explode; everything else is recorded
		// whole even when the value is an archive.
		{"explodable SBOM + archive", "SBOM_CYCLONEDX_JSON", zipPath, materials.ArchiveZip},
		{"explodable SARIF + archive", "SARIF", zipPath, materials.ArchiveZip},
		{"non-explodable ARTIFACT + archive", "ARTIFACT", zipPath, materials.ArchiveNone},
		{"non-explodable EVIDENCE + archive", "EVIDENCE", zipPath, materials.ArchiveNone},
		{"archive-native ZAP + archive", "ZAP_DAST_ZIP", zipPath, materials.ArchiveNone},
		{"no kind", "", zipPath, materials.ArchiveNone},
		{"explodable kind + non-archive", "SBOM_CYCLONEDX_JSON", plainPath, materials.ArchiveNone},
		// Non-file values must never return an error — even for an explodable kind
		// the value here is not a file path at all.
		{"explodable kind STRING-like non-file value", "SARIF", "hello world", materials.ArchiveNone},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			format, err := shouldExplode(tc.kind, tc.value)
			require.NoError(t, err)
			assert.Equal(t, tc.wantFormat, format)
		})
	}
}
