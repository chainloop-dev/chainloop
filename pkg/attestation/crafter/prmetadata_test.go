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
			name: "valid pull_request event with reviewers from event file",
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
				// Event file provides requested_reviewers without needing a token.
				require.Len(t, metadata.Reviewers, 2)
				assert.Equal(t, "reviewer1", metadata.Reviewers[0].Login)
				assert.Equal(t, "User", metadata.Reviewers[0].Type)
				assert.True(t, metadata.Reviewers[0].Requested)
				assert.Equal(t, "coderabbitai", metadata.Reviewers[1].Login)
				assert.Equal(t, "Bot", metadata.Reviewers[1].Type)
				assert.True(t, metadata.Reviewers[1].Requested)
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
			isPR, metadata, err := extractGitHubPRMetadata(context.Background(), tc.envVars)

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
				{Login: "alice", Type: "unknown", Requested: true},
				{Login: "bot-reviewer", Type: "unknown", Requested: true},
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

	envVars := map[string]string{
		"CI_PIPELINE_SOURCE":                  "merge_request_event",
		"CI_MERGE_REQUEST_IID":                "10",
		"CI_MERGE_REQUEST_TITLE":              "MR with reviewers",
		"CI_MERGE_REQUEST_PROJECT_URL":        "https://gitlab.com/group/project",
		"GITLAB_USER_LOGIN":                   "author",
		"CI_MERGE_REQUEST_SOURCE_BRANCH_NAME": "feature",
		"CI_MERGE_REQUEST_TARGET_BRANCH_NAME": "main",
		"CI_SERVER_URL":                       server.URL,
		"CI_MERGE_REQUEST_PROJECT_PATH":       "group/project",
	}

	// CI_JOB_TOKEN is read via os.Getenv (not from envVars) to avoid persisting it in attestations.
	t.Setenv("CI_JOB_TOKEN", "test-token")

	isMR, metadata, err := extractGitLabMRMetadata(context.Background(), envVars)
	require.NoError(t, err)
	require.True(t, isMR)
	require.Len(t, metadata.Reviewers, 2)
	assert.Equal(t, "alice", metadata.Reviewers[0].Login)
	assert.Equal(t, "unknown", metadata.Reviewers[0].Type)
	assert.True(t, metadata.Reviewers[0].Requested)
	assert.Equal(t, "coderabbitai", metadata.Reviewers[1].Login)
	assert.True(t, metadata.Reviewers[1].Requested)
}

func TestFetchGitHubReviews(t *testing.T) {
	testCases := []struct {
		name     string
		handler  http.HandlerFunc
		owner    string
		repo     string
		prNumber string
		token    string
		expected []prinfo.Reviewer
	}{
		{
			name: "successful response, last state wins for duplicate user",
			handler: func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, "Bearer test-token", r.Header.Get("Authorization"))
				w.Header().Set("Content-Type", "application/json")
				// reviewer1 submits two reviews with different states; last state wins
				fmt.Fprint(w, `[{"user":{"login":"reviewer1","type":"User"},"state":"COMMENTED"},{"user":{"login":"reviewer1","type":"User"},"state":"APPROVED"},{"user":{"login":"bot-reviewer","type":"Bot"},"state":"COMMENTED"}]`)
			},
			owner:    "owner",
			repo:     "repo",
			prNumber: "42",
			token:    "test-token",
			expected: []prinfo.Reviewer{
				{Login: "reviewer1", Type: "User", ReviewStatus: "APPROVED"},
				{Login: "bot-reviewer", Type: "Bot", ReviewStatus: "COMMENTED"},
			},
		},
		{
			name: "empty reviews array",
			handler: func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				fmt.Fprint(w, `[]`)
			},
			owner:    "owner",
			repo:     "repo",
			prNumber: "42",
			token:    "test-token",
			expected: nil,
		},
		{
			name: "API returns error",
			handler: func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusForbidden)
			},
			owner:    "owner",
			repo:     "repo",
			prNumber: "42",
			token:    "test-token",
			expected: nil,
		},
		{
			name:     "missing token",
			handler:  nil,
			owner:    "owner",
			repo:     "repo",
			prNumber: "42",
			token:    "",
			expected: nil,
		},
		{
			name:     "missing owner",
			handler:  nil,
			owner:    "",
			repo:     "repo",
			prNumber: "42",
			token:    "test-token",
			expected: nil,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var baseURL string
			if tc.handler != nil {
				server := httptest.NewServer(tc.handler)
				defer server.Close()
				baseURL = server.URL
			}

			reviewers := fetchGitHubReviews(context.Background(), baseURL, tc.owner, tc.repo, tc.prNumber, tc.token)

			if tc.expected == nil {
				assert.Nil(t, reviewers)
			} else {
				assert.Equal(t, tc.expected, reviewers)
			}
		})
	}
}

