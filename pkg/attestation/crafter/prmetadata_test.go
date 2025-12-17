//
// Copyright 2024-2025 The Chainloop Authors.
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
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExtractGitHubPRMetadata(t *testing.T) {
	testCases := []struct {
		name        string
		envVars     map[string]string
		expectPR    bool
		expectError bool
		validate    func(t *testing.T, metadata *PRMetadata)
	}{
		{
			name: "valid pull_request event",
			envVars: map[string]string{
				"GITHUB_EVENT_NAME": "pull_request",
				"GITHUB_EVENT_PATH": filepath.Join("testdata", "github_pr_event.json"),
				"GITHUB_HEAD_REF":   "feature-branch",
				"GITHUB_BASE_REF":   "main",
			},
			expectPR:    true,
			expectError: false,
			validate: func(t *testing.T, metadata *PRMetadata) {
				assert.Equal(t, "github", metadata.Platform)
				assert.Equal(t, "pull_request", metadata.Type)
				assert.Equal(t, "123", metadata.Number)
				assert.Equal(t, "Add new feature", metadata.Title)
				assert.Contains(t, metadata.Description, "This PR adds a new feature")
				assert.Equal(t, "feature-branch", metadata.SourceBranch)
				assert.Equal(t, "main", metadata.TargetBranch)
				assert.Equal(t, "https://github.com/owner/repo/pull/123", metadata.URL)
				assert.Equal(t, "johndoe", metadata.Author)
			},
		},
		{
			name: "pull_request_target event",
			envVars: map[string]string{
				"GITHUB_EVENT_NAME": "pull_request_target",
				"GITHUB_EVENT_PATH": filepath.Join("testdata", "github_pr_event.json"),
				"GITHUB_HEAD_REF":   "feature-branch",
				"GITHUB_BASE_REF":   "main",
			},
			expectPR:    true,
			expectError: false,
		},
		{
			name: "not a PR event",
			envVars: map[string]string{
				"GITHUB_EVENT_NAME": "push",
			},
			expectPR:    false,
			expectError: false,
		},
		{
			name: "missing event path",
			envVars: map[string]string{
				"GITHUB_EVENT_NAME": "pull_request",
			},
			expectPR:    false,
			expectError: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			isPR, metadata, err := extractGitHubPRMetadata(tc.envVars)

			if tc.expectError {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tc.expectPR, isPR)

			if tc.expectPR && tc.validate != nil {
				require.NotNil(t, metadata)
				tc.validate(t, metadata)
			}
		})
	}
}

func TestExtractGitLabMRMetadata(t *testing.T) {
	testCases := []struct {
		name        string
		envVars     map[string]string
		expectMR    bool
		expectError bool
		validate    func(t *testing.T, metadata *PRMetadata)
	}{
		{
			name: "valid merge_request event",
			envVars: map[string]string{
				"CI_PIPELINE_SOURCE":                  "merge_request_event",
				"CI_MERGE_REQUEST_IID":                "42",
				"CI_MERGE_REQUEST_TITLE":              "Test MR",
				"CI_MERGE_REQUEST_DESCRIPTION":        "This is a test MR",
				"CI_MERGE_REQUEST_SOURCE_BRANCH_NAME": "feature-branch",
				"CI_MERGE_REQUEST_TARGET_BRANCH_NAME": "main",
				"CI_MERGE_REQUEST_PROJECT_URL":        "https://gitlab.com/owner/repo",
				"GITLAB_USER_LOGIN":                   "testuser",
			},
			expectMR:    true,
			expectError: false,
			validate: func(t *testing.T, metadata *PRMetadata) {
				assert.Equal(t, "gitlab", metadata.Platform)
				assert.Equal(t, "merge_request", metadata.Type)
				assert.Equal(t, "42", metadata.Number)
				assert.Equal(t, "Test MR", metadata.Title)
				assert.Equal(t, "This is a test MR", metadata.Description)
				assert.Equal(t, "feature-branch", metadata.SourceBranch)
				assert.Equal(t, "main", metadata.TargetBranch)
				assert.Equal(t, "https://gitlab.com/owner/repo/-/merge_requests/42", metadata.URL)
				assert.Equal(t, "testuser", metadata.Author)
			},
		},
		{
			name: "not a merge request event",
			envVars: map[string]string{
				"CI_PIPELINE_SOURCE": "push",
			},
			expectMR:    false,
			expectError: false,
		},
		{
			name: "missing MR IID",
			envVars: map[string]string{
				"CI_PIPELINE_SOURCE": "merge_request_event",
			},
			expectMR:    false,
			expectError: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			isMR, metadata, err := extractGitLabMRMetadata(tc.envVars)

			if tc.expectError {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tc.expectMR, isMR)

			if tc.expectMR && tc.validate != nil {
				require.NotNil(t, metadata)
				tc.validate(t, metadata)
			}
		})
	}
}
