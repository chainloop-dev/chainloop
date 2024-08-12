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

	schemaapi "github.com/chainloop-dev/chainloop/app/controlplane/api/workflowcontract/v1"
)

type GitHubAction struct{}

func NewGithubAction() *GitHubAction {
	return &GitHubAction{}
}

func (r *GitHubAction) ID() schemaapi.CraftingSchema_Runner_RunnerType {
	return schemaapi.CraftingSchema_Runner_GITHUB_ACTION
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

func (r *GitHubAction) ListEnvVars() []*EnvVarDefinition {
	return []*EnvVarDefinition{
		{"GITHUB_ACTOR", false},
		{"GITHUB_REF", false},
		{"GITHUB_REPOSITORY", false},
		{"GITHUB_REPOSITORY_OWNER", false},
		{"GITHUB_RUN_ID", false},
		{"GITHUB_SHA", false},
		{"RUNNER_NAME", false},
		{"RUNNER_OS", false},
	}
}

func (r *GitHubAction) RunURI() (url string) {
	repo, runID := os.Getenv("GITHUB_REPOSITORY"), os.Getenv("GITHUB_RUN_ID")
	if repo != "" && runID != "" {
		url = fmt.Sprintf("https://github.com/%s/actions/runs/%s", repo, runID)
	}

	return url
}

func (r *GitHubAction) ResolveEnvVars() (map[string]string, []*error) {
	return resolveEnvVars(r.ListEnvVars())
}
