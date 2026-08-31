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

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestPluginGoldenFiles compares the generated plugin output against the
// committed golden files in testdata/. Run with UPDATE_GOLDEN=1 to
// regenerate them after an intentional template change:
//
//	UPDATE_GOLDEN=1 go test ./app/cli/internal/trace/opencode/ -run TestPluginGoldenFiles
func TestPluginGoldenFiles(t *testing.T) {
	p := New()

	cases := []struct {
		name    string
		install func(string) error
		golden  string
	}{
		{"full", p.InstallHooks, "testdata/plugin_full.ts"},
		{"tracerun", p.InstallHooksForTraceRun, "testdata/plugin_tracerun.ts"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			repoRoot := t.TempDir()
			require.NoError(t, tc.install(repoRoot))

			generated, err := os.ReadFile(filepath.Join(repoRoot, settingsFile))
			require.NoError(t, err)

			if os.Getenv("UPDATE_GOLDEN") == "1" {
				require.NoError(t, os.WriteFile(tc.golden, generated, 0600))
				return
			}

			expected, err := os.ReadFile(tc.golden)
			require.NoError(t, err, "golden file missing; run UPDATE_GOLDEN=1 to generate")
			assert.Equal(t, string(expected), string(generated),
				"generated plugin does not match golden file %s; run UPDATE_GOLDEN=1 to update", tc.golden)
		})
	}
}
