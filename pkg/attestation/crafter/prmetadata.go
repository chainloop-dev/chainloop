//
// Copyright 2025 The Chainloop Authors.
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
	"encoding/json"
	"fmt"
	"os"

	schemaapi "github.com/chainloop-dev/chainloop/app/controlplane/api/workflowcontract/v1"
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
}

// DetectPRContext checks if we're in a PR/MR context and extracts metadata
func DetectPRContext(runner SupportedRunner) (bool, *PRMetadata, error) {
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
		return extractGitHubPRMetadata(envVars)
	case schemaapi.CraftingSchema_Runner_GITLAB_PIPELINE:
		return extractGitLabMRMetadata(envVars)
	case schemaapi.CraftingSchema_Runner_DAGGER_PIPELINE:
		// When running in Dagger, check for parent CI context passed through as env vars
		// Try Github first
		if envVars["GITHUB_EVENT_NAME"] != "" {
			return extractGitHubPRMetadata(envVars)
		}
		// Then try Gitlab
		if envVars["CI_PIPELINE_SOURCE"] != "" {
			return extractGitLabMRMetadata(envVars)
		}
		return false, nil, nil
	default:
		return false, nil, nil
	}
}

// extractGitHubPRMetadata reads GITHUB_EVENT_PATH JSON and extracts PR metadata
func extractGitHubPRMetadata(envVars map[string]string) (bool, *PRMetadata, error) {
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
		} `json:"pull_request"`
	}

	if err := json.Unmarshal(data, &event); err != nil {
		return false, nil, fmt.Errorf("failed to parse event JSON: %w", err)
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
	}

	return true, metadata, nil
}

// extractGitLabMRMetadata extracts from GitLab environment variables
func extractGitLabMRMetadata(envVars map[string]string) (bool, *PRMetadata, error) {
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
	}

	return true, metadata, nil
}
