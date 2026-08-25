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

package service

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/chainloop-dev/chainloop/pkg/servicelogger"
	"github.com/stretchr/testify/require"
)

// TestSweepStagingDir: a boot-time sweep removes leftover upload temp files from
// a previous crash while preserving unrelated files and the directory itself.
func TestSweepStagingDir(t *testing.T) {
	dir := t.TempDir()

	writeFile := func(name string) string {
		p := filepath.Join(dir, name)
		require.NoError(t, os.WriteFile(p, []byte("x"), 0o600))
		return p
	}

	leftover1 := writeFile(stagingFilePrefix + "abc123")
	leftover2 := writeFile(stagingFilePrefix + "def456")
	unrelated := writeFile("some-other-file.txt")

	removed, err := SweepStagingDir(dir, servicelogger.EmptyLogger())
	require.NoError(t, err)
	require.Equal(t, 2, removed, "both leftover upload temp files must be removed")

	require.NoFileExists(t, leftover1)
	require.NoFileExists(t, leftover2)
	require.FileExists(t, unrelated, "unrelated files must be preserved")
	require.DirExists(t, dir, "the staging directory itself must be kept")
}

// TestSweepStagingDirMissing: sweeping a non-existent directory is not an error
// (the volume may not have been populated yet).
func TestSweepStagingDirMissing(t *testing.T) {
	removed, err := SweepStagingDir(filepath.Join(t.TempDir(), "does-not-exist"), servicelogger.EmptyLogger())
	require.NoError(t, err)
	require.Zero(t, removed)
}
