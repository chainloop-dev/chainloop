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

package cmd

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAttestationInitLoadsProjectFromRepositoryConfig(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, ".chainloop.yml"), []byte("projectName: web\nprojectVersion: 1.2.3\n"), 0o600))
	t.Chdir(dir)

	cmd := newAttestationInitCmd()
	require.NoError(t, cmd.Flags().Set("workflow", "build"))
	require.NoError(t, cmd.PreRunE(cmd, nil))

	project, err := cmd.Flags().GetString("project")
	require.NoError(t, err)
	version, err := cmd.Flags().GetString("version")
	require.NoError(t, err)
	assert.Equal(t, "web", project)
	assert.Equal(t, "1.2.3", version)
}

func TestAttestationInitRepositoryConfigPrecedence(t *testing.T) {
	for _, tc := range []struct {
		name        string
		project     string
		version     string
		latest      bool
		config      string
		wantProject string
		wantVersion string
	}{
		{
			name:        "project flag wins while version comes from config",
			project:     "flag-project",
			config:      "projectName: file-project\nprojectVersion: file-version\n",
			wantProject: "flag-project",
			wantVersion: "file-version",
		},
		{
			name:        "version flag wins while project comes from config",
			version:     "flag-version",
			config:      "projectName: file-project\nprojectVersion: file-version\n",
			wantProject: "file-project",
			wantVersion: "flag-version",
		},
		{
			name:        "latest version suppresses config version",
			latest:      true,
			config:      "projectName: file-project\nprojectVersion: file-version\n",
			wantProject: "file-project",
		},
		{
			name:        "resolved flags do not read malformed config",
			project:     "flag-project",
			version:     "flag-version",
			config:      ":\ninvalid: [yaml\n",
			wantProject: "flag-project",
			wantVersion: "flag-version",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			require.NoError(t, os.WriteFile(filepath.Join(dir, ".chainloop.yml"), []byte(tc.config), 0o600))
			t.Chdir(dir)

			cmd := newAttestationInitCmd()
			require.NoError(t, cmd.Flags().Set("workflow", "build"))
			if tc.project != "" {
				require.NoError(t, cmd.Flags().Set("project", tc.project))
			}
			if tc.version != "" {
				require.NoError(t, cmd.Flags().Set("version", tc.version))
			}
			if tc.latest {
				require.NoError(t, cmd.Flags().Set("latest-version", "true"))
			}

			require.NoError(t, cmd.PreRunE(cmd, nil))
			project, err := cmd.Flags().GetString("project")
			require.NoError(t, err)
			version, err := cmd.Flags().GetString("version")
			require.NoError(t, err)
			assert.Equal(t, tc.wantProject, project)
			assert.Equal(t, tc.wantVersion, version)
		})
	}
}

func TestAttestationInitRequiresProjectFromFlagOrRepositoryConfig(t *testing.T) {
	for _, tc := range []struct {
		name   string
		config string
	}{
		{name: "missing config"},
		{name: "config without project", config: "projectVersion: 1.2.3\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			if tc.config != "" {
				require.NoError(t, os.WriteFile(filepath.Join(dir, ".chainloop.yml"), []byte(tc.config), 0o600))
			}
			t.Chdir(dir)

			cmd := newAttestationInitCmd()
			require.NoError(t, cmd.Flags().Set("workflow", "build"))

			assert.EqualError(t, cmd.PreRunE(cmd, nil), "project is required, set it via --project or projectName in .chainloop.yml")
		})
	}
}
