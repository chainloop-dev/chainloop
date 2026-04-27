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

package crafter

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestResolveGitHubPRHeadSHA(t *testing.T) {
	tests := []struct {
		name      string
		eventName string
		eventJSON string
		wantSHA   string
	}{
		{
			name:      "not a PR event returns empty",
			eventName: "push",
			wantSHA:   "",
		},
		{
			name:      "no event name returns empty",
			eventName: "",
			wantSHA:   "",
		},
		{
			name:      "pull_request event returns head SHA",
			eventName: "pull_request",
			eventJSON: `{"pull_request":{"head":{"sha":"abc123def456"}}}`,
			wantSHA:   "abc123def456",
		},
		{
			name:      "pull_request_target is excluded (checks out base branch)",
			eventName: "pull_request_target",
			eventJSON: `{"pull_request":{"head":{"sha":"deadbeef1234"}}}`,
			wantSHA:   "",
		},
		{
			name:      "malformed JSON returns empty",
			eventName: "pull_request",
			eventJSON: `{invalid`,
			wantSHA:   "",
		},
		{
			name:      "missing head.sha returns empty",
			eventName: "pull_request",
			eventJSON: `{"pull_request":{"number":42}}`,
			wantSHA:   "",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Set env vars for this test
			t.Setenv("GITHUB_EVENT_NAME", tc.eventName)

			if tc.eventJSON != "" {
				eventFile := filepath.Join(t.TempDir(), "event.json")
				err := os.WriteFile(eventFile, []byte(tc.eventJSON), 0o600)
				assert.NoError(t, err)
				t.Setenv("GITHUB_EVENT_PATH", eventFile)
			} else {
				t.Setenv("GITHUB_EVENT_PATH", "")
			}

			got := resolveGitHubPRHeadSHA()
			assert.Equal(t, tc.wantSHA, got)
		})
	}
}
