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

package e2e

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/chainloop-dev/chainloop/app/cli/internal/trace/copilot"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseCopilotVersion(t *testing.T) {
	cases := []struct {
		name    string
		out     string
		want    string
		wantErr bool
	}{
		{name: "bare version", out: "1.0.82\n", want: "1.0.82"},
		{name: "named version", out: "GitHub Copilot CLI 1.0.82.\nCommit: abc123\n", want: "1.0.82"},
		{name: "v prefix", out: "copilot v1.1.0\n", want: "1.1.0"},
		{name: "empty", out: "\n", wantErr: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := parseCopilotVersion([]byte(tc.out))
			if tc.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.want, got)
		})
	}
}

func TestCopilotDisableSessionStart(t *testing.T) {
	repo := t.TempDir()
	require.NoError(t, copilot.New().InstallHooks(repo))

	h := &Copilot{}
	require.NoError(t, h.DisableSessionStart(repo))

	data, err := os.ReadFile(filepath.Join(repo, copilotHooksPath))
	require.NoError(t, err)
	var settings struct {
		Hooks map[string]json.RawMessage `json:"hooks"`
	}
	require.NoError(t, json.Unmarshal(data, &settings))
	assert.NotContains(t, settings.Hooks, copilotSessionStartEvent)
	assert.Contains(t, settings.Hooks, "postToolUse")

	// The file stays a Chainloop file: uninstall still deletes it.
	installed, err := copilot.New().HooksInstalled(repo)
	require.NoError(t, err)
	assert.True(t, installed)

	require.Error(t, h.DisableSessionStart(repo), "a second call finds no sessionStart hook")
}

func TestCopilotHookConfigMarkers(t *testing.T) {
	cfg := (&Copilot{}).HookConfig()
	assert.Contains(t, cfg.UserConfig, cfg.UserMarker)
	assert.NotContains(t, cfg.UserConfig, cfg.ChainloopMarker)
	assert.NotEqual(t, cfg.Path, cfg.UserFile())
}
