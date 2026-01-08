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

package runners

import (
	"os"

	schemaapi "github.com/chainloop-dev/chainloop/app/controlplane/api/workflowcontract/v1"
)

type DaggerPipeline struct{}

func NewDaggerPipeline() *DaggerPipeline {
	return &DaggerPipeline{}
}

func (r *DaggerPipeline) ID() schemaapi.CraftingSchema_Runner_RunnerType {
	return schemaapi.CraftingSchema_Runner_DAGGER_PIPELINE
}

func (r *DaggerPipeline) CheckEnv() bool {
	for _, envVarName := range []string{"CHAINLOOP_DAGGER_CLIENT"} {
		if os.Getenv(envVarName) == "" {
			return false
		}
	}

	return true
}

func (r *DaggerPipeline) ListEnvVars() []*EnvVarDefinition {
	return []*EnvVarDefinition{
		// Version of the Chainloop Client
		{"CHAINLOOP_DAGGER_CLIENT", false},
		// Github Actions PR-specific variables (optional - only present in PR contexts)
		{"GITHUB_EVENT_NAME", true},
		{"GITHUB_HEAD_REF", true},
		{"GITHUB_BASE_REF", true},
		{"GITHUB_EVENT_PATH", true},
		// Gitlab CI MR-specific variables (optional - only present in MR contexts)
		{"CI_PIPELINE_SOURCE", true},
		{"CI_MERGE_REQUEST_IID", true},
		{"CI_MERGE_REQUEST_TITLE", true},
		{"CI_MERGE_REQUEST_DESCRIPTION", true},
		{"CI_MERGE_REQUEST_SOURCE_BRANCH_NAME", true},
		{"CI_MERGE_REQUEST_TARGET_BRANCH_NAME", true},
		{"CI_MERGE_REQUEST_PROJECT_URL", true},
		{"GITLAB_USER_LOGIN", true},
	}
}

// TODO: figure out an URL and or more useful information
func (r *DaggerPipeline) RunURI() string {
	return ""
}

func (r *DaggerPipeline) ResolveEnvVars() (map[string]string, []*error) {
	return resolveEnvVars(r.ListEnvVars())
}

func (r *DaggerPipeline) WorkflowFilePath() string {
	return ""
}

func (r *DaggerPipeline) IsAuthenticated() bool {
	return false
}

func (r *DaggerPipeline) Environment() RunnerEnvironment {
	return Unknown
}