func TestFetchGitHubReviewsRequestPath(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/repos/owner/repo/pulls/42/reviews", r.URL.Path)
		assert.Equal(t, "Bearer test-token", r.Header.Get("Authorization"))
		w.Header().Set("Content-Type", "application/json")
		require.NoError(t, json.NewEncoder(w).Encode([]any{}))
	}))
	defer server.Close()

	fetchGitHubReviews(context.Background(), server.URL, "owner", "repo", "42", "test-token")
}

func TestFetchGitHubReviewsPagination(t *testing.T) {
	page := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		page++
		switch page {
		case 1:
			// Set Link header pointing to page 2
			w.Header().Set("Link", fmt.Sprintf(`<%s/repos/owner/repo/pulls/42/reviews?per_page=100&page=2>; rel="next"`, r.Host))
			fmt.Fprint(w, `[{"user":{"login":"reviewer1","type":"User"},"state":"COMMENTED"}]`)
		case 2:
			// Last page — no Link header
			fmt.Fprint(w, `[{"user":{"login":"reviewer2","type":"User"},"state":"APPROVED"},{"user":{"login":"reviewer1","type":"User"},"state":"APPROVED"}]`)
		}
	}))
	defer server.Close()

	// Fix up the Link header host to use the test server URL
	page = 0
	server2 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		page++
		switch page {
		case 1:
			w.Header().Set("Link", fmt.Sprintf(`<%s/repos/owner/repo/pulls/42/reviews?per_page=100&page=2>; rel="next"`, server.URL))
			fmt.Fprint(w, `[{"user":{"login":"reviewer1","type":"User"},"state":"COMMENTED"}]`)
		case 2:
			fmt.Fprint(w, `[{"user":{"login":"reviewer2","type":"User"},"state":"APPROVED"},{"user":{"login":"reviewer1","type":"User"},"state":"APPROVED"}]`)
		}
	}))
	defer server2.Close()

	reviewers := fetchGitHubReviews(context.Background(), server2.URL, "owner", "repo", "42", "test-token")
	require.Len(t, reviewers, 2)
	// reviewer1 appears on both pages; last state (APPROVED from page 2) wins
	assert.Equal(t, "reviewer1", reviewers[0].Login)
	assert.Equal(t, "APPROVED", reviewers[0].ReviewStatus)
	assert.Equal(t, "reviewer2", reviewers[1].Login)
	assert.Equal(t, "APPROVED", reviewers[1].ReviewStatus)
}

func TestNextPageURL(t *testing.T) {
	testCases := []struct {
		name     string
		header   string
		expected string
	}{
		{
			name:     "next page present",
			header:   `<https://api.github.com/repos/o/r/pulls/1/reviews?per_page=100&page=2>; rel="next", <https://api.github.com/repos/o/r/pulls/1/reviews?per_page=100&page=5>; rel="last"`,
			expected: "https://api.github.com/repos/o/r/pulls/1/reviews?per_page=100&page=2",
		},
		{
			name:     "no next page",
			header:   `<https://api.github.com/repos/o/r/pulls/1/reviews?per_page=100&page=1>; rel="first"`,
			expected: "",
		},
		{
			name:     "empty header",
			header:   "",
			expected: "",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.expected, nextPageURL(tc.header))
		})
	}
}

func TestFetchGitHubRequestedReviewers(t *testing.T) {
	testCases := []struct {
		name     string
		handler  http.HandlerFunc
		owner    string
		repo     string
		prNumber string
		token    string
		expected []prinfo.Reviewer
	}{
		{
			name: "successful response with users",
			handler: func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, "Bearer test-token", r.Header.Get("Authorization"))
				w.Header().Set("Content-Type", "application/json")
				fmt.Fprint(w, `{"users":[{"login":"reviewer1","type":"User"},{"login":"copilot-pull-request-reviewer[bot]","type":"Bot"}],"teams":[]}`)
			},
			owner:    "owner",
			repo:     "repo",
			prNumber: "42",
			token:    "test-token",
			expected: []prinfo.Reviewer{
				{Login: "reviewer1", Type: "User", Requested: true},
				{Login: "copilot-pull-request-reviewer[bot]", Type: "Bot", Requested: true},
			},
		},
		{
			name: "empty users array",
			handler: func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				fmt.Fprint(w, `{"users":[],"teams":[]}`)
			},
			owner:    "owner",
			repo:     "repo",
			prNumber: "42",
			token:    "test-token",
			expected: nil,
		},
		{
			name: "API returns error",
			handler: func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusForbidden)
			},
			owner:    "owner",
			repo:     "repo",
			prNumber: "42",
			token:    "test-token",
			expected: nil,
		},
		{
			name:     "missing token",
			handler:  nil,
			owner:    "owner",
			repo:     "repo",
			prNumber: "42",
			token:    "",
			expected: nil,
		},
		{
			name:     "missing owner",
			handler:  nil,
			owner:    "",
			repo:     "repo",
			prNumber: "42",
			token:    "test-token",
			expected: nil,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var baseURL string
			if tc.handler != nil {
				server := httptest.NewServer(tc.handler)
				defer server.Close()
				baseURL = server.URL
			}

			reviewers := fetchGitHubRequestedReviewers(context.Background(), baseURL, tc.owner, tc.repo, tc.prNumber, tc.token)

			if tc.expected == nil {
				assert.Nil(t, reviewers)
			} else {
				assert.Equal(t, tc.expected, reviewers)
			}
		})
	}
}

