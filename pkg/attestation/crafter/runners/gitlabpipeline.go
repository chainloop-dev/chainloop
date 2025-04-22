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
	"context"
	"os"

	schemaapi "github.com/chainloop-dev/chainloop/app/controlplane/api/workflowcontract/v1"
)

type GitlabPipeline struct{}

func NewGitlabPipeline() *GitlabPipeline {
	return &GitlabPipeline{}
}

func (r *GitlabPipeline) ID() schemaapi.CraftingSchema_Runner_RunnerType {
	return schemaapi.CraftingSchema_Runner_GITLAB_PIPELINE
}

// Figure out if we are in a Github Action job or not
func (r *GitlabPipeline) CheckEnv() bool {
	for _, varName := range []string{"GITLAB_CI", "CI_JOB_URL"} {
		if os.Getenv(varName) == "" {
			return false
		}
	}

	return true
}

func (r *GitlabPipeline) ListEnvVars() []*EnvVarDefinition {
	return []*EnvVarDefinition{
		{"GITLAB_USER_EMAIL", false},
		{"GITLAB_USER_LOGIN", false},
		{"CI_PROJECT_URL", false},
		{"CI_COMMIT_SHA", false},
		{"CI_JOB_URL", false},
		{"CI_PIPELINE_URL", false},
		{"CI_RUNNER_VERSION", false},
		{"CI_RUNNER_DESCRIPTION", false},
		{"CI_COMMIT_REF_NAME", false},
	}
}

func (r *GitlabPipeline) RunURI() (url string) {
	return os.Getenv("CI_JOB_URL")
}

func (r *GitlabPipeline) ResolveEnvVars() (map[string]string, []*error) {
	return resolveEnvVars(r.ListEnvVars())
}

func (r *GitlabPipeline) WorkflowFilePath(_ context.Context) string {
	return ""
}

func (r *GitlabPipeline) IsHosted(_ context.Context) bool {
	return true
}

func (r *GitlabPipeline) IsAuthenticated(_ context.Context) bool {
	return false
}

func (r *GitlabPipeline) RunnerEnvironment(_ context.Context) string {
	return ""
}
