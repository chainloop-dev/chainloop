//
// Copyright 2025-2026 The Chainloop Authors.
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
	"net/url"
	"os"
	"strings"
	"time"

	schemaapi "github.com/chainloop-dev/chainloop/app/controlplane/api/workflowcontract/v1"
	"github.com/chainloop-dev/chainloop/internal/prinfo"
)

// PRMetadata holds extracted PR/MR information
type PRMetadata struct {
	Platform     string // "github" or "gitlab"
	Type         string // "pull_request" or "merge_request"
	Number       string
	Title        string
	Description  string
	SourceBranch string
	TargetBranch string
	URL          string
	Author       string
	Reviewers    []prinfo.Reviewer
}

// DetectPRContext checks if we're in a PR/MR context and extracts metadata
func DetectPRContext(ctx context.Context, runner SupportedRunner) (bool, *PRMetadata, error) {
	if runner == nil {
		return false, nil, fmt.Errorf("runner is nil")
	}

	envVars, errs := runner.ResolveEnvVars()
	if len(errs) > 0 {
		var combinedErrs string
		for _, err := range errs {
			combinedErrs += (*err).Error() + "\n"
		}
		return false, nil, fmt.Errorf("failed to resolve env vars: %s", combinedErrs)
	}

	switch runner.ID() {
	case schemaapi.CraftingSchema_Runner_GITHUB_ACTION:
		return extractGitHubPRMetadata(ctx, envVars)
	case schemaapi.CraftingSchema_Runner_GITLAB_PIPELINE:
		return extractGitLabMRMetadata(ctx, envVars)
	case schemaapi.CraftingSchema_Runner_DAGGER_PIPELINE:
		// When running in Dagger, check for parent CI context passed through as env vars
		// Try Github first
		if envVars["GITHUB_EVENT_NAME"] != "" {
			return extractGitHubPRMetadata(ctx, envVars)
		}
		// Then try Gitlab
		if envVars["CI_PIPELINE_SOURCE"] != "" {
			return extractGitLabMRMetadata(ctx, envVars)
		}
		return false, nil, nil
	default:
		return false, nil, nil
	}
}

// extractGitHubPRMetadata reads GITHUB_EVENT_PATH JSON and extracts PR metadata
func extractGitHubPRMetadata(ctx context.Context, envVars map[string]string) (bool, *PRMetadata, error) {
	eventName := envVars["GITHUB_EVENT_NAME"]
	// Check if this is a pull request event
	if eventName != "pull_request" && eventName != "pull_request_target" {
		return false, nil, nil
	}

	eventPath := envVars["GITHUB_EVENT_PATH"]
	if eventPath == "" {
		return false, nil, fmt.Errorf("GITHUB_EVENT_PATH not set")
	}

	// Read the event payload file
	data, err := os.ReadFile(eventPath)
	if err != nil {
		return false, nil, fmt.Errorf("failed to read event file: %w", err)
	}

	// Parse the event JSON
	var event struct {
		PullRequest struct {
			Number  int    `json:"number"`
			Title   string `json:"title"`
			Body    string `json:"body"`
			HTMLURL string `json:"html_url"`
			User    struct {
				Login string `json:"login"`
			} `json:"user"`
			RequestedReviewers []struct {
				Login string `json:"login"`
				Type  string `json:"type"`
			} `json:"requested_reviewers"`
		} `json:"pull_request"`
	}

	if err := json.Unmarshal(data, &event); err != nil {
		return false, nil, fmt.Errorf("failed to parse event JSON: %w", err)
	}

	// GITHUB_TOKEN is read via os.Getenv to avoid persisting it in the attestation envVars map.
	// GITHUB_API_BASE_URL can be overridden (e.g. in tests); defaults to api.github.com.
	owner := envVars["GITHUB_REPOSITORY_OWNER"]
	_, repo, _ := strings.Cut(envVars["GITHUB_REPOSITORY"], "/")
	prNumber := fmt.Sprintf("%d", event.PullRequest.Number)
	githubAPIBase := os.Getenv("GITHUB_API_BASE_URL")
	if githubAPIBase == "" {
		githubAPIBase = "https://api.github.com"
	}
	token := os.Getenv("GITHUB_TOKEN")

	// Seed the reviewer map from both the event payload and the API requested_reviewers endpoint.
	// Both sources mark Requested: true. The event file is always available (no token needed),
	// while the API may surface reviewers not yet reflected in the event (e.g. added after dispatch).
	reviewerMap := make(map[string]int) // login → index in reviewers slice
	var reviewers []prinfo.Reviewer

	for _, r := range event.PullRequest.RequestedReviewers {
		if _, exists := reviewerMap[r.Login]; exists {
			continue
		}
		reviewerType := r.Type
		if reviewerType == "" {
			reviewerType = "unknown"
		}
		reviewerMap[r.Login] = len(reviewers)
		reviewers = append(reviewers, prinfo.Reviewer{
			Login:     r.Login,
			Type:      reviewerType,
			Requested: true,
		})
	}

	// Also fetch from the API: the event payload is a snapshot at dispatch time, so reviewers
	// added after the event fires won't appear in event.PullRequest.RequestedReviewers.
	// The API reflects current state and may include those late additions.
	for _, r := range fetchGitHubRequestedReviewers(ctx, githubAPIBase, owner, repo, prNumber, token) {
		if _, exists := reviewerMap[r.Login]; !exists {
			reviewerMap[r.Login] = len(reviewers)
			reviewers = append(reviewers, r)
		}
	}

	// Merge review activity: update status for existing entries, add new ones with Requested: false.
	for _, r := range fetchGitHubReviews(ctx, githubAPIBase, owner, repo, prNumber, token) {
		if idx, exists := reviewerMap[r.Login]; exists {
			reviewers[idx].ReviewStatus = r.ReviewStatus
		} else {
			reviewerMap[r.Login] = len(reviewers)
			reviewers = append(reviewers, r)
		}
	}

	metadata := &PRMetadata{
		Platform:     "github",
		Type:         "pull_request",
		Number:       fmt.Sprintf("%d", event.PullRequest.Number),
		Title:        event.PullRequest.Title,
		Description:  event.PullRequest.Body,
		SourceBranch: envVars["GITHUB_HEAD_REF"],
		TargetBranch: envVars["GITHUB_BASE_REF"],
		URL:          event.PullRequest.HTMLURL,
		Author:       event.PullRequest.User.Login,
		Reviewers:    reviewers,
	}

	return true, metadata, nil
}

