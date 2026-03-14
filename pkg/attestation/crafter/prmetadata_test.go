//
// Copyright 2024-2026 The Chainloop Authors.
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
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/chainloop-dev/chainloop/internal/prinfo"
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
			name: "valid pull_request event with reviewers",
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
				require.Len(t, metadata.Reviewers, 2)
				assert.Equal(t, "reviewer1", metadata.Reviewers[0].Login)
				assert.Equal(t, "User", metadata.Reviewers[0].Type)
				assert.Equal(t, "coderabbitai", metadata.Reviewers[1].Login)
				assert.Equal(t, "Bot", metadata.Reviewers[1].Type)
			},
		},
		{
			name: "valid pull_request event without reviewers",
			envVars: map[string]string{
				"GITHUB_EVENT_NAME": "pull_request",
				"GITHUB_EVENT_PATH": filepath.Join("testdata", "github_pr_event_no_reviewers.json"),
				"GITHUB_HEAD_REF":   "fix-branch",
				"GITHUB_BASE_REF":   "main",
			},
			expectPR:    true,
			expectError: false,
			validate: func(t *testing.T, metadata *PRMetadata) {
				assert.Equal(t, "github", metadata.Platform)
				assert.Equal(t, "456", metadata.Number)
				assert.Equal(t, "janedoe", metadata.Author)
				assert.Empty(t, metadata.Reviewers)
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
				// No reviewers without API access in tests
				assert.Empty(t, metadata.Reviewers)
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
			isMR, metadata, err := extractGitLabMRMetadata(context.Background(), tc.envVars)

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

func TestFetchGitLabReviewers(t *testing.T) {
	testCases := []struct {
		name        string
		handler     http.HandlerFunc
		baseURL     string // override if empty, use server URL
		projectPath string
		mrIID       string
		token       string
		expected    []prinfo.Reviewer
	}{
		{
			name: "successful response with reviewers",
			handler: func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, "test-token", r.Header.Get("JOB-TOKEN"))
				w.Header().Set("Content-Type", "application/json")
				fmt.Fprint(w, `{"reviewers": [{"username": "alice"}, {"username": "bot-reviewer"}]}`)
			},
			projectPath: "group/project",
			mrIID:       "10",
			token:       "test-token",
			expected: []prinfo.Reviewer{
				{Login: "alice", Type: "unknown"},
				{Login: "bot-reviewer", Type: "unknown"},
			},
		},
		{
			name: "empty reviewers",
			handler: func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				fmt.Fprint(w, `{"reviewers": []}`)
			},
			projectPath: "group/project",
			mrIID:       "10",
			token:       "test-token",
			expected:    nil,
		},
		{
			name: "API returns error",
			handler: func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusForbidden)
			},
			projectPath: "group/project",
			mrIID:       "10",
			token:       "test-token",
			expected:    nil,
		},
		{
			name:        "missing token",
			handler:     nil,
			projectPath: "group/project",
			mrIID:       "10",
			token:       "",
			expected:    nil,
		},
		{
			name:        "missing base URL",
			handler:     nil,
			baseURL:     "",
			projectPath: "group/project",
			mrIID:       "10",
			token:       "test-token",
			expected:    nil,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var serverURL string
			if tc.handler != nil {
				server := httptest.NewServer(tc.handler)
				defer server.Close()
				serverURL = server.URL
			}

			baseURL := tc.baseURL
			if baseURL == "" && tc.handler != nil {
				baseURL = serverURL
			}

			reviewers := fetchGitLabReviewers(context.Background(), baseURL, tc.projectPath, tc.mrIID, tc.token)

			if tc.expected == nil {
				assert.Nil(t, reviewers)
			} else {
				assert.Equal(t, tc.expected, reviewers)
			}
		})
	}
}

func TestFetchGitLabReviewersRequestPath(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/v4/projects/group%2Fproject/merge_requests/42", r.URL.RawPath)
		w.Header().Set("Content-Type", "application/json")
		require.NoError(t, json.NewEncoder(w).Encode(map[string]any{"reviewers": []any{}}))
	}))
	defer server.Close()

	fetchGitLabReviewers(context.Background(), server.URL, "group/project", "42", "token")
}

func TestExtractGitLabMRMetadataWithReviewers(t *testing.T) {
	// Set up a mock GitLab API that returns reviewers
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"reviewers": [{"username": "alice"}, {"username": "coderabbitai"}]}`)
	}))
	defer server.Close()

	// Set env vars that extractGitLabMRMetadata reads via os.Getenv for the API call
	t.Setenv("CI_SERVER_URL", server.URL)
	t.Setenv("CI_MERGE_REQUEST_PROJECT_PATH", "group/project")
	t.Setenv("CI_JOB_TOKEN", "test-token")

	envVars := map[string]string{
		"CI_PIPELINE_SOURCE":                  "merge_request_event",
		"CI_MERGE_REQUEST_IID":                "10",
		"CI_MERGE_REQUEST_TITLE":              "MR with reviewers",
		"CI_MERGE_REQUEST_PROJECT_URL":        "https://gitlab.com/group/project",
		"GITLAB_USER_LOGIN":                   "author",
		"CI_MERGE_REQUEST_SOURCE_BRANCH_NAME": "feature",
		"CI_MERGE_REQUEST_TARGET_BRANCH_NAME": "main",
	}

	isMR, metadata, err := extractGitLabMRMetadata(context.Background(), envVars)
	require.NoError(t, err)
	require.True(t, isMR)
	require.Len(t, metadata.Reviewers, 2)
	assert.Equal(t, "alice", metadata.Reviewers[0].Login)
	assert.Equal(t, "unknown", metadata.Reviewers[0].Type)
	assert.Equal(t, "coderabbitai", metadata.Reviewers[1].Login)
}
