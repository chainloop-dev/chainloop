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
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadConfig(t *testing.T) {
	const valid = `
harnesses:
  Claude Code:
    version: "2.1.0"
    models: [haiku]
modelDriven: {runs: 3, threshold: 2}
features:
  Skills tracking: {runs: 5, threshold: 4}
scenarioTimeout: 2m
parallel: 2
`
	testCases := []struct {
		name    string
		data    string
		wantErr string
	}{
		{name: "valid", data: valid},
		{name: "unknown field", data: valid + "extra: 1\n", wantErr: "unknown field"},
		{
			name:    "harness with no model",
			data:    "harnesses: {Cursor: {version: '1'}}\nmodelDriven: {runs: 1, threshold: 1}\nscenarioTimeout: 1m\n",
			wantErr: `harness "Cursor" needs a version and at least one model`,
		},
		{
			name:    "threshold above runs",
			data:    "harnesses: {Cursor: {version: '1', models: [a]}}\nmodelDriven: {runs: 1, threshold: 2}\nscenarioTimeout: 1m\n",
			wantErr: "modelDriven",
		},
		{
			name:    "missing timeout",
			data:    "harnesses: {Cursor: {version: '1', models: [a]}}\nmodelDriven: {runs: 1, threshold: 1}\n",
			wantErr: "scenarioTimeout",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			cfg, err := LoadConfig([]byte(tc.data))
			if tc.wantErr != "" {
				assert.ErrorContains(t, err, tc.wantErr)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, 2*time.Minute, time.Duration(cfg.ScenarioTimeout))
			assert.Equal(t, Repeat{Runs: 5, Threshold: 4}, cfg.repeatFor("Skills tracking", true))
			assert.Equal(t, Repeat{Runs: 3, Threshold: 2}, cfg.repeatFor("Spec capture instruction", true))
			assert.Equal(t, Repeat{Runs: 1, Threshold: 1}, cfg.repeatFor("Transcript", false))
		})
	}
}

func TestEmbeddedConfig(t *testing.T) {
	cfg, err := LoadConfig(nil)
	require.NoError(t, err)
	assert.Contains(t, cfg.Harnesses, HarnessClaudeCode)
}
