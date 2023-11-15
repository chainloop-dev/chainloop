//
// Copyright 2023 The Chainloop Authors.
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

package runners

import (
	"fmt"
	"os"
)

const GitHubActionID = "github-action"

type GitHubAction struct{}

func NewGithubAction() *GitHubAction {
	return &GitHubAction{}
}

// Figure out if we are in a Github Action job or not
func (r *GitHubAction) CheckEnv() bool {
	for _, varName := range []string{"CI", "GITHUB_REPOSITORY", "GITHUB_RUN_ID"} {
		if os.Getenv(varName) == "" {
			return false
		}
	}

	return true
}

func (r *GitHubAction) ListEnvVars() []string {
	return []string{
		"GITHUB_ACTOR",
		"GITHUB_REF",
		"GITHUB_REPOSITORY",
		"GITHUB_REPOSITORY_OWNER",
		"GITHUB_RUN_ID",
		"GITHUB_SHA",
		"RUNNER_NAME",
		"RUNNER_OS",
	}
}

func (r *GitHubAction) ListOptionalEnvVars() []string {
	return []string{}
}

func (r *GitHubAction) ResolveEnvVars() map[string]string {
	return resolveEnvVars(r.ListEnvVars(), r.ListOptionalEnvVars())
}

func (r *GitHubAction) String() string {
	return GitHubActionID
}

func (r *GitHubAction) RunURI() (url string) {
	repo, runID := os.Getenv("GITHUB_REPOSITORY"), os.Getenv("GITHUB_RUN_ID")
	if repo != "" && runID != "" {
		url = fmt.Sprintf("https://github.com/%s/actions/runs/%s", repo, runID)
	}

	return url
}
