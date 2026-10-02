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

package opencode

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/chainloop-dev/chainloop/app/cli/internal/trace"
	"github.com/chainloop-dev/chainloop/app/cli/internal/trace/state"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// writeVersionedMockOpenCode creates a fake `opencode` binary that answers
// --version with versionOutput (or fails when versionOutput is empty). Any
// other invocation records its arguments to the returned args file and prints
// exportOutput. Returns the directory to use as PATH.
func writeVersionedMockOpenCode(t *testing.T, versionOutput, exportOutput string) (binDir, argsFile string) {
	t.Helper()
	binDir = t.TempDir()
	argsFile = filepath.Join(t.TempDir(), "args")

	versionBranch := "exit 1"
	if versionOutput != "" {
		versionBranch = "printf '%s\\n' " + quoteForSh(versionOutput) + "; exit 0"
	}
	// Only shell builtins: PATH holds nothing but this directory.
	script := "#!/bin/sh\n" +
		"if [ \"$1\" = \"--version\" ]; then " + versionBranch + "; fi\n" +
		"printf '%s ' \"$@\" > " + quoteForSh(argsFile) + "\n" +
		"printf '%s' " + quoteForSh(exportOutput) + "\n"
	//nolint:gosec // the mock is executed through PATH, so it must be executable
	require.NoError(t, os.WriteFile(filepath.Join(binDir, "opencode"), []byte(script), 0755))

	return binDir, argsFile
}

// TestCopySessionDataExportCommand covers the export command, which OpenCode 2
// moved from `opencode export` to `opencode session export` (PFM-7555).
func TestCopySessionDataExportCommand(t *testing.T) {
	const sessionID = "ses_copy"
	const export = `{"info":{"id":"ses_copy"},"messages":[]}`

	cases := []struct {
		name     string
		version  string
		wantArgs string
	}{
		{"opencode 2", "opencode v2.0.22", "session export ses_copy "},
		{"opencode 1", "1.18.34", "export ses_copy "},
		{"unknown version keeps the original command", "", "export ses_copy "},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			binDir, argsFile := writeVersionedMockOpenCode(t, tc.version, export)
			t.Setenv("PATH", binDir)

			store := state.NewGitStore(t.TempDir())
			require.NoError(t, New().CopySessionData(store, trace.SessionLocation{SessionID: sessionID}))

			args, err := os.ReadFile(argsFile)
			require.NoError(t, err)
			assert.Equal(t, tc.wantArgs, string(args))

			copied, err := os.ReadFile(state.RawSessionPath(store.RawSessionDir(), sessionID))
			require.NoError(t, err)
			assert.JSONEq(t, export, string(copied))
		})
	}
}

func TestMajorVersion(t *testing.T) {
	cases := []struct {
		output string
		want   int
	}{
		{"opencode v2.0.22\n", 2},
		{"2.1.0", 2},
		{"1.18.34\n", 1},
		{"v10.4.1", 10},
		{"0.0.0-dev-202610020321", 0},
		{"not a version", 0},
		{"", 0},
	}

	for _, tc := range cases {
		t.Run(tc.output, func(t *testing.T) {
			assert.Equal(t, tc.want, majorVersion(tc.output))
		})
	}
}