// fetchGitHubRequestedReviewers fetches the list of users explicitly requested to review a PR.
// Returns nil on any failure (best-effort).
// baseURL is the GitHub API base (e.g. "https://api.github.com"); can be overridden in tests.
func fetchGitHubRequestedReviewers(ctx context.Context, baseURL, owner, repo, prNumber, token string) []prinfo.Reviewer {
	if baseURL == "" || owner == "" || repo == "" || prNumber == "" || token == "" {
		return nil
	}

	apiURL := fmt.Sprintf("%s/repos/%s/%s/pulls/%s/requested_reviewers", baseURL, owner, repo, prNumber)

	client := &http.Client{Timeout: 10 * time.Second}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiURL, nil)
	if err != nil {
		return nil
	}
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := client.Do(req)
	if err != nil {
		return nil
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil
	}

	var result struct {
		Users []prinfo.Reviewer `json:"users"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil
	}

	if len(result.Users) == 0 {
		return nil
	}

	for i := range result.Users {
		result.Users[i].Requested = true
		if result.Users[i].Type == "" {
			result.Users[i].Type = "unknown"
		}
	}
	return result.Users
}

// fetchGitHubReviews fetches all PR reviews from the GitHub API, following pagination.
// Returns nil on any failure (best-effort).
// Reviews are deduplicated by login, keeping the most recent state (last entry wins).
// Returned reviewers have Requested: false; callers should set Requested: true for those
// already present in the requested_reviewers list.
// baseURL is the GitHub API base (e.g. "https://api.github.com"); can be overridden in tests.
func fetchGitHubReviews(ctx context.Context, baseURL, owner, repo, prNumber, token string) []prinfo.Reviewer {
	if baseURL == "" || owner == "" || repo == "" || prNumber == "" || token == "" {
		return nil
	}

	client := &http.Client{Timeout: 10 * time.Second}

	// Deduplicate by login, keeping insertion order but updating to the most recent state.
	seen := make(map[string]int) // login → index
	var reviewers []prinfo.Reviewer

	nextURL := fmt.Sprintf("%s/repos/%s/%s/pulls/%s/reviews?per_page=100", baseURL, owner, repo, prNumber)
	for nextURL != "" {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, nextURL, nil)
		if err != nil {
			return nil
		}
		req.Header.Set("Authorization", "Bearer "+token)

		resp, err := client.Do(req)
		if err != nil {
			return nil
		}

		if resp.StatusCode != http.StatusOK {
			resp.Body.Close()
			return nil
		}

		var page []struct {
			User struct {
				Login string `json:"login"`
				Type  string `json:"type"`
			} `json:"user"`
			State string `json:"state"`
		}
		err = json.NewDecoder(resp.Body).Decode(&page)
		nextURL = nextPageURL(resp.Header.Get("Link"))
		resp.Body.Close()
		if err != nil {
			return nil
		}

		for _, r := range page {
			reviewerType := r.User.Type
			if reviewerType == "" {
				reviewerType = "unknown"
			}
			if idx, exists := seen[r.User.Login]; exists {
				reviewers[idx].ReviewStatus = r.State
				continue
			}
			seen[r.User.Login] = len(reviewers)
			reviewers = append(reviewers, prinfo.Reviewer{
				Login:        r.User.Login,
				Type:         reviewerType,
				ReviewStatus: r.State,
			})
		}
	}

	if len(reviewers) == 0 {
		return nil
	}
	return reviewers
}

// nextPageURL parses the GitHub Link header and returns the URL for the next page,
// or an empty string if there is no next page.
// Link header format: <url>; rel="next", <url>; rel="last"
func nextPageURL(linkHeader string) string {
	for _, part := range strings.Split(linkHeader, ",") {
		part = strings.TrimSpace(part)
		sections := strings.SplitN(part, ";", 2)
		if len(sections) < 2 {
			continue
		}
		urlPart := strings.TrimSpace(sections[0])
		relPart := strings.TrimSpace(sections[1])
		if relPart == `rel="next"` && len(urlPart) >= 2 {
			return urlPart[1 : len(urlPart)-1] // strip angle brackets
		}
	}
	return ""
}

// extractGitLabMRMetadata extracts from GitLab environment variables
func extractGitLabMRMetadata(ctx context.Context, envVars map[string]string) (bool, *PRMetadata, error) {
	pipelineSource := envVars["CI_PIPELINE_SOURCE"]
	// Check if this is a merge request event
	if pipelineSource != "merge_request_event" {
		return false, nil, nil
	}

	mrIID := envVars["CI_MERGE_REQUEST_IID"]
	if mrIID == "" {
		return false, nil, fmt.Errorf("CI_MERGE_REQUEST_IID not set")
	}

	// Construct MR URL
	projectURL := envVars["CI_MERGE_REQUEST_PROJECT_URL"]
	mrURL := fmt.Sprintf("%s/-/merge_requests/%s", projectURL, mrIID)

	// Fetch reviewers from GitLab API (best-effort).
	// Prefer CI_MERGE_REQUEST_PROJECT_PATH for fork-based MRs where CI_PROJECT_PATH points to the fork.
	projectPath := envVars["CI_MERGE_REQUEST_PROJECT_PATH"]
	if projectPath == "" {
		projectPath = envVars["CI_PROJECT_PATH"]
	}
	// CI_JOB_TOKEN is read via os.Getenv to avoid persisting it in the attestation envVars map.
	reviewers := fetchGitLabReviewers(ctx, envVars["CI_SERVER_URL"], projectPath, mrIID, os.Getenv("CI_JOB_TOKEN"))

	metadata := &PRMetadata{
		Platform:     "gitlab",
		Type:         "merge_request",
		Number:       mrIID,
		Title:        envVars["CI_MERGE_REQUEST_TITLE"],
		Description:  envVars["CI_MERGE_REQUEST_DESCRIPTION"],
		SourceBranch: envVars["CI_MERGE_REQUEST_SOURCE_BRANCH_NAME"],
		TargetBranch: envVars["CI_MERGE_REQUEST_TARGET_BRANCH_NAME"],
		URL:          mrURL,
		Author:       envVars["GITLAB_USER_LOGIN"],
		Reviewers:    reviewers,
	}

	return true, metadata, nil
}

// fetchGitLabReviewers fetches MR reviewers from the GitLab API.
// Returns nil on any failure (best-effort).
func fetchGitLabReviewers(ctx context.Context, baseURL, projectPath, mrIID, token string) []prinfo.Reviewer {
	if baseURL == "" || projectPath == "" || token == "" {
		return nil
	}

	encodedProject := url.PathEscape(projectPath)
	apiURL := fmt.Sprintf("%s/api/v4/projects/%s/merge_requests/%s", baseURL, encodedProject, mrIID)

	client := &http.Client{Timeout: 10 * time.Second}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiURL, nil)
	if err != nil {
		return nil
	}
	req.Header.Set("JOB-TOKEN", token)

	resp, err := client.Do(req)
	if err != nil {
		return nil
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil
	}

	var mrResponse struct {
		Reviewers []struct {
			Username string `json:"username"`
		} `json:"reviewers"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&mrResponse); err != nil {
		return nil
	}

	var reviewers []prinfo.Reviewer
	for _, r := range mrResponse.Reviewers {
		reviewers = append(reviewers, prinfo.Reviewer{
			Login:     r.Username,
			Type:      "unknown",
			Requested: true,
		})
	}

	return reviewers
}