func TestFetchGitHubRequestedReviewersRequestPath(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/repos/owner/repo/pulls/42/requested_reviewers", r.URL.Path)
		assert.Equal(t, "Bearer test-token", r.Header.Get("Authorization"))
		w.Header().Set("Content-Type", "application/json")
		require.NoError(t, json.NewEncoder(w).Encode(map[string]any{"users": []any{}, "teams": []any{}}))
	}))
	defer server.Close()

	fetchGitHubRequestedReviewers(context.Background(), server.URL, "owner", "repo", "42", "test-token")
}

func TestExtractGitHubPRMetadataWithAPIReviews(t *testing.T) {
	// Mock server handles both /requested_reviewers and /reviews endpoints.
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/repos/owner/repo/pulls/123/requested_reviewers":
			assert.Equal(t, "Bearer test-token", r.Header.Get("Authorization"))
			// reviewer1 and coderabbitai are already in the event file; newreviewer is only in the API.
			// The event file entry for reviewer1/coderabbitai wins on dedup (inserted first).
			fmt.Fprint(w, `{"users":[{"login":"reviewer1","type":"User"},{"login":"coderabbitai","type":"Bot"},{"login":"newreviewer","type":"User"}],"teams":[]}`)
		case "/repos/owner/repo/pulls/123/reviews":
			assert.Equal(t, "Bearer test-token", r.Header.Get("Authorization"))
			// reviewer2 has reviewed (not in requested); reviewer1 has also reviewed.
			fmt.Fprint(w, `[{"user":{"login":"reviewer2","type":"User"},"state":"APPROVED"},{"user":{"login":"reviewer1","type":"User"},"state":"COMMENTED"}]`)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	t.Setenv("GITHUB_TOKEN", "test-token")
	t.Setenv("GITHUB_API_BASE_URL", server.URL)

	envVars := map[string]string{
		"GITHUB_EVENT_NAME": "pull_request",
		// github_pr_event.json has requested_reviewers: reviewer1 (User), coderabbitai (Bot)
		"GITHUB_EVENT_PATH":       filepath.Join("testdata", "github_pr_event.json"),
		"GITHUB_HEAD_REF":         "feature-branch",
		"GITHUB_BASE_REF":         "main",
		"GITHUB_REPOSITORY":       "owner/repo",
		"GITHUB_REPOSITORY_OWNER": "owner",
	}

	isPR, metadata, err := extractGitHubPRMetadata(context.Background(), envVars)
	require.NoError(t, err)
	require.True(t, isPR)

	// reviewer1:   event file + API requested + has reviewed → Requested: true, ReviewStatus: COMMENTED
	// coderabbitai: event file + API requested, no review    → Requested: true, ReviewStatus: ""
	// newreviewer: API requested only, no review             → Requested: true, ReviewStatus: ""
	// reviewer2:   not requested, has reviewed               → Requested: false, ReviewStatus: APPROVED
	require.Len(t, metadata.Reviewers, 4)

	assert.Equal(t, "reviewer1", metadata.Reviewers[0].Login)
	assert.Equal(t, "User", metadata.Reviewers[0].Type)
	assert.True(t, metadata.Reviewers[0].Requested)
	assert.Equal(t, "COMMENTED", metadata.Reviewers[0].ReviewStatus)

	assert.Equal(t, "coderabbitai", metadata.Reviewers[1].Login)
	assert.Equal(t, "Bot", metadata.Reviewers[1].Type)
	assert.True(t, metadata.Reviewers[1].Requested)
	assert.Equal(t, "", metadata.Reviewers[1].ReviewStatus)

	assert.Equal(t, "newreviewer", metadata.Reviewers[2].Login)
	assert.Equal(t, "User", metadata.Reviewers[2].Type)
	assert.True(t, metadata.Reviewers[2].Requested)
	assert.Equal(t, "", metadata.Reviewers[2].ReviewStatus)

	assert.Equal(t, "reviewer2", metadata.Reviewers[3].Login)
	assert.Equal(t, "User", metadata.Reviewers[3].Type)
	assert.False(t, metadata.Reviewers[3].Requested)
	assert.Equal(t, "APPROVED", metadata.Reviewers[3].ReviewStatus)
}
