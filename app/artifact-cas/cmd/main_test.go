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

package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/chainloop-dev/chainloop/app/artifact-cas/internal/conf"
	"github.com/stretchr/testify/require"
)

// TestPrepareStagingDir: the staging directory is a hard startup requirement.
// Every way it can be unusable must fail the boot, because a CAS that starts
// without one reports healthy and then rejects every upload.
func TestPrepareStagingDir(t *testing.T) {
	testCases := []struct {
		name string
		// dir builds the staging_dir value for the case; t.TempDir() gives each
		// case its own scratch space.
		dir       func(t *testing.T) string
		expectErr string
	}{
		{
			name:      "unconfigured is rejected rather than defaulted",
			dir:       func(*testing.T) string { return "" },
			expectErr: "staging_dir is required",
		},
		{
			name: "an existing writable directory is accepted",
			dir:  func(t *testing.T) string { return t.TempDir() },
		},
		{
			name: "a missing directory is created",
			dir: func(t *testing.T) string {
				return filepath.Join(t.TempDir(), "nested", "staging")
			},
		},
		{
			name: "an existing but unwritable directory is rejected",
			dir: func(t *testing.T) string {
				dir := filepath.Join(t.TempDir(), "read-only")
				require.NoError(t, os.Mkdir(dir, 0o500))
				t.Cleanup(func() { _ = os.Chmod(dir, 0o700) })
				return dir
			},
			expectErr: "not writable",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			if os.Geteuid() == 0 && tc.expectErr == "not writable" {
				t.Skip("root ignores directory permissions")
			}

			dir := tc.dir(t)
			err := prepareStagingDir(&conf.Bootstrap{StagingDir: dir})

			if tc.expectErr != "" {
				require.ErrorContains(t, err, tc.expectErr)
				return
			}

			require.NoError(t, err)
			require.DirExists(t, dir)

			// The writability probe must not survive the check.
			entries, err := os.ReadDir(dir)
			require.NoError(t, err)
			require.Empty(t, entries, "the probe file must be removed")
		})
	}
}
