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
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, codexModelArgs(tc.model))
		})
	}
}
