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
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestColorSupported(t *testing.T) {
	testCases := []struct {
		name       string
		env        map[string]string
		isTerminal bool
		want       bool
	}{
		{
			name:       "interactive terminal gets color",
			isTerminal: true,
			want:       true,
		},
		{
			name:       "plain non-terminal (pipe/file) gets no color",
			isTerminal: false,
			want:       false,
		},
		{
			name:       "NO_COLOR disables color even on a terminal",
			env:        map[string]string{"NO_COLOR": "1"},
			isTerminal: true,
			want:       false,
		},
		{
			name:       "NO_COLOR with empty value still disables (presence wins)",
			env:        map[string]string{"NO_COLOR": ""},
			isTerminal: true,
			want:       false,
		},
		{
			name:       "CLICOLOR_FORCE forces color on a non-terminal",
			env:        map[string]string{"CLICOLOR_FORCE": "1"},
			isTerminal: false,
			want:       true,
		},
		{
			name:       "FORCE_COLOR forces color on a non-terminal",
			env:        map[string]string{"FORCE_COLOR": "1"},
			isTerminal: false,
			want:       true,
		},
		{
			name:       "CLICOLOR_FORCE=0 does not force color",
			env:        map[string]string{"CLICOLOR_FORCE": "0"},
			isTerminal: false,
			want:       false,
		},
		{
			name:       "NO_COLOR takes precedence over FORCE_COLOR",
			env:        map[string]string{"NO_COLOR": "1", "FORCE_COLOR": "1"},
			isTerminal: true,
			want:       false,
		},
		{
			name:       "TERM=dumb disables color on a terminal",
			env:        map[string]string{"TERM": "dumb"},
			isTerminal: true,
			want:       false,
		},
		// CI systems that render ANSI keep color despite no terminal.
		{
			name:       "GitHub Actions gets color",
			env:        map[string]string{"GITHUB_ACTIONS": "true", "CI": "true"},
			isTerminal: false,
			want:       true,
		},
		{
			name:       "GitLab CI gets color",
			env:        map[string]string{"GITLAB_CI": "true", "CI": "true"},
			isTerminal: false,
			want:       true,
		},
		// Jenkins is not in the color-capable list, so a non-terminal Jenkins
		// console gets no color and avoids leaking raw escape codes.
		{
			name:       "Jenkins gets no color",
			env:        map[string]string{"JENKINS_URL": "http://jenkins.local/", "BUILD_NUMBER": "10"},
			isTerminal: false,
			want:       false,
		},
		{
			name:       "Jenkins with FORCE_COLOR opts back in",
			env:        map[string]string{"JENKINS_URL": "http://jenkins.local/", "FORCE_COLOR": "1"},
			isTerminal: false,
			want:       true,
		},
		{
			name:       "generic CI not known to render ANSI gets no color",
			env:        map[string]string{"CI": "true"},
			isTerminal: false,
			want:       false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			lookup := func(key string) (string, bool) {
				v, ok := tc.env[key]
				return v, ok
			}
			assert.Equal(t, tc.want, colorSupported(lookup, tc.isTerminal))
		})
	}
}
