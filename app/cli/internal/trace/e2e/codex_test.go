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

	"github.com/pelletier/go-toml/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseCodexVersion(t *testing.T) {
	cases := []struct {
		name    string
		out     string
		want    string
		wantErr bool
	}{
		{name: "release", out: "codex-cli 0.163.0\n", want: "0.163.0"},
		{name: "no version", out: "codex-cli\n", wantErr: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := parseCodexVersion([]byte(tc.out))
			if tc.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.want, got)
		})
	}
}

func TestCodexModelArgs(t *testing.T) {
	cases := []struct {
		name  string
		model string
		want  []string
	}{
		{
			name:  "default provider",
			model: "gpt-5.5-mini",
			want:  []string{flagModel, "gpt-5.5-mini"},
		},
		{
			name:  "openrouter",
			model: "openrouter/openai/gpt-5.5-mini",
			want: []string{
				flagModel, "openai/gpt-5.5-mini",
				"-c", "model_provider=openrouter",
				"-c", codexOpenRouterProvider,
			},
		},
		{
			name:  "orcarouter",
			model: "orcarouter:orcarouter/auto",
			want: []string{
				flagModel, "orcarouter/auto",
				"-c", "model_provider=orcarouter",
				"-c", codexOrcaRouterProvider,
			},
		},
		{
			name:  "orcarouter cyber model",
			model: "orcarouter:orca/orcacyber-ultra-1.0",
			want: []string{
				flagModel, "orca/orcacyber-ultra-1.0",
				"-c", "model_provider=orcarouter",
				"-c", codexOrcaRouterProvider,
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, codexModelArgs(tc.model))
		})
	}
}

func TestCodexOrcaRouterProviderDefinition(t *testing.T) {
	// A named provider, not a generic base URL override: it keeps the
	// OrcaRouter name, the documented inference base URL, and the environment
	// variable the CLI's `orcarouter` commands use.
	const want = `model_providers.orcarouter={name="OrcaRouter Cyber Inference",base_url="https://api.orcarouter.ai/v1",env_key="ORCAROUTER_API_KEY"}`
	assert.Equal(t, want, codexOrcaRouterProvider)
	assert.Contains(t, codexOrcaRouterProvider, "orcarouter")
	assert.NotContains(t, codexOrcaRouterProvider, "openrouter.ai")
}

func TestCodexStreamErrors(t *testing.T) {
	raw := []byte("{\"type\":\"thread.started\"}\n{\"type\":\"error\",\"message\":\"missing key\"}\n{\"type\":\"turn.failed\",\"error\":{\"message\":\"turn failed\"}}\n")
	assert.Equal(t, "missing key\nturn failed", codexStreamErrors(raw))
}

func TestCodexIsolatedConfig(t *testing.T) {
	home := t.TempDir()
	for _, dir := range []string{".agents/skills/personal", ".codex/skills/legacy", ".codex/skills/.system/builtin"} {
		path := filepath.Join(home, dir, "SKILL.md")
		require.NoError(t, os.MkdirAll(filepath.Dir(path), 0700))
		require.NoError(t, os.WriteFile(path, []byte("personal instructions"), 0600))
	}
	raw, err := codexIsolatedConfig(home)
	require.NoError(t, err)
	var config struct {
		Skills struct {
			Config []struct {
				Path    string
				Enabled bool
			}
			Bundled struct{ Enabled bool }
		}
	}
	require.NoError(t, toml.Unmarshal(raw, &config))
	require.Len(t, config.Skills.Config, 3)
	for _, skill := range config.Skills.Config {
		assert.False(t, skill.Enabled)
		assert.Contains(t, skill.Path, home)
	}
	assert.False(t, config.Skills.Bundled.Enabled)
}

func TestCodexModelCatalog(t *testing.T) {
	raw := []byte(`{"models":[{"slug":"gpt-6-luna","apply_patch_tool_type":"freeform","base_instructions":"native instructions"}]}`)
	catalog, err := codexModelCatalog(raw, "openrouter/openai/gpt-6-luna")
	require.NoError(t, err)
	var got struct {
		Models []map[string]any `json:"models"`
	}
	require.NoError(t, json.Unmarshal(catalog, &got))
	require.Len(t, got.Models, 2)
	assert.Equal(t, "openai/gpt-6-luna", got.Models[1]["slug"])
	assert.Equal(t, "freeform", got.Models[1]["apply_patch_tool_type"])
	assert.Equal(t, "native instructions", got.Models[1]["base_instructions"])
	unknown, err := codexModelCatalog(raw, "openrouter/openai/unknown")
	require.NoError(t, err)
	assert.Nil(t, unknown)
}
